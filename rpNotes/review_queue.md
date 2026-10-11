# Review Queue

Spaced-repetition queue. Answer from memory before checking `<details>`; delete a question once you've nailed it 3 times.

## Initial Project Setup — Taskfile & Docker Compose (2026-09-26)

1. Why does `docker-compose.yaml` use `postgres` as the hostname in `backend`'s `POSTGRES_URL`, but `Taskfile.yml`'s `pgcli` task uses `localhost`?
   <details><summary>Answer</summary>They're resolved from two different network namespaces. Inside the `backend` container, Docker Compose's internal DNS resolves service names, so `postgres` reaches the Postgres container directly. `pgcli` in the `Taskfile.yml` task runs on the host machine (not inside a container), where the only route to the exposed Postgres port is via `localhost:5432`, thanks to the `ports: ["5432:5432"]` mapping.</details>

2. What would happen if `depends_on.postgres.condition` were changed from `service_healthy` to the default (just `depends_on: [postgres]`)?
   <details><summary>Answer</summary>The backend container would start as soon as the Postgres *container* starts, not when Postgres is actually ready to accept connections. Since Postgres takes a moment to initialize before `pg_isready` succeeds, the backend could attempt a DB connection during that window and fail — an intermittent race rather than a deterministic failure, which is worse to debug.</details>

3. If you deleted the `go_pkg` and `go_cache` volumes from `docker-compose.yaml` (and the corresponding mounts), what would you observe on the next few `reflex`-triggered reruns, and why?
   <details><summary>Answer</summary>Each `go run ./cmd/` invocation would potentially need to re-resolve/rebuild dependencies from scratch instead of reusing a persisted module and build cache, since that cache would no longer survive between container restarts — it would live only inside the ephemeral container filesystem. You'd see slower reload cycles, directly undermining the fast-feedback goal the page opens with.</details>

