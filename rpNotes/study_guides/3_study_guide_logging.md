# Study Guide: Logging — Structured Logs, Context-Based Logging, Correlation IDs

> **Page:** "Logging" (last walkthrough exercise before hands-on coding resumes — no code to write yet)
> **Track:** backend-masterclass-beta, module `02-project-setup`, exercise `03-logging`
> **Previous guide:** `2_study_guide_service_scaffolding_modular_monolith.md` — that page set up the `Module` interface, contracts, and Echo wiring. This page adds the observability layer (logging) on top of that same Echo/middleware stack.
> **What this likely unlocks next:** actual feature code (handlers, services) that will rely on `log.FromContext(ctx)` everywhere instead of ad hoc logging.
>
> **Update note:** This guide was first written before the exercise advanced, when `backend/common/log/` didn't exist yet in the workspace — that version was built entirely from the page's quoted snippets, tagged `[From page]`. The exercise has since advanced to `03-logging` and the real code is now present. This revision re-verifies every claim against the actual files (`ctx.go`, `log.go`, `correlation.go`, `slog.go`, `common/echo.go`, `common/http/middlewares.go`, `common/http/echo.go`, `common/http/truncate.go`, `svc.go`, `cmd/main.go`) and corrects one place where the page's description and the real code diverge (see the Building Blocks concept below).

---

## 1. Snapshot

- **Topic:** Why unstructured logs (`fmt.Println`) don't scale past one service or one request; how `log/slog` (structured logging), context-based logger propagation, correlation IDs, and HTTP middleware combine to make every log line traceable back to one request.
- **Objective:** After this page, I can explain what problem a correlation ID solves that structured logging alone doesn't, trace how a logger gets from middleware into a deeply nested function without being passed as a parameter, and explain why `HandleError` only needs one `log.FromContext(...).Error(...)` call to get full request context in every error log.
- **Continuity:** This builds directly on the Echo/middleware/`common` package wiring from the Service Scaffolding guide — it's the same `common/http` package, now carrying logging concerns instead of just routing.

## 2. Concepts

### Structured Logging with `log/slog`

