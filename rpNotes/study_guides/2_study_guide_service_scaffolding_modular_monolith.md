# Study Guide: Service Scaffolding — Modular Monolith, Module Interface & Contracts

> **Page:** "Service Scaffolding" (modular monolith rationale, the `Module` interface, module contracts, Echo/common building blocks)
> **Track:** backend-masterclass-beta, module `02-project-setup`, exercise `02-service-scaffolding`
> **Previous guide:** `6_study_guide_project_setup_taskfile_docker.md` — that page got you a running (empty) backend + Postgres via `task up` with live reload. This page fills in what that "Hello, World!" `main.go` scaffold has become: a real, if empty, HTTP service made of modules.
> **What this likely unlocks next:** implementing the `orders` module's first real behavior (its `Init`, HTTP handler, and eventually a non-placeholder contract), now that the wiring to plug it in already exists.

---

## 1. Snapshot

- **Topic:** Why the project is a modular monolith rather than microservices, how the `Module` interface and its 4-step init sequence work, how modules call each other through typed "module contracts," and why Echo/`pgxpool`/the `common` package were chosen.
- **Objective:** After this page, I can explain the exact order `Init → RegisterContracts → Verify → RegisterHttp` runs in and why that order is load-bearing, describe what a module contract is and why it exists instead of a raw function call, and justify Echo's error-returning handler signature.

## 2. Concepts

### Modular Monolith (monolith-first)

- **Problem:** [From page] Early in a project, domain boundaries are still being learned. Splitting into microservices before boundaries stabilize locks in wrong boundaries, and moving code between *services* (network-separated) is far more expensive to fix later than moving code between *modules* (same process).
- **Mechanism:** [From page] Architecturally, a well-structured modular monolith and microservices look the same — isolated modules with clear boundaries. The stated difference is just the network boundary and independent deployability; modules communicate in-process instead of over the network.
- **Design Decision:** [From page] The alternative is microservices from day one. The tradeoff: microservices force you to commit to boundaries (and pay operational cost — deployment, network calls, service discovery) before you understand the domain; a monolith lets you keep changing module boundaries cheaply while you're still learning the domain (restaurant onboarding, orders, courier delivery, per the page). [AI explanation] This would be the wrong choice once a module's scaling, deployment cadence, or team ownership genuinely diverges from the rest of the system — at that point the in-process boundary becomes the thing holding you back, and that's when splitting into a real service pays off.
- **In Production:** [AI explanation] A modular monolith fails differently than microservices: a bug or panic in one module can take down the whole process (no network isolation), and a resource-hungry module competes for the same CPU/memory as every other module, since there's one deployable unit. You'd observe this as one module's load spiking overall service latency/error rate, not just that module's own metrics.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Treating "modular monolith" as permission to skip module boundaries | You end up with a regular (non-modular) monolith — tightly coupled, hard to split later | Boundaries take discipline to maintain; nothing stops direct calls except code review | Enforce boundaries in code (module contracts, no cross-module internal imports), not just in intent |
  | Splitting into microservices the moment scaling is discussed | Premature operational complexity before boundaries are proven stable | Microservices are perceived as the "more serious" or "more scalable" default | Split only when a module's boundary has stayed stable and a concrete deployment/scaling need exists |

### The `Module` Interface & Initialization Sequence

- **Problem:** [From page] Without a shared contract, each module would wire itself up ad hoc — inconsistent init order, no guarantee that a module's dependencies (like another module's contract) are ready before it's used, and no single place that fails fast if wiring is incomplete.
- **Mechanism:** [From code] `backend/common/module/module.go`:

  ```go
  type Module interface {
      Name() Name
      Init(ctx context.Context) error
      RegisterHttp(ctx context.Context, e common.EchoRouter) error
      RegisterContracts(ctx context.Context, contracts *contracts.Contracts) error
  }
  ```
  `backend/svc.go`'s `New` runs these across all modules in the exact order [From page confirmed by code]:
  1. `Init` for every module (builds handlers/services/repositories) — e.g. `orders.Module.Init` constructs `http2.NewHandler()`.
  2. `RegisterContracts` for every module — e.g. `orders.Module.RegisterContracts` sets `contracts.Orders = ordersModule.Orders{}`.
  3. `moduleContracts.Verify()` — a single call that fails the whole startup if any expected contract is still `nil`.
  4. `RegisterHttp` for every module — e.g. `orders.Module.RegisterHttp` calls `http2.Register(ctx, e, m.httpHandler)`.
