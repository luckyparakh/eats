# Code Flow Lab: Error Handling — From `POST /orders/register-customer` to the JSON Response

> **Companion to:** `12_study_guide_error_handling.md` (concepts, mistakes, cheat sheet). This lab is the slow, concrete part: the full request path through this repo, a line-by-line read of the error handlers, three use cases with flows and real outputs, and a drill where every step has an **expected result you can check your prediction against**.
> **Tags:** [From code] read in this repo or the Echo source. [Verified by running] executed; output shown. [AI explanation] reasoning, not executed. *Illustrative* = not in the repo today.
> **How the outputs were produced:** a scratch module outside the repo (a `replace` directive pointing at `project/`) sends real HTTP requests through the real `NewEcho()` (middlewares), the real strict handler, the real `orders` handler and the real `app.Service`. Only the database is replaced by a fake `app.CustomerRepository` whose behaviour is chosen by the customer's `name`. Postgres, migrations and sqlc were **not** involved, so the repository's SQL is not verified here.
> **Workspace state (2026-10-11):** `common/http/echo.go:19` still sets `HandleError`; `common.EchoErrorHandler` is not installed; no validation in `app.Service`; `customer_repo.go:36` drops the DB error.

## How to use this lab

| Time | Do |
|---|---|
| 10 min | Part 1 (request path) and Part 4's scenario table |
| 30 min | Add Part 2 (handlers line by line) and Part 3 (use cases) |
| Lab | Part 5: for each step write your prediction *first*, then compare with the expected result |

---

## Part 1 — The path of `POST /orders/register-customer` through this repo

### A. Startup: how the route gets registered (before any request)

```
cmd/main.go              signal ctx → pgxpool.New(POSTGRES_URL) → backend.New(ctx, pool)
  └─ backend/svc.go:32        e := commonHTTP.NewEcho()                    ← Echo + middlewares + error handler
  └─ backend/svc.go:64        module.RegisterHttp(ctx, e)                  ← each module registers its routes
       └─ orders/module.go:70-71     http2.Register(ctx, e, m.httpHandler)
            └─ api/http/handler.go:47-48    RegisterHandlers(e, NewStrictHandler(handler, nil))
                 └─ api/http/openapi.gen.go:139
                      router.POST("/orders/register-customer", wrapper.RegisterCustomer)   ← THE route line
  backend/svc.go:92        echoRouter.Start(":8080")
```

In `NewEcho` (`common/http/echo.go:18-19`): `useMiddlewares(e)` then `e.HTTPErrorHandler = HandleError`.

### B. One request, station by station

```
0 net/http → 1 Echo routing → 2 middlewares (2a..2d) → 3 wrapper → 4 strict handler
  → 5 Handler (http→app) → 6 app.Service → 7 CustomerRepository (app→SQL) → 8 sqlc / Postgres
```

| # | Station | Where | What happens |
|---|---|---|---|
| 0 | net/http server | Go stdlib, started at `svc.go:92` | Accepts the request, hands it to Echo |
| 1 | Echo routing | Echo `echo.go:667-677` | Finds `POST /orders/register-customer`, runs `h(c)`; if it returns an error, line 677 calls `e.HTTPErrorHandler` |
| 2a | Timeout | `middlewares.go:28`, `ContextTimeout(10s)` | 10-second deadline on the request context |
| 2b | Recover | `middlewares.go:29`, `Recover()` | Turns panics into errors |
| 2c | Correlation-ID | `middlewares.go:30-53` (ID read or created at `:36`) | Puts a logger with the ID into the context; sets the `Correlation-ID` response header |
| 2d | Request log | `middlewares.go:87-128` (registered at `:55`) | Restores the body, `next(c)` at `:102`, logs "Request done" at `:127`, returns `err` |
| 3 | Wrapper | `openapi.gen.go:103-109` | Calls `w.Handler.RegisterCustomer(ctx)`, where `Handler` is the strict handler |
| 4 | Strict handler | `openapi.gen.go:200-222` | `:204` `ctx.Bind(&body)`: **malformed JSON / bad `country_code` fail here**; `:210` calls your handler |
| 5 | Your handler | `handler.go:21-45` | `:23` new UUID; `:25` address mapping; `:30-36` builds `app.Customer`; `:38` calls the service; failures returned as `nil, err` |
| 6 | Service | `app/customer.go:26-28` | Pass-through today; **validation belongs here** |
| 7 | Repository | `customer_repo.go:26-38` | `:29-35` builds `InsertCustomerParams`; `:36` runs the insert (**error dropped**) |
| 8 | sqlc / Postgres | `dbmodels/customers.sql.go` | `INSERT INTO orders.customers ...` |

