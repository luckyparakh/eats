# Master Study Guide: Logging in Go Backends
> **Platform:** Three Dots Labs Academy — Go Backend Masterclass v0
> **Page:** Logging (slog, Context-Based Logging, Correlation IDs, HTTP Middleware)
> **Generated:** 2026-04-26

---

## SECTION 1 — PAGE METADATA

### Topics Covered on This Page
1. **Structured Logging** — using Go's built-in `slog` package instead of plain `fmt.Println`
2. **Context-Based Logging** — attaching a logger to a request's `context.Context` so every function can log without receiving the logger as a parameter
3. **Correlation ID** — a unique identifier that follows a request across all services so logs from different places can be linked together
4. **HTTP Middleware** — a pattern that runs shared logic (like logging setup) before and after every HTTP request, without putting that logic in every handler

### Where This Fits in the Learning Sequence
- **What came before (from `tillnow.md`):** You learned about the project structure — the modular monolith, the `Module` interface, how `Init → RegisterContracts → RegisterHttp` run in order, and how modules are wired together in `svc.go`. You saw the *skeleton* of the application.
- **What this page does:** This is the last "walkthrough" page — no code to write yet. It equips you with the logging infrastructure that ALL future exercises will depend on. Every handler, repository, and service you write will use these tools.
- **What this enables next:** Every future exercise will use `log.FromContext(ctx)` to log events. You will also see correlation IDs appear in test output when exercises fail, making debugging possible.

### Single Learning Objective
> After this page, the learner should be able to **explain why structured, context-aware logging with correlation IDs exists, how each piece connects to the others, and trace a request's log lifecycle from HTTP arrival to response.**

### Code Patterns Introduced on This Page

| Pattern | What it does |
|---|---|
| `slog.New(handler)` | Creates a structured logger with a custom output handler |
| `slog.SetDefault(logger)` | Makes a logger the global default for the whole program |
| `log.ToContext(ctx, logger)` | Stores a logger inside a request's context |
| `log.FromContext(ctx)` | Retrieves the logger from the context (falls back to default) |
| `log.FromContext(ctx).With("key", val)` | Returns a new logger with an extra key-value attribute attached |
| `CorrelationIDFromContext(ctx)` | Retrieves the correlation ID; generates a `gen_` prefixed one if missing |
| Middleware wrapping | A function that runs before/after a handler without changing the handler |

### Concepts Referenced from Previous Pages
- [Previously learned: `context.Context`] — Every HTTP request in Go carries a `context.Context`. You saw it in the module interface (`Init`, `RegisterHttp` etc.) and it is the central carrier of per-request data.
- [Previously learned: `Module` interface] — You know that every module has `Init`, `RegisterHttp`, `RegisterContracts`. The logging middleware is wired up in the HTTP registration phase.
- [Previously learned: Modular monolith] — You know all modules run in one process, sharing infrastructure like this logger.

---

## SECTION 2 — CONTINUITY BRIDGE

### a) The Thread

On the previous page you saw how the project is *structured* — how code is organized into modules, how those modules register their HTTP routes, and how `svc.go` wires everything together at startup.

But structure alone doesn't make software *observable*. A working skeleton is useless if, when something goes wrong in production, you can't see what happened, why it happened, or which request triggered it.

This page answers the question your previous page left open: **"Once the system is running, how do we know what it's doing?"**

Logging is the answer. And this page doesn't just add `fmt.Println` — it builds the *right* kind of logging: structured, context-aware, and request-traceable.

### b) The Shared Axiom

> **The one principle connecting both pages:** Every piece of infrastructure in a well-built backend exists to make the *application code* simpler and safer — not the other way around.

The module interface (previous page) kept module code clean by separating *initialization* from *route registration*. The logging infrastructure (this page) keeps handler code clean by separating *observability setup* from *business logic*. Same principle, different dimension.

### c) Quick Recall Check

- **You'll need: `context.Context`** — reminder: it's Go's built-in "envelope" that travels with every request and can carry key-value data through function calls.
- **You'll need: HTTP Handler** — reminder: a function that receives an incoming web request and sends back a response. Middleware wraps these handlers.
- **You'll need: Module initialization order** — reminder: `Init → RegisterContracts → RegisterHttp`. Middleware is set up during `RegisterHttp` because it wraps the routes.

---

## SECTION 3 — CORE CONCEPT DEEP-DIVE

---

### Concept 1: Structured Logging with `slog`

#### a) The Problem Statement — WHY does this exist?

Imagine your food delivery platform is live. A customer calls support: "I tried to place an order 20 minutes ago and it just disappeared." You open your server logs. Here is what you find:

```
error occurred
order failed
something went wrong
request processed
error occurred
```

These lines tell you *nothing*. You don't know which order. You don't know which customer. You don't know what "error" means. You don't know if the failure at 14:23 is related to the one at 14:24.

**This is the problem.** Without structure, logs are noise, not signal.

The need is simple: every log line must carry enough *context* (what happened, to which entity, at which severity, at which time) to be useful without also reading the surrounding 200 lines.

**Without structured logging**, engineers add more and more `fmt.Println` calls, each formatted differently, with no consistent fields. Searching for all errors related to order `ord_123` becomes a regex nightmare across five services.

#### b) The Atomic Axioms

1. **Axiom 1:** A log line's value is determined by how easily a human (or a machine) can filter, search, and understand it — not by how easy it was to write.
2. **Axiom 2:** Unstructured text (a sentence) is hard for machines to parse and filter. Key-value pairs are easy.
3. **Axiom 3:** Every log line must have at minimum: *when* it happened, *how severe* it is, *what happened*, and *what it relates to*.
4. **Axiom 4:** All logs in one system should go through a single logging setup so the format is consistent and controllable.
5. **Axiom 5:** For development, human-readable output helps. For production, machine-parseable output (JSON) helps. The same code should support both by swapping the *handler*, not the *call sites*.

#### c) The Core Mechanism

Because a log line is most useful when it has structured fields (Axiom 2), Go 1.21 introduced `log/slog` — a standard library package built around key-value pairs.

Instead of:
```
fmt.Println("Processing order failed")
```

You write:
```go
slog.Error("Processing order failed", "order_id", "ord_123", "error", err)
```

The output becomes:
```
[14:23:01.042] ERROR Processing order failed  order_id=ord_123 error="connection refused"
```

Now a search for `order_id=ord_123` finds every log line related to that order — instantly, across thousands of lines (Axiom 1).

Because all logs go through `slog.SetDefault(logger)` (Axiom 4), every part of the application — including third-party packages that use `slog` internally — uses the same format. Swapping from human-readable to JSON output is one line change in `Init()` (Axiom 5).

#### d) Syntax & Code (from the webpage)

```go
// backend/common/log/slog.go

func Init(level slog.Level) {
    // Configure the human-readable handler for local development
    opts := &humanslog.Options{
        HandlerOptions: &slog.HandlerOptions{
            Level: level,           // e.g. slog.LevelDebug, slog.LevelInfo
        },
        TimeFormat: "[15:04:05.000]", // How timestamps appear: [14:23:01.042]
    }

    // Create a new slog.Logger using the humanslog handler
    // humanslog produces colored, human-friendly output (not JSON)
    logger := slog.New(humanslog.NewHandler(os.Stderr, opts))

    // Make this logger THE global default for the entire program
    // Any code that calls slog.Info(...) will now use this logger
    slog.SetDefault(logger)
}
```

**What `EchoSlogAdapter` does (from `backend/common/echo.go`):**
Echo (the HTTP framework) has its own logger interface. The adapter wraps our `slog` logger so Echo's internal log messages (like "server started on port 8080") also appear in the same format — not in Echo's own format. One system, one format (Axiom 4).

#### e) Execution / Internal Walkthrough

When the application starts (`svc.go` calls `log.Init(...)`):

1. `Init(slog.LevelInfo)` is called with a minimum severity level.
2. A `humanslog.Options` struct is built — this controls timestamp format and what level to display.
3. `humanslog.NewHandler(os.Stderr, opts)` creates a handler that writes colored, human-readable lines to `stderr` (the error output stream of the program, shown in your terminal).
4. `slog.New(handler)` wraps the handler in a `slog.Logger` value.
5. `slog.SetDefault(logger)` installs it as THE global logger.

