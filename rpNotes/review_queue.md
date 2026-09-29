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
