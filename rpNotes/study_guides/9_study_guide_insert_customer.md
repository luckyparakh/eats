# Study Guide: Insert Customer

> **Page:** "Insert Customer" (wiring the sqlc-generated `InsertCustomer` query into the HTTP handler, constructor-based dependency injection for the database pool, and explicit mapping between the OpenAPI `Address` type and `shared.Address`)
> **Track:** backend-masterclass-beta, module `04-database`, exercise `03-insert-customer` (confirmed via `.tdl-exercise`).
> **Previous guide:** `8_study_guide_generate_sqlc.md` — that page generated `dbmodels.InsertCustomer`/`GetCustomerByUUID` from annotated SQL but left `queries/customers.sql` as a placeholder and the handler disconnected from the database. This page is exactly the deferred thread: use the generated code, wire the DB pool into the handler, and (per guide 8's own deferred note) map `shared.Address` to/from the HTTP layer.
> **What this likely unlocks next:** per the page itself, a repository layer "as the project grows" to separate database logic from the handler — the page explicitly frames inserting directly in the handler as a "for now" simplification.
>
> **Workspace state (checked directly, not from `exercise.md`):** `backend/orders/adapters/db/queries/customers.sql` is no longer a placeholder — it now has `InsertCustomer :exec` and `GetCustomerByUUID :one` (`SELECT *`), and `dbmodels/` has been generated (`customers.sql.go`, `models.go`, `db.go`) confirming the shapes from guide 8: `InsertCustomerParams` and `OrdersCustomer` currently have identical fields. `backend/orders/module.go` already receives `pgxDb *pgxpool.Pool` in `NewModule(...)` and stores it on `Module.pgxDb`, but `Init()` still calls `http2.NewHandler()` with no arguments — the pool is sitting on the module, unused by the handler, which is precisely the gap this exercise's "dependency injection" section targets. `backend/orders/api/http/handler.go`'s `NewHandler()` takes no parameters and `RegisterCustomer` ignores `request.Body` entirely, generating a UUID and returning without touching the database. `openapi.gen.go` confirms two things the page's snippet doesn't call out: `CountryCode` on the generated `Address` struct is a direct type alias (`CountryCode = shared.CountryCode`), so it needs no conversion through the mapping function at all; and `RegisterCustomer`'s body has `Email openapi_types.Email` (not a plain `string`), which `InsertCustomerParams.Email` (a plain `string`) will need an explicit conversion for, something the page's mapping example only shows for `Address`, not `Email`. No "cannot be nil" constructor-panic pattern exists anywhere else in the codebase yet (confirmed by search) — this would be the first instance of it. No address-mapping helper function exists yet either.

---

## 1. Snapshot

- **Topic:** Calling the `sqlc`-generated `InsertCustomer` query directly from the HTTP handler (no repository layer yet), injecting the `*pgxpool.Pool` through the handler's constructor instead of a global or per-call connection, and writing an explicit mapping function between the OpenAPI-generated `Address` and `shared.Address` instead of sharing one struct.
- **Objective:** After this page, I can explain why the project calls sqlc queries directly in the handler for now, why constructor injection (with a nil check) beats a global variable or opening a connection per request, and why `Address` gets an explicit mapping function while `common.UUID` doesn't.
- **Continuity:** Picks up exactly where guide 8 left off — `dbmodels.InsertCustomer`/`GetCustomerByUUID` exist and compile, but nothing calls them yet, and guide 8 explicitly deferred "the actual HTTP-layer mapping" for `shared.Address` to this exercise.

## 2. Concepts

### Inserting Directly in the Handler (No Repository Yet)

- **Problem:** [From page] The handler validates a request and does nothing with it (guide 8's open thread) — it needs to actually persist the customer.
- **Mechanism:** [From page] Build a `dbmodels.Queries` from the pool (`queries := dbmodels.New(db)`), then call the generated method with a params struct: `queries.InsertCustomer(ctx, dbmodels.InsertCustomerParams{...})`. The struct's fields map positionally to the query's `$1, $2, ...` placeholders.
- **Design Decision:** [From page] The page explicitly calls this a "for now" choice: "We'll insert directly in the HTTP handler for now... As the project grows, we'll introduce a repository layer to separate database logic from the handler." For simple applications, direct-in-handler is "good enough."
- **In Production:** [AI explanation] This is a deliberate, named shortcut rather than an oversight — the page doesn't pretend direct-in-handler DB access is a best practice to keep forever, it frames it as a complexity/benefit tradeoff that flips once the handler needs more than one query, cross-table consistency, or reuse from another caller (a background job, a second handler). Introducing a repository abstraction before any of that exists would be the premature-abstraction mirror image of the "avoid DRY" tip the page gives for `Address` below — both are "wait for real pressure before adding the layer."
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Introducing a repository interface/layer before any second handler or query needs it | Extra abstraction with one implementation and one caller — indirection that doesn't pay for itself yet | Repository pattern is a well-known "best practice" that feels safer to add early | Follow the page's own sequencing: direct-in-handler now, repository layer when the project actually grows into needing it |
  | Calling `dbmodels.New(db)` fresh on every request instead of once | Minor but needless allocation per request; easy to forget `db` was supposed to be a long-lived pool reference, not per-call state | Not realizing `Queries` is a thin, stateless-ish wrapper around whatever `DBTX` it's given | Build `queries` once from the injected pool (e.g., in the handler method using `h.db`), not a `New()` call scattered elsewhere unnecessarily |

### Constructor-Based Dependency Injection with a Nil Check

- **Problem:** [From page] The handler needs a database connection to execute queries. A global variable or opening a new connection per handler method are both on the table but worse.
- **Mechanism:** [From page]
  ```go
  func NewHandler(db *pgxpool.Pool) Handler {
      if db == nil {
          panic("db cannot be nil")
      }

      return Handler{
          db: db,
      }
  }
  ```
  `pgxpool` imports from `github.com/jackc/pgx/v5/pgxpool`. Passing `*pgxpool.Pool` as a constructor parameter makes the dependency explicit and visible at the call site (`NewHandler(db)`), and keeps its lifecycle controlled from one place.
- **Design Decision:** [From page] A global variable is rejected mainly because it makes testing harder — constructor injection lets tests swap in a mock/fake database instead of the real connection. The "Past Code Review" exchange (Miłosz asking if the nil check is worth the boilerplate vs. Robert's answer) settles the nil-check question: without it, a nil `db` surfaces as a nil-pointer panic deep in a request handler's stack trace; with it, the app crashes at startup with an explicit "db cannot be nil" message — "one line that saves real debugging time." The page generalizes this to "all constructors." Two tips reinforce the pattern: it's the same constructor-injection idea as *Introducing Clean Architecture* (there wired in `main.go`; here, in `module.go`'s `Init` method), and DI frameworks (Wire, Dig, Fx) are explicitly discouraged in favor of manual wiring — "If your constructor list grows long enough to feel painful, that's not a problem with dependency injection. It's a signal that your dependency tree is too complex."
- **In Production:** [AI explanation] The codebase already has half of this wiring in place but not connected: `module.go`'s `NewModule(pgxDb *pgxpool.Pool, ...)` already receives and stores the pool on `Module.pgxDb`, exactly the "wired in one place" the tip describes — but `Init()` still calls `http2.NewHandler()` with zero arguments, so the pool reaches the module and then goes nowhere. Closing that gap (changing `NewHandler()` to `NewHandler(db)`, and `Init()` to pass `m.pgxDb`) is the concrete, mechanical form this exercise's DI section takes in this repo.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Storing the DB pool in a package-level global variable | Implicit dependency invisible from a function/constructor signature; hard to swap for a test double; multiple tests touching the same global can't safely run in parallel | Feels like less code than threading a parameter through constructors | Inject `*pgxpool.Pool` through `NewHandler`, same as every other constructor in the pattern |
  | Skipping the nil check because "it'll panic anyway when used" | True, but the panic now happens deep inside a request's stack trace, with a generic nil-pointer message, instead of immediately at startup with a clear reason | The check looks like redundant boilerplate at a glance | Keep the explicit check — the value is in *when* and *how clearly* the failure surfaces, not whether it happens at all |
  | Reaching for a DI framework (Wire/Dig/Fx) once a module has a few dependencies | Added build-time codegen step and indirection to solve a problem manual wiring already handles, for a project explicitly trying to keep wiring explicit and debuggable | DI frameworks are common in larger codebases and feel like the "grown-up" solution | Keep wiring manual; treat constructor-list pain as a signal to simplify the dependency graph, not a signal to add a framework |

### Explicit Mapping Between OpenAPI `Address` and `shared.Address`

- **Problem:** [From page] Guide 8 got `common.UUID` flowing through both HTTP and SQL layers with zero conversion — but the OpenAPI-generated `Address` type and `shared.Address` are still separate structs, even though they currently have identical fields.
- **Mechanism:** [From page]
  ```go
  func openapiAddressToSharedAddress(addr Address) (shared.Address, error) {
      return shared.NewAddress(
          addr.Line1,
          addr.Line2,
          addr.PostalCode,
          addr.City,
          addr.CountryCode,
      )
  }
  ```
  Keeping the two types separate means either one can change independently; the cost is this small helper function.
- **Design Decision:** [From page] The tip points to "When to avoid DRY in Go": duplicating structs across layers is often the right call, because "writing the boilerplate takes less time and effort than debugging mapping edge cases." This directly extends guide 8's own framing — UUID is shared because it's a stable, universal value; `Address` stays separate because its HTTP and DB shapes could evolve independently (guide 8's FAQ worked through a concrete autocomplete-field example of exactly this).
- **In Production:** [AI explanation] Two details confirmed directly in `openapi.gen.go` that the page's snippet doesn't flag: `Address.CountryCode`'s type is `CountryCode`, which is itself declared as `type CountryCode = shared.CountryCode` — a true Go type alias, not a new type. So unlike `Line1`/`Line2`/`PostalCode`/`City` (plain `string` fields that get re-validated by passing through `shared.NewAddress`), `addr.CountryCode` requires no conversion at all — it already *is* `shared.CountryCode`, passed straight through. Separately, `RegisterCustomer`'s request body has `Email openapi_types.Email`, a distinct generated type, not a plain `string` — since `dbmodels.InsertCustomerParams.Email` is a plain `string`, wiring the full handler will need its own small explicit conversion (e.g. `string(body.Email)`) alongside the `Address` mapping the page shows, following the same "explicit conversion at the boundary" principle for a different field.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Making `shared.Address` satisfy the OpenAPI-generated `Address` shape directly (e.g. via embedding or matching JSON tags) so one type serves both | HTTP contract changes (new/renamed fields, different validation) and DB/storage concerns become coupled through one struct — exactly what guide 8 warned against | The two structs look identical today, so sharing feels like free DRY | Keep both types, add the explicit mapping function — "duplication" here is paying down future coupling risk, not redundant boilerplate |
  | Assuming every field needs the same kind of conversion as `Line1`/`Line2`/etc. | Writing unnecessary conversion code for `CountryCode` (already a type alias to `shared.CountryCode`) or forgetting the real conversion `Email` needs (`openapi_types.Email` → `string`) because the page's example only covered `Address` | Pattern-matching "mapping function exists, so everything needs converting" instead of checking each field's actual type | Check each field's generated type individually — some (like `CountryCode`) are already aliases needing no work, others (like `Email`) need their own explicit conversion |

## 3. Plumbing Dissection

**Wiring map (confirmed directly in the current workspace):**

```
backend/orders/adapters/db/queries/customers.sql        (CONFIRMED — no longer a placeholder)
  -- name: InsertCustomer :exec
  INSERT INTO orders.customers (customer_uuid, name, email, address, phone_number) values ($1,$2,$3,$4,$5);
  -- name: GetCustomerByUUID :one
  SELECT * FROM orders.customers where customer_uuid = $1;

backend/orders/adapters/db/dbmodels/                     (CONFIRMED — generated from the above)
  ├─ customers.sql.go: InsertCustomer(ctx, InsertCustomerParams) error
  │                     GetCustomerByUUID(ctx, common.UUID) (OrdersCustomer, error)
  ├─ models.go:         OrdersCustomer{CustomerUuid, Name, Email, Address, PhoneNumber}
  └─ db.go:             DBTX interface, New(db DBTX) *Queries, (q *Queries) WithTx(tx pgx.Tx) *Queries

backend/orders/module.go                                 (CONFIRMED)
  NewModule(pgxDb *pgxpool.Pool, modules *contracts.Contracts) *Module   ← pool already received
  Init(ctx): httpHandler := http2.NewHandler()                           ← but NOT passed to the handler yet

backend/orders/api/http/handler.go                        (CONFIRMED, unchanged since guide 8)
  func NewHandler() Handler                                              ← no db parameter yet
  func (h Handler) RegisterCustomer(...): ignores request.Body entirely,
    only does customerUUID := common.NewUUIDv7(); returns it

backend/orders/api/http/openapi.gen.go                     (CONFIRMED)
  type Address struct{ City, Line1, Line2, PostalCode string; CountryCode CountryCode }
  type CountryCode = shared.CountryCode            ← true alias, zero conversion needed
  type CustomerUUID = common.UUID                  ← same pattern already used for UUID
  type RegisterCustomer struct{ Address Address; Email openapi_types.Email; Name, PhoneNumber string }
                                          ↑ NOT a plain string — needs explicit conversion too

(repo-wide search, CONFIRMED)
  "cannot be nil" panic pattern: not found anywhere yet — this exercise introduces it
  openapiAddressToSharedAddress / any Address-mapping helper: not found anywhere yet
```

- **Per plumbing piece:** The generated query layer (`customers.sql.go`, `models.go`, `db.go`) is fully in place and unchanged from guide 8's prediction. `module.go` already has the pool available on `Module.pgxDb` — the only missing wire is threading it into `http2.NewHandler(...)`. The handler itself hasn't moved at all since guide 8: it still fabricates a UUID and returns, never touching `request.Body` or any `dbmodels` call.
- **Non-obvious lines:** `type CountryCode = shared.CountryCode` (note the `=`, a genuine alias, not `type CountryCode shared.CountryCode`) means `addr.CountryCode` passed into `shared.NewAddress(...)` needs no cast or conversion step — it's already the exact same type the function expects. By contrast, `Email openapi_types.Email` on the request body is a distinct generated type (confirmed import `openapi_types "github.com/oapi-codegen/runtime/types"`), so it's the one field in `RegisterCustomer`'s body that will need its own explicit conversion to satisfy `InsertCustomerParams.Email string` — a wrinkle the page's `Address`-only mapping example doesn't surface.
- **What a senior notices:** `module.go` already took the dependency-injection step for the *module* (`NewModule(pgxDb, ...)`) before this exercise asks for it at the *handler* level — a reminder that DI here is threaded recursively: `main` (or whatever wires modules) injects into `Module`, `Module.Init` is meant to inject into `Handler`, and nothing currently closes that second hop. Fixing `Init()` to call `http2.NewHandler(m.pgxDb)` is a one-line, mechanical consequence of a pattern the repo already committed to one layer up.

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a scratch Go module with a local Postgres and a minimal sqlc setup (reusing module 04's pattern), reproduce the core mechanism this page describes end to end:
- Define a `NewHandler(db *pgxpool.Pool) Handler` constructor with a nil-check panic, matching the page's exact message style (`"db cannot be nil"`).
- Wire a trivial "module" that holds the pool and calls your handler's constructor with it (mirroring `module.go` → `handler.go`).
- Define two structs with identical fields today — an "API" struct and a "storage" struct — plus an explicit mapping function between them, matching the page's `openapiAddressToSharedAddress` shape.
- Call a sqlc-generated `:exec` insert query directly from the handler method, passing the mapped/converted params.

**Acceptance criteria:**
1. Calling your constructor with a `nil` pool panics immediately with your exact message, before any query ever runs.
2. The handler method builds the params struct using your mapping function's output, not by passing the "API" struct straight into the generated params type.
3. Add one more field to only the "storage" struct and confirm the "API" struct (and anything that already calls the mapping function) is completely unaffected until you deliberately update the mapper — proving the two types really are decoupled.
4. Trace one field through end to end that is a genuine type alias (like `CountryCode`) versus one that needs a real conversion (like `Email`) — write both cases and confirm only one of them needs an explicit cast/conversion line.

<details><summary>Hint</summary>For criterion 1, panicking inside the constructor (not later, inside the first query call) is the entire point — write a test that calls `NewHandler(nil)` and asserts a panic, mirroring how `common.MustUUIDFromString` already panics-on-construction elsewhere in this codebase.</details>
<details><summary>Hint</summary>For criterion 4, grep your "API" layer's generated types for `type X = Y` (an alias) versus `type X Y` or `type X struct{...}` (a distinct type) — only the latter two ever need an explicit conversion at a boundary.</details>

**Compare step:** Once `handler.go` is actually updated for this exercise, compare your scratch version against it and ask:
1. Does the real handler build `dbmodels.Queries` once (e.g. stored on `Handler`) or call `dbmodels.New(db)` fresh inside `RegisterCustomer`? Which did the page's snippet imply?
2. Does the real handler explicitly convert `body.Email` (`openapi_types.Email` → `string`), or does something else (a custom `MarshalText`/implicit conversion) make that step disappear?

## 5. Before the Exercise

- **Likely task:** [AI explanation — speculation, not derived from `exercise.md`, which hasn't been read] Change `NewHandler()` to `NewHandler(db *pgxpool.Pool)` with the nil-check panic; store `db` on `Handler`; update `module.go`'s `Init()` to call `http2.NewHandler(m.pgxDb)`; implement `openapiAddressToSharedAddress`; update `RegisterCustomer` to build `dbmodels.InsertCustomerParams` from `request.Body` (mapping `Address` via the helper, converting `Email`), call `queries.InsertCustomer(ctx, ...)`, and return the generated UUID in the existing `201` response on success.
- **Approach:**
  1. Confirm exactly what the exercise wants before writing code — in particular, whether `GetCustomerByUUID` needs wiring up too, or only `InsertCustomer`.
  2. Add the nil-check constructor first, in isolation, since it's independent of the DB-call logic.
  3. Fix the `module.go` → `handler.go` wiring gap (pass `m.pgxDb` into `NewHandler`).
  4. Write `openapiAddressToSharedAddress`, handling its returned `error` (what should happen if `shared.NewAddress` rejects the input — a 400 response, presumably, but confirm against the exercise rather than assuming).
  5. Convert `Email` explicitly, build `InsertCustomerParams`, call `InsertCustomer`, and only then return the success response.
  6. Confirm the build compiles and (if a DB is reachable) manually hit the endpoint to confirm a row actually lands in `orders.customers`.

## 6. Recall Questions

1. Why does the page call inserting directly in the HTTP handler "good enough for simple applications" instead of treating it as a mistake to avoid from the start?
   <details><summary>Answer</summary>The page frames a repository layer as something to introduce "as the project grows" — i.e. a response to real complexity (multiple callers, multiple queries needing to compose, cross-cutting DB logic), not a default to apply before that complexity exists. Adding the abstraction before anything needs it would be indirection without payoff.</details>

2. Why does the nil check in `NewHandler` matter if a nil `db` would cause a panic the first time it's used anyway?
   <details><summary>Answer</summary>Without the check, the panic happens later, deep inside a request handler's call stack, as a generic nil-pointer dereference — confusing to diagnose. With the check, the application fails immediately at startup with an explicit "db cannot be nil" message, pinpointing the cause instantly. Same eventual failure, very different time and clarity of discovery.</details>

3. Why is passing `*pgxpool.Pool` through the constructor preferred over a package-level global variable, according to the page?
   <details><summary>Answer</summary>Primarily testing: constructor injection lets tests swap in a mock/fake database in place of the real pool, while a global variable is an implicit, harder-to-override dependency. It's also more explicit — reading `NewHandler(db)` immediately shows what the handler needs, unlike a global that's invisible from any function signature.</details>

4. Why does `Address.CountryCode` need zero conversion inside `openapiAddressToSharedAddress`, while `RegisterCustomer.Email` will need an explicit one when the handler is fully wired?
   <details><summary>Answer</summary>`type CountryCode = shared.CountryCode` in `openapi.gen.go` is a true Go type alias — `CountryCode` and `shared.CountryCode` are literally the same type, so a value of one is already a value of the other with no cast required. `Email openapi_types.Email`, by contrast, is a distinct generated type (not an alias to `string`), while `dbmodels.InsertCustomerParams.Email` is a plain `string` — two different types require an explicit conversion to bridge, the same boundary-mapping principle the page applies to `Address`, just for a field the page's example doesn't cover.</details>

5. The page says duplicating `Address` across the HTTP and DB layers is often "the right call," citing "When to avoid DRY in Go." What's the actual tradeoff being made?
   <details><summary>Answer</summary>Sharing one struct across layers means any HTTP-contract change (new/renamed field, different validation rule) and any DB/storage change both ripple through the same type — coupling two things (public API shape, internal storage shape) that could reasonably evolve on different timelines. Keeping them separate costs one small, explicit mapping function; the page's claim is that writing and maintaining that boilerplate is cheaper than debugging the edge cases that sharing one type across diverging concerns eventually produces.</details>

6. `module.go`'s `NewModule` already receives and stores `pgxDb *pgxpool.Pool`, but `Init()` calls `http2.NewHandler()` with no arguments. What does this confirm about where the dependency-injection gap in this codebase actually is?
   <details><summary>Answer</summary>It confirms DI is already wired one layer up — from whatever constructs `Module` down into `Module.pgxDb` — but the chain stops there: `Module.Init` never forwards `m.pgxDb` into the handler it constructs. The fix this exercise needs isn't introducing dependency injection from scratch, it's completing an already-started chain by changing `http2.NewHandler()` to `http2.NewHandler(m.pgxDb)`.</details>

*(These questions, with answers, are also appended to `review_queue.md` under today's date.)*

## 7. Cheat Sheet

- Insert directly in the HTTP handler for now (`queries := dbmodels.New(db); queries.InsertCustomer(ctx, dbmodels.InsertCustomerParams{...})`) — a repository layer is deferred until the project actually grows into needing one.
- Inject `*pgxpool.Pool` through the constructor (`NewHandler(db *pgxpool.Pool)`), not a global variable or a per-call connection — explicit dependencies, easy to swap for tests.
- Add a nil check in every constructor (`if db == nil { panic("... cannot be nil") }`) — fails fast and clearly at startup instead of as a confusing nil-pointer panic deep in a request's stack trace.
- Wiring happens in one place per layer: `module.go`'s `Init` plays the role `main.go` plays in *Introducing Clean Architecture* — don't reach for a DI framework (Wire/Dig/Fx); a painful constructor list is a complexity signal, not a tooling gap.
- Keep `shared.Address` and the OpenAPI-generated `Address` as separate types with an explicit mapping function (`openapiAddressToSharedAddress`), even though their fields match today — per "When to avoid DRY in Go," the mapping boilerplate is cheaper than debugging coupling once either layer's shape needs to diverge.
- Check each field's actual generated type before assuming it needs conversion: `CountryCode = shared.CountryCode` is a real alias (zero conversion), while `Email openapi_types.Email` is a distinct type from `string` (needs an explicit conversion) — don't pattern-match "needs mapping" onto every field uniformly.
- Workspace gap this exercise closes: `module.go` already holds `pgxDb` but never passes it to `http2.NewHandler()` — that one-line wiring gap, plus the still-unimplemented `RegisterCustomer` body, are this exercise's concrete deliverables.