Now when any code calls `slog.Info("some message")`:
- The global logger is found.
- The severity (`INFO`) is checked against the minimum level — if `INFO >= LevelInfo`, it passes through.
- The handler formats the message as `[14:23:01.042] INFO  some message` and writes it to `stderr`.

**Where each axiom becomes visible:**
- Axiom 1 & 2: visible at step 3 — the handler produces parseable key-value output.
- Axiom 3: visible at step 5 — timestamp, severity, and message are always present.
- Axiom 4: visible at step 5 — `SetDefault` means all callers use this logger.
- Axiom 5: visible at step 3 — swapping `humanslog.NewHandler` for `slog.NewJSONHandler` changes output format without touching any other code.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| `fmt.Println` is good enough for logs | **FALSE** | Plain text is unsearchable. Key-value pairs are machine-filterable (Axiom 2). |
| `slog.SetDefault` affects only the current file | **FALSE** | It sets the global default for the ENTIRE program — all packages and files. (Axiom 4) |
| All log lines automatically include request info | **FALSE** | Without context-based logging (Concept 2), extra info must be manually passed everywhere. |
| Changing from human-readable to JSON requires changing every log call | **FALSE** | Only the handler in `Init` changes. Call sites remain identical. (Axiom 5) |
| `os.Stderr` and `os.Stdout` are the same | **FALSE** | `Stdout` is for normal program output. `Stderr` is for diagnostic output (logs, errors). Separating them lets you redirect each independently. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Using `fmt.Println` or `log.Println` for production logs | Logic error | These produce unstructured text; no fields, no severity, no consistency | Use `slog.Info`, `slog.Error` etc. with key-value pairs |
| Calling `slog.Init` multiple times | Silent bug | Each call replaces the global default — earlier configuration lost | Call `Init` exactly once, at startup in `main` or `svc.go` |
| Logging to `Stdout` instead of `Stderr` | Convention violation | Logs mixed with application output break log parsers | Always use `os.Stderr` for log handlers |
| Hardcoding log level to `Debug` in production | Security / performance issue | DEBUG logs may expose sensitive data and generate excessive volume | Make log level configurable (environment variable or flag) |
| Not including the error value in an error log | Usability issue | `slog.Error("failed")` tells you nothing without `"error", err` | Always include `"error", err` when logging errors |

#### h) Comprehension Task

> **Comprehension Task:** Without looking at the code, write in plain English: "If I call `slog.Info("order placed", "order_id", "ord_456")` in a file deep in the `orders` package, where does this log line come out, what does it look like, and why?"
>
> **What to check:** A correct answer mentions: it comes out in the terminal (stderr), it looks like `[HH:MM:SS.mmm] INFO  order placed  order_id=ord_456`, and it works because `slog.SetDefault` was called once at startup and the global logger is used by all `slog.*` calls everywhere in the program.
>
> **Common wrong answer:** "I'd need to import the logger and pass it to that function." — Wrong. That's what context-based logging (next concept) is for request-specific attributes. For general structured logging, the global default set by `SetDefault` is always available.

---

### Concept 2: Context-Based Logging

#### a) The Problem Statement — WHY does this exist?

The `slog` package solves *format*. But there's a second problem: **enriching** log lines with request-specific information.

Say you want every log line that touches order `ord_123` to say `order_id=ord_123`. You can manually add it to each log call — but you'd have to pass `order_id` as a parameter through every function that might log something, even functions that don't care about `order_id` at all.

```go
// Without context-based logging — the logger (with order_id attached) 
// must be passed through every layer even if they don't use it:
func HandlePlaceOrder(orderID string, logger *slog.Logger, ...) {
    processPayment(orderID, logger, ...)  // payment doesn't care about logger
}
func processPayment(orderID string, logger *slog.Logger, ...) {
    reserveInventory(orderID, logger, ...) // inventory doesn't care either
}
```

This is called **parameter pollution** — the logger contaminates every function signature just because *one* deep function needs to log.

Go already passes `context.Context` through almost every function for cancellation and deadlines. The insight is: **piggyback the logger on the context.** Now every function that already receives `ctx` can also retrieve a request-enriched logger with zero extra parameters.

#### b) The Atomic Axioms

1. **Axiom 1:** `context.Context` is already passed to nearly every function in Go — it is the standard request carrier.
2. **Axiom 2:** You can store any value in a `context.Context` under a typed key using `context.WithValue`.
3. **Axiom 3:** Storing a logger-with-attributes in the context means the attributes are available anywhere the context travels, without being in any function signature.
4. **Axiom 4:** If no logger is found in the context, falling back to the global default logger prevents a crash.
5. **Axiom 5:** The unexported `ctxKey` type prevents other packages from accidentally reading or overwriting these context values — the key is invisible outside its own package.

#### c) The Core Mechanism

Think of `context.Context` like a hotel room keycard envelope. When a guest checks in (a request arrives), the front desk (middleware) puts the keycard, a restaurant voucher, and a loyalty card all inside the envelope and hands it to the guest. Now the guest can walk to the restaurant, the gym, or their room — and every staff member who sees the envelope can take out exactly what they need, without the guest having to carry each card separately.

Similarly:
- Middleware creates a logger pre-loaded with `correlation_id` and other attributes.
- It stores this logger in the context: `ctx = log.ToContext(ctx, enrichedLogger)`.
- Every function that receives `ctx` can call `log.FromContext(ctx)` to get that pre-enriched logger.
- No function parameter needed. No global mutable state. Every request has its own logger in its own context.

#### d) Syntax & Code (from the webpage)

```go
// backend/common/log/log.go

// ToContext: store a logger inside the context.
// Returns a NEW context (contexts are immutable — WithValue creates a copy).
func ToContext(ctx context.Context, logger *slog.Logger) context.Context {
    return context.WithValue(ctx, loggerKey, logger)
    // loggerKey is an unexported value of unexported type ctxKey
    // This prevents accidental key collisions from other packages (Axiom 5)
}

// FromContext: retrieve the logger from the context.
// If none found, returns the global default — never panics (Axiom 4).
func FromContext(ctx context.Context) *slog.Logger {
    log, ok := ctx.Value(loggerKey).(*slog.Logger)
    // ctx.Value(loggerKey) looks up the value stored under loggerKey
    // .(*slog.Logger) is a "type assertion" — confirms it IS a *slog.Logger
    if ok {
        return log   // Found — return the request-specific logger
    }
    return slog.Default() // Not found — fall back to the global logger
}
```

**How it's used in a handler:**
```go
// In an HTTP handler (simplified):
func (h *Handler) PlaceOrder(c echo.Context) error {
    ctx := c.Request().Context()               // Get the request's context
    logger := log.FromContext(ctx)             // Get logger (already has correlation_id)
    logger.Info("placing order", "order_id", "ord_123")  // Logs with correlation_id automatically
    return nil
}
```

**The error handler uses the same pattern:**
```go
// backend/common/http/echo.go
func HandleError(err error, c echo.Context) {
    // log.FromContext picks up the logger with correlation_id already attached
    log.FromContext(c.Request().Context()).With("error", err).Error("HTTP error")
    // .With("error", err) adds the error as an extra field to this specific log line
}
```

#### e) Execution / Internal Walkthrough

Trace of a request from arrival to log output:

```
Step 1: HTTP request arrives → middleware runs first
Step 2: Middleware generates or extracts Correlation-ID (e.g., "abc-789")
Step 3: Middleware creates enriched logger:
        enrichedLogger = slog.Default().With("correlation_id", "abc-789")
Step 4: Middleware stores it: ctx = log.ToContext(ctx, enrichedLogger)
Step 5: Middleware passes the enriched ctx to the handler
Step 6: Handler calls log.FromContext(ctx) → gets enrichedLogger
Step 7: Handler logs: logger.Info("placing order", "order_id", "ord_123")
Step 8: Output: [14:23:01] INFO placing order  correlation_id=abc-789  order_id=ord_123
```

At no step does the handler need to know about the correlation ID — it just calls `FromContext` and the context already has it.