- **Problem:** [From page] A plain-text log line like `"error occurred while processing order"` tells you *something* went wrong, not *what* failed, *which* order, or *which* request — and at scale (the page's example: "50,000 lines from five different services, all mixed together"), unstructured text is effectively unsearchable.
- **Mechanism:** [From code, `backend/common/log/slog.go`] confirms the page's snippet exactly:

  ```go
  func Init(level slog.Level) {
      opts := &humanslog.Options{
          HandlerOptions: &slog.HandlerOptions{Level: level},
          TimeFormat:     "[15:04:05.000]",
      }
      logger := slog.New(humanslog.NewHandler(os.Stderr, opts))
      slog.SetDefault(logger)
  }
  ```
  It's called once, at process startup, in `cmd/main.go`: `log.Init(slog.LevelInfo)`, before anything else (DB pool, service construction). `humanslog` is a real dependency (`github.com/ThreeDotsLabs/humanslog v0.1.0` in `go.mod`), producing colored, human-readable output for local dev; the page states production would swap in a JSON handler instead.

  [From code, `backend/common/echo.go`] There's a second, separate logging path worth knowing about: `EchoSlogAdapter`. Echo's own `Logger` interface (`Output`, `SetOutput`, `Print`, `Debug`, `Info`, `Warn`, `Error`, `Fatal`, `Panic`, level variants, etc.) has nothing to do with `slog.Logger`'s API, so this project wrote a full adapter implementing every method of Echo's interface by delegating to an internal `*slog.Logger`:

  ```go
  func NewEchoSlogAdapter(logger *slog.Logger) *EchoSlogAdapter {
      return &EchoSlogAdapter{logger: logger, level: log.INFO, output: os.Stdout}
  }
  func (e *EchoSlogAdapter) Info(i ...interface{}) {
      if !e.shouldLog(log.INFO) { return }
      e.logger.Info(e.logWithPrefix(fmt.Sprint(i...)))
  }
  // ... Print/Debug/Warn/Error/Fatal/Panic, each with a `j` (JSON-payload) variant
  ```
  It's wired in via `e.Logger = common.NewEchoSlogAdapter(slog.Default())` inside `NewEcho()` — so Echo's *own* internal log calls (route registration messages, panics recovered by `middleware.Recover()`, etc.) go through the same `slog` pipeline as everything else, instead of Echo's default logger writing to a separate stream.
- **Design Decision:** [From page] The alternative is `fmt.Println`/plain text. [AI explanation] Human-readable output (`humanslog`) is explicitly local-dev-only — production would need a JSON handler for aggregation, which is why `Init` takes a `level` parameter but not a handler choice; swapping handlers would be a code change, not a config flag, as written today.
- **In Production:** [From page] The scenario the page opens with — a failed order, a generic error message, 50,000 interleaved log lines — is the failure mode structured logging fixes: filter by `order_id`/`correlation_id` directly instead of grepping raw text. [From code] Note that `log.Init(slog.LevelInfo)` is hardcoded in `main.go` — `Debug`-level calls elsewhere in the codebase (see `svc.go` below) are silently dropped unless this is changed, since `Info` is the configured floor.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Logging via `fmt.Println`/`log.Printf` alongside `slog` calls | Two log formats mixed in the same stream, one of them unparseable by aggregation tooling | Old habits, or a quick debug print left in | Route everything through `slog` (or the context-bound logger) so every line is structured |
  | Assuming `Debug`-level logs (like `svc.go`'s "Initialized module" line) will show up locally | They're silently dropped — `log.Init(slog.LevelInfo)` in `main.go` filters anything below `Info` | The level is set once, far from the call site, and easy to forget | Check `cmd/main.go`'s `log.Init(...)` argument before assuming a `Debug` log will appear |

### Context-Based Logging

- **Problem:** [From page] Once you have structured logs, you still need every log line from one request grouped together. Passing a logger as an explicit function parameter "clutters every function signature" and forces uninterested layers to thread it through.
- **Mechanism:** [From code, `backend/common/log/log.go` — matches the page exactly]:

  ```go
  func FromContext(ctx context.Context) *slog.Logger {
      log, ok := ctx.Value(loggerKey).(*slog.Logger)
      if ok {
          return log
      }
      return slog.Default()
  }

  func ToContext(ctx context.Context, logger *slog.Logger) context.Context {
      return context.WithValue(ctx, loggerKey, logger)
  }
  ```
  `FromContext`'s fallback to `slog.Default()` means a missing logger degrades gracefully rather than panicking. This is what lets `HandleError` (`backend/common/http/echo.go`, confirmed in code) do all its error logging in one line:

  ```go
  func HandleError(err error, c echo.Context) {
      log.FromContext(c.Request().Context()).With("error", err).Error("HTTP error")
      // ... status code / JSON response logic unchanged from Service Scaffolding
  }
  ```
  and lets `svc.go`'s shutdown path do the same: `log.FromContext(ctx).Error("shutting down http server failed")` if `echoRouter.Shutdown(...)` returns an error.
- **Design Decision:** [From page] The named alternative is implementing `slog`'s `Handler` interface to extract attributes from context *at log time* (so you'd call `slog.InfoContext(ctx, "msg")` directly). The page states this project chose storing-the-logger-in-context because "it's more straightforward and works well for this project."
- **In Production:** [From code, `backend/common/log/ctx.go`]:

  ```go
  type ctxKey int
  const (
      loggerKey        ctxKey = iota
      correlationIDKey ctxKey = iota
  )
  ```
  Confirms the page's claim about an unexported key type — here it's `int`-backed with `iota`, giving two distinct unexported constants (`loggerKey`, `correlationIDKey`) so the logger and the correlation ID are stored under separate, collision-proof keys, both invisible to any package outside `log`.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Passing a logger as an explicit function parameter through layers that don't use it | Every function signature down the call chain gets cluttered with a parameter it doesn't need | Feels more "explicit" than context-based passing | Rely on `context.Context`, already threaded through, via `FromContext`/`ToContext` |
  | Using a plain/exported type (or shared string) as a context key | Two unrelated packages can collide on the same key and overwrite each other's values | String keys are the obvious first choice | Define an unexported key type per package, as `ctx.go` does with `ctxKey` |

### Correlation ID

- **Problem:** [From page] A single user action can touch multiple services; reconstructing the full story of one request by reading each service's logs separately is "pretty much impossible."
- **Mechanism:** [From code, `backend/common/log/correlation.go` — matches the page, plus two details the page didn't show]:

  ```go
  func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
      return context.WithValue(ctx, correlationIDKey, correlationID)
  }

  func CorrelationIDFromContext(ctx context.Context) string {
      v, ok := ctx.Value(correlationIDKey).(string)
      if ok {
          return v
      }
      FromContext(ctx).Warn("correlation ID not found in context")
      // add "gen_" prefix to distinguish generated correlation IDs from correlation IDs passed by the client
      // it's useful to detect if correlation ID was not passed properly
      return "gen_" + shortuuid.New()
  }
  ```
  `shortuuid` is `github.com/lithammer/shortuuid/v3` (confirmed in `go.mod`) — note this is a *different* import path than the page's prose mentioned ("uses shortuuid"), but the same library family; the page didn't specify the exact module path. The `gen_` prefix comment in the code states the same rationale the page gives in prose: distinguishing generated IDs from client-propagated ones.

  [From code, `backend/common/http/middlewares.go`] The actual generation site is the correlation-ID middleware, which uses `shortuuid.New()` directly (not through `CorrelationIDFromContext`) when the incoming header is empty — `CorrelationIDFromContext` exists for *later* code that needs to read the ID back out of context (its own `gen_` fallback path is a second-line defense if something reads the correlation ID from a context that was never populated by the middleware at all).
- **Design Decision:** [From page] Correlation IDs are framed as the entry-level version of distributed tracing (OpenTelemetry trace/span IDs) — "a good starting point" for a monolith, with full tracing covered elsewhere for distributed systems.
- **In Production:** [From page + code] A `gen_`-prefixed ID in logs signals an upstream propagation bug. [From code] There's also a `test_name` attribute conditionally added to the logger (see Building Blocks below) — a training/testing-specific detail not mentioned on the page at all.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Not forwarding the `Correlation-ID` header on outgoing requests to other services | The cross-service trace breaks at that hop | Easy to forget on any new outgoing HTTP call | Always inject the current correlation ID into outgoing request headers |
  | Treating a `gen_`-prefixed ID as normal | Masks a real upstream propagation bug | The fallback is designed to be non-fatal | Treat any `gen_` ID as a bug report on your own propagation logic |

### Building Blocks: HTTP Middleware — and where the page's description and the code diverge

- **Problem:** [From page] Logging, correlation IDs, and request/response capture are cross-cutting concerns; implementing them per-handler would repeat boilerplate everywhere.
- **Mechanism:** [From code, `backend/common/http/middlewares.go`] The page describes **three** middlewares (correlation ID, request logger, body dump). The actual `useMiddlewares` registers only **two** custom ones (plus the pre-existing timeout/recover):

  ```go
  func useMiddlewares(e *echo.Echo) {
      e.Use(
          middleware.ContextTimeout(10*time.Second),
          middleware.Recover(),
          // Correlation-ID runs first: available in context for the request log middleware.
          func(next echo.HandlerFunc) echo.HandlerFunc {
              return func(c echo.Context) error {
                  req := c.Request()
                  ctx := req.Context()
                  reqCorrelationID := req.Header.Get(CorrelationIDHttpHeader)
                  if reqCorrelationID == "" {
                      reqCorrelationID = shortuuid.New()
                  }
                  logger := slog.With("correlation_id", reqCorrelationID)
                  if testName := c.Request().Header.Get("TestName"); testName != "" {
                      logger = logger.With("test_name", testName)
                  }
                  ctx = log.ToContext(ctx, logger)
                  ctx = log.ContextWithCorrelationID(ctx, reqCorrelationID)
                  c.SetRequest(req.WithContext(ctx))
                  c.Response().Header().Set(CorrelationIDHttpHeader, reqCorrelationID)
                  return next(c)
              }
          },
          requestLogMiddleware,
      )
  }
  ```
  **[AI explanation, correcting the page]:** what the page calls "the body dump middleware" is *not* a separate, third middleware — it's folded directly into `requestLogMiddleware` itself. That function reads and restores the request body, wraps the response writer in a `bodyCapturingWriter` (a `io.Writer`/`http.ResponseWriter` combo supporting `Flush`/`Hijack`/`Unwrap` so it doesn't break streaming or WebSocket upgrades), times the handler call, and only *then* builds the final log line with URI/status/method/duration plus truncated request/response bodies. So there are two registered middlewares doing three described jobs, not three middlewares one-to-one with the page's list — worth knowing if you go looking for a `bodyDumpMiddleware` function by name, because it doesn't exist as its own symbol.

  A second thing the page doesn't mention at all: body truncation (`backend/common/http/truncate.go`). `requestLogMiddleware` calls `truncateBodyForLog` on both bodies before logging them:

  ```go
  const (
      maxBodyLogBytes  = 512
      maxArrayLogItems = 6 // keep first 5 + last 1
  )
  ```
  For valid JSON, it recursively truncates any array longer than 6 items (keeping the first 5 + last item + a `"...": "N items truncated"` marker); for non-JSON or still-too-large JSON, it falls back to head+tail truncation with a byte-count marker in the middle. Response-body truncation is also conditionally skipped: `if isDebug := log.FromContext(ctx).Enabled(ctx, slog.LevelDebug); !isDebug { body = truncateBodyForLog(body) }` — at debug level, full untruncated response bodies are logged.
- **Design Decision:** [From page] The underlying principle (linked to "generic decorators" on the page) is keeping observability infrastructure out of business logic via the middleware pattern. [AI explanation] Folding body-capture into the same middleware as request logging (rather than a separate middleware) avoids needing to pass the captured-body buffers between two independent middleware closures — a real, if unstated, simplification tradeoff.
- **In Production:** [From code] Middleware order is enforced exactly as the inline comment states: `// Correlation-ID runs first: available in context for the request log middleware.` — confirmed by registration order in `e.Use(...)`.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Looking for a standalone "body dump middleware" function | You won't find one — it's inline in `requestLogMiddleware` | The page describes it as a separate concern, but the implementation merged it | Read `requestLogMiddleware` in full when you need to change body-capture behavior, not a separate file |
  | Assuming response bodies are always truncated at 512 bytes | At `Debug` log level, the full response body is logged untruncated | The truncation skip for debug level is easy to miss in a 40-line function | Check `log.Init`'s level before assuming truncation always applies |
  | Registering a new middleware before the correlation-ID one | Its own logs (and anything it logs from within) would be missing `correlation_id` | Middleware order isn't type-checked, only call-order in `e.Use(...)` | Always register after the correlation-ID middleware (which is why it's listed first, immediately after `Recover()`) |

