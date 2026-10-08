# Study Guide: Repository Pattern (Implement + Integrate)

> **Pages:** "Implement Repository" (why the pattern exists, the `CustomerRepository` + its integration tests) and "Integrate Repository" (injecting the repository into the handler as a consumer-defined interface).
> **Track:** backend-masterclass-beta, module `05-repository`.
> **Previous guide:** `9_study_guide_insert_customer.md` — that page called direct-in-handler DB access a "for now" choice and explicitly predicted a repository layer "as the project grows." This page is that prediction arriving.
> **What this likely unlocks next:** component/integration tests of the full module; the "Integrate Repository" tip calls this "a lightweight form of Clean Architecture," implying a later page may formalize ports/adapters further.
>
> **Workspace state (checked directly, not from `exercise.md`):** `git log` confirms exercise `05-repository/01-implement-repository` is **completed** (`ba07856 completed 05-repository/01-implement-repository`), and `.tdl-exercise` now points at `05-repository/02-integrate-repository` — i.e. the two pasted sections map exactly onto "the exercise just finished" and "the exercise in progress," not two abstract theory sections. `backend/orders/adapters/db/customer_repo.go`'s `RegisterCustomer` is **fully implemented** (not a stub): it builds `dbmodels.InsertCustomerParams`, converts the address, and calls `queries.InsertCustomer`. `backend/orders/api/http/handler.go` and `backend/orders/module.go` are **unchanged** since guide 9 — `Handler` still holds `db *pgxpool.Pool` directly, `RegisterCustomer` still calls `dbmodels`/`q.InsertCustomer` itself, and `module.go`'s `Init()` still builds the handler straight from `m.pgxDb`. So "Integrate Repository" — rewiring the handler to call `db.NewCustomerRepository` through an interface — is exactly the work still outstanding, confirmed by the exercise tracker rather than assumed.
> **A concrete, already-committed detail worth flagging:** `customer_repo.go` defines its **own** `addressfromOpenAPIToShared(addr http.Address) (shared.Address, error)` — a second copy of the identical function `handler.go` already has (guide 9), rather than exporting and reusing one. See Concept 1's Mistakes table for why this is a real design tension, not a hypothetical one, given guide 9's own "avoid DRY" framing applied to *types* doesn't automatically extend to *logic*.

---

## 1. Snapshot

- **Topic:** Moving database logic out of the HTTP handler into a `CustomerRepository`, verified by integration tests that run against a real Postgres; then hiding that repository behind a single-method interface declared in the *consumer* package to avoid an import cycle.
- **Objective:** After this page, I can explain what a repository buys you over direct-in-handler SQL, why the interface for a repository belongs next to the handler rather than the implementation, and how integration tests differ from unit tests in what they need and what they catch.
- **Continuity:** Guide 9 inserted directly in the handler and explicitly called it a deferred tradeoff ("as the project grows, introduce a repository layer"). Exercise 01 (now complete) built that repository. This page's second half, and the current exercise, is wiring the handler to actually use it.

## 2. Concepts

### Repository Pattern — Separating Database Logic from the Handler

- **Problem:** [From page] Handlers mixing SQL calls with business logic become hard to read; two endpoints needing the same query duplicate it; testing requires running the server and calling real HTTP endpoints instead of testing logic directly.
- **Mechanism:** [From page] The HTTP handler calls a method on a repository (`h.customerRepository.RegisterCustomer(...)`); the repository owns `sqlc` details like `InsertCustomerParams`. [From code — confirmed, committed] The actual implementation:
  ```go
  func (r *CustomerRepository) RegisterCustomer(ctx context.Context, customerUUID common.UUID, customer http.RegisterCustomer) error {
      queries := dbmodels.New(r.db)

      address, err := addressfromOpenAPIToShared(customer.Address)
      if err != nil {
          return err
      }

      args := dbmodels.InsertCustomerParams{
          CustomerUuid: customerUUID,
          Name:         customer.Name,
          Email:        string(customer.Email),
          Address:      address,
          PhoneNumber:  customer.PhoneNumber,
      }
      queries.InsertCustomer(ctx, args)
      return nil
  }
  ```
  This is a near-exact copy of guide 9's handler-side insert logic, moved one layer down — same field list, same `Email` conversion, same error-on-address-mapping-failure shape.