- **Design Decision:** [From page] The alternative is registering HTTP routes and contracts in one interleaved pass per module. The page's stated reason for the strict ordering: "HTTP handlers may call other modules via module contracts" — so every contract must exist *before* any route is registered, otherwise a handler could reference a contract that isn't wired yet. [AI explanation] This would be the wrong ordering to relax if you ever wanted a module's `RegisterHttp` to be able to reference another module's not-yet-initialized state — the whole point of the current order is that it's impossible for that race to exist.
- **In Production:** [From page] The `Verify()` call is described as "a fail-fast safety net" — if a module forgets to register its contract, the service refuses to start at all, rather than starting successfully and failing later at the first cross-module call. [AI explanation] You'd see this as a deploy that never comes up (crash-on-boot with a clear "contract is empty" error), which is a much easier incident to diagnose than a runtime nil-pointer panic on the first request that happens to hit that code path.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Calling `RegisterHttp` before all modules finish `RegisterContracts` | A handler could call a contract that's still `nil`/unregistered — panic on first request | Assuming each module can be fully wired up in isolation, one at a time | Always run each phase (`Init`, then `RegisterContracts`, then `Verify`, then `RegisterHttp`) across *all* modules before moving to the next phase |
  | Skipping or ignoring `Verify()`'s error | Missing contract only surfaces as a runtime nil dereference deep in a request path | `Verify()` looks like boilerplate/optional ceremony | Always propagate `Verify()`'s error and fail startup on it, exactly as `svc.go` does (`return Svc{}, fmt.Errorf(...)`) |

### Module Contracts (typed cross-module calls)

- **Problem:** [From page] Without an explicit boundary type, one module could reach in and call any function or type from another module directly, silently coupling internals that were meant to be independent — the in-process equivalent of two microservices sharing a database table directly instead of going through an API.
- **Mechanism:** [From code] Each module exposes a `client` package with an interface (`backend/orders/api/module/client/client.go`):

  ```go
  type Orders interface {
      PingOrders(PingOrdersRequest) error
  }
  ```
  and a concrete implementation elsewhere (`backend/orders/api/module/module.go`):

  ```go
  type Orders struct{}
  func (o Orders) PingOrders(request client.PingOrdersRequest) error { return nil }
  ```
  These get collected into one registry struct via embedding (`backend/common/module/contracts/contracts.go`):

  ```go
  type Contracts struct {
      ordersModule.Orders
  }
  ```
  Any module holding a `*contracts.Contracts` pointer can call `contracts.PingOrders(...)` (through the embedded interface) instead of importing the `orders` package's internals directly. [From page] `PingOrders` itself is explicitly called out as a placeholder — it exists purely so the contracts wiring (`Verify`) has something concrete to check at startup, not as real functionality yet.
- **Design Decision:** [From page] The alternative (rejected) is letting modules call each other's internal functions/types directly. The tradeoff of the contracts approach: it's an extra layer of interfaces and a `client` sub-package per module, in exchange for the same boundary enforcement microservices get from the network — "only able to call the service's public API" — but without paying network latency/serialization cost. [AI explanation] It would be the wrong tool if a "module" is actually just an implementation detail with no independent identity (e.g., a small internal helper package) — not every internal package needs a contracts-style interface boundary, only ones representing genuine domain modules.
- **In Production:** [AI explanation] Because contracts are plain Go interfaces called in-process, a slow or panicking call in `Orders.PingOrders` runs on the *caller's* goroutine with no network timeout to bound it — unlike a microservice call, there's no built-in circuit breaker or timeout at this layer by default. That's a real tradeoff worth watching once contracts carry real logic instead of `return nil`.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Importing another module's internal package directly instead of its `client` interface | Silently defeats the entire module-boundary design — compiles fine, but couples internals | Go doesn't enforce this at the compiler level; only structure/convention does | Only ever depend on another module's `api/module/client` package, never its internals |
  | Putting business logic inside the `contracts` package itself | Blurs the boundary the package exists to enforce; `contracts` was meant to just aggregate interfaces | It's a convenient central place that every module already imports | Keep `contracts.Contracts` as pure aggregation/verification (as it is today — just embedded interfaces and a `Verify` check) |