## 3. Plumbing Dissection

**Wiring map (now verified against real files):**

```
cmd/main.go
  ├─ log.Init(slog.LevelInfo)                  → slog.SetDefault(humanslog-backed logger)
  └─ backend.New(ctx, dbPgx)                    (backend/svc.go)
        ├─ commonHTTP.NewEcho()                  (backend/common/http/echo.go)
        │     ├─ useMiddlewares(e)                → ContextTimeout, Recover, correlation-ID func, requestLogMiddleware
        │     ├─ e.HTTPErrorHandler = HandleError
        │     ├─ e.Logger = common.NewEchoSlogAdapter(slog.Default())
        │     └─ GET /health
        ├─ for each module: Init → RegisterContracts
        │     └─ log.FromContext(ctx).With("duration", ..., "module", ...).Debug("Initialized module")
        ├─ moduleContracts.Verify()
        └─ for each module: RegisterHttp
  └─ svc.Run(ctx, ":8080")
        ├─ goroutine: on ctx.Done() → echoRouter.Shutdown(context.Background())
        │     └─ on error: log.FromContext(ctx).Error("shutting down http server failed")
        └─ echoRouter.Start(":8080")

per-request flow
  Correlation-ID header (or none)
    └─ correlation-ID middleware: extract or shortuuid.New()
          ├─ logger := slog.With("correlation_id", id) [+ "test_name" if TestName header present]
          ├─ ctx = log.ToContext(ctx, logger); ctx = log.ContextWithCorrelationID(ctx, id)
          └─ response header Correlation-ID set to same id
    └─ requestLogMiddleware: read+restore request body, wrap response writer, time next(c)
    └─ handler code: log.FromContext(ctx) already has correlation_id (+ test_name) attached
    └─ requestLogMiddleware (after handler returns): log URI/status/method/duration/truncated bodies
    └─ on handler error: HandleError → log.FromContext(ctx).With("error", err).Error("HTTP error")
```

