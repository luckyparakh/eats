# Study Guide: Custom OpenAPI Types — x-go-type, UUID v7, Shared Types, the Enum Pattern

> **Page:** "Custom OpenAPI Types" (mapping OpenAPI schemas to custom Go types via `x-go-type`, a custom `UUID`, the `shared` package, and a generic `Enum` pattern)
> **Track:** backend-masterclass-beta, module `03-http`, exercise `02-custom-openapi-types`
> **Previous guide:** `4_study_guide_http_handler_openapi.md` — that page got `RegisterCustomer` compiling and wired against generated types (`CountryCode = string`, `CustomerUUID = openapi_types.UUID`). This page's whole point is that those two generated aliases are too weak, and shows the machinery to replace them with real types.
> **What this likely unlocks next:** validating/persisting these stronger types once a repository layer exists — a `CountryCode` that's already guaranteed valid, and a `UUID` with `Scan`/`Value` ready for a database column.
>
> **Workspace state at time of writing:** `backend/common/uuid.go`, `common/enum.go`, `common/shared/country_code.go`, `common/shared/address.go`, and `common/shared/types.go` all already exist and are verified `[From code]` below — this infrastructure is pre-built. **`orders/api/http/openapi.yaml` does NOT yet have `x-go-type`/`x-go-type-import` extensions** — I checked; `CustomerUUID` and `CountryCode` schemas are still plain (`type: string, format: uuid` / `type: string`), and the currently-generated `openapi.gen.go` still has `type CountryCode = string` and `type CustomerUUID = openapi_types.UUID`. So the *wiring* this page describes (spec → custom type) is the task ahead of you, not something already done — everything about the `x-go-type` mapping itself is tagged `[From page]` below, not verified against your spec.

---

## 1. Snapshot

- **Topic:** Overriding oapi-codegen's generated types with your own via `x-go-type`/`x-go-type-import`; a custom `UUID` type built for UUID v7 instead of `google/uuid`'s default v4; a `common/shared` package for types genuinely shared across modules; and a generic `Enum[T]` pattern that makes invalid enum values unrepresentable.
- **Objective:** After this page, I can explain why a `[16]byte`-based custom `UUID` type exists instead of just using `google/uuid.UUID` directly, why UUID v7 matters for database insert performance specifically (not correctness), what makes a type a *good* candidate for `common/shared` versus a bad one, and how the `Enum[T]` generic guarantees a value is always one of a fixed set without runtime checks scattered through the codebase.
- **Continuity:** Directly extends the generated-type discussion from the HTTP Handler guide — same `CustomerUUID`/`CountryCode` schemas, now getting real Go types behind them instead of `openapi_types.UUID`/plain `string`.

## 2. Concepts

### Custom Types via `x-go-type` / `x-go-type-import`

- **Problem:** [From page] `CountryCode` is currently a plain `string` — it accepts any value, including invalid ones, unless validation is added and remembered everywhere. `CustomerUUID` is `openapi_types.UUID` — a real UUID, but if a second UUID field is added later (e.g. an order ID), the compiler can't distinguish "a customer UUID" from "an order UUID" — both are just the same generic UUID type.
- **Mechanism:** [From page — not yet applied in this workspace] Two oapi-codegen extension fields override what type gets generated for a schema:

  ```yaml
  CustomerUUID:
    type: string
    format: uuid
    description: UUID of a customer
    x-go-type: common.UUID
    x-go-type-import:
      path: eats/backend/common
  ```
  `x-go-type` names the Go type to use in place of the generated default; `x-go-type-import` gives the import path so the generated file can reference it. The OpenAPI schema keeps saying `type: string` — the override only affects the *generated Go code*, not the wire format or the documented API contract. After regenerating, the type aliases the HTTP Handler guide showed change from `type CountryCode = string` / `type CustomerUUID = openapi_types.UUID` to `type CountryCode = shared.CountryCode` / `type CustomerUUID = common.UUID` — same names, now pointing at real types you control.