**Where each axiom becomes visible:**
- Axiom 1: Step 4 — the context is already flowing through, we just add to it.
- Axiom 2: Step 4 — `context.WithValue` stores the logger.
- Axiom 3: Step 7 — the handler logs without any correlation_id parameter.
- Axiom 4: Step 6 — if the middleware failed to store the logger, `slog.Default()` is returned.
- Axiom 5: In the unexported `loggerKey` — other packages can't accidentally overwrite the logger stored at this key.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| `context.WithValue` modifies the original context | **FALSE** | Contexts are immutable. `WithValue` returns a NEW context; the original is unchanged. |
| `FromContext` panics if the logger isn't there | **FALSE** | It falls back to `slog.Default()`. Safe by design (Axiom 4). |
| Any package can read the logger from context | **FALSE** | Only the package that owns the unexported key type can access it (Axiom 5). |
| The enriched logger replaces the global logger | **FALSE** | `ToContext` stores the enriched logger only in this context. The global default is unchanged. |
| You need to pass the logger as a function parameter | **FALSE** | That's exactly what this pattern eliminates — use `log.FromContext(ctx)` instead. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Passing logger as a function parameter alongside `ctx` | Code smell | Defeats the purpose of context-based logging | Remove the logger parameter; use `log.FromContext(ctx)` |
| Using a plain `string` as context key | Silent bug | String keys collide across packages — any package using the key `"logger"` will conflict | Use an unexported custom type as the key (Axiom 5) |
| Calling `log.FromContext` before middleware runs | Logic error | At that point the context has no logger; you get the default (fine) but lose request attributes | Always call `FromContext` after the context has been enriched by middleware |
| Ignoring the `ok` check in a type assertion | Panic | If the stored value is not `*slog.Logger`, the assertion panics | Always use the two-return form: `log, ok := ctx.Value(key).(*slog.Logger)` |

#### h) Comprehension Task

> **Comprehension Task:** On paper, draw two boxes: one labeled "Handler A" and one labeled "Handler B". Both receive the same `ctx`. Handler A calls `log.FromContext(ctx).With("extra", "x").Error("failed")`. Handler B calls `log.FromContext(ctx).Info("ok")`. Question: Does Handler B's log line include `extra=x`? Why or why not? Trace through `ToContext` and `FromContext` to prove your answer.
>
> **What to check:** The answer is NO. `.With("extra", "x")` returns a new logger — it does NOT modify the logger stored in `ctx`. Handler B calls `FromContext(ctx)` which returns the original enriched logger without `extra`. Each `.With()` call creates a new child logger, leaving the parent (and the one in the context) unchanged.
>
> **Common wrong answer:** "Yes, because both handlers share the same `ctx`." — This confuses the logger in the context (unchanged by `.With`) with the child logger returned by `.With`. Context-stored values are immutable in the same way contexts themselves are.

---

### Concept 3: Correlation ID

#### a) The Problem Statement — WHY does this exist?

Your food delivery platform has three services: `orders`, `inventory`, `payments`. Each writes its own logs. A user's order triggers all three.

When the order fails, you open logs. You see:
```
[orders service]    ERROR processing order failed   user_id=u_99
[inventory service] ERROR reservation failed        item_id=i_55
[payments service]  INFO  charge attempted           amount=29.99
```

Are these three lines about the *same user request*? You don't know. The timestamps are close but not identical (network latency). There are thousands of other requests happening simultaneously.

**The problem:** Without a shared identifier, you cannot tell which log lines from different services (or even different parts of the same service) belong to the same original request.

**A correlation ID** is a single unique string — generated when the request first arrives — that gets attached to every log line that request touches, no matter which service or function processes it.

#### b) The Atomic Axioms

1. **Axiom 1:** A request often touches multiple services. Each service logs independently. Without a shared identifier, there is no way to connect those logs.
2. **Axiom 2:** The identifier must be unique enough that two simultaneous requests never share the same ID.
3. **Axiom 3:** The ID must travel with the request — passed in HTTP headers to every downstream service.
4. **Axiom 4:** When the ID is missing (someone called without it), generating a fallback is better than crashing — but the fallback must be visibly different so you know something is wrong upstream.
5. **Axiom 5:** Every log line for a request must include the correlation ID as a structured field so you can filter by it.

#### c) The Core Mechanism

Think of the correlation ID like a tracking number on a package shipment. When you order something online, the warehouse, the first courier hub, the regional depot, and the local delivery van all scan the same tracking number. Later, if the package goes missing, you query that one tracking number and see every step it took.

Similarly:
- An HTTP request arrives. The middleware checks for a `Correlation-ID` header.
- If the header exists (another service sent this request), that ID is used.
- If not (a client browser sent the request), a new unique ID is generated.
- The ID is added to the logger (via `ToContext`) and to the *response header* so the client can reference it in support tickets.
- When this service makes an outgoing HTTP call to another service, it puts the same ID in the `Correlation-ID` header of that outgoing call.
- Every service that receives the ID repeats the process. The ID propagates everywhere.

#### d) Syntax & Code (from the webpage)

```go
// backend/common/log/correlation.go

func CorrelationIDFromContext(ctx context.Context) string {
    // Try to retrieve the correlation ID from the context
    v, ok := ctx.Value(correlationIDKey).(string)
    if ok {
        return v    // Found — return it (Axiom 3: it was stored here by middleware)
    }

    // Not found — warn and generate a fallback (Axiom 4)
    FromContext(ctx).Warn("correlation ID not found in context")

    // "gen_" prefix makes it immediately obvious this ID was generated as a fallback
    // shortuuid generates a compact, URL-safe unique string (e.g. "gen_6ka7f...")
    return "gen_" + shortuuid.New()
}
```

**The request flow in the middleware (conceptual):**
```go
// Middleware pseudocode (actual code in backend/common/http/middlewares.go)

func CorrelationIDMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // Step 1: Try to get ID from incoming request header
        correlationID := c.Request().Header.Get("Correlation-ID")

        // Step 2: If missing, generate a fresh one
        if correlationID == "" {
            correlationID = shortuuid.New()
        }

        // Step 3: Build enriched logger with correlation_id field
        logger := slog.Default().With("correlation_id", correlationID)

        // Step 4: Store both in context
        ctx := log.ToContext(c.Request().Context(), logger)
        ctx = log.CorrelationIDToContext(ctx, correlationID)  // [added for clarity]

        // Step 5: Set the correlation ID in the response header
        c.Response().Header().Set("Correlation-ID", correlationID)

        // Step 6: Call the actual handler with the enriched context
        c.SetRequest(c.Request().WithContext(ctx))
        return next(c)
    }
}
```

#### e) Execution / Internal Walkthrough

```
Incoming request: GET /orders/place
Header: Correlation-ID: abc-789

Step 1: Middleware reads header → correlationID = "abc-789"
Step 2: Not empty → no new ID generated
Step 3: logger = slog.Default().With("correlation_id", "abc-789")
Step 4: ctx stored with logger and correlationID
Step 5: Response header "Correlation-ID: abc-789" is set
Step 6: Handler runs with enriched ctx

Handler logs: logger.Info("placing order", "order_id", "ord_123")
Output: [14:23:01] INFO placing order  correlation_id=abc-789  order_id=ord_123

Handler calls payments service → outgoing request includes header "Correlation-ID: abc-789"
Payments service middleware receives "abc-789" → reuses it → its logs also say correlation_id=abc-789

Later in support: grep "abc-789" across all service logs → full picture of the request
```

**Where each axiom becomes visible:**
- Axiom 1: The payments service log also has `correlation_id=abc-789` — linked.
- Axiom 2: `shortuuid.New()` generates a statistically unique ID.
- Axiom 3: The outgoing request header carries the ID forward.
- Axiom 4: `CorrelationIDFromContext` generates `gen_...` if context has no ID, with a warning.
- Axiom 5: All log lines from this request include `correlation_id=abc-789` as a field.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| The correlation ID is a database ID for the order | **FALSE** | It's a *request trace ID*, unrelated to business entities. One order might be attempted over multiple requests, each with a different correlation ID. |
| `gen_` prefixed IDs are fine in production | **FALSE** | They indicate something upstream isn't setting the header correctly. They're a warning signal (Axiom 4). |
| You need to pass the correlation ID manually to each function | **FALSE** | It's stored in the context and retrieved by `CorrelationIDFromContext(ctx)`. |
| UUIDs and shortuuids are the same thing | **FALSE** | Both are unique IDs. Shortuuid is a shorter, URL-safe encoding of the same entropy — easier to read and type. |
| Correlation IDs are only for distributed systems | **FALSE** | Even in a single service (monolith), they help trace a request through multiple layers of logs. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Generating a new ID per service call instead of propagating | Logic error | Breaks Axiom 3 — logs from different services can no longer be linked | Read and pass the incoming `Correlation-ID` header; only generate when missing |
| Not including the ID in outgoing HTTP requests | Logic error | Downstream services get a `gen_` ID — correlation breaks | Always inject the ID into outgoing request headers |
| Using a sequential counter as the ID (1, 2, 3...) | Security issue | Predictable IDs allow enumeration — an attacker can infer request volume | Use a cryptographically random generator like `shortuuid` |
| Ignoring `gen_` IDs in production alerts | Operational mistake | Misses upstream correlation ID propagation failures | Alert on `gen_` prefix appearing in production logs |