### Building Blocks: Echo, `common`, `pgxpool`

- **Problem:** [From page] Standard-library `http.Handler`/`ServeMux` (and libraries like `chi` built on the same signature) don't return an error from a handler, so every handler ends up repeating its own error-to-HTTP-response translation.
- **Mechanism:** [From code] `backend/common/http/echo.go`'s `NewEcho()` sets a single `e.HTTPErrorHandler = HandleError`, and `HandleError` centralizes the translation:

  ```go
  func HandleError(err error, c echo.Context) {
      httpCode := http.StatusInternalServerError
      msg := any("Internal server error")

      httpErr := &echo.HTTPError{}
      if errors.As(err, &httpErr) {
          httpCode = httpErr.Code
          msg = httpErr.Message
      }
      // ... c.JSON(httpCode, map[string]any{"error": msg})
  }
  ```
  Because Echo handlers have signature `func(echo.Context) error`, any handler can just `return someErr` and this single function decides the HTTP status/body — matching the page's claim that Echo "lets you handle errors in one place... instead of repeating it in every handler." The `EchoRouter` interface in `backend/common/echo.go` further narrows what a module receives (only the HTTP-verb registration methods), so a module can't reach into unrelated `*echo.Echo` capabilities (like global middleware) through what it's handed in `RegisterHttp`.
- **Design Decision:** [From page] The alternative is `chi` or the standard library, both of which use the error-less handler signature. Echo's tradeoff: it's a heavier, more opinionated framework, in exchange for centralized error handling and (implicitly) more built-in conveniences. [AI explanation] It would be the wrong choice if you needed a handler signature libraries in the wider Go ecosystem expect by default (`http.Handler`) for interop — Echo requires either wrapping or living entirely inside its own handler style.
- **Design Decision — `pgxpool.Pool` + `*sql.DB`:** [From page] `pgxpool.Pool` is used because it manages a pool of connections, necessary once the service handles concurrent requests. The page also states the project "keeps a `*sql.DB` for compatibility with libraries that require the standard `database/sql` interface" — **[AI explanation, flagged]: I could not find any `*sql.DB` usage anywhere in the current backend code** (`svc.go` and `main.go` only construct and pass around a `*pgxpool.Pool`). Treat the `*sql.DB` compatibility layer as a stated future/page-only claim until it shows up in code — don't assume it already exists.
- **Design Decision — the `common` package:** [From page] The page pre-empts the "shared `common`/`utils` package is an anti-pattern" objection directly: it says it depends on what you keep there, and in this project `common` holds only infrastructure (HTTP setup, middleware, the `Module`/`contracts` types) — explicitly "No business logic." [AI explanation] That distinction is the actual rule to apply: a `common` package becomes an anti-pattern when domain logic leaks into it (because then unrelated modules become coupled through shared business rules), not merely by existing.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Returning a raw (non-`echo.HTTPError`) error from a handler and expecting a specific status code | Client always gets a generic 500 "Internal server error", even for a validation error | `HandleError`'s `errors.As` branch only special-cases `*echo.HTTPError` | Wrap/construct an `echo.HTTPError` (or a type that satisfies it) when a handler needs a specific status code |
  | Adding domain/business logic to the `common` package because it's convenient and already imported everywhere | Re-creates the exact "shared utils" anti-pattern the page pre-empts, coupling unrelated modules | `common` is already a dependency of every module, so it's the path of least resistance for "just one more helper" | Keep new business logic inside the owning module; `common` stays infrastructure-only |

## 3. Plumbing Dissection

**Wiring map:**