4. Why does `reflex.conf` explicitly exclude `_test\.go$` files from its watch pattern instead of just watching `\.go$` for everything?
   <details><summary>Answer</summary>Because saving a test file doesn't change the running server's behavior (`go run ./cmd/` doesn't execute tests) — reloading on a test-file save would restart the process for no behavioral reason, just wasted iteration time. The exclude is a deliberate optimization matching the "don't restart unless it matters" goal of live reload.</details>

5. What is the practical difference between `task down` and `task down-volumes`, and which one would you use to reset your local Postgres state versus just stopping the stack for the night?
   <details><summary>Answer</summary>`task down` runs `docker compose down` — stops and removes containers/networks but leaves named volumes (`go_pkg`, `go_cache`, and Postgres's own data volume) intact. `task down-volumes` runs `docker compose down -v`, which additionally deletes named volumes — including your Postgres data. Use plain `down` to just stop for the night (keep your data); use `down-volumes` only when you deliberately want a clean-slate database.</details>

## Service Scaffolding — Modular Monolith, Module Interface & Contracts (2026-09-26)

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

## Logging — Structured Logs, Context-Based Logging, Correlation IDs (2026-09-27, revised against real code)

1. Why is passing a logger as an explicit function parameter through every layer worse than storing it in `context.Context`, even though both technically make the logger available?
   <details><summary>Answer</summary>Because most of the call chain between "middleware that has the logger" and "the function that eventually logs" doesn't care about logging at all — passing it as a parameter forces every intermediate function signature to carry a dependency it doesn't use. `context.Context` is already threaded through most functions by default, so storing the logger there avoids cluttering signatures that have nothing to do with logging.</details>

2. What would happen if `FromContext` did NOT fall back to `slog.Default()` when no logger is found in the context?
   <details><summary>Answer</summary>Any code path that reached a context without a logger attached (e.g., a background job, a test, or a middleware ordering bug) would get a nil logger and likely panic on the first `.Info(...)`/`.Error(...)` call, rather than degrading gracefully to a default logger. The fallback is explicitly there so a missing/misordered wiring step fails silently (produces logs without expected attributes) instead of crashing the app.</details>

3. Why does `CorrelationIDFromContext` prefix a generated fallback ID with `gen_` instead of just generating a normal-looking ID?
   <details><summary>Answer</summary>Because a correlation ID that had to be generated (rather than propagated from an incoming request or the correlation-ID middleware) means something upstream isn't forwarding it correctly — that's a bug worth knowing about. The `gen_` prefix, per the code's own comment, exists specifically "to detect if correlation ID was not passed properly" — it makes that failure mode immediately greppable in logs.</details>

4. `backend/common/log/ctx.go` defines both `loggerKey` and `correlationIDKey` as unexported `ctxKey` constants rather than plain strings. What specifically would go wrong if both were replaced with the string `"logger"` and `"correlation_id"`?
   <details><summary>Answer</summary>`context.WithValue` keys are compared by type and value together. Two unrelated packages both using the plain string `"logger"` as a key could silently read or overwrite each other's context values — and here, since both keys would be plain strings, a bug that accidentally stored a correlation ID string under the "logger" key (or vice versa) would compile fine and only fail at runtime as a bad type assertion. The unexported `ctxKey` type prevents any package outside `log` from ever constructing a colliding key.</details>

5. The request-logging middleware wraps the response writer in a `bodyCapturingWriter` that also implements `Flush` and `Hijack`, not just `Write`. What would break if it only implemented `Write`/`WriteHeader`?
   <details><summary>Answer</summary>Code elsewhere (Echo internals, or a handler) that type-asserts the response writer to `http.Flusher` (to stream partial output) or uses `http.NewResponseController(...).Hijack()` (to take over the raw connection, e.g. for WebSockets) would find those interfaces unsatisfied by the wrapper and either silently lose streaming behavior or fail outright. Implementing `Flush`/`Hijack`/`Unwrap` on the wrapper preserves those capabilities through the body-capturing layer.</details>

6. Why does the correlation-ID middleware build its first logger with the package-level `slog.With(...)` instead of `log.FromContext(ctx).With(...)`?
   <details><summary>Answer</summary>At the point this middleware runs, nothing has stored a logger in the request's context yet — this middleware *is* the thing that will do that. Calling `log.FromContext(ctx)` here would just fall back to `slog.Default()` anyway (since context is still empty), so the code goes directly to `slog.With(...)` off the global default, then stores the resulting logger into context via `log.ToContext` for every later piece of code (including the request-logging middleware and handlers) to retrieve.</details>

## HTTP Handler — OpenAPI-First Design with oapi-codegen (2026-09-27)

1. Why does defining the OpenAPI spec first (rather than writing Go handler code first) make a breaking API change show up as a compile error instead of a runtime bug?
   <details><summary>Answer</summary>Because the Go types and interfaces (`StrictServerInterface`, the response types, the request/response models) are all generated *from* the spec. Changing the spec and regenerating means every place in the code that depended on the old shape now has a type mismatch the compiler catches immediately — there's no way for stale, hand-written code to silently keep working against a contract that no longer matches the spec.</details>

2. Why does `RegisterCustomerResponseObject` being an interface (rather than, say, a single struct with optional fields) enforce that only status codes the spec actually declares can be returned?
   <details><summary>Answer</summary>`oapi-codegen` generates exactly one concrete response type per declared status code (`RegisterCustomer201JSONResponse`, `400...`, `409...`), and only those generated types implement the `RegisterCustomerResponseObject` interface. Since Go requires a concrete type to satisfy an interface to be returned as it, there is no way to construct or return a response shape for a status code (like 204) that wasn't declared in the spec — the interface only has as many implementations as the spec has responses.</details>

3. What's the practical difference between implementing `ServerInterface` versus `StrictServerInterface`, and why does the page recommend the strict one?
   <details><summary>Answer</summary>`ServerInterface` hands the implementer a raw `echo.Context`, leaving JSON parsing, validation, and response encoding to hand-written code — the same boilerplate OpenAPI-first design is meant to eliminate. `StrictServerInterface` gives typed request and response objects instead, with `oapi-codegen`-generated code handling parsing/encoding, plus the compile-time safety on response types described above. The page recommends it because it's less boilerplate and catches more mistakes at compile time.</details>

4. Why does the project use `go tool oapi-codegen` instead of requiring a separately-installed global `oapi-codegen` binary?
   <details><summary>Answer</summary>`go tool` (Go 1.24+) runs a tool that's declared as a dependency directly in `go.mod`'s `tool (...)` block, pinned to a specific version like any other dependency. This means every contributor (and CI) runs the exact same generator version automatically, without a separate manual install step that could drift out of sync with what the project expects.</details>

5. If you hand-edited `openapi.gen.go` to fix something quickly, what would happen the next time someone ran `task gen`?
   <details><summary>Answer</summary>The generator has no awareness of manual edits — it regenerates the entire file fresh from `openapi.yaml` and `oapi-codegen.yaml` every time, overwriting whatever was there before. Any hand-edit would be silently destroyed on the next generation run, which is exactly why generated files carry a "never edit this file" convention: fix the spec or config instead, and regenerate.</details>

6. The current `Register` function in `handler.go` returns `nil` and does nothing. Once implemented per the page, why does it need `NewStrictHandler(handler, nil)` wrapping `handler`, rather than passing `handler` directly to `RegisterHandlers`?
   <details><summary>Answer</summary>`handler` (once it implements `RegisterCustomer`) satisfies the *strict* interface (`StrictServerInterface`), not the plain generated `ServerInterface` that `RegisterHandlers` expects to attach routes for. `NewStrictHandler` is the generated adapter that wraps a strict implementation and produces something satisfying the plain interface instead — without it, passing a strict-interface implementation directly to `RegisterHandlers` wouldn't type-check.</details>

7. `handler.go` currently defines `RegisterCustomer` with a pointer receiver (`func (h *Handler) RegisterCustomer(...)`), while `Register`'s parameter is `handler Handler` (a value) and `orders/module.go` stores the handler as a plain `Handler` value too. What would happen if you tried to write `NewStrictHandler(handler, nil)` inside `Register` exactly as it's typed right now?
   <details><summary>Answer</summary>It would fail to compile. Go method sets mean a value of type `Handler` does not include methods declared with a pointer receiver (`*Handler`) — only an actual `*Handler` value has `RegisterCustomer` in its method set. Since `NewStrictHandler` requires its argument to satisfy `StrictServerInterface`, and `handler Handler` (a value) doesn't, the compiler would reject the call. Either the receiver needs to change to match how the value is passed around, or the value needs to become a pointer everywhere it flows (the module field, `Register`'s parameter, and the wiring call).</details>

## Custom OpenAPI Types — x-go-type, UUID v7, Shared Types, the Enum Pattern (2026-09-29)

1. Why does `x-go-type` not change what the OpenAPI spec documents as the field's type, even though it changes the generated Go type?
   <details><summary>Answer</summary>`x-go-type` and `x-go-type-import` are oapi-codegen-specific extension fields that only affect what Go code gets generated — the schema itself still declares `type: string` (with whatever `format`), which is what any other OpenAPI-consuming tool (documentation generators, other language clients) actually sees. The override is purely an implementation-detail instruction to the Go code generator, not a change to the documented contract.</details>

2. Why does `common.UUID` exist as `[16]byte` instead of the code just using `google/uuid.UUID` (which has the identical underlying layout) directly everywhere?
   <details><summary>Answer</summary>Using `google/uuid.UUID` directly for every UUID-typed field would mean the compiler can't distinguish a customer UUID from an order UUID (or any other UUID) — they'd all be the same type, so passing the wrong one to the wrong function would compile fine and fail at runtime. A distinct named type (`common.UUID`) lets the compiler catch that class of mistake, even though the underlying bytes are identical to `google/uuid.UUID`.</details>

3. Why does UUID v7 specifically help database insert performance, when UUID v4 is equally valid as a unique identifier?
   <details><summary>Answer</summary>The problem isn't uniqueness — v4 is just as unique as v7. It's insert *locality*: a database's primary-key index is a B-tree ordered by key value. Random v4 values land all over the tree, so as the table grows, more of those tree pages fall out of memory and each insert may require a disk read to locate the right spot. UUID v7 embeds a timestamp in its first 48 bits, so new IDs are always numerically larger than old ones — inserts always append to the end of the index (like an auto-increment integer would), avoiding the scattered-page problem entirely.</details>

4. According to the page's own guidance, why would a 15-field `CustomerProfile` struct be a bad candidate for `common/shared`, while a `UUID` or `CountryCode` type is a good one?
   <details><summary>Answer</summary>`common/shared` types create coupling between every module that uses them — any change requires involving every consuming module's owning team. A `UUID` or `CountryCode` is tiny, stable, and universal (unlikely to ever need a breaking change), so that coupling cost is low. A `CustomerProfile` with 15 fields is a full entity owned by one specific module; changes to it are exactly the kind of frequent, module-specific evolution that would turn `common/shared` into a cross-team bottleneck if that entity lived there instead of inside its owning module.</details>

5. Why is `Enum[T]`'s `value` field unexported, and what would break about the pattern's guarantee if it were exported instead?
   <details><summary>Answer</summary>Being unexported means no code outside the `common` package can construct or mutate a non-zero `Enum[T]` except by going through methods like `UnmarshalText`/`Scan`, both of which validate against `T.Values()` first. If `value` were exported, any code anywhere could write `Enum[CountryCodeType]{value: "XX"}` directly, bypassing validation entirely — the type would no longer guarantee that every in-memory instance holds a valid value, defeating the entire point of the pattern.</details>

6. Why does `UnmarshalText` treat an empty input as always valid, regardless of what `Values()` returns?
   <details><summary>Answer</summary>Per the code (and confirmed by `enum_test.go`'s `empty_string` test case), the empty-string check runs before consulting `Values()` at all — it's a deliberate carve-out for the zero value, letting a `CountryCode{}` exist as "not yet set" without that state counting as invalid. If empty had to appear in `Values()` to be accepted, every enum type would need to explicitly list "" as a valid value just to support its own zero state, which the pattern avoids by special-casing it once, centrally.</details>

## Gitattributes & Generated Files (2026-09-30)

1. Why doesn't `.gitattributes`' `linguist-generated=true` prevent the merge conflict scenario the page opens with?
   <details><summary>Answer</summary>`linguist-generated=true` is read by GitHub/GitLab's UI tooling to change how a file is *displayed* during review (collapsed by default, excluded from language stats) — it says nothing to Git's actual merge algorithm. Git still performs a normal line-based merge on the file's content regardless of this attribute, so two branches with different regenerated output still produce real conflict markers. The attribute is a review-noise fix, not a merge-conflict fix.</details>

2. When a merge conflict appears inside a `.gen.go` file, why is hand-resolving the conflict markers the wrong move, even if the result compiles?
   <details><summary>Answer</summary>The generated file's content is a deterministic function of the (already hand-written, small) spec files. A manually resolved version might compile and even look correct, but if it doesn't exactly match what running the generator against the merged spec would produce, it has silently drifted from its source of truth. The next time anyone regenerates the file for an unrelated reason, that manual fix gets silently overwritten — so the correct workflow is merging the spec, regenerating, and committing the fresh output instead.</details>

3. What does the glob pattern `**/**.gen.go` do differently from a plain `*.gen.go` in `.gitattributes`?
   <details><summary>Answer</summary>`*.gen.go` alone would only match `.gen.go` files sitting directly in whatever directory the pattern is evaluated relative to (typically the repo root). `**/**.gen.go` matches at any directory depth, which is necessary here since the real generated file (`backend/orders/api/http/openapi.gen.go`) is nested three directories deep, not in the repo root.</details>

4. According to the page, what three costs does the `.gitignore` + regenerate-in-CI alternative impose, that committing generated files avoids?
   <details><summary>Answer</summary>It complicates developer experience (you must always remember to run the generator before building locally, since the file won't exist otherwise), it makes CI jobs slower (regeneration has to happen on every run instead of being a checked-out file), and it loses the ability to track how the generated code changes over time in git history, since the file is never actually committed.</details>

5. Why does asserting "no changes after running the generator in CI" (`git status --porcelain` is empty) only work reliably if CI uses the exact same tool version as local development?
   <details><summary>Answer</summary>If the generator version differs between CI and local dev, CI could produce a diff purely from version differences in output format/behavior — a false positive unrelated to anyone forgetting to regenerate — or, in the opposite case, mask a real drift if the differing version happens to produce similarly-shaped output. `go tool`'s `go.mod`-pinned version (covered in the HTTP Handler guide) removes this risk, since every environment resolves to the identical generator version automatically, making a detected diff a reliable signal of an actual missed regeneration.</details>

## Generate sqlc — Query Annotations, Type Overrides, Codegen Workflow (2026-09-30)

1. Why can `sqlc generate` run successfully with Postgres completely stopped, when the generated code is meant to run real queries against Postgres later?
   <details><summary>Answer</summary>`sqlc` infers the schema by statically parsing the migration files in the configured `schema` directory — it never connects to a live database to learn table/column shapes. The database is only needed later, at runtime, when the generated functions actually execute queries through `pgx/v5` against a real connection.</details>

2. Why does `column: "orders.customers.address"` exist as a separate override from the `db_type: "uuid"` override, instead of both being expressed the same way?
   <details><summary>Answer</summary>`db_type` overrides are global — every column of that database type gets the same Go type, appropriate for `uuid` because every UUID column really should map to `common.UUID` identically. `json` isn't like that: not every `json` column in the schema is necessarily an `Address`, so a blanket `db_type: "json" → shared.Address` override would be wrong for any future unrelated `json` column. `column` overrides target one exact `schema.table.column`, avoiding that false generalization.</details>

3. The `uuid, nullable: true` override in `sqlc.yaml` points to `common.NullUUID`, a type that doesn't exist anywhere in `backend/common` yet. Why doesn't `sqlc generate` already fail because of this?
   <details><summary>Answer</summary>`orders.customers` (the only table defined so far) has no nullable `uuid` column, so this override entry is never actually resolved against real generated code — it sits dormant. It would only become a real failure the moment some query or schema change introduces a nullable `uuid` column that `sqlc` has to type using the still-missing `common.NullUUID`.</details>

4. Why does the project reuse `common.UUID` directly between the OpenAPI-generated HTTP layer and the `sqlc`-generated database layer, but deliberately keep `shared.Address` separate from whatever HTTP-layer address type exists?
   <details><summary>Answer</summary>The page's stated tradeoff: shared types across layers work well for stable, universal concepts like UUID, where the type's meaning never really diverges between layers. `Address` is a more complex type whose HTTP-layer and database-layer shapes could reasonably evolve independently, so keeping them separate (with an explicit mapping step, deferred to a later exercise) avoids coupling those two layers together through one shared struct.</details>

5. What would happen if a contributor had a different globally-installed `sqlc` version than the one pinned in `go.mod`, and ran that global binary directly instead of `go tool sqlc generate`?
   <details><summary>Answer</summary>They could get subtly different generated output than what `go tool sqlc generate` (using the pinned version) would produce — a generator-version mismatch masquerading as a real code change, the same risk guide 6 already covered for `oapi-codegen`. Using `go tool` guarantees every contributor and CI resolve to the exact version declared in `go.mod`, removing this class of drift.</details>

6. Why does hand-editing a file under `dbmodels/` to quickly patch something "work" in the moment but still count as a mistake?
   <details><summary>Answer</summary>It compiles and may even look correct immediately, but the generated file's real source of truth is the annotated query plus the schema `sqlc` parsed it against — not the file's current on-disk content. The next time anyone runs `task gen`/`sqlc generate` for any unrelated reason, the hand-edit is silently overwritten with fresh output, exactly as guide 6 described for `openapi.gen.go`.</details>

## Insert Customer — Handler DB Wiring, Constructor DI, Address Mapping (2026-10-02)

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

## Repository Pattern — Implement + Integrate (2026-10-05)

1. Why can't the `CustomerRepository` interface live in the `db` package next to its implementation, even though that's where the method body actually is?
   <details><summary>Answer</summary>`db` already imports `http` (confirmed in `customer_repo.go`, for `http.RegisterCustomer` and `http.Address`). If the interface also lived in `db`, the `http` package would need to import `db` to reference that interface — creating a two-way import cycle that fails to compile. Declaring the interface in `http` instead works because Go interfaces are satisfied implicitly: the `db` package's concrete type never needs to import or reference the interface at all.</details>

2. `customer_repo.go` already imports `http` for `http.RegisterCustomer` and `http.Address`. Why does it define its own `addressfromOpenAPIToShared` instead of calling `handler.go`'s version through that same import?
   <details><summary>Answer</summary>`handler.go`'s `addressfromOpenAPIToShared` is unexported (lowercase), so importing the `http` package doesn't grant access to it — unexported identifiers are only visible within their own package. The import cycle concern doesn't apply here, since this would be a plain function call, not an interface; the actual blocker is Go's visibility rules, which is why the current code has two separate copies instead of one shared one.</details>

3. Why does the page recommend against unit-testing the thin `RegisterCustomer` handler with a mocked `CustomerRepository`, even though the interface makes mocking trivial?
   <details><summary>Answer</summary>For a handler this thin (generate a UUID, call one repository method, return), a mock-based test mostly re-asserts "the handler calls the method with these arguments" — it breaks on signature changes and catches few real bugs, so its maintenance cost rarely pays for itself. The page's guidance: extract real logic into helper functions and test those directly, and rely on later component tests for full-stack coverage.</details>

4. Why does `customer_repo_test.go` use `cmpopts.EquateComparable(shared.SharedTypes...)` instead of calling `cmp.Diff` with no options?
   <details><summary>Answer</summary>Types like `CountryCode` embed `Enum[T]`, which carries unexported fields. `cmp.Diff` panics by default when it encounters unexported fields inside a struct it's asked to compare. `EquateComparable` tells it to use Go's `==` operator for the listed types instead of reflecting into their internals — `shared.SharedTypes` is the explicit list of types that need this treatment.</details>

5. Guide 9 argued that duplicating `Address` as both `shared.Address` and `http.Address` is often "the right call." Does that same argument justify `customer_repo.go` and `handler.go` each having their own copy of `addressfromOpenAPIToShared`?
   <details><summary>Answer</summary>Not automatically. Guide 9's argument was about *types* that might reasonably evolve differently across layers (HTTP contract vs. storage shape). The *conversion function* between two already-separate types doesn't carry that same justification — exporting one copy and importing it (plain function import, not an interface) costs nothing and carries no cycle risk, so keeping two identical copies in sync by hand is closer to accidental duplication than the deliberate type-separation the earlier page endorsed.</details>

6. Given the current workspace state, why would adding `db.NewCustomerRepository(m.pgxDb)` wiring to `module.go` right now not actually fix anything by itself?
   <details><summary>Answer</summary>`handler.go`'s `Handler` struct still only has a `db *pgxpool.Pool` field and no `CustomerRepository` interface field to receive it — there's nothing in the `http` package yet for the repository to be injected into. Wiring the repository into `module.go` without first adding the interface and the field to `Handler` (the "Integrate Repository" work) would just produce a constructed repository with nowhere to go.</details>


## Application Layer + Dedicated UUIDs (2026-10-08)

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


## Error Handling — Transport-Agnostic Errors, Slugs, common.Error (2026-10-08)

1. Why must `RegisterCustomer` in `app` return `common.NewInvalidInputError(...)` rather than `echo.NewHTTPError(400, ...)`?
   <details><summary>Answer</summary>`app` can be called by a gRPC handler, a message consumer or a CLI, none of which have an HTTP response to set. An HTTP error returned from there is meaningless and misleading in logs. `app` says *what* went wrong; each API layer translates to its own protocol.</details>

2. What does the client receive for (a) a `common.Error`, (b) an `*echo.HTTPError` with code 404, (c) `errors.New("pg: connection refused")`?
   <details><summary>Answer</summary>(a) The error's own status, `PublicError` and `ErrorSlug`, plus `details` if any. `InternalError` is never sent. (b) Status 404, message `Not Found`, slug `not_found`. (c) Status 500, `Internal Server Error`, slug `internal_server_error`. The cause is only in the log line.</details>

3. `errors.As(err, &commonErr)` uses a value-typed `Error`. What happens if a function returns `&common.Error{...}`, and why does it compile?
   <details><summary>Answer</summary>It compiles because a value-receiver `Error()` is in the pointer's method set, so `*Error` also implements `error`. But `errors.As` with a value target `Error` doesn't match a `*Error`, so the handler falls through to a generic 500 and the slug and status are lost.</details>

4. `err := common.NewInvalidInputError(...).WithInternalError(pgErr)`. Why does `errors.As(err, &pgErr2)` fail, and what are two ways around it?
   <details><summary>Answer</summary>`common.Error` has no `Unwrap()` method, so `InternalError` isn't part of the error chain. Either inspect/classify the underlying error before wrapping it (decide the `common.Error` kind from it), or add an `Unwrap() error` that returns `InternalError`.</details>

5. `go test ./backend/common/...` passes before you've done anything. Why doesn't that prove errors work end-to-end, and what test would?
   <details><summary>Answer</summary>The tests call `common.EchoErrorHandler` directly. `NewEcho` in `common/http/echo.go:19` still installs the old `HandleError`, so production behavior is unchanged. A test that builds the server via `NewEcho()` (or hits the real route with `httptest`) and asserts the response slug would fail until the wiring line is changed.</details>

6. A request has `"country_code": "XX"`. Walk the error from `UnmarshalText` to the response body.
   <details><summary>Answer</summary>`Enum.UnmarshalText` returns a `common.Error` (slug `invalid-enum-value`). The strict handler's `ctx.Bind` returns it; echo's `BindBody` wraps non-`HTTPError` errors in `NewHTTPError(400, err.Error()).SetInternal(err)`. `EchoErrorHandler` finds the `*echo.HTTPError` (status 400), then `errors.As` reaches the `common.Error` through `HTTPError.Unwrap()` and overrides message and slug. The client gets 400 with slug `invalid-enum-value`.</details>


## Error Handling — Deep Dive (2026-10-11)

1. Go has no exceptions. What exactly makes Echo call `HTTPErrorHandler`, and where in Echo's source?
   <details><summary>Answer</summary>`ServeHTTP` runs the middleware chain plus handler as `h(c)`; if it returns a non-nil error, `echo.go:676-677` calls `e.HTTPErrorHandler(err, c)`. The error is an ordinary return value checked once at the top.</details>

2. After `errors.As(err, &httpErr)` succeeds, what happened to the blank `&echo.HTTPError{}` you created on the line before?
   <details><summary>Answer</summary>Nothing fills it. `errors.As` overwrites the pointer variable `httpErr` so it points at the `*echo.HTTPError` found in the chain; the blank struct is discarded. Verified by running: `same pointer as the blank struct? false`, and `Code` goes from 0 to 404.</details>

3. Why does `Error()` on `common.Error` include `InternalError` while the JSON response doesn't?
   <details><summary>Answer</summary>An error has two audiences. The log gets the whole struct (operator: slug, internal cause, details); the response is a projection (`HttpErrorResponse`) that omits the cause (client). The handler logs `err` at `errors_echo.go:20` but builds the response from selected fields.</details>

4. A user reports a 500. How do you find the cause, and what is misleading about the request log for that call?
   <details><summary>Answer</summary>Ask for the `Correlation-ID` response header and grep the logs for it; the `Handling HTTP error` line carries the full error. The "Request done" line is written before the error handler runs, so for failed requests it shows `status=200` and an empty `response_body` (verified by running), so don't trust that field for failures.</details>