#### h) Comprehension Task

> **Comprehension Task:** You see this line in your production logs: `[14:55:01] WARN correlation ID not found in context  correlation_id=gen_8xT2...`. Write a plain English explanation of what this means, what caused it, and what you would investigate first.
>
> **What to check:** It means a request arrived at the service without a `Correlation-ID` header, so the system generated a fallback. This could mean: (a) a direct browser/client call that never had a correlation ID, or (b) some upstream service or load balancer is not propagating the header. The first thing to investigate is whether the calling client or upstream service sets the `Correlation-ID` header in its outgoing requests.
>
> **Common wrong answer:** "The logger failed." — The logger is working fine; it's the *correlation ID propagation* that broke. The WARN is about a missing context value, not a logger malfunction.

---

### Concept 4: HTTP Middleware

#### a) The Problem Statement — WHY does this exist?

You have 20 HTTP handlers: one for placing orders, one for tracking orders, one for listing restaurants, and so on.

Every handler needs correlation ID extraction, logging setup, request logging (URI, method, status, latency), and error logging.

Without middleware, you'd copy the same 15 lines of setup code into every single handler. That's 20 × 15 = 300 lines of duplicated boilerplate. Change the log format? Update 20 files.

**The problem:** Cross-cutting concerns (things that apply to every request) don't belong in any single handler — they belong *around* all handlers.

**Middleware** is the pattern that wraps a handler with extra behavior — running code before and after the handler — without the handler knowing or caring that it's being wrapped.

#### b) The Atomic Axioms

1. **Axiom 1:** Some code must run for every HTTP request, regardless of which handler processes it.
2. **Axiom 2:** Duplicating that code in every handler creates maintenance problems and inconsistency.
3. **Axiom 3:** A middleware is a function that takes a handler and returns a new handler — it "wraps" the original without modifying it.
4. **Axiom 4:** Middlewares can be chained — the output of one middleware becomes the input of the next, forming a pipeline.
5. **Axiom 5:** Middleware runs before the handler (setup) and optionally after (cleanup/logging) — in a predictable, consistent order.

#### c) The Core Mechanism

Imagine a hotel's front door (middleware) vs. the rooms (handlers). Every guest who enters the hotel (every request) goes through the front door: the security check, the check-in desk, the room assignment — in that order, every time. The rooms themselves don't have their own security checks or check-in desks. The door handles all of that before the guest arrives at the room.

In code:
- An HTTP framework (like Echo) receives a request.
- Before calling the handler, it passes the request through a chain of middlewares.
- Each middleware can inspect or modify the request, add data to the context, then call the next middleware (or the handler) in the chain.
- After the handler returns, middlewares can run cleanup code (like logging the response status).

#### d) Syntax & Code (from the webpage)

The project uses three middlewares in `backend/common/http/middlewares.go`:

```go
// Middleware 1: Correlation ID
// - Extracts or generates the correlation ID
// - Adds correlation-ID-aware logger to context
// - Sets Correlation-ID in response headers
// (See pseudocode in Concept 3 above)

// Middleware 2: Request Logger
// Logs each request: URI, HTTP method, response status, and latency (time taken)
// Example output: [14:23:01] INFO GET /orders/place  status=200  latency=12ms

// Middleware 3: Body Dump
// Captures the request body (what was sent) and response body (what was sent back)
// Used for debugging — seeing the exact payload that caused a failure
```

**How middlewares are chained in Echo (conceptual):**
```go
// In RegisterHttp (simplified):
e := echo.New()
e.Use(CorrelationIDMiddleware)   // Runs first — adds correlation ID to context
e.Use(RequestLoggerMiddleware)   // Runs second — logs the request (now has correlation ID)
e.Use(BodyDumpMiddleware)        // Runs third — captures bodies for debugging

// Now when a request comes in, it flows:
// CorrelationID Middleware → RequestLogger Middleware → BodyDump Middleware → Handler
//                         ← cleanup (response logging) ←                  ←
```

**The middleware shape (every middleware follows this pattern):**
```go
func SomeMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // [BEFORE handler]: setup, enrich context, start timer, etc.

        err := next(c)  // Call the next middleware OR the actual handler

        // [AFTER handler]: log response, cleanup, measure latency, etc.

        return err
    }
}
```

#### e) Execution / Internal Walkthrough

Request: `POST /orders/place` with body `{"restaurant_id": "r_5"}`

```
Step 1: CorrelationID Middleware starts
        → Reads "Correlation-ID" header = "abc-789"
        → Stores enriched logger in ctx
        → Calls next(c)

Step 2: RequestLogger Middleware starts
        → Records start time: T=0ms
        → Calls next(c)

Step 3: BodyDump Middleware starts
        → Sets up capture of request/response bodies
        → Calls next(c)

Step 4: Handler PlaceOrder runs
        → Calls log.FromContext(ctx).Info("placing order", "order_id", "ord_123")
        → Output: [14:23:01] INFO placing order correlation_id=abc-789 order_id=ord_123
        → Handler returns status 200

Step 5: BodyDump Middleware resumes (after next(c) returns)
        → Logs request body: {"restaurant_id": "r_5"}
        → Logs response body: {"order_id": "ord_123", "status": "created"}

Step 6: RequestLogger Middleware resumes
        → Calculates latency = 12ms
        → Output: [14:23:01] INFO POST /orders/place  status=200  latency=12ms  correlation_id=abc-789

Step 7: CorrelationID Middleware resumes
        → Nothing to do after the handler (response header already set at start)
        → Returns
```

**Where each axiom becomes visible:**
- Axiom 1: Correlation ID extraction happens in EVERY request at Step 1 — no handler codes this.
- Axiom 2: Zero logging setup code appears in the PlaceOrder handler (Step 4).
- Axiom 3: Each middleware takes `next echo.HandlerFunc` and returns a new `echo.HandlerFunc`.
- Axiom 4: Steps 1–3 show three middlewares chained — each calls `next(c)` to pass control.
- Axiom 5: Steps 1–3 run before the handler; Steps 5–6 run after (the "after" of each middleware runs in reverse order of registration).

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| Middleware modifies the handler's code | **FALSE** | The handler has no idea middleware exists. Middleware wraps it from the outside. |
| Middleware only runs code before the handler | **FALSE** | Code after `next(c)` runs after the handler returns. This is how response logging works. |
| Middlewares run in random order | **FALSE** | They run in the order they are registered with `e.Use(...)`. Order matters. |
| Every handler needs its own middleware setup | **FALSE** | `e.Use(...)` applies middleware globally to all routes. |
| Middleware is specific to Echo | **FALSE** | The middleware pattern is universal across all HTTP frameworks and languages. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Registering request-logging middleware BEFORE correlation ID middleware | Logic error | Request logger runs before correlation ID is in context — log lines missing `correlation_id` | Register correlation ID middleware first |
| Adding observability code directly in each handler | Maintenance problem | Duplicated code; changes must be made in every handler | Move to middleware |
| Not calling `next(c)` inside middleware | Runtime bug | The actual handler never runs — every request hangs or returns empty | Always call `next(c)` unless intentionally blocking the request (e.g., auth failure) |
| Putting business logic in middleware | Architecture mistake | Middleware should be cross-cutting only; business logic belongs in handlers/services | Keep middleware to: auth, logging, rate-limiting, correlation IDs |

#### h) Comprehension Task

