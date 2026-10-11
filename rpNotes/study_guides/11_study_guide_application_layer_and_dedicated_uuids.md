# Study Guide: Application Layer Types + Dedicated UUIDs

> **Pages:** "Application Layer Types" and "Dedicated UUIDs".
> **Track:** backend-masterclass-beta, module `06-application-layer` (exercises `01-app-types` and `02-dedicated-uuids`).
> **Previous guide:** `10_study_guide_repository_pattern.md` — it put the `CustomerRepository` interface in the `http` package to dodge a `db → http` import cycle, and flagged two copies of `addressfromOpenAPIToShared` as a loose thread. This page moves the contract one layer inward and closes that thread.
> **Tags:** [From page], [From code], [AI explanation].
>
> **Workspace state (checked directly, not from `exercise.md`, which is stale and still contains the SQL-migrations page):**
> - `git log`: `91f43b2 completed 06-application-layer/02-dedicated-uuids`; next scaffold is `07-errors-and-testing/01-error-handling` (touches only `common/errors*.go` and tests).
> - `go build ./...` passes. `go vet -tags integration ./backend/orders/...` **fails**: `customer_repo_test.go:34: cannot use customerUUID (variable of array type common.UUID) as app.CustomerUUID value`. The test file was last written in `45ea9d3` (`01-app-types`) and not updated for `02`. Likely `tdl` overlays its own copy when it runs the exercise, but I haven't verified that. Treat `task test-integration` as broken locally until checked.

---

## 1. Snapshot

- **Topic:** Introduce an `app` package (service + domain types + repository contract) between HTTP and DB, then make `CustomerUUID` a distinct type via struct embedding, and point `oapi-codegen` and `sqlc` at it.
- **Objective:** After this page, I can say what belongs in the application layer, where each mapping happens, why `struct{ common.UUID }` beats both a type alias and `type X common.UUID`, and what that type does *not* protect against.
- **Continuity:** Guide 10: `http.CustomerRepository` took `http.RegisterCustomer`, so `db` imported `http`. Now `db` and `http` both import `app` and never each other. **Next (likely):** `07-errors-and-testing/01-error-handling`, which needs the `app`/`db` seam to carry errors. See Section 5.

## 2. Concepts

### The Application Layer — one set of types, many entry points

- **Problem:** [From page] A generated HTTP type used as the repository parameter couples the DB layer to the API schema. Fields that belong to one side (e.g. `PasswordHash`) get hidden with `json:"-"`. Every extra entry point (gRPC, CLI, jobs, message consumers) duplicates the logic that sits in the HTTP handler.
- **Mechanism:** [From code] Dependencies point inward to `app`:

  ```
  http (handler) ──▶ app ◀── db (repository impl)
                      │
                      └──▶ common, shared
  ```

  ```go
  // app/customer.go — app owns the contract it needs from storage
  type CustomerRepository interface {
      RegisterCustomer(ctx context.Context, customer Customer) error
  }

  // api/http/handler.go — the handler's only job: map in, call, map out
  customerApp := app.Customer{
      CustomerUUID: app.CustomerUUID{UUID: customerUUID},
      Email:        string(customer.Email), // openapi_types.Email → string
      Address:      address,                // http.Address → shared.Address
      // ...
  }
  if err := h.svc.RegisterCustomer(ctx, customerApp); err != nil { ... }
  ```

  `db.CustomerRepository` satisfies the interface implicitly. `dbmodels` also imports `app` (`models.go:9`), so sqlc output references `app.CustomerUUID`.
- **Design Decision:** The alternatives are (a) skip the layer, as guide 9 did, and (b) keep the interface in the consumer package, as guide 10 did. The page keeps guide 10's rule ("interface lives with its consumer") but changes who the consumer is: now the *service*, not the handler. The cost is one more struct and one mapping per boundary, plus a pass-through `Service` today (`app/customer.go:25`). It's the wrong choice for one entry point, one table and no logic. [From page] The page says so itself.
- **In Production:** [AI explanation] The failure mode isn't a crash. It's change cost: a new column means edits in `openapi.yaml`, the handler mapping, `app.Customer`, the repo params and the migration. That's the intended cost. Review that diff for a missing mapping. Test the mapping with a table test of `handler → app.Customer`, and keep DB behavior in the integration test. The page's own code-review thread (positional vs named struct literals) names the real risk: a **new `Customer` field that nobody maps**. Named fields compile and zero-initialize silently, so the only guard is a test that asserts every field.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Putting `http.*` or `dbmodels.*` types in `app` signatures | Layers re-couple. The next API change forces repo changes (the problem this page opens with) | The generated type is already there and the mapping looks like boilerplate | `app` types use primitives and `shared`/`common` types only |
  | Business rules written in the handler "because the service just passes through" | gRPC/CLI/job entry points each re-implement or skip the rule | Today's `Service` is a pass-through, so the logic seems to belong in the handler | Rules go in `Service`. The handler only maps and returns |
  | Mapping test absent, new field added to `app.Customer` | Field silently stored as `""`/zero. No compile error with named literals | Named fields were chosen deliberately in the review thread, and nothing replaced the safety positional literals would have given | Round-trip test per boundary that sets every field to a distinct value |
  | Empty `ModulesContract` filled by copying `*contracts.Contracts` by value | Contracts registered after `Init` (see `svc.go` comment) are invisible to the service | Value copy looks harmless | Pass the pointer; `Init` runs before `RegisterContracts` populates it |

