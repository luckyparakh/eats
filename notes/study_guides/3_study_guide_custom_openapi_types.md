# Master Study Guide: Custom OpenAPI Types, UUID v7, Shared Types & the Enum Pattern
> **Platform:** Three Dots Labs Academy — Go Backend Masterclass v0
> **Page:** Custom OpenAPI Types (x-go-type, UUID v7, Shared Types, Enum Pattern)
> **Generated:** 2026-04-26
> **Previous Guide:** `2_study_guide_http_handler_openapi.md`

---

## SECTION 1 — PAGE METADATA

### Topics Covered on This Page
1. **Custom OpenAPI Types** — using `x-go-type` and `x-go-type-import` in `openapi.yaml` to override generated type aliases with your own Go types, so the compiler enforces type correctness at the API boundary
2. **UUID Type (`common.UUID`)** — a custom UUID type that wraps `[16]byte`, implements JSON serialization, database scanning, and uses **UUID v7** (time-ordered) instead of UUID v4 (random) for better database performance
3. **Shared Types (`backend/common/shared/`)** — a dedicated package for small, stable types used by multiple modules; why sharing too much creates coupling and what to keep there
4. **The Enum Pattern** — a generic `Enum[T]` struct that wraps a string and validates it against a fixed list of allowed values during JSON unmarshaling and database scanning, making invalid values impossible at the Go type level

### Where This Fits in the Learning Sequence
- **What came before:** You implemented your first HTTP handler using `StrictServerInterface`. Your handler returned `RegisterCustomer201JSONResponse{CustomerUuid: uuid.New()}` — using `openapi_types.UUID` and a plain `string` for `CountryCode`. The compiler couldn't distinguish a customer UUID from an order UUID, and any string was a valid country code.
- **What this page does:** It replaces those weak "generic" types with **strong, domain-specific types** — types that can only hold valid values. A `CountryCode` that only accepts `"US"`, `"DE"`, `"GB"`, `"JP"`, `"PL"`. A `common.UUID` that can't be confused with any other UUID type.
- **What this enables next:** Every future handler, repository, and service will use these strong types. Invalid data is rejected at the API boundary — before it ever reaches your business logic.

### Single Learning Objective
> After this page, the learner should be able to **configure oapi-codegen to use custom Go types via `x-go-type`, explain why UUID v7 outperforms UUID v4 at scale, and define a new enum type using the `Enum[T]` pattern — with a first-principles explanation for each.**

### Code Patterns Introduced on This Page

| Pattern | What it does |
|---|---|
| `x-go-type: common.UUID` in YAML | Overrides the generated type alias to use your custom Go type |
| `x-go-type-import: path: eats/backend/common` | Tells oapi-codegen which Go package to import for the custom type |
| `type UUID [16]byte` | A custom UUID type backed by a 16-byte array — same data, different Go type |
| `NewUUIDv7()` | Generates a time-ordered UUID v7 instead of a random v4 |
| `type Enumerable interface { Values() []string }` | The contract a type must satisfy to be usable with `Enum[T]` |
| `type Enum[T Enumerable] struct { value string }` | Generic struct that holds and validates a string against `T.Values()` |
| `type CountryCode struct { Enum[CountryCodeType] }` | Embeds `Enum[T]` to get all serialization/validation for free |
| `MustEnum[CountryCode]("US")` | Creates a validated enum value in one call; panics on invalid input |

### Concepts Referenced from Previous Pages
- [Previously learned: `StrictServerInterface`] — You implemented the handler method; it returned typed response structs. Now those structs use stronger custom types for their fields.
- [Previously learned: `oapi-codegen` config] — You used `oapi-codegen.yaml` to control what gets generated. This page adds two new YAML extension fields inside `openapi.yaml` itself.
- [Previously learned: `task gen`] — Still the command to run after changing `openapi.yaml`. After adding `x-go-type`, regenerating produces different type aliases.

---

## SECTION 2 — CONTINUITY BRIDGE

### a) The Thread

On the previous page, you wrote your first handler. It returned `RegisterCustomer201JSONResponse{CustomerUuid: uuid.New()}`. That worked — the UUID was generated, the response was sent.

But there's a silent problem hiding in that code.

`uuid.New()` returns `openapi_types.UUID`. If you later have an `OrderUUID` field and a `CustomerUUID` field — both are `openapi_types.UUID`. The Go compiler sees them as identical. You can accidentally pass an `OrderUUID` where a `CustomerUUID` is expected, and the compiler will not warn you. The bug reaches production.

Similarly, `CountryCode` is a plain `string`. You can pass `"INVALID_CODE"`, `"banana"`, or an empty string — and the compiler accepts it. The only defense is runtime validation that someone remembered to write.

**This page solves both problems.** It teaches you how to make the type system work FOR you — so that invalid data is impossible by construction, not just by convention.

### b) The Shared Axiom

> **The one principle connecting both pages:** The earlier you catch an error in the development process, the cheaper it is to fix. Compile-time errors are free. Production errors are expensive.

The previous page used `StrictServerInterface` to move JSON errors from runtime to compile time. This page uses custom types to move *invalid domain values* from runtime to compile time. Same principle — push the error boundary as early as possible.

### c) Quick Recall Check

- **You'll need: `openapi.yaml` schemas** — reminder: these define the shape of request/response bodies. `x-go-type` is added INSIDE a schema definition to override the generated Go type.
- **You'll need: `task gen`** — reminder: run this after ANY change to `openapi.yaml`. Without it, the generated code still uses the old types.
- **You'll need: `StrictServerInterface` typed responses** — reminder: your handler returns struct types like `RegisterCustomer201JSONResponse`. The fields of those structs now change type when you add `x-go-type`.

---

## SECTION 3 — CORE CONCEPT DEEP-DIVE

---

### Concept 1: Custom OpenAPI Types (`x-go-type` and `x-go-type-import`)

#### a) The Problem Statement — WHY does this exist?

By default, oapi-codegen maps OpenAPI types to Go types mechanically:
- `type: string, format: uuid` → `openapi_types.UUID`
- `type: string` → `string`

This is fine for generating working code quickly. But it creates a weakness: **two different UUID fields in your system are the same Go type.** There's nothing stopping you from doing this:

```go
// Both are openapi_types.UUID — the compiler cannot tell them apart:
var customerUUID openapi_types.UUID = getCustomerUUID()
var orderUUID   openapi_types.UUID = getOrderUUID()

// This compiles fine — but it's WRONG: passing customer UUID where order UUID is expected
processOrder(customerUUID)   // BUG: should be processOrder(orderUUID)
```

The compiler sees `openapi_types.UUID` on both sides and happily compiles it. The bug runs in production for weeks before someone notices.

The irreducible need: **fields that represent different things in your domain must be different types in Go, so the compiler can verify you're using them correctly.**

Without `x-go-type`, you'd have to manually convert types in every handler — pulling the generated `openapi_types.UUID` and converting it to your own type every time. 30 handlers × 3 UUID fields = 90 manual conversion lines. And you'd forget one. And that one would be the bug.

#### b) The Atomic Axioms