> **Comprehension Task:** Without looking at code, describe in writing what would happen to a request if the correlation ID middleware was registered AFTER the request logger middleware. Trace through the middleware execution order and identify exactly which axiom is violated and what the consequence is.
>
> **What to check:** The request logger runs first (before the handler), so it logs the request. But the correlation ID hasn't been added to the context yet — it's added later by the correlation ID middleware. The request log line will be missing `correlation_id`. Axiom 5 (middleware runs in registration order) and Axiom 1 (correlation ID must be present for ALL request logs) are violated. The consequence: request logs cannot be linked to other logs from the same request.
>
> **Common wrong answer:** "It doesn't matter — middleware all runs before the handler." — True that both run before the handler, but they run in *sequence*, not in parallel. The first registered middleware runs first, calling `next(c)` which runs the second, then the handler. Order within the chain is precise.

---

## SECTION 4 — EXERCISE READINESS

### a) What the Exercise Will Likely Ask

This is a **walkthrough exercise with no code to write**. The page explicitly states: "This is the last walkthrough exercise. No code to write yet."

However, based on the concepts covered, future exercises will likely ask you to:
- Add `log.FromContext(ctx)` calls inside handlers to log specific business events.
- Use `log.FromContext(ctx).With("key", val)` to attach business-specific fields to log lines.
- Potentially add a new middleware or a new context helper for a new type of data.

### b) Pre-Implementation Checklist

For when you start writing handlers in future exercises:

- [ ] I can state in one sentence why `slog.SetDefault` is called once at startup and not in each handler.
- [ ] I can explain the difference between `log.ToContext` (stores a logger) and `slog.SetDefault` (sets the global logger).
- [ ] I can trace what `log.FromContext(ctx).With("order_id", id).Info("message")` produces as output.
- [ ] I can explain why `gen_` prefixed correlation IDs in production logs are a warning signal.
- [ ] I can describe the order of middlewares in `middlewares.go` and why the correlation ID middleware must be first.

### c) Implementation Blueprint

For when you need to add logging to a handler in a future exercise:

**Step 1:** Get the context from the Echo request: `ctx := c.Request().Context()` — relies on Axiom 1 of context-based logging (context is already flowing through).

**Step 2:** Retrieve the logger: `logger := log.FromContext(ctx)` — relies on Axiom 3 (attributes already in context from middleware).

**Step 3:** If this function is several layers deep (a service, not a handler), accept `ctx context.Context` as a parameter and call `log.FromContext(ctx)` directly — same approach, no extra parameters.

**Step 4:** Log the event with relevant structured fields: `logger.Info("event name", "field_name", value)` — relies on Axiom 2 of structured logging (key-value pairs are searchable).

**Step 5:** For errors, always include the error: `logger.Error("operation failed", "error", err, "entity_id", id)`.

### d) Debugging Guide

| Failure Symptom | Violated Axiom | Fix |
|---|---|---|
| Log lines missing `correlation_id` | Middleware order axiom (Axiom 5 of middleware) | Check that correlation ID middleware is registered first |
| `gen_` IDs appearing in logs | Correlation ID Axiom 4 — fallback triggered | Check that `ToContext` is being called with the correlation ID before the handler runs |
| Logs from a handler missing custom fields | Context-based logging Axiom 3 | Ensure you're calling `.With(...)` BEFORE `.Info(...)`, not after |
| All log lines look identical (no `order_id` etc.) | Structured logging Axiom 2 | Add key-value fields to your log calls: `logger.Info("msg", "order_id", id)` |

---

## SECTION 5 — STRUCTURED PRACTICE (AEIOU Framework)

### A — ACQUIRE (Axioms First)

**Structured Logging axioms to internalize:**
1. Key-value pairs in logs are machine-filterable; plain sentences are not.
2. All logs must go through one consistent system (`SetDefault`).
3. The logging handler (human-readable vs. JSON) can be swapped without changing log call sites.

**Context-Based Logging axioms to internalize:**
1. `context.Context` is already passed to most Go functions — piggyback on it.
2. `WithValue` creates a new context; it never modifies the original.
3. An unexported key type prevents cross-package key collisions.

**Write from memory (no copy-paste):**
- The `Init` function: create options, create handler, create logger, set default.
- The `FromContext` / `ToContext` pair — with the unexported key type pattern.
- The `CorrelationIDFromContext` function — with the `gen_` fallback.

---

### E — EXERCISE (Reason from Nothing)

**Problem 1 (Simple — Structured Logging):**
You see this in a codebase:
```go
fmt.Printf("Failed to process order %s: %v\n", orderID, err)
```
Rewrite it as a proper `slog` call. What fields should be included? Why?

**Problem 2 (Simple — Context-Based Logging):**
A function signature is: `func ProcessOrder(orderID string, logger *slog.Logger, db *sql.DB) error`
How would you redesign this to use context-based logging instead? Write the new signature and the first two lines of the function body.

**Problem 3 (Medium — Correlation ID):**
Trace this scenario in writing: Service A generates a correlation ID `"xyz-001"`. It calls Service B with a `Correlation-ID: xyz-001` header. Service B calls Service C but forgets to set the header. What does Service C's log output look like? What does the `gen_` prefix tell you?

**Problem 4 (Medium — Middleware Order):**
You have three middlewares: `Auth`, `CorrelationID`, `RequestLogger`. A handler needs correlation IDs in its auth-failure logs. In what order should these middlewares be registered? Why? (Trace the execution order in plain English.)

**Problem 5 (Complex — All Concepts):**
A request comes in. The middleware chain is: `CorrelationID → RequestLogger`. Inside the handler, this code runs:
```go
ctx := c.Request().Context()
log.FromContext(ctx).Info("handler start")
log.FromContext(ctx).With("step", "validate").Info("validating")
log.FromContext(ctx).Error("validation failed", "error", errors.New("invalid email"))
```
Write out exactly three log lines you would see in the terminal, including all fields.

**Problem 6 (Complex — Axiom Identification):**
For each line of this code, state which axiom from this page it demonstrates:
```go
func Init(level slog.Level) {
    opts := &humanslog.Options{HandlerOptions: &slog.HandlerOptions{Level: level}}
    logger := slog.New(humanslog.NewHandler(os.Stderr, opts))
    slog.SetDefault(logger)
}
```

---

### I — INSPECT (Identify the Violation)

**Task 1:** What is wrong with this code? Which axiom does it violate?
```go
func FromContext(ctx context.Context) *slog.Logger {
    log := ctx.Value("logger").(*slog.Logger)  // Note: string key, no ok check
    return log
}
```

**Task 2:** What is wrong?
```go
func MyMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        logger := slog.Default().With("correlation_id", "hardcoded-123")
        ctx := log.ToContext(c.Request().Context(), logger)
        c.SetRequest(c.Request().WithContext(ctx))
        // handler never called
        return nil
    }
}
```

**Task 3:** What is wrong?
```go
e.Use(RequestLoggerMiddleware)   // registered first
e.Use(CorrelationIDMiddleware)   // registered second
```

**Task 4:** What is wrong?
```go
logger := log.FromContext(ctx)
logger.With("order_id", orderID)     // Note: return value discarded
logger.Info("processing order")      // will order_id appear?
```

**Task 5:** What is wrong?
```go
func CorrelationIDFromContext(ctx context.Context) string {
    v := ctx.Value(correlationIDKey).(string)  // no ok check
    return v
}
```

---

### O — ORCHESTRATE (Synthesis Design)

**Scenario:** You are building a new `payments` module. It has a handler `ChargeCustomer`. When a charge is attempted, you need to log: the start of the attempt, the amount, the customer ID, and any error. All log lines must include the correlation ID automatically.

**Write in plain English:**
1. Which concept handles the correlation ID automatically? Where does it get set up?
2. How does the handler get its logger? (No logger parameter in the function signature.)
3. What does the log call look like for a successful charge of $29.99 for customer `cust_77`?
4. What does the log call look like for a failed charge?
5. If you searched production logs for all charges this customer made today, what field would you filter on?

**Then write the implementation blueprint** (plain English, step-by-step, before any code).

---

### U — UNDERSTAND (Feynman Reconstruction Test)

**Most commonly misunderstood concept: Context-Based Logging**

**Step 1 — Explain it simply (write this out):**
Pretend you're explaining to a friend who has never coded: "Imagine you're making a pizza delivery. The context is like a delivery bag that the driver carries. Inside the bag, there's a delivery note with the customer's name. Every time the driver opens the bag to do something (log a step), the note is already there — they don't need to memorize the customer's name or carry it separately."

