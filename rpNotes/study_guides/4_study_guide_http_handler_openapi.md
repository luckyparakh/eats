# Study Guide: HTTP Handler — OpenAPI-First Design with oapi-codegen

> **Page:** "HTTP Handler" (introduces the `orders` module's first real endpoint, spec-first via OpenAPI + oapi-codegen)
> **Track:** backend-masterclass-beta, module `03-http`, exercise `01-http-handler`
> **Previous guide:** `3_study_guide_logging.md` — that page wired up structured logging, context propagation, and correlation IDs on top of the Echo/middleware stack from Service Scaffolding. This page adds the first real HTTP contract on top of that same stack, in the `orders` module specifically.
> **What this likely unlocks next:** implementing `RegisterCustomer` against a real service/repository layer — this page only gets you to a compiling, wired, but presumably still-trivial handler.
>
> **Update note:** This guide was first written before `task gen` had been run — the generated-file claims below were tagged `[From page]` only. Generation has since run: `backend/orders/api/http/openapi.gen.go` now exists (223 lines), and `handler.go` now has a stub `RegisterCustomer` implementation (though `Register` is still unwired). Every claim below has been re-verified against the real generated file and re-tagged `[From code]` where confirmed. Two things surfaced that neither the page nor the original guide mentioned — see the end of "Generated `StrictServerInterface` & Typed Responses" and the new **Current Workspace State** note in Section 3.

---

## 1. Snapshot

- **Topic:** Defining the `orders` module's first HTTP endpoint (`POST /orders/register-customer`) by writing an OpenAPI spec first, generating Go types/interfaces/routing from it via `oapi-codegen`, and implementing the generated `StrictServerInterface` instead of hand-writing Echo routes.
- **Objective:** After this page, I can explain why the OpenAPI spec is the source of truth instead of the Go code, trace how `oapi-codegen.yaml`'s config flags map to what gets generated, and explain why returning the wrong response type for an endpoint is a compile error here instead of a runtime bug.
- **Continuity:** This is the first concrete work inside `backend/orders/api/http` — the same package whose `Handler`/`Register` scaffolding was already wired into `orders.Module.RegisterHttp` back in the Service Scaffolding guide, just empty until now.

## 2. Concepts

### The `api` Package as Module Entry Point

- **Problem:** [From page] A module needs a well-defined boundary for how the outside world (frontend, mobile apps, internal tools, or — per Service Scaffolding — other modules) reaches it. Without a designated entry-point layer, HTTP concerns and business logic have no natural separation.
- **Mechanism:** [From page + code] `backend/orders/api/http` is explicitly framed as "the first layer of the module" — the entry point, with more layers to come later. Right now it holds exactly the OpenAPI definitions and HTTP handler code; nothing else.
- **Design Decision:** [From page] The alternative is putting HTTP routing/handling logic directly wherever business logic lives, with no dedicated `api` boundary. [AI explanation] Keeping the API layer distinct means the module's internal implementation (once it grows repositories, services, etc.) can change without touching how external callers interact with it — the contract lives in one place.
- **In Production:** [AI explanation] Nothing to observe yet — the module has no business logic beneath this layer at this stage. Worth revisiting once repository/service layers are introduced, to see whether the boundary holds.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Treating `api/http` as "just where handlers live" and putting business logic there too | The entry-point layer stops being a clean boundary; business logic becomes coupled to HTTP concerns | It's the first (and currently only) code in the module, so it's tempting to keep growing it | Keep `api/http` limited to translating HTTP ↔ typed requests/responses; push logic into layers introduced later |

### OpenAPI-First Design

- **Problem:** [From page] Defining endpoints directly in code means the contract only exists implicitly, scattered across handler code — frontend and backend teams have no single shared source of truth, and there's no automatic documentation.
- **Mechanism:** [From code, `backend/orders/api/http/openapi.yaml`] The spec defines the contract declaratively — paths, request/response schemas, status codes — before any Go code exists for it:

  ```yaml
  paths:
    /orders/register-customer:
      post:
        operationId: registerCustomer
        requestBody:
          required: true
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/RegisterCustomer'
        responses:
          '201':
            description: Customer registered successfully
            content:
              application/json:
                schema:
                  $ref: '#/components/schemas/RegisterCustomerResponse'
          '400':
            $ref: '#/components/responses/BadRequest'
          '409':
            description: Customer already exists
            content:
              application/json:
                schema:
                  $ref: '#/components/schemas/ErrorResponse'
  ```
  Schemas like `RegisterCustomer` (required fields: `name`, `email`, `address`, `phone_number`, with field-level constraints like `maxLength`) and `Address` (required: `line1`, `line2`, `postal_code`, `city`, `country_code`) are defined once, then referenced (`$ref`) from both the request body and reused across responses (`ErrorResponse`/`ErrorDetails` are shared by the `BadRequest`/`Unauthorized`/`Forbidden`/`NotFound`/`Gone` reusable response components — most of which aren't used by this endpoint yet, but exist as shared building blocks for the module).
- **Design Decision:** [From page] The alternative is code-first (define routes/types in Go directly). The page's stated tradeoffs for spec-first: the spec becomes the shared contract between backend/frontend, documentation comes free from tools that read OpenAPI directly (e.g. Swagger UI), and regenerating code after a spec change turns breaking changes into compile errors instead of runtime surprises. [AI explanation] Code-first would be the better choice for a tiny, single-team service where the overhead of maintaining a separate spec file outweighs the contract/documentation benefits — but for a module meant to be called by "the frontend web app, mobile apps, and possibly some internal tools" (per the page), a shared spec is exactly the scenario spec-first is designed for.
- **In Production:** [AI explanation] The main production-relevant property is the one named on the page: a spec change that removes or renames a field becomes a compile error in every place that consumed the old generated type, rather than a silent runtime mismatch (e.g. a frontend sending an old field name that the backend silently ignores).
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Editing Go handler signatures directly to "fix" a mismatch instead of updating the spec | The spec (the supposed source of truth) drifts from what the code actually does, defeating the entire point of spec-first | Feels faster in the moment than editing YAML and regenerating | Always change `openapi.yaml` first, regenerate, then adjust the implementation to match the new generated interface |
  | Forgetting to regenerate after editing `openapi.yaml` | Code still compiles against the *old* generated interface — the spec and the running code silently disagree | Regeneration is a separate manual step (`task gen`), not automatic on save | Run `task gen` (or `go generate ./...`) immediately after any spec change, before touching handler code |

### The `oapi-codegen` Config & Generation Command

- **Problem:** [From page] Hand-writing the marshaling/routing boilerplate that turns an OpenAPI spec into working Go code is repetitive and error-prone to keep in sync with the spec by hand.
- **Mechanism:** [From code, `backend/orders/api/http/oapi-codegen.yaml`]:

  ```yaml
  package: http
  generate:
    echo-server: true
    strict-server: true
    models: true
  output: openapi.gen.go
  ```
  `package: http` names the generated file's package (matching this directory's `package http`); `echo-server: true` generates Echo-compatible routing code; `strict-server: true` generates `StrictServerInterface` (typed request/response objects instead of raw `echo.Context`); `models: true` generates Go structs for every OpenAPI schema (`Address`, `RegisterCustomer`, `ErrorResponse`, etc.); `output: openapi.gen.go` names the target file.

  The actual generation trigger, [From code, `backend/orders/api/http/openapi.go`]:

  ```go
  package http

  //go:generate go tool oapi-codegen --config=oapi-codegen.yaml openapi.yaml
  ```
  `go tool` (Go 1.24+) runs a tool declared as a dependency in `go.mod` without a separate global install — [From code] confirmed: `go.mod`'s `tool (...)` block lists `github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen`, and `go.mod` also has `go 1.25.0`, well past the 1.24 minimum the page mentions for this syntax. Running `task gen` (from `Taskfile.yml`, covered in the Project Setup guide) or `go generate ./...` executes this directive, reading `openapi.yaml` + `oapi-codegen.yaml` and writing `openapi.gen.go` in the same directory.
- **Design Decision:** [From page] The alternative is installing `oapi-codegen` as a separate global binary (the more traditional approach for Go code-generation tools before `go tool` existed). [AI explanation] `go tool`'s advantage is that the exact generator version is pinned in `go.mod` per-project, so every contributor (and CI) generates identical code without needing a separately-managed global tool version.
- **In Production:** [From page] `openapi.gen.go` is explicitly "Never edit this file" — a generated-file convention. [AI explanation] If someone did hand-edit it, the next `task gen` run would silently overwrite their changes, since the generator has no way to know custom edits were made.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Hand-editing `openapi.gen.go` | Any manual edit is silently destroyed the next time someone runs `task gen` | The file is right there, and a quick fix looks tempting | Change `openapi.yaml` (or `oapi-codegen.yaml`) and regenerate instead |
  | Assuming `oapi-codegen` needs a separate global install | Confusion when `go generate` "just works" without an install step anyone remembers doing | Habit from tools that predate `go tool` | Recognize `go tool <name>` as running a `go.mod`-pinned dependency, not a system-wide binary |

### Generated `StrictServerInterface` & Typed Responses

- **Problem:** [From page] Without a typed contract, a handler could return a response shape or status code that doesn't match what the spec promises, and nothing would catch that until a client noticed at runtime.
- **Mechanism:** [From code, `backend/orders/api/http/openapi.gen.go` — confirmed after running `task gen`] The key generated interface, matching the page exactly:

  ```go
  type StrictServerInterface interface {
      RegisterCustomer(ctx context.Context, request RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error)
  }
  ```
  Note there's no mention anywhere in this signature of the path (`POST /orders/register-customer`) — routing is handled entirely by generated code based on the spec's `operationId`/paths, not by anything the implementer writes. Response types follow the naming convention `{OperationID}{StatusCode}JSONResponse`, confirmed in the generated file:

  ```go
  type RegisterCustomer201JSONResponse RegisterCustomerResponse
  type RegisterCustomer400JSONResponse struct{ BadRequestJSONResponse }
  type RegisterCustomer409JSONResponse ErrorResponse

  type RegisterCustomerResponseObject interface {
      VisitRegisterCustomerResponse(w http.ResponseWriter) error
  }
  ```
  `RegisterCustomer201JSONResponse` does have the `CustomerUuid CustomerUUID` field the page describes, where `CustomerUUID = openapi_types.UUID` (a type alias, generated because the spec's `CustomerUUID` schema is `type: string, format: uuid` — `format: uuid` is what triggers the special type instead of a plain `string`). The same pattern applies to `email`: the spec's `format: email` on `RegisterCustomer.email` generates `Email openapi_types.Email`, not a plain string — but `CountryCode`, which has no `format` at all, generates as a plain alias `type CountryCode = string`. **Format-bearing fields get a distinct generated type; plain `string` fields with no `format` don't**, a detail the page never spells out.

  **One structural detail the page doesn't mention:** `RegisterCustomer400JSONResponse` isn't a simple alias like the other two — it's `struct{ BadRequestJSONResponse }`, wrapping `type BadRequestJSONResponse ErrorResponse`. That's because the spec's `400` response uses `$ref: '#/components/responses/BadRequest'` (a reusable *response* component), while `409` inlines `$ref: '#/components/schemas/ErrorResponse'` directly as its content schema. Referencing a shared response component versus inlining a schema produces a different generated shape — concrete proof that the spec's structure, not just its data, drives what code comes out.

  Each response type implements `VisitRegisterCustomerResponse` itself — e.g. `RegisterCustomer201JSONResponse.VisitRegisterCustomerResponse` sets the `Content-Type` header, calls `w.WriteHeader(201)`, then JSON-encodes itself. The return type `RegisterCustomerResponseObject` is itself a generated interface — only the exact set of response types the spec defines (201/400/409, per this spec) implement it, so returning an undeclared status code (e.g. a 204) is a **compile error**, not a runtime one.
- **Design Decision:** [From page] The named alternative is the plain (non-strict) `ServerInterface`, which hands the implementer a raw `echo.Context` and leaves JSON parsing/validation/response encoding to hand-written code. `strict-server: true` trades that flexibility for the typed-objects, compile-time-checked approach — the page explicitly recommends implementing `StrictServerInterface`, not `ServerInterface`.
- **In Production:** [From page] The compile-time safety is the headline production benefit: "If you try to return a 204 No Content response but the spec only defines 201, 400, and 409, the code won't compile" — an entire category of contract-violation bugs becomes impossible to ship rather than something you'd discover from a client bug report.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Implementing `ServerInterface` instead of `StrictServerInterface` | Loses typed request/response objects and the compile-time status-code safety — back to raw `echo.Context` handling | Both interfaces get generated; picking the wrong one still compiles fine on its own | Always implement `StrictServerInterface` per the page's explicit guidance |
  | Trying to return a status code the spec doesn't declare for that operation | Compile error (a good failure mode, but confusing if you don't know why) | Assuming any valid HTTP status is always returnable, as with a hand-written handler | Add the status code to the spec's `responses` first, regenerate, then return the newly-generated response type |

### Wiring the Handler

- **Problem:** [From code, `backend/orders/api/http/handler.go` — current workspace state] `RegisterCustomer` is now implemented (a stub), but `Register` still does nothing — the module compiles, but the endpoint still isn't reachable:

  ```go
  type Handler struct{}

  func NewHandler() Handler {
      return Handler{}
  }

  func (h *Handler) RegisterCustomer(ctx context.Context, request RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error) {
      return RegisterCustomer201JSONResponse{}, nil
  }

  func Register(ctx context.Context, e common.EchoRouter, handler Handler) error {
      return nil
  }
  ```
- **Mechanism:** [From code, confirmed in `openapi.gen.go`] Per the page, wiring the generated routing to the actual Echo router takes one call inside `Register`:

  ```go
  RegisterHandlers(e, NewStrictHandler(handler, nil))
  ```
  `RegisterHandlers(router EchoRouter, si ServerInterface)` and `NewStrictHandler(ssi StrictServerInterface, middlewares []StrictMiddlewareFunc) ServerInterface` are both confirmed in the generated file — `NewStrictHandler` wraps a `StrictServerInterface` implementation into a `*strictHandler`, which itself implements `ServerInterface` (see the previous concept for the full bridge). The `nil` second argument is a slice of strict-server middleware, unused here.
- **Design Decision:** [AI explanation] This is a thin, mechanical wiring step by design — all the actual routing logic (matching `POST /orders/register-customer` to the `RegisterCustomer` method) lives in generated code, so `Register` only needs to connect three things: the router, the strict-handler wrapper, and the concrete `Handler` value.
- **In Production:** [From code — a real gotcha visible right now in this file, not from the page] `RegisterCustomer` is defined with a **pointer receiver** (`func (h *Handler) RegisterCustomer(...)`), but `Register`'s parameter is `handler Handler` (a value, not `*Handler`), and `orders/module.go`'s `Init` stores `m.httpHandler` as a plain `Handler` value too. Go's method sets mean a *value* of type `Handler` does **not** include pointer-receiver methods — only `*Handler` does. As written today, writing `NewStrictHandler(handler, nil)` inside `Register` (with `handler Handler`) would **fail to compile**: `Handler` doesn't satisfy `StrictServerInterface`, only `*Handler` does. This is the compile-time contract enforcement the page promises, caught here even before the wiring line is added — it's the exact "if you try to return a 204 but the spec doesn't declare it, it won't compile" idea from the page, applied one layer earlier (a whole implementation not satisfying the interface, not just one wrong response type).
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Calling `RegisterHandlers` with the raw `handler` instead of `NewStrictHandler(handler, nil)` | Type mismatch — `RegisterHandlers` expects the plain `ServerInterface`, and a `StrictServerInterface` implementation doesn't satisfy that directly | Easy to skip the wrapper step when the two interfaces look superficially similar | Always wrap a strict-server implementation with `NewStrictHandler(...)` before passing it to `RegisterHandlers` |
  | Defining strict-interface methods on `*Handler` while passing `Handler` (value) around elsewhere (module field, `Register` parameter) | Compile error the moment the value is passed anywhere expecting `StrictServerInterface` — method set of a value type excludes pointer-receiver methods | Easy to default to a pointer receiver out of habit without checking how the type is stored/passed elsewhere in the same module | Keep the receiver type consistent with how the value flows through the module — either pointer-receiver methods with a `*Handler` threaded through everywhere, or value-receiver methods matching the `Handler` value already in use |

## 3. Plumbing Dissection

> **Current Workspace State:** `task gen` has now run — `openapi.gen.go` exists (223 lines) and is fully verified below. `handler.go` has a `RegisterCustomer` stub (`return RegisterCustomer201JSONResponse{}, nil`) but `Register` still returns `nil`, unwired. The wiring map below reflects this actual in-progress state, not a hypothetical.

**Wiring map (verified against real generated + handler code):**

```
orders.Module.RegisterHttp(ctx, e)        (backend/orders/module.go — from Service Scaffolding)
  └─ http2.Register(ctx, e, m.httpHandler)  (backend/orders/api/http/handler.go)
        └─ currently: return nil            → RegisterHandlers/NewStrictHandler not yet called,
                                                endpoint still unreachable despite RegisterCustomer existing

go:generate directive (openapi.go) → task gen / go generate ./... → openapi.gen.go (confirmed, 223 lines)
      ├─ models: Address, ErrorResponse, ErrorDetails, RegisterCustomer, RegisterCustomerResponse, ...
      │     CountryCode = string (plain alias, no `format` in spec)
      │     CustomerUUID = openapi_types.UUID (format: uuid)
      │     Email openapi_types.Email (format: email)
      ├─ ServerInterface { RegisterCustomer(ctx echo.Context) error }              ← what Echo routes to
      ├─ StrictServerInterface { RegisterCustomer(ctx, RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error) }  ← what we implement
      ├─ response types: RegisterCustomer201JSONResponse / 400 (wraps BadRequestJSONResponse) / 409
      ├─ RegisterHandlers(router, si) → router.POST("/orders/register-customer", wrapper.RegisterCustomer)
      └─ NewStrictHandler(ssi, middlewares) → *strictHandler (implements ServerInterface by
              calling ctx.Bind, then ssi.RegisterCustomer, then response.VisitRegisterCustomerResponse)

Handler{} (value) → NewHandler()
  (h *Handler) RegisterCustomer(...)  ← pointer receiver, stub implemented
  Register(ctx, e, handler Handler)   ← still `return nil`; wiring this per the page,
                                          as currently typed, would NOT compile (see below)
```

- **Per plumbing piece:** The `Register` function is the single seam between this module's `api/http` package and the shared `common.EchoRouter` (from Service Scaffolding) — everything above it (module init order, contracts, `Init`/`RegisterContracts`/`Verify`/`RegisterHttp` sequencing) is unchanged from the Service Scaffolding guide; this page only fills in what happens *inside* `RegisterHttp`'s call to `Register`.
- **Non-obvious lines:**
  - The `go:generate` line's location — a standalone `openapi.go` file whose only content is a package declaration and the generate directive — is a common Go convention: a dedicated file purely to host `//go:generate` comments, rather than attaching the directive to an arbitrary existing file.
  - `openapi.gen.go` defines its **own** `EchoRouter` interface (line 111) — a second, separate interface from `common.EchoRouter` (Service Scaffolding guide), with an identical method set (`CONNECT`/`DELETE`/`GET`/.../`TRACE`, same signatures). `RegisterHandlers` takes the *generated* package's `EchoRouter`, not `common.EchoRouter` — but because Go interfaces are satisfied structurally (not by name), a `common.EchoRouter` value passed into `Register` as `e` still satisfies the generated `EchoRouter` type without any explicit conversion. Two interfaces, never unified, working together only because their shapes happen to match exactly.
  - `handler.go`'s current receiver mismatch (see the Wiring the Handler concept above) — as of right now, adding the page's exact wiring line inside `Register` would fail to compile, because `handler Handler` is a value and `RegisterCustomer` has a pointer receiver.
- **What a senior notices:**
  - The spec already defines shared response components (`Unauthorized`, `Forbidden`, `NotFound`, `Gone`) that the single current endpoint doesn't use at all — these exist as forward-looking shared building blocks for the module's future endpoints, not because `register-customer` needs them.
  - Two structurally-identical-but-separately-declared `EchoRouter` interfaces (one in `common`, one generated per-package) is a small duplication cost of the oapi-codegen approach — every module ends up with its own copy of this interface, which only stays compatible with `common.EchoRouter` because both were written by hand to have the same method set. Nothing enforces that they stay in sync except convention.

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a blank scratch Go module, rebuild the core OpenAPI-first plumbing from this page:
- A minimal `openapi.yaml` with one `POST` endpoint, one request schema, and at least two response status codes (success + one error).
- An `oapi-codegen.yaml` config with `echo-server`, `strict-server`, and `models` all enabled.
- A `go:generate` directive using `go tool oapi-codegen` (add `oapi-codegen` to your scratch module's `tool` block first).
- A hand-written `Handler` implementing the generated `StrictServerInterface`, wired into an `echo.Echo` instance via `RegisterHandlers(e, NewStrictHandler(handler, nil))`.

**Acceptance criteria:**
1. `task gen`-equivalent (`go generate ./...`) produces a file containing `StrictServerInterface` with exactly one method matching your spec's `operationId`.
2. Your `Handler` compiles only once it returns one of the exact generated response types for a status code your spec declares — prove this by temporarily returning an undeclared status code's shape and confirming it fails to compile.
3. A real HTTP request to your endpoint (via `curl` or a Go test) returns the JSON shape your spec's success schema defines, with the correct status code.
4. A request with a malformed body demonstrates whatever automatic parsing/validation the strict server provides, without you writing any manual JSON-decoding code in your handler.
5. (Failure case) Add a new required field to your spec's response schema and regenerate *without* updating your handler — confirm it now fails to compile, demonstrating the "breaking changes show up as compile errors" claim directly.

<details><summary>Hint</summary>Start with the smallest possible spec (one path, one 200-ish response) and confirm generation + wiring works end-to-end before adding more schemas or status codes.</details>
<details><summary>Hint</summary>`go tool` requires the tool's module to be listed in your `go.mod`'s `tool (...)` block — run <code>go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen</code> (or your Go version's equivalent) before the generate directive will find it.</details>
<details><summary>Hint</summary>For criterion 2, the compile error will point at your `return` statement trying to satisfy an interface (`RegisterXResponseObject`) — read what type it expected versus what you gave it, that's the whole mechanism.</details>

**Compare step:** After finishing, diff against `backend/orders/api/http/openapi.yaml`, `oapi-codegen.yaml`, and `openapi.go` (and, once you've run `task gen` for real in the actual exercise, the generated `openapi.gen.go`), and ask yourself:
1. Did my spec reuse a shared schema (like `ErrorResponse`) across multiple status codes the way this project's spec does, or did I duplicate error shapes per endpoint?
2. Did I correctly distinguish `ServerInterface` from `StrictServerInterface` in my own generated output, or did I only ever look at one of them?
3. Does my `Register`-equivalent function do anything beyond the two-line wiring call, and if so, is that extra logic actually necessary at this layer?

## 5. Before the Exercise

- **Likely task:** [From page — stated directly, not a guess] The page says outright: "Your job is to add the `RegisterCustomer` method and wire it up," and even gives the exact wiring line (`RegisterHandlers(e, NewStrictHandler(handler, nil))`). So the concrete task is: implement `RegisterCustomer` on `Handler` so it satisfies `StrictServerInterface`, and update `Register` to call `RegisterHandlers`/`NewStrictHandler` instead of returning `nil`.
- **Approach:**
  1. Run `task gen` (or `go generate ./...`) first, and actually open the resulting `openapi.gen.go` — confirm `StrictServerInterface`'s exact method signature and the generated response type names before writing any implementation code.
  2. Add a `RegisterCustomer` method to `Handler` matching that signature.
  3. Decide what the method should actually do at this stage — the page doesn't show a full implementation, so check what response(s) make sense given there's no persistence layer yet (this module's `Init` doesn't wire a repository).
  4. Update `Register` to call `RegisterHandlers(e, NewStrictHandler(handler, nil))` instead of `return nil`.
  5. Confirm the service still compiles and starts (`task up`), and that hitting `POST /orders/register-customer` reaches your new method instead of 404ing.

## 6. Recall Questions

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

*(These questions, with answers, have also been appended to `notes/review_queue.md` under today's date.)*

## 7. Cheat Sheet

- OpenAPI-first flow: edit `openapi.yaml` → `task gen` (runs `go generate ./...` → `go tool oapi-codegen --config=oapi-codegen.yaml openapi.yaml`) → implement/update `StrictServerInterface` → wire via `RegisterHandlers(e, NewStrictHandler(handler, nil))`.
- `oapi-codegen.yaml` flags: `echo-server` (Echo-compatible routing), `strict-server` (typed request/response objects — implement this one), `models` (Go structs for every schema).
- Response naming convention: `{OperationID}{StatusCode}JSONResponse` — only types generated for status codes declared in the spec satisfy the operation's `...ResponseObject` interface. Returning an undeclared status code is a compile error.
- Never hand-edit generated files (`openapi.gen.go`) — the next `task gen` run silently overwrites any manual change.
- Two generated interfaces exist: `ServerInterface` (raw `echo.Context`, what Echo actually routes to) and `StrictServerInterface` (typed objects, what you implement) — `NewStrictHandler` is the generated adapter bridging strict → plain. Implement the strict one.
- Format-aware codegen: a schema field with `format: uuid`/`format: email` generates a distinct type (`openapi_types.UUID`/`openapi_types.Email`); a plain `string` with no `format` generates as a plain `type X = string` alias. Referencing a shared `#/components/responses/...` component (vs. inlining a schema) also changes the generated response type's shape (struct-wrapped vs. plain alias) — the spec's structure, not just its data, drives generated code shape.
- Gotcha (real, not hypothetical): every module's generated file declares its own `EchoRouter` interface, separate from `common.EchoRouter` — they only interoperate because both happen to declare the identical method set; nothing enforces that they stay in sync.
- Gotcha (real, not hypothetical): if a `StrictServerInterface` method is declared with a pointer receiver, every place that value is stored/passed (module struct field, `Register`'s parameter) must also use a pointer, or `NewStrictHandler(...)` won't compile — Go method sets don't include pointer-receiver methods on a value type.
- ID convention in this codebase: `shortuuid` (compact, URL-safe) for log correlation IDs; `google/uuid` (`uuid.New()`, standard UUID format) for entity IDs like customer UUIDs — different tools for different purposes, not interchangeable by convention.
- Decision rule: use OpenAPI-first when multiple consumers (frontend, mobile, other services) need a shared, documented, versionable contract; code-first is reasonable only when that contract-sharing benefit doesn't apply.