- **Design Decision:** [From page] The alternative (today's actual state) is accepting the generated defaults and validating manually wherever the value is used. [AI explanation] `x-go-type` trades a small amount of extra spec verbosity for compiler-enforced type distinctions and zero manual conversion code in the handler — the response/request structs use your type directly, no translation layer needed between "the OpenAPI type" and "your domain type."
- **In Production:** [AI explanation] Nothing observable at the wire level — the JSON payload looks identical before and after this change, since the OpenAPI schema's `type: string` is unchanged. The entire benefit is at compile time and in-process: stronger typing, not a different API contract.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Forgetting `x-go-type-import` after adding `x-go-type` | Generated code references a type from a package it never imports — compile error after regeneration | Easy to add the type override and forget the matching import path | Always pair `x-go-type` with `x-go-type-import` pointing at the real package path |
  | Assuming `x-go-type` changes the API's wire contract | Confusion when the JSON schema documentation still says `type: string` after the Go type changes | The override is a codegen-only concern; the spec's declared schema type is unchanged | Remember `x-go-type` only affects generated Go code, never the documented/wire schema |

### The Custom `UUID` Type & Why UUID v7

- **Problem:** [From page] `uuid.New()` (UUID v4) generates fully random values. As a primary key, random values scatter across a database's B-tree index; as the table grows, more of the index falls out of memory, and inserts increasingly require reading pages from disk just to find where the new row belongs — inserts get slower over time, an effect you won't notice on a small table.
- **Mechanism:** [From code, `backend/common/uuid.go` — fully verified]:

  ```go
  type UUID [16]byte

  func NewUUIDv7() UUID {
      u, err := uuid.NewV7()
      if err != nil {
          panic(err)
      }
      return UUID(u)
  }
  ```
  `UUID` wraps the same underlying 16-byte layout as `google/uuid.UUID` (confirmed: `UUID(u)` converts directly between them), but is a distinct named type — exactly the type-safety distinction the previous concept motivates. It implements the full set of interfaces needed to behave like a first-class value everywhere: `String()`, `MarshalText`/`UnmarshalText` (JSON), `Value()`/`Scan()` (`database/sql/driver` — for database columns), plus `IsZero()` and `Equals()` helpers. `MustUUIDFromString` is a panic-on-error constructor for cases (like tests or fixtures) where you already know the string is valid.
- **Design Decision:** [From page] The alternative is `uuid.New()` (v4, fully random). The concrete, cited tradeoff: [From page] a PostgreSQL benchmark inserting 10 million rows took ~36 seconds with a UUID v7 primary key versus over 4 minutes with UUID v4; a MySQL benchmark (by Percona) showed the random-UUID table growing almost 50% larger than the ordered one, with the gap widening as the table grows. UUID v7 (RFC 9562) fixes this by putting a timestamp in the first 48 bits, so new IDs are always larger than old ones — inserts append to the end of the index instead of scattering across it, giving auto-increment-like insert locality without a central counter, and without losing the ability for each service instance to generate IDs independently.
- **In Production:** [From page] The failure mode being avoided is specifically about **insert performance at scale**, not correctness — a table using UUID v4 works fine until it grows large enough that index pages routinely miss cache, at which point insert latency degrades in a way that's easy to misattribute to something else (disk I/O, general DB load) if you don't know to look at index locality.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Using `uuid.New()` (v4) for a new primary key out of habit | Insert latency degrades as the table grows, for reasons that look like generic "the database is slow" rather than a UUID-version choice | v4 is the most commonly reached-for UUID generator, and the problem doesn't show up on a small/dev-sized table | Use `common.NewUUIDv7()` for any ID that will be a primary key in a growing table |
  | Treating `common.UUID` and `google/uuid.UUID` as interchangeable without conversion | Type mismatch at compile time (they're distinct named types, even though same underlying `[16]byte`) | Both wrap the same layout, so it's tempting to assume they unify automatically | Convert explicitly (`UUID(googleUUID)` / `uuid.UUID(commonUUID)`) at the boundary, as `NewUUIDv7` itself does |

### Shared Types (`common/shared`)

- **Problem:** [From page] With only one module (`orders`) right now, adding a shared-types package feels premature — but not setting up the pattern now means refactoring every module later, once a second module needs to share a type.
- **Mechanism:** [From code, `backend/common/shared/`] `country_code.go` and `address.go` are the two shared types that exist today, both confirmed simple, dependency-free values:

  ```go
  type Address struct {
      Line1, Line2, PostalCode, City string
      CountryCode                    CountryCode
  }
  // Scan/Value implemented via json.Marshal/Unmarshal — a JSON-in-a-DB-column pattern
  ```
  `types.go` registers them for cross-module use:

  ```go
  var SharedTypes = []any{
      CountryCode{},
      Address{},
  }
  ```
  [From page] this variable is used "for test comparisons across modules" — a list of the actual shared-type values, kept in one place so tests elsewhere in the codebase can reference it without redeclaring what "the shared types" are.
- **Design Decision:** [From page] The named tradeoff: every type added to `common/shared` creates coupling between every module that uses it — changing a type there means every team owning a consuming module needs to be involved. The page gives a concrete table of what belongs and what doesn't:

  | Good for shared types | Bad for shared types |
  |---|---|
  | `UUID` — tiny, stable, universal | `Customer` struct — entity owned by one module |
  | `CountryCode` — small enum, cross-module | `OrderStatus` — only one module's concern |
  | `Address` — simple value, no business logic | Database row structs — coupled to schema |
  | `Currency` — fixed set, used in prices | Request/response structs — API-layer concern |

  [From page] Explicitly called out: avoid sharing types that serve as inter-module communication contracts (module contracts, covered in the Service Scaffolding guide, are the sanctioned mechanism for that), and avoid database models — a schema change in one module's database shouldn't force changes in another module.
- **In Production:** [AI explanation] The real cost of over-sharing shows up as review/deploy friction, not a runtime failure: a 15-field `CustomerProfile` struct in `common/shared` would mean any team wanting to add a field has to coordinate with every module consuming it, exactly the cross-team bottleneck the page warns about — slow, not broken.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Putting an entity struct (like a full `Customer`) in `common/shared` | Every module using it becomes coupled to that entity's shape; changing one field requires touching every consumer | `common/shared` already exists and importing from it is the path of least resistance | Keep entities inside the module that owns them; only small, stable, dependency-free values belong in `shared` |
  | Sharing a database row struct | A schema change in one module now forces changes in every other module using that struct | Reusing an existing struct feels efficient | Keep DB row structs private to the module owning that table; never share them |

### The Enum Pattern (`Enum[T]` + `Enumerable`)

- **Problem:** [From page] A plain `string`-typed field (like the current `CountryCode`) accepts any value at all — validation has to happen somewhere, and if it's not enforced at the type level, it's easy to skip in one code path and forget in another.
- **Mechanism:** [From code, `backend/common/enum.go` + `backend/common/shared/country_code.go` — fully verified, matches the page]:

  ```go
  type Enumerable interface {
      Values() []string
  }

  type Enum[T Enumerable] struct {
      value string   // unexported
  }

  func (e *Enum[T]) UnmarshalText(text []byte) error {
      var enum T
      expectedValues := enum.Values()
      if len(text) == 0 {
          e.value = ""
          return nil
      }
      for _, v := range expectedValues {
          if v == string(text) {
              e.value = v
              return nil
          }
      }
      return fmt.Errorf("invalid enum value for %T: '%s', expected values %q", enum, string(text), expectedValues)
  }
  ```
  `CountryCode` (in `common/shared`) is the concrete usage:

  ```go
  type CountryCode struct {
      common.Enum[CountryCodeType]
  }
  type CountryCodeType string
  func (c CountryCodeType) Values() []string {
      return []string{"US", "DE", "GB", "JP", "PL"}
  }
  ```
  Because `value` is unexported, the *only* way to produce a non-zero `CountryCode` is through `UnmarshalText` (directly, or via `Scan`, which calls it) — and `UnmarshalText` always validates against `Values()`. So a `CountryCode` in memory is either the zero value (empty) or guaranteed to be one of `"US"`, `"DE"`, `"GB"`, `"JP"`, `"PL"` — there's no code path that produces an invalid one. `CountryCode` embeds `Enum[CountryCodeType]`, so it gets `MarshalText`/`UnmarshalText`/`Scan`/`Value`/`String`/`IsZero` for free via Go's embedding/promotion — no extra code needed beyond `Values()` and (in this file) a small `Code()` convenience wrapper around `String()`.
- **Design Decision:** [From page] The alternative is validating enum-like strings manually wherever they're used (the current, pre-pattern state of a plain-string `CountryCode`). [From page] This is stated as "the next iteration" of an approach from the team's own earlier article ("Safer Enums in Go"), now using generics — implying the manual/non-generic version was the prior iteration this replaces.
- **In Production:** [AI explanation] The practical guarantee is: if you have a `CountryCode` value anywhere in running code, you never need a runtime "is this actually valid?" check — construction is the only validation point, and it's structurally impossible to bypass since the field is unexported.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Constructing an enum wrapper via a struct literal instead of `UnmarshalText`/`Scan`/`MustEnum`/`MustNewCountryCode` | Not directly possible from outside the package — the `value` field is unexported — but easy to *attempt* and get a confusing "unexported field" compile error | Assuming a simple wrapper struct can be built like any other struct | Always go through the provided constructors (`UnmarshalText`, `Scan`, or a `MustXxx` helper) so validation always runs |
  | Adding a new valid value to `Values()` without checking every place that already persisted/serialized the old set | Old data still round-trips fine (still in the list), but there's no compiler help confirming every consumer actually intends to accept the new value yet | `Values()` is just a slice literal — trivial to edit without further thought | Treat changing `Values()` as changing the type's contract, not a trivial edit — audit consumers before adding/removing values |

## 3. Plumbing Dissection

> This page is unusual: the "plumbing" being taught (custom OpenAPI types via `x-go-type`) is **not yet wired into this workspace's spec**. The dissection below covers what's real (the `UUID`/`Enum`/`shared` infrastructure) and separately marks what's still page-only (the spec-level wiring).

**Wiring map — real, verified infrastructure (not yet connected to the OpenAPI spec):**

```
common.UUID [16]byte
  ├─ NewUUIDv7()              → uuid.NewV7() (google/uuid) wrapped
  ├─ MarshalText/UnmarshalText → JSON round-trip
  └─ Scan/Value                → database/sql/driver round-trip (ready for a repository layer)

common.Enum[T Enumerable] { value string (unexported) }
  ├─ T.Values() []string       ← supplied by whatever concrete type parameter is used
  ├─ UnmarshalText: validates text against T.Values(), only way to set `value`
  └─ Scan calls UnmarshalText   → same validation path from a DB read

common/shared.CountryCode struct { common.Enum[CountryCodeType] }
  common/shared.CountryCodeType.Values() → ["US","DE","GB","JP","PL"]
  common/shared.Address struct { ..., CountryCode CountryCode }
  common/shared.SharedTypes = []any{ CountryCode{}, Address{} }   ← used in cross-module test comparisons
```

**Wiring map — page-described, not yet present in `openapi.yaml`/`openapi.gen.go`:**

```
openapi.yaml: CustomerUUID schema
  + x-go-type: common.UUID
  + x-go-type-import: { path: eats/backend/common }
        │
        ▼ task gen
openapi.gen.go: type CustomerUUID = common.UUID   (currently: = openapi_types.UUID)
                type CountryCode  = shared.CountryCode  (currently: = string)
```

- **Per plumbing piece:** `Enum[T]`'s unexported `value` field is the entire enforcement mechanism — every other property of the pattern (safety, no-runtime-checks-needed) follows from the fact that Go doesn't let code outside the `common` package construct a non-zero `Enum[T]` except through the methods that validate first.
- **Non-obvious lines:** `func (e *Enum[T]) UnmarshalText(text []byte) error { var enum T; ... enum.Values() }` — declaring `var enum T` just to call `.Values()` on it looks odd (it's a zero-value instance never otherwise used), but it's the only way to call a method defined on the type parameter `T` when you don't have any other value of that type on hand yet.
- **What a senior notices:** The empty-string special case in `UnmarshalText` (`if len(text) == 0 { e.value = ""; return nil }`) means the zero value is *always* valid regardless of `Values()` — a deliberate design choice (confirmed by the `enum_test.go` `empty_string` test case) that lets a zero-value `CountryCode{}` exist (e.g., before it's been set) without it counting as "invalid"; only a *non-empty, non-listed* value is rejected.

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a blank scratch Go module, rebuild the `Enum[T]` pattern from scratch:
- A generic `Enum[T Enumerable]` struct with an unexported string field.
- `Enumerable` interface requiring `Values() []string`.
- `UnmarshalText`, `MarshalText`, `Scan`, `Value` methods on `Enum[T]`, matching the validation/round-trip behavior verified above (including the empty-string-is-always-valid case).
- A concrete enum (e.g. a `Status` type with 3 values) embedding `Enum[YourStatusType]`.

**Acceptance criteria:**
1. `UnmarshalText` with a valid value succeeds and `String()` returns it; with an invalid value, it returns an error naming the type and the allowed values (mirror `enum_test.go`'s exact error format if you want a strict check).
2. `UnmarshalText` with an empty byte slice succeeds and produces a zero value, without consulting `Values()` at all.
3. There is no way, using only exported package API, to construct a non-zero instance of your enum type holding a value not in `Values()` — try to break this and confirm you can't.
4. `Scan` (simulating a DB read) rejects a non-string input type with a clear error, and accepts/validates a string input the same way `UnmarshalText` does.
5. (Failure case) Add a JSON struct embedding your enum type, marshal a valid instance to JSON and back, and confirm the round-trip preserves the value exactly.

<details><summary>Hint</summary>Start with the non-generic version — one concrete `Status` type with its own unexported field and its own `UnmarshalText` — before parameterizing it into `Enum[T Enumerable]`.</details>
<details><summary>Hint</summary>The unexported field only enforces safety within the same package. If your concrete enum type embeds `Enum[T]`, both need to live in a package boundary such that "outside" code can't reach into `Enum[T].value` directly — confirm this by trying (and failing) to write `MyEnum{Enum: common.Enum[MyType]{value: "anything"}}` from a different package.</details>
<details><summary>Hint</summary>For criterion 2, re-read the real `UnmarshalText` carefully — the empty-string check runs *before* the loop over `Values()`, not as a fallback after a failed match.</details>

**Compare step:** Diff against `backend/common/enum.go` and `backend/common/shared/country_code.go`, and ask yourself:
1. Did my `Scan` call my own `UnmarshalText` (reusing validation), or did I duplicate the validation logic in both places?
2. Did I make the empty-string case a genuine no-validation early return, or did I accidentally require it to also appear in `Values()`?
3. Is my error message specific enough to be useful (naming the type and listing the valid values), or just a generic "invalid enum" message?

## 5. Before the Exercise

- **Likely task:** [AI explanation — a guess, not derived from the exercise itself, since I haven't read `exercise.md`] Given the workspace state (the `UUID`/`Enum`/`shared` infrastructure already exists, but `openapi.yaml` doesn't yet have `x-go-type` extensions and the generated file still uses `openapi_types.UUID`/plain `string`), the exercise likely asks you to add `x-go-type`/`x-go-type-import` to the `CustomerUUID` and `CountryCode` schemas in `openapi.yaml`, regenerate, and update any code that broke as a result (e.g. `handler.go`'s current `uuid.New()` call, which would need to become `common.NewUUIDv7()` to produce a `common.UUID` instead of a `uuid.UUID`).
- **Approach:**
  1. Re-read the `x-go-type`/`x-go-type-import` example on the page and locate exactly where `CustomerUUID` and `CountryCode` are defined in `openapi.yaml` (lines 78-81 and 107-109, per the current file).
  2. Add the extensions, matching the exact Go type names and import paths that already exist (`common.UUID` in `eats/backend/common`; `shared.CountryCode` in whatever the `common/shared` import path is).
  3. Regenerate (`task gen`) and read the new `openapi.gen.go` to confirm the aliases changed as expected.
  4. Fix whatever now fails to compile in `handler.go` — check what type `RegisterCustomer`'s current implementation assumes for the UUID field, since it currently uses `uuid.New()` from `google/uuid`.
  5. Confirm the response still serializes to the same JSON shape as before (`x-go-type` shouldn't change the wire format) — this is worth actually checking, not assuming.

## 6. Recall Questions

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

*(These questions, with answers, have also been appended to `notes/review_queue.md` under today's date.)*

## 7. Cheat Sheet

- `x-go-type` / `x-go-type-import`: override the Go type oapi-codegen generates for a schema, without changing the documented/wire schema (`type: string` stays `type: string`). Requires both fields together.
- `common.UUID [16]byte`: same layout as `google/uuid.UUID`, but a distinct named type for compile-time UUID-field safety. Implements `MarshalText`/`UnmarshalText` (JSON) and `Scan`/`Value` (`database/sql/driver`). Use `common.NewUUIDv7()`, not `uuid.New()`, for anything that will be a DB primary key.
- UUID v7 vs v4: v7 embeds a timestamp in the first 48 bits, so new IDs sort after old ones — appends to the end of a B-tree index instead of scattering across it. Matters for insert performance at scale, not for uniqueness/correctness.
- `common/shared` litmus test: tiny + stable + universal (UUID, CountryCode, Address, Currency) → good. Entity structs, module-specific concerns, DB row structs, inter-module contracts → bad, keep those in the owning module.
- `Enum[T Enumerable]{ value string }` (unexported): only `UnmarshalText`/`Scan` (both validating against `T.Values()`) can set a non-zero value. Empty input is always valid (the zero-value case), independent of `Values()`.
- To add a new enum: define a `type XType string` with a `Values() []string` method, then a wrapper struct embedding `Enum[XType]` — you get `MarshalText`/`UnmarshalText`/`Scan`/`Value`/`String`/`IsZero` for free via embedding.
- `MustEnum[W ~struct{ Enum[T] }, T Enumerable]("value")` — a generic helper for one-call enum construction (e.g. in tests/fixtures) when you already trust the value is valid; panics otherwise, so don't use it on unvalidated input.