**Step 2 — Find the gap:**
A beginner might say: "So the context IS the logger?" — That's the gap. The context is the *carrier*; the logger is the *passenger*.

**Step 3 — Go back to axioms:**
Axiom 1: `context.Context` is just a carrier — it holds key-value pairs.
Axiom 2: The logger (with request-specific fields) is stored IN the context as a value.
The context is NOT the logger. The context CONTAINS the logger.

**Step 4 — Proof of Understanding:**
**Question:** If two concurrent requests are being handled simultaneously, each with its own `ctx`, and both call `log.FromContext(ctx)` at the same time — will they interfere with each other's log output? Why or why not?

**Expected Answer:** No. Each request has its own `ctx` with its own logger stored under the same key. `context.WithValue` creates independent context trees — one request's context is completely separate from another's. The loggers inside them are different objects (each with their own correlation ID). There is no shared mutable state between them.

---

## SECTION 6 — KNOWLEDGE CHECK

### a) Scenario-Based Reasoning Questions

**Q1:** A developer says: "I added `slog.Info("order placed")` in my handler but the log line has no `correlation_id` field. The middleware is registered correctly." What is the most likely cause? Which axiom explains it?

**Expected Answer:** The middleware is not correctly storing the enriched logger in the context, OR the handler is calling `slog.Info` (the global logger) instead of `log.FromContext(ctx).Info(...)`. The global logger never has request-specific fields. Structured logging Axiom 4 (all logs through one system) is being followed, but context-based logging Axiom 3 (attributes from middleware travel in the context) is being bypassed.

---

**Q2:** You see `gen_abc123` as a correlation ID in logs from your payments service. The orders service, which called payments, shows correlation ID `real-xyz`. What happened?

**Expected Answer:** The orders service made an outgoing HTTP call to payments but did NOT set the `Correlation-ID` header on that outgoing request. Payments middleware received a request with no header, triggered the fallback, and generated `gen_abc123`. The `gen_` prefix makes this immediately visible. Correlation ID Axiom 3 (ID must travel with the request) was violated by the orders service.

---

**Q3:** Why does `FromContext` fall back to `slog.Default()` instead of returning `nil`?

**Expected Answer:** Because returning `nil` would cause a nil pointer panic the first time any code calls `.Info(...)` on the returned logger. Axiom 4 of context-based logging says the fallback must be non-crashing. The global default always exists (set at startup by `Init`), so it's a safe fallback. The application degrades gracefully — logs without request attributes — rather than crashing.

---

**Q4:** A new developer proposes: "Let's just use a global variable to store the current request's correlation ID. Then we don't need all this context stuff." What is fundamentally wrong with this approach in a web server?

**Expected Answer:** A web server handles many requests *concurrently* (at the same time). A global variable is shared between all concurrent requests. If Request A sets the global to `"abc-789"` and then Request B immediately sets it to `"def-012"`, Request A's log calls now show `"def-012"`. This is a race condition. Context-based logging solves this by giving each request its own isolated context tree — no shared mutable state.

---

**Q5:** The body dump middleware captures request and response bodies. Why is this registered LAST (after correlation ID and request logger), not first?

**Expected Answer:** By the time the body dump middleware runs (after correlation ID and request logger), the context already has the enriched logger with `correlation_id`. When the body dump logs the captured bodies, those log lines automatically include `correlation_id` — linking the body dump output to the same request's other logs. If it ran first, the correlation ID wouldn't be in the context yet and body dump logs would be orphaned.

---

### b) "What If?" First-Principles Challenges

**What if context-based logging did NOT exist?**

You would have to pass a logger as a parameter to every function that might log. Consider a request that touches 10 functions across 3 layers (HTTP handler → service → repository). Each of those 10 functions would need `logger *slog.Logger` in its signature. Functions that don't log at all would still need to accept and pass the logger through, just so the function they call can log.

Worse: if the logging format or enrichment needs to change (e.g., add a new field to every log line), you'd change the logger creation in one place, but every function signature and call site remains polluted with the logger parameter. The parameter never goes away.

Context-based logging collapses this to zero extra parameters: the logger travels in the context that's already there, and only the functions that actually log call `FromContext(ctx)`.

---

**What if correlation IDs did NOT exist?**

Each service would log independently with no shared identifier. When a request fails, you'd have to:
1. Identify the approximate time range.
2. Look at logs from all services in that range.
3. Try to match log lines by timestamp — but timestamps drift across servers and requests overlap.
4. Hope that the combination of user ID + timestamp + request type narrows it down to one request.

In practice, at any meaningful scale (thousands of requests per minute), this is effectively impossible. You'd be guessing. The correlation ID makes log correlation an O(1) operation: one search, one result set, the complete picture.

---

**What if the `gen_` prefix did NOT exist on fallback correlation IDs?**

Fallback IDs (generated when no header is present) would look identical to real propagated IDs. You'd have no way to distinguish "this ID was propagated correctly from the originating client" from "this ID was generated as an emergency fallback because propagation failed." Silent failures in correlation ID propagation would be invisible. The `gen_` prefix makes the failure mode immediately observable — you can write a log alert for any `gen_` prefix appearing in production and instantly know something upstream is broken.

---

### c) Page FAQ

**Q: Why use `os.Stderr` for log output instead of `os.Stdout`?**
The reason this works this way is that `Stdout` and `Stderr` are two separate output streams by design. `Stdout` is meant for the program's actual data output (results, responses). `Stderr` is meant for diagnostic output (errors, logs). By keeping them separate, you can redirect or suppress each independently in production — for example, piping `Stdout` to a file and `Stderr` to a log aggregation system — without them interfering.

**Q: Why is the context key type unexported (`ctxKey`)?**
The reason this works this way is that Go's `context.WithValue` uses equality comparison on keys. If two packages both use the plain string `"logger"` as a key, they collide — one overwrites the other. An unexported custom type (`type ctxKey struct{}`) is invisible outside its package — no other package can create a value of that type, so no other package can accidentally use the same key. The key is effectively namespace-private.

**Q: Why does `.With("key", val)` return a new logger instead of modifying the existing one?**
The reason this works this way is that loggers are designed to be safe to use from multiple concurrent goroutines (parallel request handlers). If `.With` modified the logger in place, two concurrent handlers sharing the same logger would corrupt each other's fields. Returning a new logger means each handler gets its own enriched copy — no shared mutable state, no race conditions.

**Q: Why does the middleware set the `Correlation-ID` response header?**
The reason this works this way is that the caller (browser, another service, a developer using `curl`) can then see the exact correlation ID assigned to their request. If something goes wrong and they contact support, they provide the `Correlation-ID` value from the response. Support staff search logs by that ID and immediately see the complete request trace — without needing any other identifying information.

**Q: Why is the logging setup called a "walkthrough exercise" with no code to write?**
The reason this works this way is that logging infrastructure is shared by every module in the project. Writing it yourself from scratch would mean building on an unstable foundation for all future exercises. The platform pre-builds it and walks you through understanding it, so that from this point forward you can *use* it confidently — the exercises will test your ability to apply `log.FromContext(ctx)` and structured log calls, not to re-implement the infrastructure.

---

## SECTION 7 — JARGON BUSTER DICTIONARY

---

**Term: `context.Context`**
First-Principles Origin: Go functions needed a way to carry request-scoped data and cancellation signals through call chains without adding parameters to every function.
Meaning: A read-only "envelope" that travels with a request through every function call, carrying key-value data, deadlines, and cancellation signals.
Analogy: A hotel guest's keycard envelope — the front desk puts everything in it (room number, meal voucher, checkout time), and every staff member can look inside without the guest handing each item separately.
Example:
```go
ctx := context.Background()                         // Start with an empty context
ctx = context.WithValue(ctx, "key", "some value")  // Put a value in the envelope
val := ctx.Value("key")                             // Retrieve it anywhere ctx travels
```
Don't Confuse With: `context.Background()` — that's just the empty starting context; `context.WithValue` is what enriches it with data.

---

**Term: Correlation ID**
First-Principles Origin: In systems with multiple services, engineers needed one token that would appear in every log line for a single user request, regardless of which service or function processed it.
Meaning: A unique string assigned to a request the moment it arrives, attached to every log line that request touches — including in other services it calls.
Analogy: A parcel tracking number — one number, every scan from warehouse to doorstep, full history in one search.
Example:
```go
correlationID := "abc-789"                            // Could be from header or generated
logger := slog.Default().With("correlation_id", correlationID) // Attach to logger
logger.Info("processing request")
// Output: INFO processing request  correlation_id=abc-789
```
Don't Confuse With: A business entity ID (like `order_id`) — the correlation ID tracks the *HTTP request*, not the business object. One order might span multiple requests, each with different correlation IDs.