- **Per plumbing piece:**
  - `log.Init(slog.LevelInfo)` runs before `pgxpool.New` and `backend.New` in `main.go` — must happen first, since `slog.Default()` (read later by `NewEchoSlogAdapter` and any un-contexted `FromContext` fallback) would otherwise be the zero-value stdlib default instead of the `humanslog`-backed one.
  - `moduleContracts := &contracts.Contracts{}` and the per-module `Debug("Initialized module")` log — this Debug-level call is invisible at the current `Info` level setting; it exists for a stricter local level (e.g. flipping `main.go` to `slog.LevelDebug` during debugging), not for default runs.
  - `bodyCapturingWriter` implementing `Flush`, `Hijack`, and `Unwrap` in addition to `Write`/`WriteHeader` — without these, wrapping the response writer would silently break any handler relying on streaming (`Flush`) or protocol upgrades (`Hijack`), because Echo/net-http type-assert the underlying writer for these optional capabilities.
  - `c.Request().Body = io.NopCloser(bytes.NewBuffer(reqBody))` — restoring the body after reading it for logging; without this, the actual handler would see an already-drained, empty request body.
  - `err := s.echoRouter.Shutdown(context.Background())` — same design as the Service Scaffolding guide's note: uses a fresh context (not the already-cancelled `ctx`) so shutdown can actually wait for in-flight requests, now with an added `log.FromContext(ctx).Error(...)` if it fails.