1. **Axiom 1:** In Go, two variables are type-compatible only if their Go types are identical (or one is convertible to the other). Using the same type for semantically different things removes the compiler's ability to catch mistakes.
2. **Axiom 2:** `x-go-type` is an OpenAPI extension field (not standard OpenAPI — it's oapi-codegen specific) that replaces the default generated type alias with a type you specify.
3. **Axiom 3:** `x-go-type-import` tells oapi-codegen which Go package to import so the custom type is available in the generated file. Without the import, the type name alone is unresolvable.
4. **Axiom 4:** The OpenAPI schema (`type: string, format: uuid`) still describes the JSON wire format. `x-go-type` only affects the Go type in the generated code — the JSON API contract is unchanged.
5. **Axiom 5:** Once `x-go-type` is set and `task gen` is run, the generated type alias changes — and your handler code automatically uses the new type because it references the generated struct fields.

#### c) The Core Mechanism

Think of it like upgrading ingredients in a recipe. The recipe (OpenAPI spec) says "one cup of flour." By default the kitchen gives you generic all-purpose flour (`openapi_types.UUID`). With `x-go-type`, you say "I specifically want bread flour (`common.UUID`) from this specific mill (`eats/backend/common`)." The recipe itself (the JSON wire format) is unchanged — the customer still gets bread. But now your kitchen has a rule: you can't accidentally use cake flour where bread flour is required, because they're labeled differently.

Because `x-go-type` is placed inside the schema definition (Axiom 2), oapi-codegen reads it during generation and replaces the type alias:

```
Before: type CustomerUUID = openapi_types.UUID
After:  type CustomerUUID = common.UUID
```

Because `x-go-type-import` provides the package path (Axiom 3), oapi-codegen adds:
```go
import "eats/backend/common"
```
to the generated file automatically.

Now when your handler returns `RegisterCustomer201JSONResponse{CustomerUuid: common.NewUUIDv7()}`, the compiler checks that `common.UUID` is the right type for `CustomerUuid`. If you accidentally pass an `OrderUUID` (which would be a different type), the compiler rejects it (Axiom 1).

#### d) Syntax & Code (from the webpage)

**In `backend/orders/api/http/openapi.yaml` — adding `x-go-type` to the `CustomerUUID` schema:**

```yaml
components:
  schemas:
    CustomerUUID:
      type: string        # Still string in JSON — this is the wire format (Axiom 4)
      format: uuid        # Still UUID format for validation
      description: UUID of a customer
      x-go-type: common.UUID           # Use THIS Go type instead of openapi_types.UUID (Axiom 2)
      x-go-type-import:                # Tell oapi-codegen where to find common.UUID
        path: eats/backend/common      # The Go import path (Axiom 3)
```

**The same pattern for `CountryCode`:**

```yaml
    CountryCode:
      type: string
      x-go-type: shared.CountryCode    # Use our validated enum type
      x-go-type-import:
        path: eats/backend/common/shared
```

**What the generated code looks like BEFORE (default):**

```go
// openapi.gen.go — before x-go-type:
type CountryCode = string              // Any string is valid — no validation
type CustomerUUID = openapi_types.UUID // Same type as any other UUID field
```

**What the generated code looks like AFTER (`task gen` with x-go-type):**

```go
// openapi.gen.go — after x-go-type:
import "eats/backend/common"
import "eats/backend/common/shared"

type CountryCode = shared.CountryCode  // Only valid country codes accepted
type CustomerUUID = common.UUID        // Alias — same type as common.UUID; NOT distinct from OrderUUID if it's also = common.UUID
```

#### e) Execution / Internal Walkthrough

When you add `x-go-type` to `openapi.yaml` and run `task gen`:

```
Step 1: oapi-codegen reads openapi.yaml
Step 2: It finds CustomerUUID schema with x-go-type: common.UUID
Step 3: Instead of generating: type CustomerUUID = openapi_types.UUID
        It generates:          type CustomerUUID = common.UUID
Step 4: It finds x-go-type-import: path: eats/backend/common
Step 5: It adds: import "eats/backend/common" to the generated file
Step 6: All structs that had openapi_types.UUID for CustomerUUID now use common.UUID

In your handler (handler.go — unchanged):
Step 7: handler.RegisterCustomer returns RegisterCustomer201JSONResponse{CustomerUuid: common.NewUUIDv7()}
Step 8: The compiler checks: is common.UUID assignable to CustomerUuid? YES (they're the same type now)
Step 9: If you accidentally wrote: CustomerUuid: someOrderUUID (of type OrderUUID)
        The compiler rejects it — type mismatch caught at build time (Axiom 1)
```

**Where each axiom becomes visible:**
- Axiom 1: Step 9 — compiler catches the wrong type before code runs.
- Axiom 2: Steps 2–3 — `x-go-type` in the YAML directly changes the generated alias.
- Axiom 3: Steps 4–5 — `x-go-type-import` adds the import statement automatically.
- Axiom 4: The JSON wire format is unchanged — clients still send/receive plain UUID strings.
- Axiom 5: Steps 7–8 — handler code references the generated struct field; it automatically uses the new type.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| Adding `x-go-type` changes what clients send over the network | **FALSE** | The JSON wire format is still a plain UUID string. `x-go-type` only affects the Go code, not the API contract (Axiom 4). |
| You need to update handler.go after adding `x-go-type` | **SOMETIMES** | If your handler still uses `uuid.New()` (returns `google/uuid.UUID`), you must change it to `common.NewUUIDv7()` (returns `common.UUID`). The struct field type changed. |
| `x-go-type` is standard OpenAPI | **FALSE** | It's an oapi-codegen extension (a vendor-specific field). Other OpenAPI tools ignore it. It's only meaningful to oapi-codegen. |
| Both `x-go-type` and `x-go-type-import` are always required | **FALSE** | `x-go-type-import` is only needed when the custom type is in a different package than the generated file. If the type is in the same package, only `x-go-type` is needed. |
| You can use `x-go-type` with any Go type | **TRUE** | Any valid Go type works. The type just needs to be able to unmarshal from the JSON format the spec defines. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Adding `x-go-type` but forgetting to run `task gen` | Stale generated code — old type still used | The YAML changed but the generated file didn't | Run `task gen` after every `openapi.yaml` change |
| Using `x-go-type` without `x-go-type-import` for a type in another package | Compile error — undefined type | oapi-codegen doesn't know where to import the type from | Add `x-go-type-import: path: your/package/path` |
| Leaving `uuid.New()` in handler after switching to `common.UUID` | Compile error — type mismatch | `uuid.New()` returns `google/uuid.UUID`, not `common.UUID` | Switch to `common.NewUUIDv7()` |
| Adding `x-go-type` at the wrong YAML level (outside the schema) | oapi-codegen ignores it or errors | Extension fields must be inside the schema definition block | Place `x-go-type` at the same indent level as `type` and `format` |

#### h) Comprehension Task

> **Comprehension Task:** On paper, draw two columns: "Before x-go-type" and "After x-go-type". For each column, write: (1) What type does `CustomerUuid` have in the generated struct? (2) What happens at compile time if you put an `OrderUUID` value there? (3) What does the client receive over the network (JSON)?
>
> **What to check:** Before: `openapi_types.UUID`, no compile error (both are the same type), client receives a UUID string. After: `common.UUID`, compile error (different types now!), client still receives a UUID string (unchanged). The key insight is that column 3 is IDENTICAL in both cases — the network format doesn't change.
>
> **Common wrong answer:** "After x-go-type, clients need to send a different format." — Wrong. The JSON wire format is defined by the OpenAPI schema (`type: string, format: uuid`), which is unchanged. Only the Go code changes.

---

### Concept 2: The `common.UUID` Type and UUID v7

#### a) The Problem Statement — WHY does this exist?

Two separate problems needed solving:

**Problem A — Type distinction:** Even after mapping to `common.UUID`, you need `common.UUID` to actually exist and work correctly — it must marshal to/from JSON, and later it must read from and write to a database. `google/uuid.UUID` already does this. But if you just use `google/uuid.UUID` directly everywhere, all your UUIDs are still the same type.

**Problem B — Database performance:** `uuid.New()` generates UUID v4 — completely random. A database stores rows sorted by primary key in a tree structure (called a B-tree index). Random UUIDs scatter insertions across the entire tree. As the table grows to millions of rows, inserting a new row requires finding a random spot in the tree, which may not be in memory — causing a slow disk read. In a benchmark with 10 million rows: UUID v4 took **4+ minutes** to insert; UUID v7 took **36 seconds**.

The irreducible need: **a custom UUID type that is distinguishable from other UUID types at compile time AND generates time-ordered values that databases insert efficiently.**

#### b) The Atomic Axioms

1. **Axiom 1:** In Go, `type UUID [16]byte` creates a NEW named type. Even though it has the same underlying data as `google/uuid.UUID` (which is also `[16]byte`), the compiler treats them as different types — they are NOT interchangeable without explicit conversion.
2. **Axiom 2:** Any Go type can implement `MarshalText`/`UnmarshalText` to control how it's encoded/decoded as text (used for JSON). Any type implementing `Value`/`Scan` can read from and write to a SQL database.
3. **Axiom 3:** UUID v7 (RFC 9562) embeds a millisecond-precision timestamp in the first 48 bits. New UUIDs are always greater than old ones — they maintain sort order over time. This is called "monotonically increasing."
4. **Axiom 4:** Because v7 UUIDs are time-ordered, database insertions always append to the END of the B-tree index. No random scattering — same performance as auto-increment integers, but without a central counter that creates a single point of failure.
5. **Axiom 5:** Each service instance can independently generate UUID v7 values with near-zero collision probability — no database round-trip or central counter needed. Distributed ID generation at no cost.

#### c) The Core Mechanism

Think of UUID v4 vs v7 like two approaches to seating guests at a very large restaurant.

**UUID v4:** Every guest gets a random table number — table 4,812,033, then table 201, then table 7,591,442. The maitre d' has to search the entire restaurant each time. As the restaurant fills up, the walks get longer and longer.

**UUID v7:** Every new guest gets the NEXT available table — 1, 2, 3, 4, always sequential. The maitre d' always knows: "new guests go to the back." No searching, always fast, doesn't slow down as the restaurant fills.

Because `common.UUID` is `type UUID [16]byte` (Axiom 1), it is a distinct Go type. You cannot accidentally assign `google/uuid.UUID` to a `common.UUID` without an explicit conversion — the compiler enforces the distinction.

Because `common.UUID` implements `MarshalText`/`UnmarshalText` by delegating to `google/uuid.UUID`'s implementation (Axiom 2), JSON encoding and decoding work identically to before — clients see no difference in the API.

Because `common.UUID` implements `Value`/`Scan` (Axiom 2), it can be stored in and retrieved from a PostgreSQL database column of type `uuid` — ready for the next module where you add database access.

#### d) Syntax & Code (from the webpage)

**Full `backend/common/uuid.go` with annotations:**

```go
package common

import (
    "database/sql/driver"   // For Scan/Value — database interface
    "github.com/google/uuid" // The underlying UUID library we delegate to
)

// type UUID [16]byte — a 16-byte array (128 bits = UUID size)
// This creates a NEW named type in Go (Axiom 1).
// It has the same underlying data as google/uuid.UUID but is a DIFFERENT type.
type UUID [16]byte

// NewUUIDv7 generates a new time-ordered UUID v7 (Axiom 3).
// Use this instead of uuid.New() everywhere in the project.
func NewUUIDv7() UUID {
    u, err := uuid.NewV7()   // Delegate to google/uuid for generation
    if err != nil {
        panic(err)           // UUID generation failure is truly exceptional
    }
    return UUID(u)           // Convert google/uuid.UUID → common.UUID (explicit conversion)
}

// String returns the UUID as a human-readable string: "550e8400-e29b-41d4-a716-446655440000"
func (u UUID) String() string {
    return uuid.UUID(u).String()  // Convert back to google/uuid.UUID temporarily to use its String()
}

// MarshalText encodes the UUID to text (used for JSON marshaling) (Axiom 2)
func (u UUID) MarshalText() ([]byte, error) {
    return uuid.UUID(u).MarshalText()
}

// UnmarshalText decodes a text representation into the UUID (used for JSON unmarshaling) (Axiom 2)
func (u *UUID) UnmarshalText(data []byte) error {
    var guuid uuid.UUID
    if err := guuid.UnmarshalText(data); err != nil {
        return err  // Invalid UUID string — returns error to the JSON decoder
    }
    *u = UUID(guuid)  // Store the result in this UUID (pointer receiver — modifies the original)
    return nil
}

// Value implements the database/sql/driver.Valuer interface (Axiom 2)
// Called when writing a UUID to a database column.
func (u UUID) Value() (driver.Value, error) {
    return uuid.UUID(u).Value()
}

// Scan implements the database/sql.Scanner interface (Axiom 2)
// Called when reading a UUID from a database row.
func (u *UUID) Scan(src any) error {
    var guuid uuid.UUID
    if err := guuid.Scan(src); err != nil {
        return err
    }
    *u = UUID(guuid)
    return nil
}
```

**In your handler — the one change required:**
```go
// Before: (guide 2 — handler.go)
return RegisterCustomer201JSONResponse{
    CustomerUuid: uuid.New(),       // uuid.New() returns google/uuid.UUID — WRONG TYPE now
}, nil

// After: (this page)
return RegisterCustomer201JSONResponse{
    CustomerUuid: common.NewUUIDv7(),   // Returns common.UUID — CORRECT TYPE
}, nil
```

#### e) Execution / Internal Walkthrough

When a client sends `POST /orders/register-customer` and your handler calls `common.NewUUIDv7()`:

```
Step 1: common.NewUUIDv7() is called
Step 2: Internally calls uuid.NewV7() from google/uuid library
Step 3: uuid.NewV7() reads the current timestamp (milliseconds since Unix epoch)
        → Encodes timestamp in the first 48 bits of the 128-bit UUID
        → Fills remaining bits with random data (for uniqueness within the same millisecond)
        → e.g., 019512b7-3b0a-7000-9123-456789abcdef
           ^^^^^^^^^^^^^^^^ ← timestamp portion (always increasing)
Step 4: Returns as google/uuid.UUID
Step 5: UUID(u) converts it to common.UUID (same bytes, different type label)
Step 6: Handler returns RegisterCustomer201JSONResponse{CustomerUuid: theCommonUUID}
Step 7: Strict wrapper calls CustomerUuid.MarshalText()
Step 8: MarshalText() converts to "019512b7-3b0a-7000-9123-456789abcdef" string
Step 9: JSON response: {"customer_uuid": "019512b7-3b0a-7000-9123-456789abcdef"}

Later — if two rows are inserted to the database:
Insert 1: UUID 019512b7-3b0a-7000-... (time T)
Insert 2: UUID 019512b7-5c1f-7000-... (time T + 500ms — always GREATER)
Database B-tree: both appended to the end — no random scatter (Axiom 4)
```

**Where each axiom becomes visible:**
- Axiom 1: Step 5 — `UUID(u)` is an explicit conversion; without it, the compiler would reject assigning `google/uuid.UUID` to `common.UUID`.
- Axiom 2: Steps 7–8 — `MarshalText` is called by the JSON encoder; `Scan`/`Value` will be used by the database in the next module.
- Axiom 3: Steps 3–4 — timestamp in the first 48 bits.
- Axiom 4: The final two inserts show ordering — always appending, never random.
- Axiom 5: Steps 1–5 — no database round-trip, no central counter. Any service instance calls this and gets a unique, time-ordered ID.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| `common.UUID` and `google/uuid.UUID` are the same type | **FALSE** | They have the same underlying data (`[16]byte`) but are DIFFERENT named types in Go. The compiler does not allow implicit conversion (Axiom 1). |
| `uuid.New()` and `common.NewUUIDv7()` return the same type | **FALSE** | `uuid.New()` returns `google/uuid.UUID`. `common.NewUUIDv7()` returns `common.UUID`. After adding `x-go-type`, the handler's return struct expects `common.UUID`. |
| UUID v7 is less unique than UUID v4 | **FALSE** | Both have sufficient entropy for uniqueness. v7's timestamp uses 48 bits for time and 74 bits for random data — still billions of possible IDs per millisecond. |
| UUID v7 requires a central server to generate | **FALSE** | Axiom 5 — each service instance generates v7 UUIDs independently. The timestamp comes from the local clock, not a central counter. |
| Changing from v4 to v7 breaks existing data | **FALSE** | Both are valid UUIDs in standard string format. A database column that accepts `uuid` accepts both v4 and v7. The version is encoded in one nibble of the UUID. |
| `MarshalText`/`UnmarshalText` are called by you manually | **FALSE** | The Go JSON encoder (`encoding/json`) calls `MarshalText` automatically when encoding a type that implements it. Same for the database driver and `Scan`/`Value`. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Using `uuid.New()` after switching to `common.UUID` | Compile error — type mismatch | `uuid.New()` returns `google/uuid.UUID`; the generated struct field now expects `common.UUID` | Switch all UUID generation to `common.NewUUIDv7()` |
| Manually calling `MarshalText` in the handler | Unnecessary boilerplate | The JSON encoder calls it automatically | Just return the `common.UUID` value; encoding is automatic |
| Using `common.UUID(uuid.New())` instead of `common.NewUUIDv7()` | Silent wrong-version bug | You'd get a UUID v4 (random) disguised as `common.UUID` — compiles but loses performance benefit | Always use `common.NewUUIDv7()` which explicitly generates v7 |
| Comparing `common.UUID` to `google/uuid.UUID` with `==` | Compile error — type mismatch | Different types are not `==`-comparable without conversion | Convert: `uuid.UUID(myCommonUUID) == someGoogleUUID` |

#### h) Comprehension Task

> **Comprehension Task:** You are explaining UUID v7 to a non-technical colleague. They ask: "Why does time-ordering matter? A UUID is just an ID — why does it matter what's inside?" Write a plain English answer using only the restaurant analogy (or your own). Your answer must explain: what a B-tree is (without that word), why random IDs make it slow, and how ordered IDs fix it.
>
> **What to check:** A good answer uses the "guests finding tables" or a similar analogy. It explains: the database keeps records in sorted order (like an ordered filing cabinet). With random IDs, every new record goes to a random position — the cabinet has to be rearranged every time. With ordered IDs, every new record goes at the end — just open the last drawer and add it. The more records you have, the bigger the difference.
>
> **Common wrong answer:** "UUID v7 is more unique." — Wrong. Both are unique enough. The difference is INSERT performance into sorted data structures, not uniqueness.

---

### Concept 3: Shared Types (`backend/common/shared/`)

#### a) The Problem Statement — WHY does this exist?

Your project will eventually have multiple modules: `orders`, `restaurants`, `couriers`, `payments`. Many of these modules deal with the same real-world concepts:

- Every module that ships to an address needs `Address`.
- Every module that handles international customers needs `CountryCode`.
- Every module that creates entities needs `UUID`.

Without a shared package, each module would define its own `Address` struct. When you try to pass an address from the `orders` module to the `restaurants` module, Go rejects it — `orders.Address` and `restaurants.Address` are different types, even if they have the same fields. You'd write dozens of conversion functions.

**But sharing too much is equally dangerous.** If you put a `CustomerProfile` struct in shared code, every module that uses it is "coupled" (linked together) to that struct. When the orders team needs to add a field to `CustomerProfile`, they must coordinate with the payments team, the restaurants team, and the couriers team — because changing the shared type breaks all of them simultaneously.

The irreducible need: **a place for types that are small, stable, and truly needed by multiple modules — and a clear rule about what DOESN'T belong there.**

#### b) The Atomic Axioms

1. **Axiom 1:** Two Go types are only interchangeable if they are the same type. Modules need shared types to avoid writing conversion code for genuinely common data structures.
2. **Axiom 2:** Every type in a shared package creates coupling — a dependency between every module that uses it. Changing a shared type forces changes in all consuming modules simultaneously.
3. **Axiom 3:** Good candidates for shared types are small, stable, and have no business logic — they describe data that hasn't changed in years and won't need to.
4. **Axiom 4:** Bad candidates for shared types are types that are "owned" by one business domain — `OrderStatus` belongs to the orders domain; sharing it gives other modules intimate knowledge of orders internals.
5. **Axiom 5:** `backend/common/shared/types.go` registers shared types in a `SharedTypes` variable — this allows test infrastructure to compare these types across module boundaries without knowing their internal structure.

#### c) The Core Mechanism

Think of a shared package like a standardized shipping container. All shipping companies in the world agree on the dimensions of a container. A ship can carry containers from any company, and cranes can lift any container, because the standard never changes. The contents of the container are NOT standardized — that's each company's business.

Similarly:
- `Address`, `CountryCode`, `UUID` are the shipping container — agreed upon by all modules.
- What's INSIDE an order, what's INSIDE a customer profile — that stays inside each module.

Because the shared package is in `backend/common/shared/` (not `backend/orders/` or `backend/restaurants/`), no single business module "owns" it. It belongs to the infrastructure layer — like the logging and UUID code already in `backend/common/`.

Because changing a shared type breaks all consumers simultaneously (Axiom 2), you protect yourself by keeping shared types as simple as possible. `CountryCode` is just a validated string — it will never need a `description` field or a `currency` field. If it did, it would stop being a shared type and become something each module manages independently.

#### d) Syntax & Code (from the webpage)

**`backend/common/shared/country_code.go` — the CountryCode type:**

```go
package shared

import (
    "fmt"
    "eats/backend/common"   // For common.Enum[T]
)

// CountryCode wraps Enum[CountryCodeType] — it gets all serialization methods for free (Axiom 3)
type CountryCode struct {
    common.Enum[CountryCodeType]   // Embed the generic Enum struct — explained in Concept 4
}

// Code() is a convenience method — returns the string value of the country code
func (c CountryCode) Code() string {
    return c.String()
}

// CountryCodeType is the "catalog" type — it knows the list of valid values
type CountryCodeType string

// Values() satisfies the Enumerable interface — lists ALL valid country codes
func (c CountryCodeType) Values() []string {
    return []string{
        "US",   // United States
        "DE",   // Germany
        "GB",   // Great Britain
        "JP",   // Japan
        "PL",   // Poland
    }
}
```

**`backend/common/shared/address.go` — the Address type:**

```go
package shared

// Address is a simple value type — no business logic, just data (Axiom 3)
type Address struct {
    Line1       string      `json:"line_1,omitempty"`
    Line2       string      `json:"line_2,omitempty"`
    PostalCode  string      `json:"postal_code,omitempty"`
    City        string      `json:"city,omitempty"`
    CountryCode CountryCode `json:"country_code"`   // Uses our validated enum type
}
```

**`backend/common/shared/types.go` — the SharedTypes registry:**

```go
package shared

// SharedTypes lists all shared types for test infrastructure.
// When a test compares two Address values from different modules, this list
// tells the test framework "these are the same type across all modules." (Axiom 5)
var SharedTypes = []any{
    CountryCode{},
    Address{},
}
```

**The good/bad table from the page:**

| Good for shared types | Bad for shared types |
|---|---|
| UUID — tiny, stable, universal | Customer struct — entity owned by one module |
| CountryCode — small enum, cross-module | OrderStatus — only one module's concern |
| Address — simple value, no business logic | Database row structs — coupled to schema |
| Currency — fixed set, used in prices | Request/response structs — API-layer concern |

#### e) Execution / Internal Walkthrough

When the `orders` module creates a new customer with an address:

```
Step 1: orders handler receives a request with JSON body:
        {"address": {"line_1": "123 Main St", "postal_code": "10001", 
                     "city": "New York", "country_code": "US"}}

Step 2: The strict wrapper JSON-decodes the body into RegisterCustomerRequestObject
        → request.Body.Address is of type shared.Address (from the openapi.gen.go type)

Step 3: The Address.CountryCode field is type shared.CountryCode (an Enum)
        → shared.CountryCode.UnmarshalText([]byte("US")) is called
        → CountryCodeType.Values() returns ["US","DE","GB","JP","PL"]
        → "US" is in the list → valid → stored
        
Step 4: If someone sends "XX" as country code:
        → shared.CountryCode.UnmarshalText([]byte("XX")) is called
        → CountryCodeType.Values() returns ["US","DE","GB","JP","PL"]
        → "XX" is NOT in the list → error returned
        → Strict wrapper returns 400 automatically — handler never called

Step 5: The validated address is available as a typed Go value in the handler
        → shared.Address{Line1: "123 Main St", PostalCode: "10001", 
                         City: "New York", CountryCode: shared.CountryCode{"US"}}
```

**Where each axiom becomes visible:**
- Axiom 1: Step 2 — `shared.Address` is the same type for any module that uses it — no conversion needed.
- Axiom 2: If `Address` needed a new field, every module consuming it would need to be updated simultaneously.
- Axiom 3: Steps 2–5 — `Address` and `CountryCode` are simple, no business logic, purely data.
- Axiom 4: `OrderStatus` would NOT be here — it belongs in the orders module exclusively.
- Axiom 5: `SharedTypes` allows test infrastructure to know these types cross module boundaries.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| More shared types = more code reuse = better | **FALSE** | More shared types = more coupling. Each shared type increases the coordination cost of future changes (Axiom 2). |
| The `orders` module can define its own `Address` struct | **TRUE but creates problems** | It can, but then `orders.Address` and `restaurants.Address` are different types — passing between modules requires conversion code everywhere. |
| Shared types should include validation business rules | **FALSE** | Validation rules change as business rules evolve. Shared types should be stable data structures. `CountryCode` validates against a fixed list — that list rarely changes. |
| `SharedTypes` variable is used at runtime for something important | **PARTLY** | It's primarily for test infrastructure — telling test comparison utilities which types are shared. It doesn't affect production request handling. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Adding a business entity to shared (e.g., `CustomerProfile`) | Architecture problem — cross-team coupling | Business entities evolve with requirements; sharing them locks all teams to the same evolution | Keep business entities in their owning module |
| Adding request/response structs to shared | Architecture problem | API layer contracts belong to the API layer — not shared infrastructure | Keep request/response types in `api/http/` |
| Not using shared types when two modules genuinely need the same simple type | Code duplication + conversion boilerplate | Fear of coupling | Use shared for genuinely stable, small, cross-module types (UUID, Address, CountryCode) |

#### h) Comprehension Task

> **Comprehension Task:** Below are five candidate types for the shared package. For each one, write "YES — shared is correct" or "NO — belongs in a specific module" and give a one-sentence reason based on the axioms.
> 1. `Currency` (a string enum: USD, EUR, GBP, JPY)
> 2. `PaymentMethod` (credit card, debit card, PayPal — only used by payments module)
> 3. `PhoneNumber` (a validated string, used by orders and couriers)
> 4. `OrderItem` (a struct with dish, quantity, price — belongs to an order)
> 5. `GeoCoordinate` (latitude + longitude float64 pair — used by restaurants and couriers for location)
>
> **What to check:** (1) YES — small, stable, used across all money-handling modules. (2) NO — only payments module cares. (3) YES — small, stable, used by multiple modules. (4) NO — owned entirely by orders domain. (5) YES — simple data value, no business logic, needed by at least two modules (restaurants need location, couriers need routing).
>
> **Common wrong answer:** Putting `OrderItem` in shared "because orders and restaurants both deal with items." — Wrong. `OrderItem` as placed in an order is an orders-domain concept. Restaurants have their own concept of menu items. Sharing an `OrderItem` type creates coupling between two domains that should be independent.

---

### Concept 4: The Enum Pattern (`Enum[T Enumerable]`)

#### a) The Problem Statement — WHY does this exist?

You have a `CountryCode` field in your API. A customer can be from the US, Germany, Great Britain, Japan, or Poland. Only those five values are valid.

**Without a custom enum type**, you'd use a plain `string`. Every function that accepts a `CountryCode` would have to validate it:
```go
// Without Enum — manual validation everywhere:
func processOrder(countryCode string, ...) error {
    valid := []string{"US", "DE", "GB", "JP", "PL"}
    isValid := false
    for _, v := range valid {
        if v == countryCode { isValid = true; break }
    }
    if !isValid {
        return fmt.Errorf("invalid country code: %s", countryCode)
    }
    // ... actual logic
}
```

This validation code is duplicated in every function that accepts a country code. Miss it in one place, and invalid data flows through to the database.

Worse, nothing prevents creating a `CountryCode` value with an invalid string anywhere in the codebase:
```go
// Both compile fine:
validCode := "US"
invalidCode := "MOONBASE_ALPHA"  // Also just a string — compiler can't tell
```

The irreducible need: **it must be impossible to create a `CountryCode` value that contains an invalid country code — not just unlikely, but impossible by construction.**

#### b) The Atomic Axioms

1. **Axiom 1:** The only way to create a `CountryCode` value in Go is through `UnmarshalText` (JSON decoding) or `Scan` (database reading) or `MustEnum` (direct creation). ALL of these go through validation. There is no public constructor that skips validation.
2. **Axiom 2:** The `value` field inside `Enum[T]` is unexported (lowercase `value`). This means no code outside the `common` package can create an `Enum` with an arbitrary string — they can't set `value` directly.
3. **Axiom 3:** The `Enumerable` interface requires one method: `Values() []string`. Any type that declares this method can be used as the type parameter `T` in `Enum[T]`.
4. **Axiom 4:** `Enum[T]` is generic — the `T` parameter specifies WHICH type determines the valid values. `Enum[CountryCodeType]` validates against `CountryCodeType.Values()`. `Enum[OrderStatusType]` would validate against `OrderStatusType.Values()`.
5. **Axiom 5:** Embedding `Enum[T]` in a struct (like `CountryCode` embeds `Enum[CountryCodeType]`) makes all of `Enum[T]`'s methods available directly on `CountryCode` — including `MarshalText`, `UnmarshalText`, `Scan`, `Value`, `String`.

#### c) The Core Mechanism

Think of the Enum pattern like a passport control checkpoint. You can only enter the country (create a valid `CountryCode` value) by going through the checkpoint (validation). There is no side entrance (no public field to set directly). Every person who enters has been checked. Inside the country, you never need to check IDs again — if they're there, they're valid.

The generic `Enum[T]` struct is the checkpoint machinery. It's the same machinery regardless of what country (what enum type) it's checking. You just configure it with the list of valid values by implementing `Values()` on type `T`.

Step by step:
1. `CountryCodeType` (just a `string` alias) implements `Values()` — it knows the valid list.
2. `Enum[CountryCodeType]` embeds the validation machinery parameterized to use `CountryCodeType.Values()`.
3. `CountryCode` embeds `Enum[CountryCodeType]` — it IS an Enum with CountryCodeType's valid values.
4. When Go's JSON decoder sees a `CountryCode` field, it calls `UnmarshalText`. `Enum[T].UnmarshalText` checks the incoming string against `T.Values()`. If invalid → error. If valid → stored in the private `value` field.
5. The `CountryCode` value that comes out of this process is guaranteed valid. Forever.

#### d) Syntax & Code (from the webpage)

**`backend/common/enum.go` — the generic Enum infrastructure:**

```go
package common

import (
    "database/sql/driver"
    "fmt"
)

// Enumerable is the interface a type must satisfy to be used with Enum[T].
// The type just needs to declare which values are valid.
type Enumerable interface {
    Values() []string    // Return the complete list of valid string values
}

// Enum[T Enumerable] is the generic enum struct.
// T must implement Enumerable (must have a Values() method).
// value is UNEXPORTED — impossible to set directly from outside this package (Axiom 2)
type Enum[T Enumerable] struct {
    value string    // The stored string value — private, can only be set via UnmarshalText
}

// UnmarshalText is called by the JSON decoder when it reads a string into an Enum field.
// This is the ONLY way to store a value (other than Scan from DB).
func (e *Enum[T]) UnmarshalText(text []byte) error {
    var enum T                          // Create an instance of the type parameter T
    valid := false
    expectedValues := enum.Values()    // Ask T what values are valid

    if len(text) == 0 {
        e.value = ""                   // Empty is allowed (zero value)
        return nil
    }

    for _, v := range expectedValues {
        if v == string(text) {
            valid = true
            e.value = v               // Store the validated value
            break
        }
    }
    if !valid {
        // Validation failed — return error with helpful message
        return fmt.Errorf("invalid enum value for %T: '%s', expected values %q", enum, string(text), expectedValues)
    }
    return nil
}

// MarshalText serializes the value back to text (for JSON encoding)
func (e Enum[T]) MarshalText() (text []byte, err error) {
    return []byte(e.value), nil
}

// String returns the string value (for fmt.Println etc.)
func (e Enum[T]) String() string {
    return e.value
}

// IsZero returns true if no value has been set (zero value)
func (e Enum[T]) IsZero() bool {
    return e.value == ""
}

// Scan reads from a database (implements sql.Scanner) (Axiom 2)
func (e *Enum[T]) Scan(src any) error {
    text, ok := src.(string)
    if !ok {
        return fmt.Errorf("invalid type for enum: %T, expected string", src)
    }
    return e.UnmarshalText([]byte(text))  // Reuse validation logic
}

// Value writes to a database (implements driver.Valuer)
func (e Enum[T]) Value() (driver.Value, error) {
    return e.value, nil
}
```

**`MustEnum` helper — for creating enum values in code:**

```go
// MustEnum creates an enum value in one call. Panics if the value is invalid.
// Example: MustEnum[CountryCode]("US") → returns a valid CountryCode
// The advanced generic constraint ~struct{ Enum[T] } means:
// "W must be a struct that embeds Enum[T]" — ensures this only works on proper wrappers.
func MustEnum[W ~struct{ Enum[T] }, T Enumerable](value string) W {
    return W{MustEnumFromString[T](value)}
}
```

**How to define a NEW enum (the three-step pattern):**

```go
// STEP 1: Define the "catalog" type — just a string alias
type OrderStatusType string

// STEP 2: Implement Values() — return all valid strings for this enum
func (o OrderStatusType) Values() []string {
    return []string{"pending", "confirmed", "preparing", "delivered", "cancelled"}
}

// STEP 3: Create the wrapper struct embedding Enum[T]
// This wrapper gets MarshalText, UnmarshalText, Scan, Value, String for free (Axiom 5)
type OrderStatus struct {
    common.Enum[OrderStatusType]
}

// Usage:
status := MustEnum[OrderStatus]("confirmed")    // Valid — creates OrderStatus
invalid := MustEnum[OrderStatus]("banana")      // PANIC — not in Values()
```

#### e) Execution / Internal Walkthrough

A request arrives with `"country_code": "DE"`:

```
Step 1: JSON decoder reads the field "country_code": "DE"
Step 2: It sees the field type is shared.CountryCode (which embeds Enum[CountryCodeType])
Step 3: JSON decoder calls CountryCode.UnmarshalText([]byte("DE"))
Step 4: Enum[CountryCodeType].UnmarshalText runs:
        → var enum CountryCodeType (zero value — it's just a string type)
        → enum.Values() returns ["US", "DE", "GB", "JP", "PL"]
        → Loop: "US" ≠ "DE", "DE" == "DE" → valid = true, e.value = "DE"
        → Returns nil (no error)
Step 5: The CountryCode value is now: {Enum[CountryCodeType]{value: "DE"}}
        → value is private — nobody can change it outside this package
Step 6: Handler receives request.Body.Address.CountryCode — it IS "DE", guaranteed

Now with "country_code": "MARS":
Step 4 (alternate): Loop: "US" ≠ "MARS", "DE" ≠ "MARS", ..., exhausted
        → valid = false
        → Returns error: "invalid enum value for CountryCodeType: 'MARS', expected values ["US" "DE" "GB" "JP" "PL"]"
Step 5: JSON decoder surfaces the error → strict wrapper returns 400
Step 6: Handler is never called — invalid data never enters business logic
```

**Where each axiom becomes visible:**
- Axiom 1: Steps 5–6 — after passing through `UnmarshalText`, the value is guaranteed valid. No other code path can create a `CountryCode` without this validation.
- Axiom 2: Step 4 — `e.value` is written here and nowhere else — the field is private.
- Axiom 3: Step 4 — `CountryCodeType.Values()` provides the validation list. Any type with `Values() []string` works.
- Axiom 4: Generic `T` is `CountryCodeType` here — but the same `Enum[T]` code works for `OrderStatusType`, `CurrencyType`, etc.
- Axiom 5: `CountryCode` gets `MarshalText`, `UnmarshalText`, `Scan`, `Value`, `String` — all from embedding.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| You can create a `CountryCode` with any string like `CountryCode{value: "INVALID"}` | **FALSE** | `value` is unexported — code outside the `common` package can't set it directly (Axiom 2). Only `UnmarshalText`/`Scan`/`MustEnum` can create non-zero values. |
| `Enum[T]` requires `T` to be a `string` type | **FALSE** | `T` must implement `Enumerable` — meaning it has `Values() []string`. `CountryCodeType` is a `string` alias, but the constraint is about the method, not the underlying type. |
| Creating a new enum type requires writing all the serialization methods | **FALSE** | Embedding `Enum[T]` gives you `MarshalText`, `UnmarshalText`, `Scan`, `Value`, `String` for free (Axiom 5). You only define the `Values()` method. |
| An empty string is always invalid for an enum | **FALSE** | The `UnmarshalText` implementation explicitly allows empty strings (zero value). An empty `CountryCode` is valid as a zero value. |
| `MustEnum` is safe to use in production request handling | **FALSE** | `MustEnum` panics on invalid input. It's for use in test fixtures and hardcoded constants where you know the value is correct. For user input, always go through JSON decoding. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Checking `if countryCode.String() == "US"` repeatedly | Code smell — defeats the purpose | You're doing runtime string checks on a type that's already validated | Trust the type — if you have a `CountryCode`, it's already valid. Use the value directly. |
| Using `MustEnum` for user-provided values in production | Runtime panic | `MustEnum` panics on invalid input — not safe for user input | Use JSON decoding (which calls `UnmarshalText`) for user input; `MustEnum` is for known-good constants |
| Forgetting to implement `Values()` on the catalog type | Compile error | `T` in `Enum[T]` must implement `Enumerable`; without `Values()`, `CountryCodeType` doesn't satisfy the interface | Add `func (c CountryCodeType) Values() []string { return []string{...} }` |
| Adding a new valid value to `Values()` but not in the OpenAPI spec | Silent API inconsistency | The OpenAPI spec may still say only 5 values are valid, but Go now accepts 6 | Always update both the spec and the `Values()` function together |
| Defining the enum wrapper struct with fields instead of embedding | Missing methods | `type CountryCode struct { code Enum[CountryCodeType] }` — methods of `Enum[T]` are not promoted | Use embedding: `type CountryCode struct { common.Enum[CountryCodeType] }` |

#### h) Comprehension Task

> **Comprehension Task:** On paper, trace through what happens when this code runs. At each step, identify WHICH axiom of the Enum pattern is active:
> ```go
> // Scenario: JSON: {"country_code": "GB"}
> var cc shared.CountryCode
> cc.UnmarshalText([]byte("GB"))   // Step A
> fmt.Println(cc.String())          // Step B
>
> // Scenario 2: JSON: {"country_code": "UNKNOWN"}
> var cc2 shared.CountryCode
> err := cc2.UnmarshalText([]byte("UNKNOWN"))   // Step C
> fmt.Println(err)                               // Step D
> fmt.Println(cc2.String())                     // Step E
> ```
>
> **What to check:** Step A → Axiom 3 (Values() called, "GB" found valid), Axiom 1 (only path to store value). Step B → prints "GB". Step C → Axiom 3 (Values() called, "UNKNOWN" not found), returns error. Step D → prints `invalid enum value for CountryCodeType: 'UNKNOWN', expected values ["US" "DE" "GB" "JP" "PL"]`. Step E → prints `""` (empty string — the zero value, because `UnmarshalText` failed and never set `e.value`).
>
> **Common wrong answer:** "Step E would print 'UNKNOWN'." — Wrong. `UnmarshalText` only writes to `e.value` after validation succeeds. If validation fails, `e.value` remains the zero value (empty string). The value is never stored if it's invalid.

---

## SECTION 4 — EXERCISE READINESS

### a) What the Exercise Will Likely Ask

Based on the page, the exercise likely asks you to:

1. **Add `x-go-type` and `x-go-type-import`** to `CustomerUUID` and `CountryCode` schemas in `openapi.yaml`.
2. **Run `task gen`** to regenerate `openapi.gen.go` with the new type aliases.
3. **Update `handler.go`** to use `common.NewUUIDv7()` instead of `uuid.New()` (type changed from `google/uuid.UUID` to `common.UUID`).
4. **Possibly define a new enum type** (like `OrderStatus`) using the three-step pattern: define type, implement `Values()`, embed `Enum[T]`.

### b) Pre-Implementation Checklist

Before touching any code:

- [ ] I can state in one sentence what `x-go-type` does and why it matters for type safety.
- [ ] I can predict: after adding `x-go-type: common.UUID` and running `task gen`, what EXACTLY changes in `openapi.gen.go`?
- [ ] I can explain why `uuid.New()` must be replaced with `common.NewUUIDv7()` after the type change.
- [ ] I can trace through what happens when a client sends `"country_code": "INVALID"` — which layer catches it and how.
- [ ] I can write the three-step enum definition pattern from memory for a hypothetical `OrderStatus` type.

### c) Implementation Blueprint

**Step 1:** Open `backend/orders/api/http/openapi.yaml`. Find the `CustomerUUID` schema definition. Add `x-go-type: common.UUID` at the same indent level as `type` and `format`. Add `x-go-type-import: path: eats/backend/common` below it.
*(Relies on Axiom 2 of Custom OpenAPI Types — `x-go-type` inside the schema overrides the generated type alias.)*

**Step 2:** Find the `CountryCode` schema definition. Add `x-go-type: shared.CountryCode` and `x-go-type-import: path: eats/backend/common/shared`.
*(Same axiom — each schema gets its own type override.)*

**Step 3:** Run `task gen`. Check `openapi.gen.go` — the type aliases should now reference `common.UUID` and `shared.CountryCode` instead of `openapi_types.UUID` and `string`.
*(Relies on Axiom 5 of Custom OpenAPI Types — generated code automatically uses new types after regeneration.)*

**Step 4:** Open `handler.go`. Find `uuid.New()`. Replace with `common.NewUUIDv7()`. Add `import "eats/backend/common"` if not already present. Remove `import "github.com/google/uuid"` if no longer used.
*(Relies on Axiom 1 of UUID — `common.UUID` and `google/uuid.UUID` are different types; the field now expects `common.UUID`.)*

**Step 5:** Check that the code compiles. If a new enum type is needed (like `OrderStatus`), follow the three-step pattern: define `OrderStatusType string`, implement `Values()`, create `type OrderStatus struct { common.Enum[OrderStatusType] }`.
*(Relies on Axiom 4 of Enum — the same generic machinery works for any type that implements `Enumerable`.)*

### d) Debugging Guide

| Failure Symptom | Violated Axiom | Fix |
|---|---|---|
| `cannot use uuid.New() (type google/uuid.UUID) as type common.UUID` — compile error | UUID Axiom 1 — different named types are not interchangeable | Replace `uuid.New()` with `common.NewUUIDv7()` in handler.go |
| `openapi.gen.go` still shows `openapi_types.UUID` after editing `openapi.yaml` | Custom Type Axiom 5 — generated code not updated | Run `task gen` — you edited the spec but didn't regenerate |
| `invalid enum value for CountryCodeType: 'XX'` in test output | Not a bug — this is the enum working correctly | The test is sending an invalid country code. Fix the test data or check the request body. |
| `undefined: common.UUID` in generated file | Custom Type Axiom 3 — import path missing | Add `x-go-type-import: path: eats/backend/common` under the `x-go-type` line |
| Enum `Values()` not called — any string accepted | Enum Axiom 3 — `Values()` not implemented or type doesn't satisfy `Enumerable` | Ensure your catalog type (e.g., `OrderStatusType`) has `func (o OrderStatusType) Values() []string` |

---

## SECTION 5 — STRUCTURED PRACTICE (AEIOU Framework)

### A — ACQUIRE (Axioms First)

**Custom OpenAPI Types axioms to internalize:**
1. `x-go-type` overrides the generated type alias without changing the JSON wire format.
2. `x-go-type-import` provides the Go package path so the generated file can import the type.
3. After `task gen`, handler code using the generated struct fields automatically uses the new types.

**UUID v7 axioms to internalize:**
1. `type UUID [16]byte` creates a new, compiler-distinct named type — not interchangeable with `google/uuid.UUID` without explicit conversion.
2. v7 encodes a timestamp in the first 48 bits — inserts always append to the B-tree end.
3. `MarshalText`/`UnmarshalText` are called automatically by the JSON encoder/decoder.

**Enum Pattern axioms to internalize:**
1. `value` is unexported — no external code can create an `Enum` with an arbitrary string.
2. ALL creation paths go through validation (`UnmarshalText`, `Scan`, `MustEnum`).
3. Embedding `Enum[T]` gives the wrapper struct all serialization methods for free.

**Write from memory (no copy-paste):**
- The three-step enum definition pattern for a hypothetical `CurrencyType`.
- The `x-go-type` + `x-go-type-import` block in YAML for `CustomerUUID`.
- The `common.UUID` type declaration and `NewUUIDv7()` function signature.

---

### E — EXERCISE (Reason from Nothing)

**Problem 1 (Simple — x-go-type):**
After adding `x-go-type: common.UUID` and running `task gen`, your `openapi.gen.go` has:
```go
type CustomerUUID = common.UUID
```
Your handler still has `CustomerUuid: uuid.New()`. Will the code compile? Why or why not? Which axiom explains it?

**Problem 2 (Simple — UUID v7):**
A database table has 10 million rows with UUID v4 primary keys. You add UUID v7 for all new rows going forward. Explain in plain English what the insertion performance difference will look like over the next 5 million inserts, and why.

**Problem 3 (Medium — Enum Validation):**
You send a JSON request: `{"country_code": ""}` (empty string). Does `CountryCode.UnmarshalText` reject this? What does the `CountryCode` value look like after this call succeeds? Which axiom explains the behavior of empty strings?

**Problem 4 (Medium — Shared Types):**
Your team proposes adding `type RestaurantMenu struct { Items []MenuItem }` to `backend/common/shared/`. You are the tech lead. Do you approve or reject? Write your reasoning using the axioms from Concept 3.

**Problem 5 (Complex — Full Chain):**
A client sends: `POST /orders/register-customer` with body:
```json
{"name":"Bob","email":"bob@example.com","address":{"line_1":"42 Elm St","postal_code":"10001","city":"New York","country_code":"DE"},"phone_number":"+49123456"}
```
Trace the full path from HTTP request arrival to the handler receiving the validated `CountryCode`. At each step, identify which concept from this page is active (x-go-type, Enum, UUID, Shared Types).

**Problem 6 (Complex — New Enum):**
Using ONLY the three-step pattern from this page, design a `CurrencyCode` enum accepting `"USD"`, `"EUR"`, `"GBP"`, `"JPY"`. Write out the three-step definition in Go (no actual code execution needed — just the structure). Then add the `x-go-type` entries for the corresponding `Price.Currency` field in the OpenAPI schema.

---

### I — INSPECT (Identify the Violation)

**Task 1:** What is wrong? Which axiom is violated?
```yaml
CustomerUUID:
  type: string
  format: uuid
x-go-type: common.UUID           # Note the indentation
x-go-type-import:
  path: eats/backend/common
```

**Task 2:** What is wrong?
```go
// After adding x-go-type: common.UUID and running task gen:
func (h *Handler) RegisterCustomer(ctx context.Context, req RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error) {
    return RegisterCustomer201JSONResponse{
        CustomerUuid: uuid.New(),   // Note: uuid.New() still here
    }, nil
}
```

**Task 3:** What is wrong?
```go
type CurrencyType string

func (c CurrencyType) AllValues() []string {   // Note: AllValues, not Values
    return []string{"USD", "EUR", "GBP"}
}

type Currency struct {
    common.Enum[CurrencyType]
}
```

**Task 4:** What is wrong?
```go
// Creating a CountryCode for use in test data:
countryCode := shared.CountryCode{value: "US"}   // Direct field set
```

**Task 5:** What is wrong?
```go
// Checking if a country code is valid before processing:
func processOrder(cc shared.CountryCode) {
    validCodes := []string{"US", "DE", "GB", "JP", "PL"}
    isValid := false
    for _, v := range validCodes {
        if v == cc.String() { isValid = true }
    }
    if !isValid { return }
    // ... process
}
```

---

### O — ORCHESTRATE (Synthesis Design)

**Scenario:** You're adding a `PlaceOrder` endpoint to the orders module. The request includes:
- `customer_uuid` (must be a customer UUID, not an order UUID — distinct type)
- `restaurant_uuid` (must be a restaurant UUID — a DIFFERENT distinct type from customer UUID)
- `items` (a list of order items — each with a `dish_uuid` and a `quantity`)
- `status` (an enum: `"pending"`, `"confirmed"`, `"cancelled"`)

**Write in plain English:**
1. What do you add to `openapi.yaml` for `customer_uuid` and `restaurant_uuid` to make them distinct types?
2. What three-step enum definition do you write for `OrderStatus`?
3. Is `OrderStatus` a candidate for `backend/common/shared/`? Why or why not?
4. After running `task gen`, what does the generated `StrictServerInterface` look like for `PlaceOrder`?
5. In the handler method, what calls do you make to generate the new order UUID?

**Write the implementation blueprint** step by step, citing the relevant axiom at each step.

---

### U — UNDERSTAND (Feynman Reconstruction Test)

**Most commonly misunderstood concept: The Enum pattern — specifically why the unexported `value` field makes invalid values impossible**

**Step 1 — Explain it simply:**
"Imagine a vending machine that only accepts 1, 2, 5, and 10 dollar bills. There's no slot to push in a 3-dollar bill — the machine's design physically prevents it. The Enum struct is like that vending machine. You can only put valid values in through the official slot (UnmarshalText). The internal storage (the `value` field) has no opening from the outside."

**Step 2 — Find the gap:**
A beginner might ask: "But couldn't you just do `countryCode.value = 'INVALID'` directly?" The gap: they don't understand what "unexported" means — that the lowercase field is completely invisible to code in other packages.

**Step 3 — Go back to axioms:**
Axiom 2: `value` is unexported (lowercase). In Go, an unexported field is completely invisible — it has no name, no address, no existence — from the perspective of any code in a DIFFERENT package. A struct from `backend/common` with an unexported `value` field cannot have that field set by code in `backend/orders` or anywhere outside `backend/common`. The only way to put a non-empty string into `value` is to call `UnmarshalText`, `Scan`, or `MustEnum` — ALL of which validate first.

**Step 4 — Proof of Understanding:**
**Question:** Two developers have a debate. Dev A says: "Structs with unexported fields are annoying because you can't create them with struct literals." Dev B says: "That's exactly the point — it's a feature, not a bug." Who is right and why, specifically in the context of the Enum pattern?

**Expected Answer:** Dev B is right. The inability to create a `CountryCode{value: "ANYTHING"}` struct literal is precisely what makes the guarantee work. If you COULD create it with a struct literal, you could bypass the validation — setting `value: "MARS"` without going through `UnmarshalText`. The restriction IS the safety mechanism. The "inconvenience" of not being able to use struct literals is the price of the correctness guarantee.

---

## SECTION 6 — KNOWLEDGE CHECK

### a) Scenario-Based Reasoning Questions

**Q1:** You have two handler methods: `RegisterCustomer` (returns `CustomerUUID`) and `CreateOrder` (returns `OrderUUID`). Both are `common.UUID` under the hood. Why would you still want them to be different types? What would you do to make them different Go types while using the same underlying UUID implementation?

**Expected Answer:** Even though both are `common.UUID` at the bit level, you don't want `OrderUUID` assignable to `CustomerUUID` in Go. The solution is to define wrapper types: `type CustomerUUID common.UUID` and `type OrderUUID common.UUID`. Now they're distinct named types — the compiler prevents passing one where the other is expected. This extends the same principle of `common.UUID` being distinct from `google/uuid.UUID` — but applied to domain-specific identity types.

---

**Q2:** A developer says: "I can just add my own `Address` type in the `orders` module — I don't need the shared package." Six months later, the `restaurants` module also needs an `Address`. What problem arises, and what is the fix?

**Expected Answer:** `orders.Address` and `restaurants.Address` are different Go types. When the orders module tries to pass an address to the restaurants module (e.g., "deliver to the restaurant's address"), Go rejects it — type mismatch. The developer must write an `OrderAddressToRestaurantAddress(addr orders.Address) restaurants.Address` conversion function. If `Address` has 10 fields, each conversion risks forgetting one. The fix is `shared.Address` from the start — one type, used everywhere, no conversion needed. Axiom 1 of Shared Types: types must be identical to be interchangeable.

---

**Q3:** Why does `Enum[T].UnmarshalText` return an error rather than silently using the zero value when an invalid string is provided?

**Expected Answer:** If `UnmarshalText` silently returned a zero value instead of an error, an invalid `"MOON"` country code would become an empty `CountryCode` — and the caller would have no idea the input was bad. The purpose of the Enum pattern (Axiom 1) is to make invalid values impossible to create. Returning an error surfaces the problem at the exact point of creation — the JSON decoder then surfaces it as a 400 response before the handler runs. Silently ignoring invalid input would defeat the entire purpose and is exactly the bug the pattern was designed to prevent.

---

**Q4:** You're benchmarking two tables: one with UUID v4 primary keys, one with UUID v7. The v4 table has already been around for 2 years with 50 million rows. You just switched new inserts to UUID v7. Will the INSERT performance improve immediately or gradually? Why?

**Expected Answer:** Immediately and dramatically for new inserts. UUID v7 values are always greater than existing v4 values (because v4 is random, v7 encodes current time which is after the creation time of the old rows). Every new UUID v7 insert goes to the END of the B-tree — it never touches the random pages scattered throughout the index from the v4 era. The OLD pages stay scattered, but new inserts don't touch them. Performance for new inserts improves from the very first v7 insert. (The old random pages are a separate problem that could be fixed by rebuilding the index offline.)

---

**Q5:** The `MustEnum[CountryCode]("US")` function is described as "advanced." What does the generic constraint `W ~struct{ Enum[T] }` mean, and why is it necessary? (Explain without diving deep into Go generics — just the purpose.)

**Expected Answer:** The constraint `~struct{ Enum[T] }` means: "W must be a struct type whose underlying layout is a struct containing exactly an `Enum[T]`." This ensures `MustEnum` can only be called on proper wrapper types (like `CountryCode`, not on `Enum[T]` directly). Without this constraint, you could call `MustEnum[Enum[CountryCodeType]]("US")` — bypassing the wrapper type entirely. The constraint enforces that the result is the wrapper type (`CountryCode`), not the raw `Enum[T]`. This preserves the design: users work with `CountryCode`, not with raw `Enum[CountryCodeType]`.

---

### b) "What If?" First-Principles Challenges

**What if `x-go-type` did NOT exist in oapi-codegen?**

Without `x-go-type`, all generated type aliases would use oapi-codegen's default mappings: `openapi_types.UUID` for UUID fields, `string` for string fields. You would have two options, both painful:

**Option 1: Manual conversion in every handler.** Your handler receives `openapi_types.UUID` from the request and must convert it to `common.UUID` before passing it to business logic:
```go
customerUUID := common.UUID(request.Body.CustomerUuid)  // Repeat in every handler
```
With 30 handlers each having 3 UUID fields, that's 90 conversion lines — any forgotten conversion is a bug.

**Option 2: Don't use custom types.** Use `openapi_types.UUID` everywhere. Now the compiler can't tell `CustomerUUID` from `OrderUUID` — you lose all compile-time type safety. The bug where you pass an order UUID where a customer UUID is expected compiles fine and runs fine until a customer sees someone else's order.

`x-go-type` exists precisely because both options are unacceptable. It collapses the gap between "what the spec says" and "what Go types you want to use" — making the right thing automatic and the wrong thing a compile error.

---

**What if the `value` field in `Enum[T]` was EXPORTED (uppercase `Value string`)?**

If `value` were exported, any code in any package could create an invalid enum:
```go
// Would compile and run — no validation:
badCode := shared.CountryCode{Value: "MOONBASE_ALPHA"}
processOrder(badCode)   // "MOONBASE_ALPHA" reaches the database
```

The entire guarantee of the Enum pattern — "if you have a `CountryCode`, it is valid" — collapses. Every function receiving a `CountryCode` would need to validate it manually:
```go
func processOrder(cc shared.CountryCode) error {
    if !isValidCountryCode(cc.Value) { return ErrInvalidCountryCode }
    // ... actual logic
}
```

That's exactly the situation we started with (the problem statement). The unexported field IS the pattern. Without it, you have a regular struct with a `Value` field — no different from using a plain string except for more typing.

---

**What if UUID v7 did NOT exist — only UUID v4 and auto-increment integers?**

You'd face a dilemma:

**Use auto-increment integers:** Fast inserts, ordered, great for database performance. But requires a central database counter. You cannot generate an ID before inserting to the database — you must insert first, then read the ID back. In a distributed system where multiple service instances need to generate IDs, all instances compete for the same counter. The database becomes a bottleneck. Also, IDs are predictable — someone can enumerate records by incrementing the ID.

**Use UUID v4:** No central counter, fully distributed ID generation, not enumerable. But random insertion order causes fragmentation. At 10 million rows: 4 minutes to insert vs 36 seconds for ordered UUIDs. At 100 million rows, the gap widens further.

UUID v7 gives you the best of both worlds: generated independently (no central counter), not enumerable (random bits beyond the timestamp), time-ordered (database inserts always go to the end). It exists because the distributed-system benefits of UUIDs (no central coordination) and the database-performance benefits of ordered keys (sequential inserts) are not mutually exclusive — if you encode time in the key.

---

### c) Page FAQ

**Q: Why does `common.NewUUIDv7()` panic on error instead of returning an error?**
The reason this works this way is that UUID v7 generation failure is caused by the system clock — an operating system-level failure that makes the entire application fundamentally broken, not a recoverable business error. If the clock isn't working, nothing in the application can function correctly anyway. Panicking immediately is the right response: it surfaces the problem instantly rather than letting the application silently generate bad IDs. This is distinct from a `CountryCode.UnmarshalText` error — that's a user input error (recoverable). UUID generation failure is an infrastructure error (not recoverable).

**Q: The page says "the value field is unexported — you can't create an Enum value without going through UnmarshalText." But can't you use `var cc shared.CountryCode` and get an empty value?**
The reason this works this way is that `var cc shared.CountryCode` gives you the zero value — a `CountryCode` with `value: ""`. The `IsZero()` method returns `true` for this. An empty enum value is intentionally allowed as the "not set" state (like a nil pointer for a pointer type). The guarantee is: if a `CountryCode` has a non-empty value, that value has been validated. You can check `cc.IsZero()` before using it if you need to distinguish "not set" from "set to a valid code."

**Q: Why is `CountryCodeType` a separate type from `CountryCode`? Why not just put `Values()` directly on `CountryCode`?**
The reason this works this way is that `Enum[T]` requires `T` to be the type that knows the valid values. The constraint `T Enumerable` means `T` must have a `Values()` method. `CountryCode` itself embeds `Enum[CountryCodeType]` — it can't be BOTH the wrapper and the type parameter without a circular definition. `CountryCodeType` is the clean separation: it IS just a string alias whose only job is to carry the `Values()` method. `CountryCode` IS the usable wrapper that embeds the validation machinery. Two distinct roles, two distinct types.

**Q: Why does the `Address` type live in `shared` instead of `orders` if orders is the first module using it?**
The reason this works this way is that putting `Address` in `orders` would mean the `restaurants` module (and eventually `couriers` and `payments`) would depend on the `orders` package just to use an address. That creates a dependency from restaurants on orders — architecturally wrong. Modules should depend on shared infrastructure, not on each other. Even though `orders` is the first module, the `Address` type's natural home is shared infrastructure because it describes a universal real-world concept, not an orders-domain concept.

**Q: What's the difference between `MustEnum[CountryCode]("US")` and `MustNewCountryCode("US")`?**
The reason this works this way is that both do the same thing — create a `CountryCode` with value `"US"` and panic if invalid. `MustEnum` is the generic helper that works for ANY enum type: `MustEnum[OrderStatus]("pending")`, `MustEnum[CurrencyCode]("EUR")`. `MustNewCountryCode` is a type-specific constructor that existed before `MustEnum` was introduced. Both are equivalent for `CountryCode`. In new code, `MustEnum[T]` is preferred because it's one consistent pattern for all enums.

---

## SECTION 7 — JARGON BUSTER DICTIONARY

---

**Term: B-tree Index**
First-Principles Origin: Databases needed a data structure to find any row quickly without reading every row — like a sorted filing system where you can jump to the right drawer immediately.
Meaning: The internal data structure a database uses to keep rows sorted by a column (like the primary key). It works like a branching tree of sorted values — you follow branches to find your target without checking every row.
Analogy: A library's card catalog. Each card is sorted alphabetically. To find "Tolstoy," you don't read every card — you open the "T" drawer and go from there. Adding a new book in alphabetical order is easy. Adding a book whose title starts with a random letter means reshuffling.
Example:
```
UUID v4 inserts (random):
[   5   ][   2   ][   8   ][   1   ][   9   ]  ← random positions, scattered
UUID v7 inserts (time-ordered):
[   1   ][   2   ][   3   ][   4   ][   5   ]  ← always appended to the right
```
Don't Confuse With: A regular sorted array — a B-tree has multiple levels; appending to the end is O(log n) work, not O(n). The point is that time-ordered keys ALWAYS go to the rightmost position.

---

**Term: Embedding (in Go)**
First-Principles Origin: Go needed a way to share methods between types without traditional class inheritance — embedding lets one type gain all the methods of another type automatically.
Meaning: Placing one type inside another struct without giving it a field name. The outer type automatically "inherits" all the methods of the embedded type without having to redeclare them.
Analogy: A Swiss Army knife. The base knife (embedded type) has blade, bottle opener, scissors. When you buy the "Pro" model (outer type), it includes everything from the base model plus extras — you didn't write the blade again, it's just there.
Example:
```go
type Base struct{ value string }
func (b Base) Hello() string { return "hello from base" }

type Derived struct {
    Base   // Embedding — no field name
}

d := Derived{}
d.Hello()   // Works! Method promoted from Base to Derived automatically
```
Don't Confuse With: Composition (named field) — `type Derived struct { b Base }` — with a name, you'd call `d.b.Hello()`. With embedding (no name), you call `d.Hello()` directly.

---

**Term: Enumerable (interface)**
First-Principles Origin: The generic `Enum[T]` needed a constraint specifying WHAT `T` must provide — a way to say "T must know its valid values."
Meaning: A Go interface with one method: `Values() []string`. Any type that implements this method can be used as the type parameter in `Enum[T]`.
Analogy: A job requirement: "Must be able to list all valid account types." Any candidate who can do that (any type with `Values()`) qualifies for the job (can be used as `T`).
Example:
```go
type Enumerable interface {
    Values() []string   // Must return the list of valid values
}

// CountryCodeType satisfies Enumerable:
type CountryCodeType string
func (c CountryCodeType) Values() []string {
    return []string{"US", "DE", "GB"}   // Lists all valid values
}
```
Don't Confuse With: `Enum[T]` (the struct) — `Enumerable` is the interface (the requirement); `Enum[T]` is the implementation (the machinery that uses types satisfying that requirement).

---

**Term: Generic Type / Generics**
First-Principles Origin: Developers needed to write one implementation of a data structure or algorithm that works correctly with many different types, without code duplication and without losing type safety.
Meaning: A type or function parameterized by a type variable (`T`). `Enum[T Enumerable]` is one piece of code that works for ANY type `T` that satisfies `Enumerable` — `Enum[CountryCodeType]`, `Enum[OrderStatusType]`, `Enum[CurrencyType]`, all from the same source code.
Analogy: A universal charging cable adapter. One design works with any phone brand — you just plug in the specific brand's connector at one end. The adapter itself doesn't change; only the type parameter (brand connector) changes.
Example:
```go
// Generic function — works for any Enumerable type T:
func IsValid[T Enumerable](value string) bool {
    var t T
    for _, v := range t.Values() {
        if v == value { return true }
    }
    return false
}

// Same code, different T:
IsValid[CountryCodeType]("US")     // Uses CountryCodeType.Values()
IsValid[OrderStatusType]("pending") // Uses OrderStatusType.Values()
```
Don't Confuse With: An interface — an interface defines what a type CAN do; generics define a template that WORKS WITH types satisfying a constraint. An interface is a runtime concept; generics are a compile-time concept.

---

**Term: `MarshalText` / `UnmarshalText`**
First-Principles Origin: Go's JSON encoder needed a standard way for custom types to define their own text representation — without modifying the encoder itself.
Meaning: Two methods that define how a type converts to and from a text string. `MarshalText` converts the type to `[]byte` text (for JSON encoding, YAML, etc.). `UnmarshalText` reads `[]byte` text and populates the type (for JSON decoding).
Analogy: A passport converter. `MarshalText` is like stamping your information into an official document format. `UnmarshalText` is like reading a passport and extracting the person's details into your system.
Example:
```go
type Temperature float64

func (t Temperature) MarshalText() ([]byte, error) {
    return []byte(fmt.Sprintf("%.1f°C", t)), nil   // "36.6°C"
}

func (t *Temperature) UnmarshalText(data []byte) error {
    // Parse "36.6°C" back to float64
    _, err := fmt.Sscanf(string(data), "%f°C", t)
    return err
}
```
Don't Confuse With: `MarshalJSON`/`UnmarshalJSON` — those are JSON-specific; `MarshalText`/`UnmarshalText` work for any text encoding (JSON uses them when available, but so do other text formats).

---

**Term: Named Type**
First-Principles Origin: Go needed a way to create distinct types from the same underlying data, so the compiler could enforce that they're not accidentally interchangeable.
Meaning: A new type created with `type NewName ExistingType`. Even if the underlying data is identical, `NewName` and `ExistingType` are different Go types — you cannot use one where the other is expected without explicit conversion.
Analogy: A U.S. dollar bill vs. a Canadian dollar bill. Both are paper rectangles with numbers printed on them (same underlying "type"). But you can't spend Canadian dollars at a U.S. cash register — different labels, different acceptance rules, explicit exchange (conversion) required.
Example:
```go
type Celsius float64
type Fahrenheit float64

var c Celsius = 100.0
var f Fahrenheit = c     // COMPILE ERROR — different named types
var f2 Fahrenheit = Fahrenheit(c)  // OK — explicit conversion
```
Don't Confuse With: Type alias (`type UUID = openapi_types.UUID` with `=`) — a type alias IS the same type (just a different name). A named type (`type UUID [16]byte` without `=`) creates a NEW, distinct type.

---

**Term: `Scan` / `Value` (database interface)**
First-Principles Origin: Go's database/sql package needed a standard way for custom types to read from and write to database rows without requiring the database driver to know about every possible type.
Meaning: Two methods that connect a custom Go type to a SQL database. `Value()` converts the Go type to a database-compatible value (string, int, etc.) for writing. `Scan()` reads a database value and populates the Go type.
Analogy: A customs form. `Value()` is like filling in the customs form to declare what you're bringing into a country (sending Go data to the database). `Scan()` is like reading the customs form to understand what arrived from another country (reading database data into Go).
Example:
```go
// Reading from DB: database has "US" → CountryCode.Scan("US") → validates → stores
// Writing to DB: CountryCode.Value() → returns "US" (string) → database stores it
```
Don't Confuse With: `MarshalText`/`UnmarshalText` — those are for JSON/text encoding. `Scan`/`Value` are specifically for SQL database rows. A type implementing both works with BOTH JSON and SQL.

---

**Term: Type Alias**
First-Principles Origin: oapi-codegen needed to map its generated names (like `CustomerUUID`) to Go types while allowing the underlying type to be swapped without touching every usage.
Meaning: A type alias (`type A = B`) creates a new NAME for an existing type — but it IS the same type. `A` and `B` are interchangeable everywhere. No conversion needed.
Analogy: A nickname. Your name might be "Alexander" and your nickname "Alex" — both refer to the same person. You can use either name and everyone knows who you mean.
Example:
```go
type CustomerUUID = common.UUID   // Alias — CustomerUUID IS common.UUID
type OrderUUID    = common.UUID   // Also an alias — OrderUUID IS ALSO common.UUID

var c CustomerUUID = common.NewUUIDv7()
var o OrderUUID    = c   // ⚠️ Compiles! Both are common.UUID — NO protection between them

// For real compile-time protection, use named types (no =):
type CustomerUUID common.UUID   // NEW distinct type
type OrderUUID    common.UUID   // ANOTHER distinct type
var o2 OrderUUID  = OrderUUID(c) // explicit conversion required — compiler enforces distinction
```
Don't Confuse With: Named type (`type UUID [16]byte` without `=`) — a named type creates a NEW distinct type. An alias (`=`) is just a second name for the same type. Aliases are interchangeable; named types are not.

---

**Term: UUID v7 (vs UUID v4)**
First-Principles Origin: Distributed systems needed unique IDs that could be generated independently by any node AND that maintained time-ordering for database index performance.
Meaning: A UUID where the first 48 bits encode the current time (milliseconds). New UUIDs are always greater than old ones — they sort correctly by creation time and database inserts always append to the end of sorted indexes.
Analogy: A date-stamped ticket system at a bakery. Each ticket has today's date and a sequential number: `20260426-001`, `20260426-002`. New tickets are always after old ones — easy to sort, easy to file, new tickets always go at the back of the stack.
Example:
```go
id1 := common.NewUUIDv7()   // e.g., "019512b7-3b0a-7000-..."  ← time T
id2 := common.NewUUIDv7()   // e.g., "019512b7-5c1f-7000-..."  ← time T+500ms
// id2 > id1 guaranteed — database appends id2 after id1 in the B-tree
```
Don't Confuse With: UUID v4 — fully random, no time component, fast to generate but causes random B-tree insertions that slow down as the table grows.

---

**Term: `x-go-type`**
First-Principles Origin: oapi-codegen needed a way for developers to override the default generated type mappings in OpenAPI specs without modifying the code generator itself.
Meaning: A YAML extension field (not standard OpenAPI — only oapi-codegen understands it) placed inside a schema definition to replace the default generated Go type alias with a custom type.
Analogy: A note in a recipe: "For this recipe, substitute butter with ghee." The recipe is otherwise unchanged (same API contract), but the kitchen (Go code) uses a different specific ingredient (type) than the default.
Example:
```yaml
CustomerUUID:
  type: string
  format: uuid
  x-go-type: common.UUID          # Override: use common.UUID instead of openapi_types.UUID
  x-go-type-import:
    path: eats/backend/common     # Import path for common.UUID
```
Don't Confuse With: `x-go-type-import` — that's the companion field providing the import path. `x-go-type` specifies the type name; `x-go-type-import` specifies where to find it.

---

## SECTION 8 — RETENTION & REVISION PLAN

### a) The "Right Now" Rule — Do This Before Closing the Page

1. **Re-write from memory:** Write the three-step enum definition for a hypothetical `OrderStatus` (values: `"pending"`, `"confirmed"`, `"delivered"`). While writing, say out loud which axiom each line relies on.
2. **Reconstruct the YAML block:** Without looking, write the `x-go-type` + `x-go-type-import` block for `CustomerUUID` pointing to `common.UUID` in `eats/backend/common`.
3. **Explain the UUID v7 performance benefit:** Without notes, explain to an imaginary 10-year-old why time-ordered IDs make a database faster. Use only the filing cabinet / ticket analogy.
4. **Debug this code:** `type CountryCode struct { common.Enum[CountryCodeType] }` then `cc := shared.CountryCode{value: "US"}` — identify the bug (unexported field access) and which axiom it violates.

---

### b) 3-Day Revision Checklist (Axiom-Level Mastery)

- [ ] I can write the `x-go-type` + `x-go-type-import` block in YAML, explain that it only changes Go code (not JSON wire format), and state why type distinction matters — all without notes.
- [ ] I can write `type UUID [16]byte`, `NewUUIDv7()`, and explain why v7 beats v4 in database performance using the B-tree concept — all without notes.
- [ ] I can write the three-step enum pattern (catalog type → `Values()` → wrapper struct with embedding), explain why the unexported field makes invalid values impossible, and state when to use `MustEnum` vs JSON decoding — all without notes.
- [ ] I can state the "good vs bad for shared types" rule with three examples of each, and explain which axiom (coupling) determines the boundary — all without notes.

---

### c) 7-Day Challenge

**Scenario:** The platform is adding a `payments` module. It needs:
- A `PaymentUUID` that is distinct from `CustomerUUID` and `OrderUUID` at compile time.
- A `Currency` enum accepting `"USD"`, `"EUR"`, `"GBP"`, `"JPY"`.
- A `PaymentStatus` enum with values `"pending"`, `"processing"`, `"completed"`, `"failed"`, `"refunded"`.
- The question: should `Currency` go in `shared/`, and should `PaymentStatus` go in `shared/`?

**Success looks like:** A written design document with: the three-step enum definitions for both enums, the `x-go-type` YAML blocks for the payments OpenAPI spec, the justification (using axioms) for where each type lives (shared vs. module-specific), and the handler method signature for `ProcessPayment` using all these types.

**Start here:** Axiom 4 of Shared Types — does `PaymentStatus` have a natural owner? Which module's team should own the definition of "what payment statuses exist"?

---

### d) 30-Day Connection Bridge

- **Every future handler** will use `common.NewUUIDv7()` to generate entity IDs — and each entity will have its own UUID named type to prevent mix-ups. The shared axiom: different domains need distinct types even when the underlying data is the same.
- **Every future module** that adds an enum field (order status, courier status, payment method) will use the three-step Enum pattern. The shared axiom: validation at the boundary (UnmarshalText) is always cheaper than validation in every consuming function.
- **The database module** (next few exercises) will use `common.UUID`'s `Scan`/`Value` methods to read/write UUIDs from PostgreSQL — you'll see exactly why those methods exist. The shared axiom: a type should know how to persist itself.
- **Future shared types** (currency, coordinate) will be evaluated using the stability/coupling trade-off table from Concept 3. The shared axiom: coupling cost must be weighed against deduplication benefit before adding to shared.
- **In real-world Go projects**, this exact enum pattern (unexported field, `UnmarshalText` validation, generic `Enum[T]`) is the standard approach for domain value types. You'll recognize it immediately in any well-structured Go codebase.

---

### e) Flashcard Set

**Card 1**
FRONT: Why does `x-go-type` exist? What problem does oapi-codegen's default type mapping create?
BACK: By default, ALL UUID fields generate `openapi_types.UUID` — the same Go type. The compiler can't tell `CustomerUUID` from `OrderUUID`. Axiom 1 of Custom Types: types that represent different things must BE different Go types for the compiler to catch mistakes. `x-go-type` assigns a distinct type to each schema.

**Card 2**
FRONT: After adding `x-go-type: common.UUID` to the `CustomerUUID` schema and running `task gen`, your handler still has `uuid.New()`. What happens when you compile?
BACK: Compile error. `uuid.New()` returns `google/uuid.UUID`. After `task gen`, the `CustomerUuid` field in the response struct now expects `common.UUID` (a different named type — Axiom 1 of UUID). The fix: replace `uuid.New()` with `common.NewUUIDv7()`.

**Card 3**
FRONT: Why does UUID v7 outperform UUID v4 for database inserts at scale?
BACK: UUID v4 is random — each insert lands at a random position in the B-tree index. As the table grows, more positions are not in memory → disk reads per insert → slow. UUID v7 encodes a timestamp in the first 48 bits (Axiom 3 of UUID) — new IDs are always greater → inserts always go to the END of the B-tree (Axiom 4) → no random access, no disk reads. At 10M rows: 36s (v7) vs 4+ min (v4).

**Card 4**
FRONT: Why is the `value` field in `Enum[T]` unexported? What would break if it were exported?
BACK: Axiom 2 of Enum — `value` is unexported so no external package can create an Enum with an arbitrary string. If it were exported, any code could do `CountryCode{Value: "MOONBASE"}` — bypassing validation entirely. The guarantee "every non-zero CountryCode is valid" would collapse. The restriction IS the safety mechanism.

**Card 5**
FRONT: What are the three steps to define a new enum type using the `Enum[T]` pattern?
BACK: Step 1: Define the catalog type — `type OrderStatusType string`. Step 2: Implement `Values()` — `func (o OrderStatusType) Values() []string { return []string{"pending","confirmed",...} }`. Step 3: Create the wrapper — `type OrderStatus struct { common.Enum[OrderStatusType] }`. The wrapper gets `MarshalText`, `UnmarshalText`, `Scan`, `Value`, `String` for free via embedding (Axiom 5).

**Card 6**
FRONT: What happens when `CountryCode.UnmarshalText([]byte("INVALID"))` is called?
BACK: The method creates a zero `CountryCodeType` value, calls `.Values()` to get the valid list, loops through it — "INVALID" is not found. It returns an error: `invalid enum value for CountryCodeType: 'INVALID', expected values ["US" "DE" "GB" "JP" "PL"]`. The `CountryCode.value` field remains empty (unchanged). The JSON decoder surfaces this as a 400 error — the handler never runs (Axiom 1 of Enum — no invalid value can be stored).

**Card 7**
FRONT: Is `type CustomerUUID = common.UUID` (with `=`) the same as `type CustomerUUID common.UUID` (without `=`)?
BACK: NO. With `=` it's a TYPE ALIAS — `CustomerUUID` IS `common.UUID`, they're interchangeable. Without `=` it's a NAMED TYPE — `CustomerUUID` is a NEW distinct type that cannot be used where `common.UUID` is expected without explicit conversion. The generated `openapi.gen.go` uses aliases (`=`) — that's fine because they're all `common.UUID` under the hood. For domain-specific types you want truly distinct, use named types (no `=`).

**Card 8**
FRONT: Why does `Address` belong in `shared/` and `OrderItem` does not?
BACK: Axiom 3 of Shared Types — good shared types are small, stable, no business logic. `Address` is a universal real-world concept: street, city, postal code, country. It doesn't change when business rules change. Axiom 4 — `OrderItem` is owned by the orders domain: its fields (quantity, price adjustments, discounts) change as the orders module evolves. Sharing it would force the payments or restaurants team to update their code every time orders changes its menu item structure.

**Card 9**
FRONT: Reconstruct the `common.UUID` type declaration, `NewUUIDv7()` function, and `MarshalText`/`UnmarshalText` pair from memory.
BACK:
```go
type UUID [16]byte

func NewUUIDv7() UUID {
    u, err := uuid.NewV7()
    if err != nil { panic(err) }
    return UUID(u)
}

func (u UUID) MarshalText() ([]byte, error) {
    return uuid.UUID(u).MarshalText()
}

func (u *UUID) UnmarshalText(data []byte) error {
    var guuid uuid.UUID
    if err := guuid.UnmarshalText(data); err != nil { return err }
    *u = UUID(guuid)
    return nil
}
```

**Card 10**
FRONT: What problem does `x-go-type-import` solve? What would happen without it?
BACK: `x-go-type-import` provides the Go import path for the custom type. Without it, oapi-codegen generates `type CustomerUUID = common.UUID` in `openapi.gen.go` but doesn't add `import "eats/backend/common"` — the generated file references `common.UUID` without knowing where `common` is. Compile error: `undefined: common`. Axiom 3 of Custom OpenAPI Types: the import path is required when the custom type is in a different package from the generated file.

---

## SECTION 9 — QUICK REFERENCE CHEAT SHEET

### Custom OpenAPI Types — YAML Syntax

```yaml
# Inside a schema definition in openapi.yaml:
CustomerUUID:
  type: string
  format: uuid
  x-go-type: common.UUID           # Go type to use instead of default
  x-go-type-import:
    path: eats/backend/common      # Import path for the Go type

CountryCode:
  type: string
  x-go-type: shared.CountryCode
  x-go-type-import:
    path: eats/backend/common/shared
```

**After change:** Always run `task gen`. Check `openapi.gen.go` for the new type aliases.

---

### UUID v7 — Key Facts

```go
// Use this (not uuid.New()):
id := common.NewUUIDv7()   // Returns common.UUID — time-ordered, distinct type

// common.UUID vs google/uuid.UUID:
type UUID [16]byte          // Same underlying data, DIFFERENT named type
                            // Cannot use one where the other is expected
```

| | UUID v4 | UUID v7 |
|---|---|---|
| **Order** | Random | Time-ordered |
| **DB insert at scale** | Slow (random scatter) | Fast (always append) |
| **Central coordination** | Not needed | Not needed |
| **10M row benchmark** | 4+ minutes | 36 seconds |

---

### Enum Pattern — Three Steps

```go
// Step 1: Catalog type
type OrderStatusType string

// Step 2: Values() method
func (o OrderStatusType) Values() []string {
    return []string{"pending", "confirmed", "delivered", "cancelled"}
}

// Step 3: Wrapper struct with embedding
type OrderStatus struct {
    common.Enum[OrderStatusType]  // Gets MarshalText, UnmarshalText, Scan, Value, String for free
}

// Usage:
status := common.MustEnum[OrderStatus]("confirmed")  // In tests/constants
// For user input: use JSON decoding (UnmarshalText is called automatically)
```

---

### Shared Types — Decision Table

| Put in `shared/` ✓ | Keep in the module ✗ |
|---|---|
| UUID, Address, CountryCode, Currency | Customer struct, OrderStatus, PaymentMethod |
| Small, stable, no business logic | Business entities, request/response structs |
| Used by 2+ modules | Used by only 1 module |
| Won't need new fields as business evolves | Fields evolve with one team's requirements |

---

### Key Rules / Gotchas

- **Always run `task gen`** after editing `openapi.yaml` — without it, code and spec drift apart.
- **Replace `uuid.New()` with `common.NewUUIDv7()`** when `CustomerUUID` type changes to `common.UUID`.
- **Never set Enum `value` directly** — it's unexported. Use `MustEnum`, `UnmarshalText`, or `Scan`.
- **Use `MustEnum` only for known-good constants** — it panics on invalid input (not safe for user data).
- **Type alias (`=`)** = same type, interchangeable. **Named type (no `=`)** = new distinct type, NOT interchangeable.
- **`x-go-type-import` is required** when the custom type is in a different package than `openapi.gen.go`.

---

### One-Line Plain-English Reminders

- **`x-go-type`:** "Tell oapi-codegen to use MY type instead of the default — same JSON, better Go safety."
- **`common.UUID`:** "A custom type that looks like a UUID but can't be confused with other UUID types at compile time."
- **UUID v7:** "Time in the first half of the ID — database inserts always go at the end, never scatter."
- **Shared types:** "Small and stable = good to share. Big and evolving = belongs in one module."
- **Enum pattern:** "A string that can only hold pre-approved values — validation happens once at entry, never again inside."
- **Unexported `value`:** "The lock that makes the Enum guarantee work — no external code can bypass validation."