### Dedicated UUID Types — distinct type, promoted methods

- **Problem:** [From page] `CustomerUUID` and `RestaurantUUID` are both `[16]byte`. Passing one where the other is expected compiles, and it is only caught by a "no rows" query, or worse, by returning another entity's data.
- **Mechanism:** [From page] Three ways to declare it, and what each does:

  ```go
  type CustomerUUID = common.UUID            // alias: identical type, zero safety
  type CustomerUUID common.UUID              // distinct, but method set is empty
  type CustomerUUID struct{ common.UUID }    // distinct, methods promoted — chosen
  ```

  [From page] The defined type loses `MarshalText`, `UnmarshalText`, `Value`, `Scan`, `String`, `IsZero`. Embedding promotes them, so the JSON and `database/sql`/pgx paths keep working without re-implementation. [From code] `common/uuid.go` defines `Scan`/`UnmarshalText` on `*UUID`; they sit in `*CustomerUUID`'s method set because the embedded field is addressable.
  [AI explanation] Without `MarshalText`, `encoding/json` would encode a `[16]byte`-based type as an array of 16 numbers, not a string. That's the production symptom to watch for. Verify with a 3-line test.
- **Design Decision:** Alternatives are the alias, the plain defined type, or a generic `ID[T any]` (`type ID[T any] struct{ common.UUID }`, `type CustomerUUID = ID[customerTag]` — [AI explanation], not on the page). The generic route avoids one declaration per ID. It costs you a phantom-type tag and a worse error message, and is arguably over-clever for four IDs. Embedding is the wrong choice when you need the *ID itself* to carry rules (e.g. a prefix or checksum). Then use a named field with explicit methods.
  **What the type does not protect:**
  1. [AI explanation — verify in a scratch file] An explicit conversion `RestaurantUUID(customerUUID)` should compile, because both underlying types are the identical `struct{ common.UUID }`. The guard is against *implicit* mix-ups only.
  2. [From code] `common.UUID.Equals(other UUID)` is promoted, so `c.Equals(r.UUID)` compiles — you reach through `.UUID` and the type check is gone. `c == r` is rejected.
- **Validation boundary:** [From page, code review] Validate on the way in (HTTP/external input), trust data read from our own DB, and use `NewUUIDv7()` for new IDs. [AI explanation] Consequence: `app.CustomerUUID{}` is a valid Go value (zero UUID). `IsZero()` exists but nothing in the service calls it, so the zero UUID is never rejected inside `app`. Whether that's acceptable depends on whether HTTP validation covers it.
- **In Production:** Wrong-ID bugs now surface at compile time. Remaining runtime failure: conversions/`.UUID` reach-throughs. Observe via repo query returning `pgx.ErrNoRows`. Test the JSON/DB round trip once (marshal → string; insert → select equal) — `TestRegisterCustomer` already does the DB half.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Alias `=` instead of a defined type | Compiler accepts any UUID. Bug found at runtime via empty results | `=` reads as "just a rename" | Struct embedding, as the page does |
  | `type X common.UUID` | JSON serializes as a numeric array; pgx/`database/sql` can't scan/encode properly | The distinct-type goal is satisfied, so it looks done | Embed so methods are promoted |
  | Unwrapping with `.UUID` / `RestaurantUUID(x)` to get past a compile error | Reintroduces the exact bug class the type prevents, silently | The error message feels like friction, not a signal | Fix the call site. If you must convert, do it in one named function with a comment |
  | Strict validation inside the read path (`Scan`) | Historical rows that violate a later rule can no longer be loaded | Wanting "always valid" values everywhere | Validate at boundaries, as the review thread concludes. Revisit per type |

### Pointing codegen at `app` types

