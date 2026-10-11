# Study Guide: Error Handling — Transport-Agnostic Errors, Slugs, `common.Error`

> **Page:** "Error Handling". **Track:** backend-masterclass-beta, module `07-errors-and-testing`, exercise `01-error-handling`.
> **Previous guide:** `11_study_guide_application_layer_and_dedicated_uuids.md` — `app` is the layer that should say *what* went wrong; this page defines how.
> **Companion lab (full request path, handlers line by line, use-case flows with real outputs, drill with expected results):** `12b_study_guide_error_handling_code_flow_lab.md`.
> **Tags:** [From page], [From code], [Verified by running] (ran in a scratch module outside the repo), [AI explanation]. Code marked *illustrative* is not in the repo.
>
> **5-minute path:** Snapshot → Concept 1 (two audiences) → Section 5 → Section 8. Everything else is for after the exercise.
>
> **Workspace state (checked directly):**
> - `git log` shows `7e6865c`/`04a0c39` for `07-errors-and-testing/01-error-handling`. I haven't read the exercise text (`project/exercise.md` is stale), so the task in Section 5 is a guess.
> - The scaffold adds `common/errors.go`, `common/errors_echo.go`, `errors_echo_test.go`. `go test ./backend/common/...` passes today.

---

## 1. Snapshot

- **Topic:** One `common.Error` type (status + public message + slug + internal cause + per-field details), four constructors, and an Echo handler that turns it into JSON.
- **Objective:** After this page, I can return a typed error from `app` with a stable slug and field details, explain how an error travels from the service to the JSON response, and say what the client gets for a `common.Error`, an `*echo.HTTPError` and any other error.
- **Continuity:** Likely next: testing slugs as an API contract (the page says "later") and a repository that attaches DB errors via `WithInternalError`.

## 2. Concepts

### 1. Transport-agnostic errors and slugs — an error has two audiences

- **Problem:** [From page] If `app` returns `echo.NewHTTPError(400, ...)`, a Pub/Sub consumer or CLI gets a meaningless HTTP error, and every failure reaches the frontend as the same generic 500.
- **Mechanism:** When something fails, two people need different things:

  | Audience | Needs | Example |
  |---|---|---|
  | **Client** (frontend, user) | What *they* should do. Safe, stable. | "Name cannot be empty", slug `empty-name` |
  | **Operator** (you at 3am) | What actually broke. Full detail. | `pq: connection refused at 10.0.0.5:5432` |

  `common.Error` holds both in one struct and the edge decides who sees what. Two rules follow: **(1)** the layer that knows what happened creates the error (`app` knows the name was empty, not what HTTP is); **(2)** the layer that knows how to say it translates (the HTTP edge). A **slug** (`empty-name`, `customer_not_found`) is the greppable ID the frontend maps to UI text: a stable contract.
- **Design Decision:** Alternatives: numeric codes (opaque), sentinel errors with `errors.Is` (fine inside Go, but no client message). Slugs aren't enforced: [From page] the page mixes `empty-name` and `customer_not_found`, so pick one style before the contract exists. [From code] `common.Error` still stores `HttpErrorCode` (`errors.go:60-87`), so a non-HTTP adapter would need to map from it: pragmatic, not fully protocol-neutral.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Returning `echo.NewHTTPError` from `app` | Non-HTTP callers get a meaningless error | Echo is already imported in the handler | `app` returns `common.Error`; the edge translates |
  | Renaming a slug in a refactor | Frontend shows the wrong message, no compile error | Slugs are plain strings | Treat as public constants; change only with a contract test |

### 2. `common.Error` — value type, builders, details

- **Problem:** A user with three mistakes shouldn't submit three times, and internal causes must be logged but never shown.
- **Mechanism:** [From code] `errors.go:8-16`:
  ```go
  type Error struct {
      HttpErrorCode int
      PublicError, ErrorSlug string // sent to the client
      InternalError error           // logs only
      Details       []ErrorDetails  // per-field
  }
  ```
  Constructors: `NewInvalidInputError` 400, `NewUnauthorizedError` 401, `NewNotFoundError` 404, `NewExpiredError` 410, all `(slug, format string, a ...any)`. `WithDetails` / `WithInternalError` return a new value. The log gets the *whole* struct (`Error()` includes slug, internal cause, details); the response gets a projection (`HttpErrorResponse`) without `InternalError`.
- **Design Decision:** Value semantics let you build a base error once and enrich it per call. Costs, [From code]:
  1. **`*common.Error` is not matched.** `Error()` has a value receiver, so `&common.Error{}` compiles as an `error`, but `errors.As` with a value target misses it and the client gets a generic 500.
  2. **No `Unwrap()`.** `InternalError` isn't in the error chain, so `errors.Is/As` can't reach it. Classify the cause *before* wrapping.
  3. **No 409 constructor,** yet `openapi.yaml` declares a 409 for `registerCustomer`. The test builds it as a raw struct (`errors_echo_test.go:97`).