---

**Term: `gen_` prefix**
First-Principles Origin: Engineers needed a visual signal in logs to distinguish fallback-generated IDs (indicating a propagation failure) from properly propagated IDs.
Meaning: A prefix added to correlation IDs that were generated as fallbacks when no `Correlation-ID` header was found in the incoming request.
Analogy: A package arriving at a sorting facility without a tracking label — the facility prints a temporary label starting with "TEMP-" so staff know this one wasn't tracked from the source.
Example:
```go
// In CorrelationIDFromContext:
return "gen_" + shortuuid.New()   // "gen_" signals this was a fallback
// vs a normal ID: "abc-789" (no prefix = properly propagated)
```
Don't Confuse With: A regularly generated correlation ID — those have no prefix and indicate normal origin. `gen_` specifically means the propagation chain broke.

---

**Term: HTTP Middleware**
First-Principles Origin: Web applications needed a way to run the same logic (logging, auth, rate limiting) for every request without duplicating code in every handler.
Meaning: A function that wraps an HTTP handler, running code before and/or after the handler processes a request.
Analogy: A hotel's front door — every guest passes through security, check-in, and room assignment before reaching their room. The rooms themselves don't do security checks.
Example:
```go
func LoggingMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {  // Returns a new handler
        fmt.Println("Before handler")    // Runs before the actual handler
        err := next(c)                   // Call the actual handler
        fmt.Println("After handler")     // Runs after the actual handler
        return err
    }
}
```
Don't Confuse With: A handler — a handler is the final destination that processes the request; middleware wraps around handlers and never IS the final processing.

---

**Term: Key-Value Pair**
First-Principles Origin: Computers needed a way to store and retrieve named pieces of information — like a labeled filing cabinet — where any value could be found by its name instantly.
Meaning: A pair of a name (the "key") and its associated value. `"order_id": "ord_123"` — the key is `order_id`, the value is `ord_123`.
Analogy: A name badge at a conference — the badge has "Name: Alice", "Company: Acme". Each label (key) paired with its content (value).
Example:
```go
slog.Info("order placed", "order_id", "ord_123", "amount", 29.99)
// Key-value pairs: order_id=ord_123, amount=29.99
```
Don't Confuse With: A plain sentence in a log — `"order ord_123 placed for $29.99"` is a string, not key-value pairs. It's unstructured and hard to search.

---

**Term: Middleware Pipeline / Chain**
First-Principles Origin: Multiple cross-cutting concerns (logging, auth, rate limiting) needed to compose without each knowing about the others.
Meaning: Multiple middlewares registered in sequence, where each one wraps the next, forming a chain. Each calls `next(c)` to pass control forward.
Analogy: An assembly line — each station does its job and passes the item to the next station. The final station is the handler (packaging).
Example:
```go
e.Use(MiddlewareA)  // Runs first
e.Use(MiddlewareB)  // Runs second
e.Use(MiddlewareC)  // Runs third, then calls handler
```
Don't Confuse With: Independent functions — middleware runs IN ORDER as a chain; each depends on the previous completing first.

---

**Term: `slog` (structured logging package)**
First-Principles Origin: Go needed a standard library logging package built around structured key-value output instead of plain text, to support production log aggregation tools.
Meaning: Go's standard library package (since v1.21) for writing structured log messages with key-value fields, severity levels, and timestamps.
Analogy: A structured incident report form at work — instead of writing a free-text email about the incident, you fill in labeled fields: Date, Severity, Affected System, Error. Structured forms are searchable; emails are not.
Example:
```go
slog.Info("order placed", "order_id", "ord_123")  // Structured
// vs:
fmt.Println("order placed: ord_123")               // Unstructured
```
Don't Confuse With: `log` (Go's older standard library package) — `log` writes plain text; `slog` writes structured key-value output.

---

**Term: `slog.SetDefault`**
First-Principles Origin: Programs needed one central logging configuration that all code — including third-party packages — would use without being passed a logger explicitly.
Meaning: Sets the global default `slog.Logger` for the entire program. Any call to `slog.Info(...)`, `slog.Error(...)` etc. uses this logger.
Analogy: Setting the default language on your phone — every app uses it unless it has its own override. You set it once; everything else uses it.
Example:
```go
logger := slog.New(myHandler)  // Create a configured logger
slog.SetDefault(logger)        // Make it the global default
// Now anywhere in the program:
slog.Info("hello")             // Uses myHandler automatically
```
Don't Confuse With: `log.ToContext` — `SetDefault` sets the GLOBAL default for the whole program; `ToContext` stores a logger in a SPECIFIC request's context.

---

**Term: Structured Logging**
First-Principles Origin: Production systems processing millions of log lines needed machine-readable, filterable log output instead of free-text sentences.
Meaning: Writing log messages as collections of named fields (key-value pairs) rather than plain human sentences, so log aggregation tools can search, filter, and alert on specific fields.
Analogy: Filling out a customs form at the airport — instead of writing "I have some electronics and food worth about $300", you fill labeled fields: Item Type / Electronics, Value / $250; Item Type / Food, Value / $50. Customs can sort and search the forms.
Example:
```go
// Unstructured — hard to search:
fmt.Println("ERROR: order ord_123 failed: connection refused at 14:23")

// Structured — machine-searchable:
slog.Error("order failed", "order_id", "ord_123", "error", "connection refused")
// → ERROR order failed  order_id=ord_123  error="connection refused"
```
Don't Confuse With: Pretty-printing — structured logging is about machine-parseable fields, not just making output look nice.

---

**Term: `shortuuid`**
First-Principles Origin: UUIDs (standard unique IDs) are 36 characters long with hyphens, making them unwieldy in URLs and log output; a more compact encoding of the same entropy was needed.
Meaning: A library that generates short, URL-safe unique identifiers — the same uniqueness as a UUID but in a much shorter string (e.g., `6ka7f2Th8`).
Analogy: A parcel tracking code — could be a 128-bit binary number internally, but displayed as a short alphanumeric string like `UK123456789GB` for human use.
Example:
```go
id := shortuuid.New()  // e.g. "6ka7f2Th8xQpR3mL"
// Compare to a UUID: "550e8400-e29b-41d4-a716-446655440000"
```
Don't Confuse With: A UUID — a UUID is the long standard format; shortuuid is a compact encoding. Same uniqueness, different length.

---

## SECTION 8 — RETENTION & REVISION PLAN

### a) The "Right Now" Rule — Do This Before Closing the Page

1. **Re-write from memory:** Write `FromContext` and `ToContext` from scratch on paper, including the unexported key type. While writing, say out loud why each line is necessary.
2. **Reconstruct from axioms:** Without looking at notes, state in plain English the 5 axioms of structured logging. Check them against Section 3 Concept 1.
3. **Trace a request:** On paper, write the log output you'd see for a `POST /orders` request going through CorrelationID middleware → RequestLogger middleware → a handler that logs `"processing order"`. Include every field in every log line.
4. **Debug the axiom:** Look at this code: `logger.With("order_id", id); logger.Info("done")`. Identify which axiom is being violated and what the fix is. (Answer: `.With()` return value is discarded — the `Info` call logs without `order_id`. Fix: `enriched := logger.With("order_id", id); enriched.Info("done")`)

---

### b) 3-Day Revision Checklist (Axiom-Level Mastery)

- [ ] I can write the `Init` function (slog setup) from memory, explain its 5 axioms, and state why `humanslog` is used for development but not production — all without notes.
- [ ] I can write the `FromContext` / `ToContext` pair from memory, explain why an unexported key type is necessary, and state why `context.WithValue` creates a new context — all without notes.
- [ ] I can explain `CorrelationIDFromContext` — including the `gen_` prefix decision — from first principles, without notes.
- [ ] I can describe the three middlewares in order, explain why that exact order matters, and trace a request through all three — all without notes.

---

### c) 7-Day Challenge

**Scenario:** Design (on paper, no running code required) a complete logging system for a new module — `notifications`. This module sends email and SMS notifications when orders are placed.