**The data changes type at each border** (guide 11's point):

```
JSON text
  → http.RegisterCustomer          (station 4, generated from openapi.yaml)
  → app.Customer                   (station 5, built by your handler)
  → dbmodels.InsertCustomerParams  (station 7, built by the repo)
  → SQL row
```

**Success path:** repo `nil` → service `nil` → handler returns `RegisterCustomer201JSONResponse{...}` (`handler.go:42`) → strict handler `VisitRegisterCustomerResponse(ctx.Response())` (`openapi.gen.go:221`, writes 201 + JSON) → request log → done. [Verified by running] `201 {"customer_uuid":"<uuid>"}`.

**Error path:** any station returns `err` → strict handler `return err` (`openapi.gen.go:218-219`) → middlewares pass it up (the request log adds it as an `error` attribute) → Echo `e.HTTPErrorHandler(err, c)`.

| Failure | Origin station | Result today (`HandleError`) |
|---|---|---|
| Malformed JSON, bad `country_code` | 4 (`Bind`) | 400 `{"error": "..."}` |
| Invalid address (`shared.NewAddress` fails) | 5 (`handler.go:25-27`) | Plain error, so 500 |
| Validation (empty name) | 6 | Not implemented yet |
| DB failure | 7 (`:36`) | **Dropped**: returns nil, so 201 |

### C. Two traps in this path

1. **The error handler runs after the middlewares return.** Echo calls it at the very top (`echo.go:676-677`), so "Request done" is written *before* the error response exists. [Verified by running] on the real route, for every error scenario below, both handlers:
   ```
   INFO  Request done correlation_id=... URI=/orders/register-customer status=200 ... error="insert customer: dial tcp 10.0.0.5:5432: connection refused" response_body=""
   ```
   The log says `status=200` and an empty `response_body` for a request the client saw fail. Don't alert on that line's `status`.
2. **Dropped error at `customer_repo.go:36`:** a failed insert never enters the error path, so the client gets 201 with a UUID that was never stored.

## Part 2 — The two handlers, line by line

### Today: `HandleError` (`common/http/echo.go:29-50`)

```go
func HandleError(err error, c echo.Context) {
    log...With("error", err).Error("HTTP error")   // logs every error at Error level

    httpCode := http.StatusInternalServerError     // default 500
    msg := any("Internal server error")            // default message

    httpErr := &echo.HTTPError{}                   // :35 pointer variable → blank struct
    if errors.As(err, &httpErr) {                  // only ONE kind of error is recognised
        httpCode = httpErr.Code                    // Echo's own HTTPError (e.g. from Bind)
        msg = httpErr.Message
    }

    jsonErr := c.JSON(httpCode, map[string]any{"error": msg})   // body shape: {"error": ...}
    if jsonErr != nil {
        panic(err)                                 // :48 panics with the ORIGINAL error
    }
}
```

Anything that isn't a `*echo.HTTPError` (a plain error, a `common.Error`) stays at the defaults. That is the page's sentence "every error reaches the user as an HTTP 500".

### After the fix: `EchoErrorHandler` (`common/errors_echo.go`)

```go
func EchoErrorHandler(err error, c echo.Context) {
    if c.Response().Committed { return }                       // :14 already started writing: return, NO log
    httpErrorResponse, httpStatus := httpErrorResponseFromErr(err)   // :18 pure function: error → (body, status)
    log...With("err", err).Error("Handling HTTP error")        // :20 full error to the log (all errors, 4xx too)
    if err := c.JSON(httpStatus, httpErrorResponse); err != nil {    // :22 write the response
        log...Error("Failed to send error response")           // :23 logs instead of panicking
    }
}

func httpErrorResponseFromErr(err error) (HttpErrorResponse, int) {
    publicError := "Internal Server Error"                     // :41 safe defaults first
    statusCode := http.StatusInternalServerError               // :42
    var he *echo.HTTPError
    if errors.As(err, &he) {                                   // :44-48 Echo's own error? take its code,
        statusCode = he.Code                                   //        message = http.StatusText(code)
        publicError = http.StatusText(statusCode)
    }
    errorSlug := strings.ToLower(strings.ReplaceAll(publicError, " ", "_"))   // :49 "Bad Request" → "bad_request"

    var commonErr Error                                        // :51 a VALUE target
    if errors.As(err, &commonErr) {                            // search the chain for common.Error and COPY it
        if commonErr.PublicError != "" { publicError = commonErr.PublicError }      // :53-55
        if commonErr.ErrorSlug != ""   { errorSlug = commonErr.ErrorSlug }          // :56-58
        if commonErr.HttpErrorCode != 0 { statusCode = commonErr.HttpErrorCode }    // :59-61
    }

    httpDetails := make([]HttpErrorDetail, 0, len(commonErr.Details))   // :64
    for _, detail := range commonErr.Details {
        httpDetails = append(httpDetails, HttpErrorDetail(detail))      // :66 struct conversion: fields must match
    }
    return HttpErrorResponse{Slug: errorSlug, Message: publicError, Details: httpDetails}, statusCode
}
```

Reading it as three layers: **default** (500, generic) → **Echo's error** overrides code and text → **`common.Error`** overrides message, slug and status. `InternalError` never enters the response. Only `err` (the whole thing) goes to the log at `:20`.

`errors.As` repoints the target variable at the first match in the chain (guide 12, Concept 3). Here the first target is a pointer type (`*echo.HTTPError`), the second a value type (`Error`), which is why `&common.Error{}` is *not* found.

## Part 3 — Three use cases with flows and real outputs

### Use case 1 — empty name and empty phone (multi-error validation)

*Illustrative* code in `app.Service.RegisterCustomer` (the lab simulates it with a repo that returns the same error):

```go
var details []common.ErrorDetails
if strings.TrimSpace(c.Name) == "" {
    details = append(details, common.ErrorDetails{
        EntityType: "customer",
        EntityID:   c.CustomerUUID.String(), // promoted from embedded UUID (guide 11)
        ErrorSlug:  "empty-name",
        Message:    "Name cannot be empty",
    })
}
if strings.TrimSpace(c.PhoneNumber) == "" {
    details = append(details, common.ErrorDetails{ /* ... "empty-phone-number" ... */ })
}
if len(details) > 0 {
    return common.NewInvalidInputError("invalid_customer_data", "Invalid customer data").WithDetails(details)
}
return s.customerRepository.RegisterCustomer(ctx, c)
```

Flow: service creates the error (station 6) → handler `return nil, err` (`handler.go:38-39`) → strict handler `return err` (`:218`) → middlewares → Echo → error handler.

- **Collect, then return:** one response lists every mistake. The top-level error is the *category* (`invalid_customer_data`); `Details` say *which fields*. The service never writes `400`; `NewInvalidInputError` (`errors.go:66-72`) sets it.
- **Client sees** [Verified by running]:
  - today (`HandleError`): `500 {"error":"Internal server error"}` (the structured error is ignored);
  - after the fix: `400 {"message":"Invalid customer data","slug":"invalid_customer_data","details":[{"entity_type":"customer","entity_id":"<uuid>","error_slug":"empty-name","message":"Name cannot be empty"},{"entity_type":"customer","entity_id":"<uuid>","error_slug":"empty-phone-number","message":"Phone number cannot be empty"}]}`.
  The frontend runs `switch (detail.error_slug) { case "empty-name": highlight(nameInput) }`.
- **Operator sees** [Verified by running]: `ERROR Handling HTTP error correlation_id=... err="Invalid customer data, Slug: invalid_customer_data, DocumentData: [{customer <uuid> empty-name Name cannot be empty} {customer <uuid> empty-phone-number ...}]"`.

### Use case 2 — the database is down

*Illustrative* fix for `customer_repo.go:36`, which drops the error:

```go
if err := queries.InsertCustomer(ctx, args); err != nil {
    return fmt.Errorf("insert customer: %w", err)    // plain wrapped error
}
```

Flow: pgx error → repo wraps → service → handler → strict handler → Echo → error handler. Neither `errors.As` finds anything, so the defaults stay.

| Who | Sees [Verified by running] |
|---|---|
| **Client** (today) | `500 {"error":"Internal server error"}` |
| **Client** (after the fix) | `500 {"message":"Internal Server Error","slug":"internal_server_error"}`, nothing about the DB |
| **Operator** | `ERROR Handling HTTP error correlation_id=xKRG... err="insert customer: dial tcp 10.0.0.5:5432: connection refused"` |

The `Correlation-ID` **response header and the log's `correlation_id` are the same value** (verified in every scenario), so a user who reports the ID lets you grep the cause. **Rule: if the caller can't fix it, return a plain wrapped error.**

`WithInternalError` is for a category the client *can* act on, where you also want the cause logged. [Verified by running] a `NewInvalidInputError(...).WithInternalError(dialErr)`: client gets `400 {"message":"Invalid customer data","slug":"invalid_customer_data"}` (no cause); the log has `..., InternalError: dial tcp 10.0.0.5:5432: connection refused`. Remember `InternalError` is **not** in the error chain (no `Unwrap`), so decide the category from the cause *before* wrapping it.

### Use case 3 — bad `country_code` in the JSON body

Already works at scaffold level [Verified by running]. Body `{"address": {"country_code": "XX", ...}}`:

```
json decoder → CountryCode.UnmarshalText
 → common/enum.go:65 returns NewInvalidInputError("invalid-enum-value", ...)
 → strict handler ctx.Bind fails, `return err`                       (openapi.gen.go:204)   ← handler, service, repo never run
 → Echo's Bind wraps it: NewHTTPError(400, err.Error()).SetInternal(err)   (bind.go:88-95)
 → error handler:
     errors.As(*echo.HTTPError) → status 400
     errors.As(common.Error)    → found via HTTPError.Unwrap() → Internal   (EchoErrorHandler only)
```

| Handler | Client gets |
|---|---|
| `HandleError` (today) | `400 {"error":"invalid enum value for shared.CountryCodeType: 'XX', expected values [\"US\" \"DE\" \"GB\" \"JP\" \"PL\"], Slug: invalid-enum-value"}` |
| `EchoErrorHandler` | `400 {"message":"invalid enum value for shared.CountryCodeType: 'XX', expected values [\"US\" \"DE\" \"GB\" \"JP\" \"PL\"]","slug":"invalid-enum-value"}` |

Note how today's body glues `, Slug: invalid-enum-value` onto the text: Echo set the `HTTPError` message to `err.Error()`, which for a `common.Error` includes the slug. After the fix the slug is its own field and the message is clean.

## Part 4 — Scenario table (all verified by running through the real route)

The customer's `name` makes the fake repository behave differently; everything else is the real stack. "Log" is the line from `ERROR Handling HTTP error` (after) / `ERROR HTTP error` (today). Every error row also has a "Request done" line with `status=200`.

| # | What goes wrong | `HandleError` (today) | `EchoErrorHandler` (after fix) | Operator's log shows |
|---|---|---|---|---|
| 1 | Nothing (baseline) | `201 {"customer_uuid":"<uuid>"}` | same | "Request done status=201" |
| 2 | `country_code` = `XX` | `400 {"error":"...Slug: invalid-enum-value"}` | `400 {"message":"invalid enum value ... 'XX' ...","slug":"invalid-enum-value"}` | `code=400, message=invalid enum value ..., internal=...` |
| 3 | Validation error with 2 details | `500 {"error":"Internal server error"}` | `400` + `invalid_customer_data` + 2 details | `Invalid customer data, Slug: ..., DocumentData: [...]` |
| 4 | DB down (wrapped plain error) | `500 {"error":"Internal server error"}` | `500 ... internal_server_error` | `insert customer: dial tcp 10.0.0.5:5432: connection refused` |
| 5 | Repo returns `&common.Error{...}` (pointer) | `500 {"error":"Internal server error"}` | `500 ... internal_server_error` (**the pointer is not matched**) | `Bad thing, Slug: bad_thing` |
| 6 | `fmt.Errorf("register: %w", NewNotFoundError(...))` | `500 {"error":"Internal server error"}` | `404 {"message":"Customer abc not found","slug":"customer_not_found"}` | `register: Customer abc not found, Slug: customer_not_found` |
| 7 | DB text put into `PublicError` | `500 {"error":"Internal server error"}` | `400 {"message":"db said: dial tcp 10.0.0.5:5432: connection refused","slug":"leaky"}` (**leak**) | same text |
| 8 | `NewInvalidInputError(...).WithInternalError(dialErr)` | `500 {"error":"Internal server error"}` | `400 {"message":"Invalid customer data","slug":"invalid_customer_data"}` | `..., InternalError: dial tcp 10.0.0.5:5432: connection refused` |

Things the table shows: today **no** `common.Error` survives (rows 3, 5, 6, 7, 8 are all 500). After the fix a wrapper (`register:`, row 6) is invisible to the client but visible in the log. Rows 5 and 7 are the two mistakes the drill makes you commit.

## Part 5 — The drill: predict, run, compare

Write your prediction for each step *before* running. "Expected" values were verified in Part 4.

**Setup.** Either make the changes in your repo and hit the real endpoint with a database, or reproduce with a scratch module: a fake `app.CustomerRepository` (`RegisterCustomer(ctx, app.Customer) error`) wired through `commonHTTP.NewEcho()` and `ordershttp.Register(...)` using `httptest`. In a scratch module, `GOWORK=off GOFLAGS=-mod=mod GOPROXY=off`, a `replace eats => <path to project>`, and a copy of the project's `go.sum`.

| Step | Do | Predict | Expected result |
|---|---|---|---|
| 1 | Send `"country_code":"XX"` with `HandleError`, then switch `echo.go:19` to `common.EchoErrorHandler` and send again | Status and body for each? | Both `400`. Today the body is `{"error":"...Slug: invalid-enum-value"}`; after, `{"message":"...","slug":"invalid-enum-value"}`. The response has a `Correlation-ID` header and the log's `correlation_id` is the same value |
| 2 | Make the service/repo return `&common.Error{HttpErrorCode: 400, ErrorSlug: "bad_thing", PublicError: "Bad thing"}` (pointer) | Status? Slug? | `500` and `internal_server_error`, even though you meant 400. The *log* still says `Bad thing, Slug: bad_thing`. Fix [Verified by running]: return the value (`common.Error{...}`), then you get `400 {"message":"Bad thing","slug":"bad_thing"}` |
| 3 | Return `NewInvalidInputError("leaky", "db said: %s", "dial tcp 10.0.0.5:5432: connection refused")` | What does the client see? | `400 {"message":"db said: dial tcp 10.0.0.5:5432: connection refused","slug":"leaky"}`: the DB address reached the client. Fix [Verified by running]: constant message plus `.WithInternalError(err)`, then the client gets `400 {"message":"Invalid customer data",...}` and the log line ends with `InternalError: dial tcp 10.0.0.5:5432: connection refused` |
| 4 | Breakpoints (or log lines) at `openapi.gen.go:204`, `handler.go:25`, `handler.go:38`, `customer.go:27`, `customer_repo.go:36`. Send a valid body, then the `XX` body | Which stations are hit in each case? | Valid body: all five, in that order, response `201` (with a real DB). `XX` body: only `openapi.gen.go:204` runs (it returns the error); handler, service and repo are never reached [From code, consistent with the verified response]. Find the `Correlation-ID` header value in both "Request done" and "Handling HTTP error" log lines |

## Part 6 — Decision rule and check yourself

```
Is it the caller's fault, and can they fix it?
├─ yes → common.NewXxxError(slug, msg)         400 / 401 / 404 / 410
│        (cause needed in logs too? add .WithInternalError(err))
└─ no  → return fmt.Errorf("what I was doing: %w", err)   → generic 500
```

1. Using the scenario table, which two rows are the "mistakes" and what is the one-line fix for each?
   <details><summary>Answer</summary>Row 5 (pointer `&common.Error`): return the value. Row 7 (DB text in `PublicError`): constant public message plus `WithInternalError(err)`.</details>
2. If `app` returns `fmt.Errorf("register: %w", common.NewNotFoundError(...))`, what does the client get?
   <details><summary>Answer</summary>[Verified by running] `404 {"message":"Customer abc not found","slug":"customer_not_found"}`. `errors.As` walks through the wrapper and finds the `common.Error`; the wrapper's `register:` text appears only in the log.</details>
3. After you add `Unwrap() error { return c.InternalError }` to `common.Error`, which `EchoErrorHandler` line changes behaviour?
   <details><summary>Answer</summary>[AI explanation, not run] None for the normal case: `errors.As(err, &commonErr)` still finds the outer `Error` first. It changes only when `InternalError` itself is an `*echo.HTTPError` or another `common.Error` (the chain now continues into it), and it makes `errors.Is/As` on the cause work for callers.</details>
4. Why is `HttpErrorDetail(detail)` (`errors_echo.go:66`) a safe conversion, and what happens if you add a field to only one of the two structs?
   <details><summary>Answer</summary>A struct conversion needs identical field names and types (tags are ignored). Adding a field to only one struct makes it a compile error, which is a free guard against the two drifting.</details>
5. Why does "Request done" show `status=200` for every error row?
   <details><summary>Answer</summary>Echo calls `HTTPErrorHandler` only after the whole middleware chain has returned the error. The request-log middleware logs while the response is still unwritten, so it sees the default status and an empty body.</details>