- **Use Case & Flow — empty name and phone (multi-error validation).** *Illustrative* code in `app.Service.RegisterCustomer`:
  ```go
  var details []common.ErrorDetails
  if strings.TrimSpace(c.Name) == "" {
      details = append(details, common.ErrorDetails{
          EntityType: "customer", EntityID: c.CustomerUUID.String(), // promoted from embedded UUID (guide 11)
          ErrorSlug: "empty-name", Message: "Name cannot be empty",
      })
  }
  // ... same for phone ("empty-phone-number")
  if len(details) > 0 {
      return common.NewInvalidInputError("invalid_customer_data", "Invalid customer data").WithDetails(details)
  }
  ```
  Collect all failures, then return once. The service picks the *category* and never writes `400`. The client gets `400 {"message":"Invalid customer data","slug":"invalid_customer_data","details":[{"entity_type":"customer","entity_id":"...","error_slug":"empty-name",...}, ...]}` and the frontend switches on `error_slug` to highlight fields. The operator sees the full error in the log.
- **Use Case & Flow — database is down.** *Illustrative* fix for `customer_repo.go:36`, which drops the error: `if err := queries.InsertCustomer(ctx, args); err != nil { return fmt.Errorf("insert customer: %w", err) }`. It's a plain error, so the client gets a generic 500 (`internal_server_error`) and learns nothing about your DB; the log has the full cause. The `Correlation-ID` response header and the log line share one ID, so a user reporting that ID lets you grep the cause. **Rule: if the caller can't fix it, return a plain wrapped error.** Use `WithInternalError` when it *is* a client-visible category but you also want the cause logged (e.g. a 409 struct literal plus the pg error).
- **In Production:** Today a failed insert returns 201 (guide 11). Test with a table test through the handler, and assert the body never contains the `InternalError` text.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Returning `&common.Error{...}` | Silent generic 500 | A pointer feels natural | Always return the value type |
  | Returning only the first validation failure | User submits N times | Early `return` is simplest | Collect all, return one error with `Details` |

### 3. `EchoErrorHandler` — how the error gets to the response

- **Problem:** Echo's default handler knows nothing about slugs; something must map `error` → status, body and log line.
- **Mechanism — how Echo catches it.** [From code] Echo v4.15.1 `ServeHTTP` (`echo.go:676-677`):
  ```go
  if err := h(c); err != nil {      // h = middlewares + your handler
      e.HTTPErrorHandler(err, c)    // whatever is stored in e.HTTPErrorHandler
  }
  ```
  Go has no exceptions: the error is a return value passed up the stack and checked once at the top. `common/http/echo.go:19` puts `HandleError` there today; the page wants `common.EchoErrorHandler`.
- **Mechanism — `errors.As` and the "empty struct".** [From code] `echo.go:35-37`:
  ```go
  httpErr := &echo.HTTPError{}          // a pointer variable pointing at a blank struct
  if errors.As(err, &httpErr) {         // &httpErr = address of the pointer variable
      httpCode = httpErr.Code
  }
  ```
  **Misconception:** `errors.As` fills the blank struct. **Reality:** it walks `err` and everything it wraps (`Unwrap()`) looking for a `*echo.HTTPError`, and if found **repoints your variable** at it; the blank struct is thrown away. If nothing matches it returns `false` and the defaults (500) stay.
  ```
  Before: httpErr ──▶ {Code:0}      Chain: fmt.Errorf("saving: %w", ─▶ *HTTPError{Code:404})
  After:  httpErr ──────────────────────────────────────────────────▶ {Code:404}
  ```
  [Verified by running] `before: Code=0`, `after: ok=true Code=404`, and the pointer differs from the blank struct. In `EchoErrorHandler` the target is a *value* (`var commonErr Error`), so `As` copies the found `common.Error` into it.
- **Mechanism — translation order.** [From code] `errors_echo.go:40-76`: default 500 → if `*echo.HTTPError` in the chain, take its code → if `common.Error` in the chain, its non-empty fields **override** message, slug and status. Unknown errors keep the safe default.
  [Verified by running] with `HandleError` (today) both a plain error and a `common.Error` give `500 {"error":"Internal server error"}`; with `EchoErrorHandler` the plain error gives `500 ... internal_server_error` and the `common.Error` gives `400 ... invalid_customer_data`.