```
cmd/main.go
  ├─ pgxpool.New(ctx, POSTGRES_URL)          → *pgxpool.Pool
  └─ backend.New(ctx, dbPgx)                  (backend/svc.go)
        ├─ commonHTTP.NewEcho()               → *echo.Echo w/ middlewares, /health, HandleError
        ├─ moduleContracts := &contracts.Contracts{}
        ├─ modules := []module.Module{ orders.NewModule(dbPgx, moduleContracts) }
        ├─ for each module: module.Init(ctx)              → orders.Module.Init: builds http2.Handler
        ├─ for each module: module.RegisterContracts(...)  → orders.Module: contracts.Orders = ordersModule.Orders{}
        ├─ moduleContracts.Verify()                        → fails fast if any contract field is nil
        ├─ for each module: module.RegisterHttp(ctx, e)    → orders.Module: http2.Register(ctx, e, handler)
        └─ returns Svc{echoRouter: e, modules, dbPgx}
  └─ svc.Run(ctx, ":8080")
        ├─ goroutine: on ctx.Done() → echoRouter.Shutdown(context.Background())
        └─ echoRouter.Start(":8080")   (blocks; ErrServerClosed on graceful shutdown is not an error)
```

- **Per plumbing piece:**
  - `moduleContracts := &contracts.Contracts{}` (pointer, not value) → without the pointer, each module's `RegisterContracts(ctx, contracts *contracts.Contracts)` call would mutate a copy, and the shared registry every module (and `Verify`) reads from would never actually get updated. The comment in `svc.go` (`// We use a pointer here so modules can register their contracts during Init()...`) states this design choice directly — [From code].
  - `orders.NewModule(dbPgx, moduleContracts)` receiving the *same* `moduleContracts` pointer every other module gets → this is literally what makes cross-module contract calls possible later: every module holds a reference to one shared registry, not its own copy.
  - `err := module.Init(ctx)` returning early on error (`return Svc{}, fmt.Errorf(...)`) → without this, a failed module init would be silently ignored and the service would continue starting in a half-initialized state.
  - `s.echoRouter.Shutdown(context.Background())` inside the goroutine watching `ctx.Done()`, called with a *fresh* background context rather than the (already-cancelled) `ctx` → using the cancelled `ctx` here would make `Shutdown` return immediately without waiting for in-flight requests to drain, defeating the point of a graceful shutdown.
  - `errors.Is(err, http.ErrServerClosed)` check in `Run` → without it, a clean shutdown (which Echo signals by returning `http.ErrServerClosed`) would be reported as a failure, when it's actually the expected/successful shutdown path.