- **Problem:** Generated code defaults to `common.UUID`, so the typed UUID would be re-wrapped by hand at every generated boundary.
- **Mechanism:** [From code]
  - `openapi.yaml`: `x-go-type: app.CustomerUUID` + `x-go-type-import` → `openapi.gen.go:42` becomes `type CustomerUUID = app.CustomerUUID`. The alias is generated, so the real type stays `app.CustomerUUID`.
  - `sqlc.yaml` (lines 28–38): the committed change edits the existing **`db_type: "uuid"`** overrides to `app.CustomerUUID` / `app.NullCustomerUUID`.
  - [From page] The page asked for a **column-level** override for `orders.customers.customer_uuid` and says a column override beats a `db_type` override.
- **Design Decision:** A `db_type` override is global to the schema. Column overrides scale per ID. Once a module adds a `restaurant_uuid` column, the global one would give it `CustomerUUID`.
- **In Production:** [From code] Two latent issues in the current `sqlc.yaml`:
  1. Any other non-null `uuid` column anywhere gets `app.CustomerUUID` after `sqlc generate`. It would still compile if only the wrong ID flows through, so the failure surfaces late.
  2. `app.NullCustomerUUID` **does not exist** (`grep` finds it only in `sqlc.yaml`). The original `common.NullUUID` didn't exist either, so the dangling reference is inherited from the scaffold, but the first nullable `uuid` column makes `dbmodels` fail to compile.
  Neither affects the tests today (one non-null UUID column). `tdl` passing the exercise doesn't mean it's the config the page describes.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Editing the `db_type` override instead of adding a column override | Future UUID columns silently typed as `CustomerUUID` | The existing block is right there and edit-in-place satisfies the compiler | Restore the `db_type` default, add `column: "orders.customers.customer_uuid"` |
  | Hand-editing `*.gen.go` to fix a type | Overwritten on the next `go generate` | Compiler errors point at the generated file | Fix `openapi.yaml`/`sqlc.yaml`, regenerate |
  | Referencing a type in an override that doesn't exist | `dbmodels` stops compiling the day a matching column appears | Config isn't compiled until `sqlc generate` runs | Add a CI step that runs `go generate ./... && git diff --exit-code && go build ./...` |

## 3. Plumbing Dissection

**Wiring map** [From code]:

```
cmd/main.go            signal.NotifyContext → pgxpool.New(POSTGRES_URL) → backend.New(ctx, pool)
backend/svc.go         contracts := &contracts.Contracts{}  (pointer; filled later)
                       orders.NewModule(pool, contracts)
                       per module: Init → RegisterContracts → (all) Verify → RegisterHttp
orders/module.go Init  db.NewCustomerRepository(pgxDb)               adapters/db
                       app.NewService(repo, struct{}{})               app
                       http2.NewHandler(svc)                          api/http
                       MigrateDatabaseUp(...)                         (after wiring)
RegisterHttp           http2.Register → RegisterHandlers(e, NewStrictHandler(handler, nil))
Run                    defer pool.Close(); goroutine on ctx.Done → echo.Shutdown(context.Background())
```

- **`app.NewService` nil checks** (`service.go`): constructor `panic` on nil `customerRepository`/`modules`. Fails at startup with a clear message instead of a nil dereference inside a request. Same convention as guide 9/10. [AI explanation] The check compares the *interface* to nil. A typed-nil pointer wrapped in the interface would pass it. Safe here only because `NewCustomerRepository` returns non-nil or panics.
- **`struct{}{}` as `ModulesContract`** (`module.go:48`): an empty interface accepts anything non-nil. `m.modules` (the `*contracts.Contracts`) is stored on `Module` but unused. [AI explanation] Later you'll pass it here. It must be the pointer, since contracts are registered after `Init`.
- **Pass-through service** (`app/customer.go:25`): `Service.RegisterCustomer` just forwards to the repo, so the layer exists before there's logic. [From page] That's intentional.
- **Non-obvious lines:**
  - `customer_repo.go:36`: `queries.InsertCustomer(ctx, args)` — the returned error is **dropped** and the method returns `nil`. A failed insert (e.g. duplicate key, connection error) reports success to the caller and the handler returns 201 with a UUID that was never stored. This was already in guide 10's code. The next exercise is on error handling, so likely a deliberate gap.
  - `customer_repo.go:30`: `app.CustomerUUID{UUID: customer.CustomerUUID.UUID}` rebuilds a value that is already a `CustomerUUID`. `string(customer.Email)` on a `string` is also a no-op. Both are leftovers from when `Customer` held `common.UUID` / `openapi_types.Email`.
  - `handler.go:12`: `svc *app.Service` — the handler depends on the concrete service, not a consumer-defined interface, unlike guide 10's repository.
  - `module.go:24` comment says "http importing db would create the cyclic import the repository pattern avoids". [From code] That's now stale: `db` imports `app`/`dbmodels`, not `http`. Importing `db` from `http` would still be wrong layering but is no longer a cycle.