- **Use Case & Flow — bad `country_code`.** Already works at scaffold level [Verified by running; step-by-step in the lab]: `body {"country_code":"XX"}` → `CountryCode.UnmarshalText` → `common/enum.go:65` returns `NewInvalidInputError("invalid-enum-value", ...)` → strict handler `ctx.Bind` fails and `return err` (`openapi.gen.go:204`) → Echo's `Bind` wraps it as `NewHTTPError(400, ...).SetInternal(err)` → `EchoErrorHandler` finds the `*echo.HTTPError` (400), then `errors.As` reaches the `common.Error` through `HTTPError.Unwrap()` → slug `invalid-enum-value` overrides `bad_request`. Client gets `400 {"message":"invalid enum value for shared.CountryCodeType: 'XX', expected values [...]","slug":"invalid-enum-value"}` (today: `400 {"error":"...Slug: invalid-enum-value"}`).
- **Design Decision:** A central handler keeps handlers at `return nil, err`; the alternative is typed `400JSONResponse`s per route. Cost: the contract lives twice. [From code] `HttpErrorResponse` is hand-written, while `openapi.yaml` marks `details` **required** and Go omits it when empty ([Verified by running]: a 400 with no details has no `details` key).
- **In Production:** [From code] `errors_echo.go:20` logs every error, 4xx included, at `Error` level (alert noise); a committed response returns at `:14` with no log. [Verified by running] The "Request done" log line shows `status=200` and an empty body for a failed request, because the error handler runs after the middlewares return. Don't alert on that field.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Testing `EchoErrorHandler` directly but not that `NewEcho` installs it | Green tests, old 500-for-everything behavior in prod | The provided tests call the function directly | A test through `NewEcho()` that expects a slug |
  | Logging 4xx at `Error` | Alert fatigue, real 5xx hidden | One log line for all is simplest | `Warn`/`Info` for client errors, `Error` for 5xx |

## 3. Plumbing Dissection

**Wiring map** [From code]:

```
common/http/echo.go  NewEcho(): useMiddlewares(e) → e.HTTPErrorHandler = HandleError   ← page: common.EchoErrorHandler
backend/svc.go:32,64 e := NewEcho() → modules Init → module.RegisterHttp(e)
orders/module.go:71  http2.Register → handler.go:48 RegisterHandlers(e, NewStrictHandler(handler, nil))
openapi.gen.go:139   router.POST("/orders/register-customer", wrapper.RegisterCustomer)      ← the route registration
```

**Request path: `POST /orders/register-customer`** [From code] (compact; the full 12-row station table, startup tree and data-shape diagram are in the lab, Part 1):

| # | Station | Where | What happens |
|---|---|---|---|
| 1 | Echo routing | Echo `echo.go:667-677` | Finds the route, runs `h(c)`; on error calls `HTTPErrorHandler` |
| 2 | Middlewares | `common/http/middlewares.go:28-29,36,55,87-128` | Timeout 10s → Recover → Correlation-ID (read/create, logger in ctx, response header) → request log ("Request done", logged *before* the error handler runs) |
| 3 | Wrapper + strict handler | `openapi.gen.go:103-109,200-222` | `:204` `ctx.Bind` (**bad JSON / `country_code` fail here**); `:210` calls your handler |
| 4 | Handler | `handler.go:21-45` | `:25` address mapping; `:30-36` builds `app.Customer`; `:38` calls service; errors returned as `nil, err` |
| 5 | Service | `app/customer.go:26-28` | Pass-through; **validation belongs here** |
| 6 | Repository | `customer_repo.go:26-38` | `:29-35` builds `InsertCustomerParams`; `:36` insert (**error dropped**) |
| 7 | sqlc / Postgres | `dbmodels/customers.sql.go` | `INSERT INTO orders.customers ...` |

The data changes type at each border: JSON → `http.RegisterCustomer` (3) → `app.Customer` (4) → `dbmodels.InsertCustomerParams` (6) → row. **Success:** `RegisterCustomer201JSONResponse` (`handler.go:42`) → `VisitRegisterCustomerResponse` (`openapi.gen.go:221`). **Error:** any station returns `err` → strict handler `return err` (`:218`) → middlewares → Echo → `HTTPErrorHandler`.

| Failure | Origin | Today's result |
|---|---|---|
| Malformed JSON, bad `country_code` | 3 (`Bind`) | 400 `{"error": "..."}` |
| Invalid address | 4 (`handler.go:25-27`) | Plain error → 500 |
| Validation (empty name) | 5 | Not implemented yet |
| DB failure | 6 (`:36`) | **Dropped** → returns nil → 201 |

**Not yet working** [confirmed in code]:

| Page describes | Repo today |
|---|---|
| `e.HTTPErrorHandler = common.EchoErrorHandler` | `echo.go:19` still `HandleError`; `EchoErrorHandler` used only by its test |
| Validation in `app` returning `Details` | None in `app.Service.RegisterCustomer` |
| Repo surfaces DB errors | `customer_repo.go:36` drops the error |