- **Non-obvious lines:**
  - `moduleContracts := &contracts.Contracts{}` — the `&` is easy to skim past, but is the entire reason mutation-through-registration works at all.
  - `defer s.dbPgx.Close()` at the very top of `Run` — deferred at the *start* of the function, so it fires whenever `Run` returns for any reason (including the error path), not just after a clean shutdown.
  - `Contracts` embedding `ordersModule.Orders` (from the `client` package) rather than holding a named field — this is what lets `moduleContracts.PingOrders(...)` be called directly on the registry (embedding promotes the interface's methods), instead of needing `moduleContracts.Orders.PingOrders(...)`.

- **What a senior notices:**
  - The four-phase loop in `svc.go` iterates `modules` four separate times (once per phase) rather than doing everything per-module in one pass — a bit more verbose, but it's the direct enforcement of the ordering guarantee the page insists on; collapsing it into one loop per module would silently reintroduce the exact race this design prevents.
  - `Verify()` currently only checks one field (`Orders`) because there's only one module — this doesn't yet prove the pattern scales cleanly to N modules with N contract fields, and the manual `errors.Join` construction in `Verify()` will get repetitive; worth watching how this evolves as more modules are added.
  - `EchoRouter` (the interface each module's `RegisterHttp` receives) exposes only the HTTP-verb registration methods, not the full `*echo.Echo` — a deliberate narrowing that stops a module from, say, adding global middleware from inside its own `RegisterHttp`. That's good boundary discipline that's easy to miss on a skim.

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a blank scratch Go module, rebuild the core init-sequence plumbing from Section 3 — without looking at the provided code:
- A `Module` interface with `Name`, `Init`, `RegisterContracts`, `RegisterHttp` methods.
- A shared, pointer-based contracts registry with a `Verify() error` method.
- At least two fake modules (`A` and `B`), where `B`'s `RegisterHttp` calls a method on `A`'s contract through the registry (proving contracts must exist before HTTP registration).
- The 4-phase orchestration loop (`Init` all → `RegisterContracts` all → `Verify` → `RegisterHttp` all), returning an error immediately if any phase fails for any module.

**Acceptance criteria:**
1. If module `A` never implements/registers its contract, `Verify()` returns a non-nil error and startup aborts before any `RegisterHttp` runs.
2. Module `B`'s `RegisterHttp` can successfully call module `A`'s contract method — proving contracts really are available by that phase.
3. If you deliberately swap the order to run `RegisterHttp` before `RegisterContracts`, construct a case where module `B` would call a still-unregistered contract from `A` (nil dereference or explicit nil check) — this demonstrates *why* the order matters, not just that it's followed.
4. A single module's `Init` returning an error stops the whole startup sequence (no partial startup), and the error names which module failed.
5. (Failure/shutdown case) Simulate a "server" with a context that gets cancelled mid-run, and confirm your shutdown path is given a fresh, non-cancelled context to do cleanup with — not the already-cancelled one.

<details><summary>Hint</summary>Start with the `Module` interface and two trivial modules that do nothing yet — get the 4-phase loop compiling and running in order before adding any contract logic.</details>
<details><summary>Hint</summary>The contracts registry needs to be a pointer shared by every module from construction time — decide how each module receives it (constructor parameter) before writing `Init`.</details>
<details><summary>Hint</summary>To prove ordering matters (criterion 3), don't just reason about it — actually reorder your loop and watch a nil-contract call panic or fail; then restore the correct order and confirm it stops happening.</details>

**Compare step:** After finishing, diff against `backend/svc.go`, `backend/common/module/module.go`, and `backend/common/module/contracts/contracts.go`, and ask yourself:
1. Did I use a pointer for the shared contracts registry, or did I accidentally give each module its own copy?
2. Did I make `Verify()` a fail-fast, single call after all `RegisterContracts` calls, or did I check contract completeness some other way (e.g., per-module, or not at all)?
3. Does my `Module` interface expose exactly what's needed for each phase, or did I leak unrelated capabilities (like giving `Init` access to the HTTP router too early)?

## 5. Before the Exercise

- **Likely task:** [AI explanation — a guess, not derived from the exercise itself] Given the scaffold already contains a full, working (if empty) `Module`/`contracts`/`svc.go` wiring for the `orders` module, the exercise likely asks you to add a second module (or extend the existing wiring) following the same 4-method `Module` interface pattern, and confirm the service still starts (contracts `Verify()` passes) and responds on `/health`.
- **Approach:**
  1. Re-read `backend/svc.go`'s `New` function and trace the 4-phase loop until you can say out loud why each phase must fully complete before the next one starts.
  2. Look at how `orders.NewModule` is constructed and registered in the `modules` slice — that's the pattern any new module will need to follow.
  3. Check what `contracts.Contracts` currently verifies (just `Orders`), and think about what a second module's contract field and `Verify()` check would look like.
  4. Confirm the backend still starts cleanly (`task up`, watch logs) and `GET /health` still returns 200 before making any change, so you have a known-good baseline.
  5. Keep new module code inside the module's own package, and only add to `common` if it's genuinely infrastructure every module needs — not domain logic.

## 6. Recall Questions

1. Why must every module finish `RegisterContracts` (and pass `Verify()`) *before* any module runs `RegisterHttp`?
   <details><summary>Answer</summary>Because HTTP handlers may call other modules through their module contracts. If `RegisterHttp` ran before all contracts were registered, a handler could be wired up to call a contract that doesn't exist yet, leading to a nil-contract call the first time that route is hit. Running all `RegisterContracts` (plus a fail-fast `Verify()`) before any `RegisterHttp` guarantees every contract a handler might call already exists.</details>

2. What would happen if `moduleContracts` in `svc.go` were a value (`contracts.Contracts{}`) instead of a pointer (`&contracts.Contracts{}`), passed to each module's `RegisterContracts`?
   <details><summary>Answer</summary>Each module would receive/mutate its own copy of the struct instead of a shared one. `RegisterContracts` assignments like `contracts.Orders = ordersModule.Orders{}` would never be visible outside that call, so the shared registry `Verify()` checks (and that other modules would later call through) would remain permanently empty — `Verify()` would always fail, or worse, silently not reflect real registrations if copied around inconsistently.</details>

3. Why does `Verify()` exist as a single explicit call rather than each module just trusting that its dependencies were registered?
   <details><summary>Answer</summary>It's a deliberate fail-fast safety net: if any module forgets to register its contract implementation, the service refuses to start at all (a clear, immediate startup error) instead of starting successfully and only failing later — as a nil-pointer panic or silent no-op — the first time some other module's handler actually calls the missing contract.</details>

4. Why are module contracts (typed interfaces in a `client` sub-package) preferred over one module directly importing and calling another module's internal package?
   <details><summary>Answer</summary>Direct internal imports would let modules reach past their boundary and depend on each other's implementation details, exactly the kind of coupling the modular monolith is trying to avoid. Module contracts enforce, in code structure, the same restriction microservices get from the network: a module can only call what's exposed as its public "API" (the contract interface) — not its internals — while still avoiding network overhead.</details>

5. Why does Echo's handler signature (`func(echo.Context) error`) matter for how errors are handled across the whole service, compared to the standard library's `http.Handler`?
   <details><summary>Answer</summary>Because Echo handlers return an error, a single centralized function (`HandleError`, set as `e.HTTPErrorHandler`) can translate any handler's returned error into the right HTTP status/body in one place. With the standard library's error-less handler signature, each handler would have to call its own response-writing logic for every error case, repeating that translation throughout the codebase instead of centralizing it.</details>

6. In `Svc.Run`, why is the shutdown goroutine given `context.Background()` for `echoRouter.Shutdown(...)` instead of the `ctx` that was just cancelled?
   <details><summary>Answer</summary>`ctx` is the context whose cancellation triggered the shutdown in the first place — it's already done. Passing an already-cancelled context to `Shutdown` would make it return immediately without waiting for in-flight requests to finish, defeating the purpose of a graceful shutdown. `context.Background()` gives `Shutdown` a context that isn't already cancelled, so it can actually wait (up to Echo's own shutdown behavior) for requests to drain.</details>

*(These questions, with answers, have also been appended to `notes/review_queue.md` under today's date.)*

## 7. Cheat Sheet

- `Module` interface: `Name() Name`, `Init(ctx) error`, `RegisterContracts(ctx, *contracts.Contracts) error`, `RegisterHttp(ctx, common.EchoRouter) error`. Init order across *all* modules: `Init` → `RegisterContracts` → `Verify` → `RegisterHttp`. Never interleave phases per-module.
- Contracts registry: one shared `*contracts.Contracts` pointer, passed to every module's constructor; each module sets its own embedded field during `RegisterContracts`; `Verify()` checks all expected fields are non-nil and fails startup if not.
- Module contracts (`client` package interfaces) are the *only* sanctioned way one module calls another — never import another module's internals directly.
- Echo handler signature: `func(echo.Context) error` → set `e.HTTPErrorHandler` once (`HandleError`) instead of writing error-to-response logic in every handler. Use `echo.HTTPError` when a handler needs a specific status code — a plain error always maps to 500.
- `common` package rule: infrastructure only (HTTP setup, middleware, `Module`/`contracts` types) — no business logic. That's what keeps a shared package from becoming the "shared utils" anti-pattern.
- `pgxpool.Pool` for concurrent-safe DB access. Page claims a `*sql.DB` compatibility layer exists too — **not present in code yet**, don't assume it until you see it.
- Graceful shutdown gotcha: give the shutdown call a fresh context, never the one that was just cancelled to trigger the shutdown.