- **What a senior notices:**
  1. Guide 10's loose thread is closed structurally, not by discipline: `addressfromOpenAPIToShared` now exists once (`handler.go:53`) because `app.Customer` carries `shared.Address` and the repo no longer maps addresses. [AI explanation] The type used for the sqlc column and the app type are the same here, so `app` is coupled to column shapes for `Address` and the ID.
  2. The concrete `*app.Service` in `Handler` is fine while there's one caller. When a second entry point or a handler test needs a fake, define a small interface in `http` (same rule as guide 10).
  3. Layering is now enforced only by convention. [AI explanation] A cheap guard: a test (or `depguard`/`go list -deps`) asserting that `app` imports neither `api/http` nor `adapters/db`.

## 4. Rebuild Challenge

Do this once at the end of the module, not per page (see `faq.md`). In a blank scratch module, **without looking at the project**:

- **Spec:**
  - Package `app` defines `CustomerUUID`/`RestaurantUUID` (embedding a local `UUID [16]byte` type with text marshaling), a `Customer` type, a `CustomerRepository` interface it owns, and a `Service` (`NewService` rejects nil).
  - Two entry points map their own input types to `app.Customer` and call the same `Service`: a `net/http` JSON handler and a CLI `main`.
  - An in-memory (or SQLite) repository implements `app.CustomerRepository` and imports only `app`.
  - A composition root wires repo → service → both entry points.
- **Acceptance criteria:**
  1. `go list -deps` shows `app` imports neither the entry points nor the repository, and the repository doesn't import either entry point.
  2. A function taking `CustomerUUID` rejects a `RestaurantUUID` argument at compile time. Then swap in the alias and the plain-defined-type variants and record what changes (it compiles; JSON becomes a number array).
  3. JSON round trip: `CustomerUUID` marshals to a string and unmarshals back; `RestaurantUUID(customerUUID)` and `c.Equals(r.UUID)` both compile — write a test or comment documenting this limitation.
  4. **Failure case:** the repo returns an error; the HTTP handler returns 500 (not 201) and the CLI exits non-zero. Contrast with `customer_repo.go:36`.
  5. A test sets every `Customer` field to a distinct value in the entry-point input and asserts the repo received them all.

<details><summary>Hint 1</summary>For criterion 2, the compile failure is the point. Keep a `_ = GetCustomer(restaurantUUID)` line commented out, or put it behind a build tag, so the module still builds.</details>
<details><summary>Hint 2</summary>For criterion 3, `encoding/json` uses `encoding.TextMarshaler` on the value, so promoting `MarshalText` through the embedded field is enough. Test `UnmarshalText` through a pointer.</details>
<details><summary>Hint 3</summary>For criterion 5, use distinct strings per field (`"name-x"`, `"email-x"`) so a swapped pair is visible, which is the other half of the positional-vs-named review thread.</details>

**Compare step** — diff against the real `app`, `handler.go`, `module.go` and ask:
1. Where did my mapping live, and did the entry point or the service own it?
2. Did my repo error propagate, and who decides the HTTP status?
3. Did I need `.UUID` anywhere to make something compile, and what does that say about the type's boundary?

## 5. Before the Exercise

- **Likely task:** [AI explanation, inferred from `git show --stat` of `7e6865c`; the real task is in the CLI] `07-errors-and-testing/01-error-handling` scaffolds `common/errors.go`, `common/errors_echo.go` and `errors_echo_test.go` — a typed `common.Error` (slug, public vs internal message, HTTP code, details) mapped to a JSON response by `EchoErrorHandler`. Expect to complete the stubs there until the test passes.
- **Approach:**
  1. Read `errors_echo_test.go` first; it's the spec.
  2. Read `errors.go` for the `Error` type, its `Error()` string and `WithDetails`, so you know what's public vs internal.
  3. Implement the mapping so internal causes are logged but not returned to the client.
  4. Run `task test`, then decide separately whether the repository's dropped error (`customer_repo.go:36`) is in scope for this exercise or for a later one.

## 6. Recall Questions