- **Non-obvious lines:**
  - `common/http/echo.go:48`: `panic(err)` in the old `HandleError` panics with the *original* error, not the write failure. The replacement logs instead (`errors_echo.go:22-24`).
  - `errors_echo.go:66`: `HttpErrorDetail(detail)` is a struct conversion, so it compiles only while both structs have identical fields: a free guard against drift.
  - `common/enum.go:65`: the scaffold changed `UnmarshalText` to return a `common.Error`; that's what gives a bad `country_code` a slug.
- **What a senior notices:** Translation policy is centralized, but the response contract is duplicated and already differs from the spec on `details`; fix with a contract test, not by hand.

## 4. Rebuild Challenge

Not yet: your `faq.md` says once per module, and module 07 isn't finished. When you do it, in a scratch module: an error type with slug, public message, internal cause, details and a *kind* (not an HTTP code), plus two translators (HTTP and a fake RPC) the service doesn't import. Prove: 3 invalid fields give one 400 with 3 details; a plain error gives a generic 500 without leaking its text; a `*Error` pointer is handled by a deliberate, documented choice; a test through the real router proves the translator is installed.

## 5. Before the Exercise

- **Likely task:** [AI explanation, inferred from the scaffold diff; the real task is in the CLI] Install `common.EchoErrorHandler` in `NewEcho` (replacing `HandleError`, `echo.go:19`), and add validation to `app.Service.RegisterCustomer` returning `NewInvalidInputError("invalid_customer_data", ...).WithDetails(...)` with per-field slugs like `empty-name`.
- **Approach:**
  1. Read the exercise text in the CLI and the test it grades against first.
  2. Make the one-line wiring change; decide what to do with the unused `HandleError`.
  3. Validate in `app`, not the handler: collect all failures, return one error with `Details` (`entity_type` `customer`, UUID as `entity_id`).
  4. Decide whether returning the dropped error at `customer_repo.go:36` is in scope now or later.
  5. Hit the endpoint with an empty name and phone and compare to the page's JSON; also try `country_code: "XX"`.

## 6. Build the Intuition

Intuition comes from predicting, running and being wrong a few times, not from reading more.

- **Three questions for any function that can fail:** (1) Who is the audience: the client who can fix it, or the operator who must debug it? (2) Who knows the cause, and who knows how to say it? (creator = `app`, translator = the edge) (3) What is the safe default? (unknown → generic 500, detail in the log).
- **Design checklist for a new failure:**

  | Question | Example |
  |---|---|
  | Can the caller fix it? | Yes, they sent an empty name |
  | Category and slug (permanent) | Invalid input → `NewInvalidInputError`, `empty-name` |
  | Log-only detail | the underlying DB error |
  | How will I test it? | Assert status and slug through the real server |
- **30-minute drill on the real exercise** (write the prediction *before* each run; the expected result for every step is in the lab, Part 5): (1) send a bad request with `HandleError`, note the body, switch `echo.go:19` to `common.EchoErrorHandler`, send again; (2) return `&common.Error{...}` from the service and predict the status; (3) put the DB error text into `PublicError`, see it reach the client, then fix it; (4) breakpoints at `openapi.gen.go:204`, `handler.go:25`, `handler.go:38`, `customer_repo.go:36`, send `"country_code":"XX"` and watch it stop at `Bind`, then find the `Correlation-ID` header in the log.
- **Test as design tool** *(illustrative)*: `resp := post(t, `{"name":"","phone_number":""}`)`, then assert 400, slug `invalid_customer_data`, 2 details. Written first, it also catches a missing `echo.go:19` change.

## 7. Recall Questions

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

*(These six are in `rpNotes/review_queue.md`, plus four more answered by Concepts 1-3 above: how Echo catches an error, what `errors.As` does to the blank struct, why `Error()` includes `InternalError` but the JSON doesn't, and how to find the cause of a 500 by Correlation-ID.)*

## 8. Cheat Sheet

- **Rule:** `app` returns `common.Error` for expected failures and a wrapped plain error for unexpected ones (→ generic 500). The edge translates. `app` never imports Echo.
- **Install:** `e.HTTPErrorHandler = common.EchoErrorHandler` (currently `HandleError`, `common/http/echo.go:19`).
- **`errors.As(err, &target)`:** repoints `target` at the first match in the chain; target type decides what matches (`Error` value vs `*Error` pointer).
- **Gotchas:** return `common.Error` by value; `InternalError` isn't reachable via `errors.Is/As`; no 409 constructor; 4xx logged at `Error`; "Request done" shows `status=200` for failures; `details` is required in OpenAPI but omitted when empty; `customer_repo.go:36` still drops the insert error.
- **Decision rules:** caller can fix it → `common.NewXxxError` (add `WithInternalError` to keep the cause in logs); caller can't → wrapped plain error. Validation → collect all failures into `Details`. Slugs → one style, never renamed without a contract test.