- **Design Decision:** [From page] The alternative is guide 9's own "for now" choice — keep inserting straight in the handler. The repository layer costs one more indirection/file but buys: handlers that can be read without knowing the database exists, de-duplicated queries across endpoints, and business-logic tests that mock the repository instead of running a server. It would be the *wrong* choice (per guide 9's own framing) for a handler that will only ever have one caller and one query — this page's whole premise is that growth pressure has now arrived.
- **In Production:** [AI explanation] Notice what did *not* change in the move: the `Email`/`Address` conversion logic is identical to guide 9's handler code, just relocated. The repository didn't introduce new behavior — it relocated existing behavior behind a boundary, which is exactly what the pattern promises (same logic, better seams) rather than a rewrite.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Duplicating `addressfromOpenAPIToShared` in both `handler.go` and `customer_repo.go` instead of exporting one copy — **confirmed present in the current code** | Two implementations of the same conversion can drift silently (e.g. one gets a validation fix, the other doesn't) | `handler.go`'s version is unexported, so writing a local copy in `db` was the fastest way past the compiler, and guide 9's "avoid DRY" tip (about *types*) makes duplication feel endorsed everywhere | Guide 9's tip was specifically about keeping `shared.Address` and `http.Address` as separate *types* because their shapes may diverge — the *conversion function* between them doesn't have that same justification; exporting one copy (e.g. `AddressFromOpenAPIToShared`) and importing it from `db` costs nothing extra (no cycle risk — see Concept 3) and removes the drift risk entirely |
  | Having the repository method accept/return `dbmodels` types at its public boundary | Leaks `sqlc`-generated shapes into whatever calls the repository, defeating the "handler doesn't know about `InsertCustomerParams`" goal the page states explicitly | `dbmodels` types are already right there, convenient to pass through | Keep the repository's public signature in terms of domain/HTTP-layer types (`common.UUID`, `http.RegisterCustomer`), as the actual code does; translate to `dbmodels` types only inside the method body |

### Integration Tests — Real Infra, Build Tags, `cmp.Diff`

- **Problem:** [From page] Need to verify `RegisterCustomer` actually persists correctly against a real database — not just that it calls the right function — without making every `go test ./...` run slow or require Postgres.
- **Mechanism:** [From page/code] `customer_repo_test.go` and `setup_test.go` both start with `//go:build integration`, confirmed in the actual files — this excludes them from plain `go test ./...` / `task test`. `task test-integration` (or `go test -tags integration ./...`) is the only thing that compiles and runs them. [From code] `setup_test.go`'s `TestMain` calls `testutils.RunMigrations("orders", embedMigrations, "migrations")` once before any test in the package runs; `testutils.NewDB(t)` (used per-test) reads `POSTGRES_URL` from the environment and panics if it's unset — there's no fallback or default DSN.
  The test then compares with `cmp.Diff`:
  ```go
  if diff := cmp.Diff(
      dbmodels.OrdersCustomer{ /* expected fields */ },
      dbCustomer,
      cmpopts.EquateComparable(shared.SharedTypes...),
  ); diff != "" {
      t.Errorf("customer mismatch (-want +got):\n%s", diff)
  }
  ```
  [From code] `shared.SharedTypes = []any{CountryCode{}, Address{}}` (confirmed in `shared/types.go`) is the concrete list `EquateComparable` needs.
- **Design Decision:** [From page] `cmp.Diff` over manual field-by-field comparison: adding a new struct field later makes the test fail automatically (you forgot to set it in the expected value), where field-by-field comparison would silently skip the new field. The tradeoff is reflection-based comparison, which the page says to avoid in production code but is fine in tests. `EquateComparable` is needed specifically because `CountryCode` embeds `Enum[T]`, which has unexported fields — `cmp.Diff` panics on unexported fields by default unless told to treat a type as comparable via `==`.
- **In Production:** [AI explanation] A missing `POSTGRES_URL` fails loudly (a `panic`) rather than quietly skipping tests or connecting to some default — in CI this means a misconfigured environment variable shows up as an obvious crash, not a suite that silently reports zero failures because nothing ran. Since this test already passed (exercise 01 is marked complete), the committed `RegisterCustomer` body above is confirmed to satisfy it.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Adding a new field to `OrdersCustomer`/the expected struct without updating the test's expected value | With manual comparison this test would keep passing despite a real regression | Forgetting the test exists, or that it's exhaustive | `cmp.Diff` forces this to fail loudly — treat that failure as the intended signal, not test flakiness |
  | Comparing structs containing `CountryCode`/`Address` with plain `cmp.Diff(want, got)` (no options) | Test panics instead of failing with a diff, since `cmp` refuses to inspect unexported fields by default | Not realizing `Enum[T]` embedding introduces unexported fields | Pass `cmpopts.EquateComparable(shared.SharedTypes...)` for every type in that list |
  | Running integration tests as part of the default `task test` / pre-commit loop | Every contributor needs Postgres running locally just to get fast feedback on unrelated unit tests | The build tag feels like an obstacle rather than a deliberate speed/infra boundary | Keep them behind `-tags integration` / `task test-integration`, run them less frequently (still "seconds", per the page) but not on every save |

### Consumer-Defined Interfaces — Avoiding Import Cycles

- **Problem:** [From page] The handler needs to call the repository without knowing about `sqlc`/database details, but the interface can't simply live in the `db` package.
- **Mechanism:** [From page]
  ```go
  // declared in the http package, next to the handler that uses it
  type CustomerRepository interface {
      RegisterCustomer(ctx context.Context, customerUUID common.UUID, customer RegisterCustomer) error
  }
  ```
  Go interfaces are satisfied implicitly — `db.CustomerRepository` (the concrete struct) never imports or references this interface. It just happens to have a matching method set.
- **Design Decision:** [From page] The interface could in principle live in the `db` package next to the implementation (common in some other languages). The page rejects this for a concrete, mechanical reason, not a style preference: `db` already imports `http` (for `http.RegisterCustomer` as a parameter type — confirmed in `customer_repo.go`'s actual import list). If the interface lived in `db`, the `http` package would need to import `db` to reference it — and `db` already imports `http` — a direct import cycle that fails to compile. [From page] This is called "a lightweight form of Clean Architecture": the handler defines what it needs as an interface, `module.go` connects the concrete implementation to it, without adopting a full ports-and-adapters layout.
- **In Production:** [AI explanation] This pattern only resolves the *interface's* direction of dependency — the mapping-function duplication from Concept 1 is a separate, ordinary dependency (a function, not an interface), so `db` importing `http` to call an exported `AddressFromOpenAPIToShared` would **not** create a cycle; only an interface declared in `db` referencing `http` types while `http` also needs to import `db` would. The duplication in the current code is therefore avoidable, not a forced consequence of the cycle rule.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Declaring `CustomerRepository` interface in the `db` package since "that's where the implementation is" | Won't compile once `http` needs to import it, because `db` already imports `http` for parameter types | Habit from languages where interface and implementation are co-located | Declare the interface in the consumer (`http`) package; Go's implicit satisfaction means the implementer needs no reference to it |
  | Assuming *any* cross-package dependency between `db` and `http` is forbidden because of the cycle risk | Over-engineering to avoid imports that are actually fine (e.g. a plain function call) — or, conversely, duplicating a function to "be safe" when exporting it would have been fine | Overgeneralizing the specific interface-placement rule | Only the interface's location creates the cycle risk here; ordinary type/function imports in the already-existing direction (`db` → `http`) are fine |

### Decoupling the Handler from the Database

- **Problem:** [From page] Even with a repository implementation written and tested, the handler still directly holds `*pgxpool.Pool` and calls `dbmodels`/sqlc types — it still "knows" about the database. **Confirmed still true right now**: `handler.go` is unchanged since guide 9.
- **Mechanism:** [From page] Target end state (this is the current exercise's goal, not yet in the workspace):
  ```go
  func (h Handler) RegisterCustomer(ctx context.Context, request RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error) {
      customerUUID := common.NewUUIDv7()
      err := h.customerRepository.RegisterCustomer(ctx, customerUUID, *request.Body)
      if err != nil {
          return nil, err
      }
      // ...
  ```
  The handler keeps UUID generation ("that's not a database concern," per the page) and delegates everything else. `Handler` gets a `customerRepository CustomerRepository` field (the interface type) injected through its constructor, same pattern as guide 9's `db *pgxpool.Pool` injection — just one level removed.
- **Design Decision:** [From page] The page explicitly recommends *against* unit-testing thin handlers like this one with a mocked repository — "the maintenance cost rarely pays off." If a handler grows real logic, extract it into a plain function/helper and test that directly; full-stack behavior is covered later by component tests. This means the value of the interface here is architectural decoupling (can swap implementations, clear dependency direction), not unlocking handler unit tests as the primary payoff.
- **In Production:** [AI explanation] "If you read the handler code and can't tell what database it uses, that's a good sign" (page's own framing) — a useful litmus test: after this refactor, `handler.go` should have zero imports of `dbmodels`, `pgxpool`, or anything `sqlc`-flavored. Right now it fails that test on all three counts; that's precisely the gap this exercise closes.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Writing a unit test for `RegisterCustomer` with a hand-rolled mock `CustomerRepository` | Test effort spent asserting "the handler calls the method with these arguments," which breaks on every signature change and catches almost no real bugs | The interface makes mocking trivially possible, so it feels like it should be done | Skip it for thin handlers per the page's own guidance; rely on component tests later for real coverage |
  | Keeping `*pgxpool.Pool` on `Handler` *alongside* the new `customerRepository` field, "just in case" | Handler still imports database infra packages, silently reopening the exact coupling this exercise removes | Incremental refactor leaves old field in place during the transition | Remove `db *pgxpool.Pool` from `Handler` entirely once the repository field replaces it |

## 3. Plumbing Dissection

**Wiring map (confirmed directly in the current workspace):**

```
backend/orders/module.go                                          (CONFIRMED, unchanged — this exercise's target)
  Init(ctx): httpHandler := http2.NewHandler(m.pgxDb)              ← still builds handler from the raw pool
                                                                       db.NewCustomerRepository is never called anywhere in module.go

backend/orders/api/http/handler.go                                 (CONFIRMED, unchanged — this exercise's target)
  type Handler struct { db *pgxpool.Pool }                         ← no CustomerRepository interface defined here yet
  RegisterCustomer(...): still calls dbmodels.New(h.db) / q.InsertCustomer itself
  addressfromOpenAPIToShared(addr Address) (shared.Address, error) ← unexported; only callable from this file

backend/orders/adapters/db/customer_repo.go                        (CONFIRMED — exercise 01, DONE)
  type CustomerRepository struct { db *pgxpool.Pool }
  NewCustomerRepository(db) *CustomerRepository                    ← nil-check panic, mirrors NewHandler (guide 9)
  RegisterCustomer(ctx, customerUUID common.UUID, customer http.RegisterCustomer) error
      queries := dbmodels.New(r.db)
      address, err := addressfromOpenAPIToShared(customer.Address)  ← its OWN copy of the mapping function
      queries.InsertCustomer(ctx, dbmodels.InsertCustomerParams{...})
  imports: context, pgxpool, common, shared, dbmodels, http

backend/orders/adapters/db/customer_repo_test.go    (READ-ONLY, //go:build integration — PASSING)
  db.NewCustomerRepository(dbPool).RegisterCustomer(ctx, customerUUID, customer)
  queries.GetCustomerByUUID(ctx, customerUUID) → cmp.Diff(..., cmpopts.EquateComparable(shared.SharedTypes...))

backend/orders/adapters/db/setup_test.go            (READ-ONLY, //go:build integration)
  TestMain → testutils.RunMigrations("orders", embedMigrations, "migrations") before any test runs
```

- **Per plumbing piece:** `NewCustomerRepository` carries the same nil-check pattern guide 9 introduced for `NewHandler` — a consistent constructor convention across both layers. `customer_repo.go`'s `RegisterCustomer` is complete and passing its integration test. The repository exists and works, but nothing calls it yet outside the test file — `module.go` never constructs it, and `handler.go` has no field to receive it.
- **Non-obvious lines:** `customer_repo.go` imports `eats/backend/orders/api/http` for exactly one reason — the `customer http.RegisterCustomer` parameter type — and then defines `addressfromOpenAPIToShared(addr http.Address) (shared.Address, error)` as a second, independent implementation of a function `handler.go` already has under the same name. Same name, same body, two packages — easy to misread as "the same function" when grepping, but they're genuinely separate symbols.
- **What a senior notices:** The exercise sequencing mirrors guide 9's own DI gap exactly — back then, `module.go` already held `pgxDb` before the handler was wired to use it; here, the repository and its tests already exist, pass, and are committed before the handler/module ever reference the repository. The codebase's consistent pattern: scaffold the consumer-independent piece and its tests first, wire it into callers in a later, separate exercise. The duplicated mapping function is the one loose thread this sequencing left behind — worth resolving (export one copy) while touching `handler.go` for the interface work anyway, since that file is being edited regardless.

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a scratch Go module, reproduce the core plumbing this page teaches — the consumer-defined interface that avoids an import cycle, not the Postgres integration-test setup (guide 9's rebuild already covered real-DB wiring).

- Two packages: `api` (plays the role of `http`) and `storage` (plays the role of `db`). `storage` imports `api` for one plain type (e.g. `api.CreateWidgetRequest`).
- Declare a single-method interface in `api`, next to a `Handler` struct that holds it as a field.
- Implement a concrete `storage.WidgetRepository` that satisfies the interface *without importing `api`'s interface* — only the plain request type.
- Wire them in a third package (`app`, playing `module.go`'s role): construct the concrete `storage.WidgetRepository`, pass it into `api.NewHandler(repo)` as the interface type.

**Acceptance criteria:**
1. `storage` imports `api` (for the request type) — confirm this compiles and is intentional, not a mistake.
2. Attempting to also declare the interface in `storage` and have `api` import it should fail to compile with an import cycle — deliberately cause this once, to see the actual compiler error, then revert.
3. A hand-written fake (not a mocking library) that satisfies the `api` interface can be passed into `api.NewHandler` instead of the real `storage` implementation, with zero changes to `api`'s code.
4. Removing the interface and making `api.Handler` hold `*storage.WidgetRepository` directly should make `api` fail to compile without importing `storage` — demonstrating what the interface was preventing.
5. Add a small mapping function needed by both packages (e.g. request → domain struct); export it from one package and call it from the other, proving — unlike the interface — this direction never risks a cycle. Compare this against the real codebase's two separate `addressfromOpenAPIToShared` copies.

<details><summary>Hint</summary>For criterion 2, the cycle only appears if the interface itself (not just plain structs) is declared in `storage` and referenced from `api` — plain type imports in one direction are never the problem.</details>
<details><summary>Hint</summary>For criterion 5, put the mapping function in whichever package is "upstream" in the existing import direction (the one already imported by the other), so no new import edge is needed at all.</details>

**Compare step:** Once the real `module.go`/`handler.go` are updated for "Integrate Repository," diff against your scratch version and ask:
1. Does the real `Handler` keep *any* database-related field alongside `customerRepository`, or is `db *pgxpool.Pool` fully removed?
2. Did the real refactor resolve the duplicated `addressfromOpenAPIToShared` (by exporting one copy), or do both copies still exist afterward?

## 5. Before the Exercise

- **Likely task:** [AI explanation — speculation, not derived from `exercise.md`, which hasn't been read; grounded in `.tdl-exercise` confirming the current exercise is `05-repository/02-integrate-repository`] Add a `CustomerRepository` interface to `handler.go` with one method matching `db.CustomerRepository.RegisterCustomer`'s signature; change `Handler` to hold that interface instead of `db *pgxpool.Pool`; update `NewHandler` to accept and nil-check the interface; simplify `RegisterCustomer` to just generate the UUID and call `h.customerRepository.RegisterCustomer(...)`; update `module.go`'s `Init()` to construct `db.NewCustomerRepository(m.pgxDb)` and pass it to `http2.NewHandler(...)` instead of the raw pool.
- **Approach:**
  1. Confirm with the exercise exactly what the interface's method set should be (just `RegisterCustomer`, or does it anticipate `GetCustomerByUUID` too) before writing it.
  2. Add the interface to `handler.go`, next to `Handler` — not in `db` (Concept 3).
  3. Change `Handler`'s field and `NewHandler`'s parameter to the interface type; keep the nil-check panic pattern guide 9 established.
  4. Strip `RegisterCustomer` down to UUID generation + the repository call; remove the now-unused `dbmodels`/`pgxpool`/address-mapping code from this file.
  5. Decide what to do with `handler.go`'s now-possibly-unused `addressfromOpenAPIToShared` — delete it, or export it and have `customer_repo.go` import and reuse it instead of keeping its own copy.
  6. Update `module.go` to build `db.NewCustomerRepository(m.pgxDb)` and pass that into `http2.NewHandler(...)`.
  7. Run `task test` (or the project's normal test command) and confirm the handler package compiles with no `dbmodels`/`pgxpool` imports left.

## 6. Recall Questions

1. Why can't the `CustomerRepository` interface live in the `db` package next to its implementation, even though that's where the method body actually is?
   <details><summary>Answer</summary>`db` already imports `http` (confirmed in `customer_repo.go`, for `http.RegisterCustomer` and `http.Address`). If the interface also lived in `db`, the `http` package would need to import `db` to reference that interface — creating a two-way import cycle that fails to compile. Declaring the interface in `http` instead works because Go interfaces are satisfied implicitly: the `db` package's concrete type never needs to import or reference the interface at all.</details>

2. `customer_repo.go` already imports `http` for `http.RegisterCustomer` and `http.Address`. Why does it define its own `addressfromOpenAPIToShared` instead of calling `handler.go`'s version through that same import?
   <details><summary>Answer</summary>`handler.go`'s `addressfromOpenAPIToShared` is unexported (lowercase), so importing the `http` package doesn't grant access to it — unexported identifiers are only visible within their own package. The import cycle concern (Concept 3) doesn't apply here, since this would be a plain function call, not an interface; the actual blocker is Go's visibility rules, which is why the current code has two separate copies instead of one shared one.</details>

3. Why does the page recommend against unit-testing the thin `RegisterCustomer` handler with a mocked `CustomerRepository`, even though the interface makes mocking trivial?
   <details><summary>Answer</summary>For a handler this thin (generate a UUID, call one repository method, return), a mock-based test mostly re-asserts "the handler calls the method with these arguments" — it breaks on signature changes and catches few real bugs, so its maintenance cost rarely pays for itself. The page's guidance: extract real logic into helper functions and test those directly, and rely on later component tests for full-stack coverage.</details>

4. Why does `customer_repo_test.go` use `cmpopts.EquateComparable(shared.SharedTypes...)` instead of calling `cmp.Diff` with no options?
   <details><summary>Answer</summary>Types like `CountryCode` embed `Enum[T]`, which carries unexported fields. `cmp.Diff` panics by default when it encounters unexported fields inside a struct it's asked to compare. `EquateComparable` tells it to use Go's `==` operator for the listed types instead of reflecting into their internals — `shared.SharedTypes` is the explicit list of types that need this treatment.</details>

5. Guide 9 argued that duplicating `Address` as both `shared.Address` and `http.Address` is often "the right call." Does that same argument justify `customer_repo.go` and `handler.go` each having their own copy of `addressfromOpenAPIToShared`?
   <details><summary>Answer</summary>Not automatically. Guide 9's argument was about *types* that might reasonably evolve differently across layers (HTTP contract vs. storage shape). The *conversion function* between two already-separate types doesn't carry that same justification — exporting one copy and importing it (plain function import, not an interface) costs nothing and carries no cycle risk, so keeping two identical copies in sync by hand is closer to accidental duplication than the deliberate type-separation the earlier page endorsed.</details>

6. Given the current workspace state, why would adding `db.NewCustomerRepository(m.pgxDb)` wiring to `module.go` right now not actually fix anything by itself?
   <details><summary>Answer</summary>`handler.go`'s `Handler` struct still only has a `db *pgxpool.Pool` field and no `CustomerRepository` interface field to receive it — there's nothing in the `http` package yet for the repository to be injected into. Wiring the repository into `module.go` without first adding the interface and the field to `Handler` (the "Integrate Repository" work) would just produce a constructed repository with nowhere to go.</details>

*(These questions, with answers, are also appended to `review_queue.md` under today's date.)*

## 7. Cheat Sheet

- Repository pattern: handler calls `h.customerRepository.RegisterCustomer(ctx, uuid, body)`; the repository alone knows about `sqlc`/`dbmodels` types. Use it once a query is needed by more than one caller, or the handler needs to stay readable without knowing the database exists — not before.
- Integration tests: `//go:build integration` tag, run via `task test-integration` / `go test -tags integration ./...`; `TestMain` runs migrations once per package before any test; tests need `POSTGRES_URL` set (no fallback — panics if missing).
- `cmp.Diff(want, got, cmpopts.EquateComparable(shared.SharedTypes...))` over field-by-field comparison — a new struct field makes the test fail automatically instead of silently passing; `EquateComparable` is required for any type with unexported fields (like `CountryCode`, via `Enum[T]` embedding).
- Declare a repository's interface in the *consumer* package (`http`, next to the handler), never in the implementation package (`db`) — `db` already imports `http` for request types, so the reverse interface reference would cycle. Implementations satisfy the interface implicitly; no import needed on their side.
- A plain function import (e.g. an address-mapping helper) in the `db → http` direction is fine and doesn't cycle — only an *interface* declared in `db` and referenced from `http` would. Don't duplicate a function across layers just because exporting it feels riskier than it is.
- Don't unit-test thin, mostly-delegating handlers with a mocked repository — extract real logic into testable helpers instead; component tests cover the full stack later.
- Current state: exercise 01 (repository implementation, including its own copy of the address mapping) is done and passing; exercise 02 (wiring `handler.go`/`module.go` to the interface) is the outstanding work — confirmed via `.tdl-exercise` and `git log`, not guessed.