1. Why is `type CustomerUUID struct{ common.UUID }` better than both `type CustomerUUID = common.UUID` and `type CustomerUUID common.UUID`?
   <details><summary>Answer</summary>The alias is the same type as `common.UUID`, so the compiler treats every UUID as interchangeable and you find the bug at runtime. `type CustomerUUID common.UUID` is distinct but has an empty method set, so it loses `MarshalText`, `UnmarshalText`, `Value`, `Scan`, `String`, `IsZero`. JSON and DB drivers would break unless re-implemented. Embedding in a struct gives a distinct type and promotes all methods.</details>

2. Even with `CustomerUUID` and `RestaurantUUID` as distinct embedded-struct types, name two ways a wrong-ID bug still compiles.
   <details><summary>Answer</summary>(1) An explicit conversion `RestaurantUUID(customerUUID)`, which should compile because the underlying struct types are identical (verify in a scratch file). (2) Reaching through the promoted field, e.g. `c.Equals(r.UUID)` or passing `r.UUID` to a `common.UUID` parameter. The type system protects against accidental mix-ups, not deliberate unwrapping.</details>

3. What changed about *who imports whom* between guide 10 and now, and what problem from guide 10 did that remove?
   <details><summary>Answer</summary>Guide 10: `db` imported `http` (for `http.RegisterCustomer`) and the interface lived in `http`. Now both `http` and `db` import `app` and neither imports the other; `app` owns the repository interface. That removed the `db → http` dependency and the duplicated `addressfromOpenAPIToShared`, because the repo receives `app.Customer` with `shared.Address` already mapped.</details>

4. `customer_repo.go:36` calls `queries.InsertCustomer(ctx, args)` and then `return nil`. What does the HTTP client see when the insert fails, and why won't `TestRegisterCustomer` necessarily catch it?
   <details><summary>Answer</summary>The error is discarded, so the repo returns `nil`, the service returns `nil`, and the handler returns 201 with a UUID that was never persisted. The test only covers the success path: a working DB means the insert succeeds. It would fail only on the follow-up `GetCustomerByUUID` if the row is missing, and only if the failure reproduces in the test.</details>

5. The page's `sqlc.yaml` guidance is a column override for `orders.customers.customer_uuid`. What goes wrong with the committed `db_type: "uuid"` → `app.CustomerUUID` change?
   <details><summary>Answer</summary>`db_type` applies to every non-null `uuid` column, so the next module's `restaurant_uuid` column would be generated as `app.CustomerUUID`. The nullable override points at `app.NullCustomerUUID`, which doesn't exist, so the first nullable `uuid` column makes `dbmodels` fail to compile. The page says a column override takes precedence over `db_type`, which is why it's the safer shape.</details>

6. With the `ModulesContract` empty today, why must the eventual argument be `m.modules` (a `*contracts.Contracts`) rather than a copy?
   <details><summary>Answer</summary>`svc.go` creates `&contracts.Contracts{}` and each module fills its own fields in `RegisterContracts`, which runs *after* `Init`. A copy taken in `Init` would hold nil contracts forever; the pointer sees them once registered.</details>

*(These questions, with answers, are also appended to `rpNotes/review_queue.md` under today's date.)*

## 7. Cheat Sheet

- **Layering:** `http → app ← db`; `app → common, shared`. Handlers and repos map to/from `app` types. `app` imports no adapter or API package.
- **Where interfaces live:** with the consumer. `CustomerRepository` is in `app` because the service calls it. The service is a concrete `*app.Service` in the handler for now.
- **Mapping:** handler `http → app.Customer`; repo `app.Customer → dbmodels.InsertCustomerParams`. Test that every field is carried.
- **Typed IDs:** `type XUUID struct{ common.UUID }`. Never `=`, never `type X common.UUID`.
- **Codegen:** `x-go-type` + `x-go-type-import` in `openapi.yaml` → generated alias to `app.XUUID`. In `sqlc.yaml`, use a **column** override per ID, never edit the global `db_type` default.
- **Validation:** validate at boundaries (HTTP, external APIs). Trust our own DB reads. New IDs via `NewUUIDv7()`.
- **Gotchas:**
  - `Equals(common.UUID)` and explicit conversions bypass the type.
  - Named struct literals zero-initialize forgotten fields; positional ones catch a missing field but not a swapped pair of equal types.
  - `app.NullCustomerUUID` doesn't exist; the repo drops the `InsertCustomer` error; `module.go:24` comment is stale; `go vet -tags integration` fails on `customer_repo_test.go:34`.
- **Decision rules:**
  - Application layer: use it with more than one entry point or non-trivial logic; skip it for a single-endpoint CRUD service.
  - Embedding vs `type X Y`: embed when the underlying type's methods matter (marshaling, scanning).
  - Column vs `db_type` override: column per ID; `db_type` only for a genuinely global mapping.