- **Non-obvious lines:**
  - `logger := slog.With("correlation_id", reqCorrelationID)` in the correlation-ID middleware uses the **package-level** `slog.With` (i.e., off the global default logger set by `log.Init`), not `log.FromContext(ctx).With(...)` — at this point in the chain there's no logger in `ctx` yet (this middleware is what puts one there), so it has to start from the global default.
  - `if isDebug := log.FromContext(ctx).Enabled(ctx, slog.LevelDebug); !isDebug` — easy to misread as "if debug mode is on, truncate"; it's actually inverted: truncation is skipped (full body logged) only *when* debug is enabled.
  - `e.Logger = common.NewEchoSlogAdapter(slog.Default())` passes `slog.Default()` at `NewEcho()` call time — since `log.Init` already ran in `main.go` by then, this captures the `humanslog`-backed logger, not the stdlib zero-value one.

- **What a senior notices:**
  - The page's "three middlewares" framing is a simplification of the actual two-middleware, three-responsibility implementation — a reminder that page prose is a teaching simplification, and the real dissection (this section) is what to trust when the two disagree on structural detail.
  - `truncateBodyForLog`'s JSON-aware truncation (walking arrays specifically, not just byte-truncating) is a meaningfully more sophisticated design than "just cut the string at 512 bytes" — it preserves structure so truncated JSON logs are usually still valid JSON, which matters if log tooling parses them.
  - `TestNameHeader` (`"TestName"`) being read and attached as a `test_name` log attribute is very likely there to make the training's own automated test runs greppable/traceable in the same log stream as everything else — a detail entirely absent from the page, presumably because it's infrastructure for the training platform itself, not the lesson.

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a blank scratch Go module, rebuild the core logging plumbing verified in Section 3:
- A package-level `Init` that sets `slog.Default()` to a custom-formatted handler.
- A `ToContext`/`FromContext` pair using an unexported context-key type, with `FromContext` falling back to `slog.Default()`.
- A correlation-ID middleware that extracts/generates an ID, builds a logger with it attached (starting from the package-level default, since no request-scoped logger exists yet), and stores both the logger and the raw ID in context under separate keys.
- A request-logging middleware that captures and restores the request body, wraps the response writer to capture the response body (supporting at least `Flush`), and logs URI/status/method/duration plus both bodies — truncated past some byte threshold, except at debug level.

**Acceptance criteria:**
1. A handler deep in a call chain retrieves the request's logger via `FromContext(ctx)` without it being passed as a parameter.
2. `FromContext` on a context that never went through your middleware returns a working logger (the fallback), not a panic.
3. A request with an incoming correlation-ID header preserves that exact ID through to the handler's logger; one without gets a freshly generated, distinctly-prefixed ID plus a warning log.
4. Your response-writer wrapper doesn't break a streaming handler — write a handler that calls `Flush()` mid-response and confirm the client still receives incremental writes, not one buffered blob at the end.
5. (Failure/behavior-toggle case) At your default log level, a large response body is truncated in the log; at debug level, it's logged in full — prove both behaviors with the same handler, just a different configured level.