**Requirements:**
- Every function that logs must use the correlation ID from the incoming request.
- Log lines must include: `notification_type` (email/sms), `recipient_id`, and any errors.
- There must be no logger parameter in any function signature beyond the handler.
- If the function is called without a context that has a logger, it must not crash.

**Success looks like:** A written implementation blueprint with every step labeled with the axiom it relies on, plus sample log output for a successful email notification.

**Start here:** Axiom 3 of context-based logging — "Attributes from middleware are available to every function that receives the context." This determines your function signatures.

---

### d) 30-Day Connection Bridge

- **Future handler code** will use `log.FromContext(ctx).With("entity_id", id).Info("event")` — every time you do, you are applying Axiom 3 of context-based logging and Axiom 2 of structured logging simultaneously.
- **Future debugging** of failed exercises will use the correlation ID in the `tdl` test output to trace what happened — the shared axiom is "a unique ID makes distributed debugging tractable."
- **Future middleware additions** (authentication, rate limiting) will follow the same pattern as the logging middleware — same Axiom 3 (middleware runs before handler) and Axiom 4 (don't touch business logic).
- **Future repository code** (database queries) will also receive `ctx context.Context` and log failures — the shared axiom is "context is the carrier of all per-request infrastructure."
- **Production incident response** in any future role will involve searching for a correlation ID — this page's concepts directly map to real-world on-call debugging workflows.

---

### e) Flashcard Set

**Card 1**
FRONT: Why does structured logging (slog) exist? What problem does plain `fmt.Println` create that it doesn't?
BACK: `fmt.Println` produces unstructured text that machines cannot parse or filter. Axiom 2: key-value pairs are machine-filterable; plain sentences are not. In production with 50,000 log lines, searching `order_id=ord_123` returns only relevant lines instantly; searching for "ord_123" in a sentence might match unrelated occurrences.

**Card 2**
FRONT: Why is `slog.SetDefault` called once at startup, not once per request?
BACK: Axiom 4 — all logs should go through one consistent system. `SetDefault` sets the global default for the ENTIRE program. Calling it per-request would create race conditions (two requests trying to set different defaults simultaneously) and inconsistent formatting.

**Card 3**
FRONT: Why does `context.WithValue` return a NEW context instead of modifying the existing one?
BACK: Contexts are immutable by design. If `WithValue` modified in place, two concurrent goroutines holding references to the same context would see each other's changes — a race condition. Immutability makes contexts safe to share and pass freely across concurrent code.

**Card 4**
FRONT: What would happen if you used a `string` as a context key instead of an unexported type?
BACK: Any other package that also uses the same string key (e.g., `"logger"`) would collide with yours — their `WithValue` call would overwrite your logger. Axiom 5: the unexported key type is invisible to other packages — no collision possible.

**Card 5**
FRONT: Why does `FromContext` fall back to `slog.Default()` instead of returning `nil` or panicking?
BACK: Axiom 4 of context-based logging — the fallback must be non-crashing. The global default always exists (set at startup). Returning `nil` would panic on the first `.Info(...)` call. The application degrades gracefully with default logging rather than crashing.

**Card 6**
FRONT: What does a `gen_` prefixed correlation ID in production logs tell you?
BACK: It tells you that `CorrelationIDFromContext` did not find an ID in the context, so it generated a fallback. This means an upstream service or client did NOT set the `Correlation-ID` header on the outgoing request. It's a visible signal that Axiom 3 of correlation IDs (ID must travel with the request) was violated somewhere upstream.

**Card 7**
FRONT: Why must the correlation ID middleware be registered BEFORE the request logger middleware?
BACK: Axiom 5 of middleware: middlewares run in registration order. The request logger logs each request — but to log the `correlation_id` field, it needs the logger with `correlation_id` already attached. That logger is only in the context AFTER the correlation ID middleware runs. Register correlation ID first → logger enriched → request logger logs with `correlation_id`.

**Card 8**
FRONT: What is the difference between `log.ToContext` (stores logger in context) and `slog.SetDefault` (sets global default)?
BACK: `slog.SetDefault` affects the ENTIRE program — every call to `slog.Info(...)` anywhere uses it. `log.ToContext` stores a logger inside ONE specific request's context — only code that calls `log.FromContext(ctx)` on THAT context gets that logger. One is global; one is per-request.

**Card 9**
FRONT: What problem does the middleware pattern solve that putting logging code in every handler does not?
BACK: Axiom 2 of middleware — duplicating code in every handler creates maintenance problems. With middleware, the logging/correlation ID code lives in ONE place. When the format changes, ONE file changes. With handler-embedded code, 20 handlers each need updating. Middleware also guarantees consistency — every handler gets the SAME setup with zero risk of forgetting it.

**Card 10**
FRONT: What would happen if the `HandleError` function in Echo used `slog.Default().Error(...)` instead of `log.FromContext(c.Request().Context()).Error(...)`?
BACK: The error log line would be missing the `correlation_id` and all other request-specific fields. Axiom 3 of context-based logging: request-specific attributes live in the context's logger, not the global default. The global default is the bare logger from `Init` — it has no `correlation_id`. Using `FromContext` retrieves the per-request enriched logger.

---

## SECTION 9 — QUICK REFERENCE CHEAT SHEET

### Structured Logging (`slog`)

```go
// Setup (once, at startup):
slog.SetDefault(slog.New(handler))

// Log at different severity levels:
slog.Debug("msg", "key", val)
slog.Info("msg", "key", val)
slog.Warn("msg", "key", val)
slog.Error("msg", "key", val, "error", err)

// Multiple fields:
slog.Info("order placed", "order_id", "ord_123", "amount", 29.99)
```

**Key rules:**
- Always use key-value pairs — never just a message string alone.
- Always include `"error", err` when logging errors.
- `SetDefault` is called ONCE at startup, not per request.

---

### Context-Based Logging

```go
// Store enriched logger in context (middleware does this):
ctx = log.ToContext(ctx, enrichedLogger)

// Retrieve logger anywhere you have ctx:
logger := log.FromContext(ctx)

// Add a field to this specific log call (returns new logger — don't discard!):
enriched := logger.With("order_id", orderID)
enriched.Info("processing")

// In a handler:
ctx := c.Request().Context()
log.FromContext(ctx).Info("event", "key", val)
```

**Key rules:**
- Never pass logger as a function parameter — use `log.FromContext(ctx)`.
- `.With(...)` returns a NEW logger — always assign the return value.
- `FromContext` never panics — falls back to `slog.Default()`.

---

### Correlation ID

```go
// Retrieve from context (middleware stores it):
id := log.CorrelationIDFromContext(ctx)

// gen_ prefix = fallback generated (propagation broke somewhere upstream)
// No prefix = properly propagated correlation ID
```

**Key rules:**
- Correlation ID is set in middleware, not in handlers.
- It's attached to every log line by the middleware-enriched logger.
- Always inject it into outgoing HTTP request headers.
- Alert on `gen_` in production — it signals upstream propagation failure.

---

### Middleware

```go
// Register (order matters!):
e.Use(CorrelationIDMiddleware)  // FIRST — enriches context
e.Use(RequestLoggerMiddleware)  // SECOND — logs (needs correlation ID)
e.Use(BodyDumpMiddleware)       // THIRD — captures bodies

// Middleware shape:
func MyMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // [before handler]
        err := next(c)  // ALWAYS call next (unless intentionally blocking)
        // [after handler]
        return err
    }
}
```

**Key rules:**
- Middleware runs in registration order.
- Always call `next(c)` unless intentionally blocking (e.g., auth failure).
- Middleware is for cross-cutting concerns — NOT business logic.
- Code after `next(c)` runs after the handler returns (reverse cleanup).

---

### Comparison: When to Use What

| Situation | Tool |
|---|---|
| Log a business event in a handler or service | `log.FromContext(ctx).Info("event", "field", val)` |
| Log an error | `log.FromContext(ctx).Error("failed", "error", err, "entity_id", id)` |
| Set up logging for all requests | Middleware with `log.ToContext` |
| Get the correlation ID for an outgoing call | `log.CorrelationIDFromContext(ctx)` |
| Configure the global log format | `slog.SetDefault(slog.New(handler))` once at startup |
| Add a field only to this one log line | `logger.With("field", val).Info("event")` |