<details><summary>Hint</summary>Start with `ToContext`/`FromContext` and a trivial unexported key type — confirm a logger survives being passed through an unrelated function before adding correlation IDs.</details>
<details><summary>Hint</summary>For criterion 1's correlation-ID logger, build it from the package-level default logger (`slog.With(...)`), not from `FromContext` — there's nothing in context yet at that point in the chain, which is exactly what the real code does.</details>
<details><summary>Hint</summary>For the `Flush` criterion, your wrapper type needs to satisfy `http.Flusher` by delegating to the underlying `http.ResponseWriter` — a plain `io.Writer` embed alone won't be type-asserted as flushable by callers checking for that interface.</details>

**Compare step:** Diff your rebuild against `backend/common/log/*.go` and `backend/common/http/middlewares.go`/`truncate.go`, and ask yourself:
1. Did I fold body-capture into the same middleware as request logging, or did I split it into two — and if I split it, did I have to pass extra state between them that the merged version avoids?
2. Does my truncation preserve JSON structure (truncating arrays) or does it just cut the byte string, potentially producing invalid JSON in logs?
3. Did I correctly build the correlation-ID middleware's *first* logger from the global default rather than from context (since context has nothing in it yet at that point)?

## 5. Before the Exercise

- **Likely task:** [From page + confirmed by code state] The page states directly: "This is the last walkthrough exercise. No code to write yet" — and indeed, the entire `backend/common/log/` package plus the middleware/adapter wiring already exists in the workspace without any student implementation step. This is a read-and-understand exercise, not an implementation one.
- **Approach:**
  1. Re-read the four-step correlation ID flow until you can narrate it from memory, now against the real file names: `middlewares.go` (extract/generate + attach), `log.go`/`correlation.go` (store/retrieve), `echo.go`'s `HandleError` (surface on error).
  2. Open `ctx.go` first, before the rest of the package — the two unexported keys it defines are the foundation everything else relies on.
  3. Trace one full request in your head: a request with no `Correlation-ID` header hits `/health` — what gets logged, in what order, and with what attributes?
  4. If you have `tdl tr run` available, trigger a request without a `Correlation-ID` header and confirm you actually see a `gen_`-prefixed ID and the "correlation ID not found in context" warning — don't just take this guide's description on faith.

## 6. Recall Questions

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

*(These questions, with answers, have also been appended to `notes/review_queue.md` under today's date — the earlier, page-only version of these questions has been replaced there with this corrected set.)*

## 7. Cheat Sheet

- `log/slog` (Go 1.21+): structured key-value logging. `log.Init(level)` sets `slog.SetDefault(...)` once at startup, before anything else runs — order matters.
- `ToContext(ctx, logger)` / `FromContext(ctx) *slog.Logger` — the only two functions most code needs; `FromContext` never panics, it falls back to `slog.Default()`.
- Context key gotcha: unexported key type (`ctxKey`, backed by `int`/`iota`) per distinct value (`loggerKey`, `correlationIDKey`) — never a plain/shared string.
- Correlation ID: header in → context + logger attribute → header out on any outgoing call. A `gen_`-prefixed ID in logs = upstream propagation bug, not normal.
- Middleware registration order (`e.Use(...)` in `middlewares.go`): `ContextTimeout`, `Recover`, correlation-ID func, `requestLogMiddleware` — correlation-ID must precede anything that logs.
- **Correction vs. the page:** there is no standalone "body dump middleware" — body capture/truncation lives inside `requestLogMiddleware` itself.
- `truncateBodyForLog` (512-byte threshold, 6-item array cap): JSON-aware — truncates arrays and re-marshals, so logged JSON usually stays valid; falls back to head+tail string truncation for non-JSON. Skipped entirely (full body logged) at `Debug` level.
- `EchoSlogAdapter` routes Echo's own internal logger calls through the same `slog` pipeline via `e.Logger = common.NewEchoSlogAdapter(slog.Default())` — separate mechanism from the request-scoped `log.FromContext` path.
- Decision rule: use context-based logger storage (this project) when you want simple, explicit retrieval; use a custom `slog.Handler` when you want every `slog.*Context` call site to automatically pick up context attributes without fetching a logger first.
