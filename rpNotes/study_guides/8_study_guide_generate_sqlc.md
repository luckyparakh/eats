# Study Guide: Generate sqlc

> **Page:** "Generate sqlc" (SQL-first code generation with `sqlc`, the two key files — `sqlc.yaml` and annotated `queries/*.sql` — query annotations, type-mapping overrides, and the `go generate` / `task gen` workflow)
> **Track:** backend-masterclass-beta, module `04-database`, exercise `02-generate-sqlc` (confirmed via `.tdl-exercise`).
> **Previous guide:** `7_study_guide_sql_migrations.md` — that page built the schema (`orders.customers` table, migration mechanics). This page is exactly the "next module" that guide explicitly deferred to: *why* `sqlc` needs `CREATE SCHEMA IF NOT EXISTS orders` inside the migration file itself, and how to actually query the table that now exists.
> **What this likely unlocks next:** a repository/adapter in `backend/orders/adapters/db/` that calls the generated `dbmodels` functions from the `RegisterCustomer` HTTP handler — the handler currently validates a request and does nothing with it (noted as the open thread in guide 7).
>
> **Workspace state (checked directly, not from `exercise.md`):** `backend/orders/adapters/db/sqlc.yaml` exists and matches the page's shape closely, with more overrides than the page's example snippet shows (it also maps nullable/non-nullable `timestamptz` to `*time.Time`/`time.Time`, not just `uuid` and `address`). `backend/orders/adapters/db/sqlc.go` has the exact `//go:generate go tool sqlc generate` directive. `backend/orders/adapters/db/queries/customers.sql` exists but currently contains only `-- todo: implement` — writing the actual annotated queries is this exercise's deliverable, so its intended content isn't guessed at here. `backend/orders/adapters/db/dbmodels/` doesn't exist yet — `sqlc generate` hasn't been run against real queries. `backend/common/uuid.go` confirms `common.UUID` is `[16]byte` implementing `MarshalText`/`UnmarshalText` and, importantly for this page, `Value() (driver.Value, error)` and `Scan(src any) error` — i.e. it already satisfies `database/sql`'s `driver.Valuer`/`sql.Scanner`. `backend/common/shared/address.go` confirms `shared.Address` implements the same two methods, marshaling to/from JSON. One gap worth flagging: `sqlc.yaml`'s `uuid, nullable: true` override points at `common.NullUUID`, but no `NullUUID` type exists anywhere in `backend/common` yet — see the Plumbing Dissection for why this is currently dormant rather than broken.

---

## 1. Snapshot

- **Topic:** `sqlc` reads migration files (not a live database) to learn the schema, then turns hand-written, annotated `.sql` query files into type-safe Go functions and structs — one pair of types per query, not one shared model per table.
- **Objective:** After this page, I can explain why `sqlc` generates code instead of using reflection, why a write query and a read query against the same table get different generated types, how the two config files (`sqlc.yaml`, `queries/*.sql`) divide responsibility, and why `common.UUID` and `shared.Address` need explicit overrides instead of being inferred.
- **Continuity:** Picks up exactly where guide 7 stopped — the migration file's `CREATE SCHEMA IF NOT EXISTS orders` statement (previously a forward reference) exists specifically so `sqlc` has a schema to parse when it scans `migrations` for table definitions.

## 2. Concepts

### SQL-First Code Generation (vs. Struct-First ORMs)

- **Problem:** [From page] Struct-first ORMs (Rails/Django-style: define a struct, the tool generates SQL) hide the actual SQL being run and rely on reflection, which pushes schema/query mismatches to runtime instead of catching them earlier.
- **Mechanism:** [From page] `sqlc` works in reverse — you write the SQL you'd write anyway, annotate it with a `-- name: ... :cmd` comment, and `sqlc` generates a matching Go function with exact argument and return types. No reflection, no runtime type surprises. `sqlc` reads migration files to infer the schema; it doesn't need a running database.
- **Design Decision:** [From page] Writes and reads get separate generated types even against the same table — an `INSERT` touching five columns produces a different params struct than a ten-table dashboard join. The page explicitly frames this as CQRS (Command Query Responsibility Segregation: write models and read models don't have to match), linking out to "How to use basic CQRS in Go" for more.
- **In Production:** [AI explanation] Because `sqlc` generates at build/gen time from the schema it parses out of migration files, a query that references a dropped column or wrong type fails `sqlc generate` (or CI, if generation is checked there) before the code ever ships — the same "compile-time over runtime" tradeoff guide 4 covered for OpenAPI-generated HTTP types, just one layer down at the database boundary.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Assuming one generated struct should serve both an insert and a complex read, and hand-editing it to fit both | Fighting the generator instead of using it — and any edit gets silently overwritten on the next `sqlc generate` | Coming from struct-first ORM habits where one model maps to one table | Let each annotated query get its own generated params/result type; write a second query if the read shape differs from the write shape |
  | Expecting `sqlc` to validate against a live database | Confusing errors when `sqlc generate` succeeds/fails independent of whether Postgres is even running | Not realizing `sqlc` only parses migration files as its schema source | Remember `sqlc generate` needs no running database — it's a static analysis over `migrations` + `queries` |

### The Two Key Files — `sqlc.yaml` and Annotated Queries

- **Problem:** [From page] Without a fixed convention for where queries live and how they're annotated, there's no consistent signal telling `sqlc` which SQL statements to turn into code, or what kind of function to generate for each.
- **Mechanism:** [From page] `sqlc.yaml` configures the generator: `engine` (postgresql), `queries` (directory of `.sql` files), `schema` (migration files `sqlc` parses for schema — no running DB needed), and a `gen.go` block (`package`, `out`, `sql_package: "pgx/v5"`, `emit_empty_slices: true` so `:many` queries return `[]T{}` instead of `nil`, `emit_pointers_for_null_types: true` so nullable columns become `*T`). Query files use `-- name: QueryName :command` annotations: `:exec` → function returns only `error` (INSERT/UPDATE/DELETE); `:one` → returns a single struct + `error`; `:many` → returns a slice + `error`.
- **Design Decision:** [From page] The same generate-from-spec pattern already used for OpenAPI (write the spec → `go generate` → type-safe code) is reused here, just with a `.sql` file as the spec instead of a YAML HTTP description. The tip for auto-incremented IDs: combine `RETURNING id` with `:one` to get the generated ID back from an insert; `RETURNING *` gets the full row. The page ties this to why the project uses UUIDs instead of sequential IDs at all — sequential IDs leak how many entities exist (a competitor can estimate order volume from an order ID) and require a single sequence that becomes a bottleneck in distributed systems.
- **In Production:** [AI explanation] `:exec`/`:one`/`:many` map directly onto how the calling code has to handle the result (ignore vs. check-one-exists vs. iterate), so picking the wrong annotation for a query's actual cardinality (e.g. `:one` on a query that can legitimately return zero or many rows) surfaces as a runtime error or silently wrong behavior the first time that edge case occurs, rather than at generation time — generation-time safety only covers schema/type mismatches, not cardinality assumptions.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Using `:one` for a query that can return zero rows | The generated function returns an error (e.g. no-rows) that calling code must explicitly handle as "not found," easy to treat as a generic failure instead | `:one` looks like the natural choice for "get by ID"-style queries | Confirm the query's real cardinality; handle the no-rows case deliberately, don't let it fall through as a generic error |
  | Choosing sequential/auto-increment IDs instead of UUIDs for a public-facing entity like an order | Order volume becomes inferable from consecutive IDs exposed to customers/competitors; a single sequence can bottleneck writes across distributed instances | Auto-increment feels simpler and is the default in many tutorials/ORMs | Use UUIDs (this training uses v7 specifically for insert locality, per the earlier Custom OpenAPI Types guide) for anything that doesn't need strict sequential ordering |

### Type Mapping via Overrides

- **Problem:** [From page] By default `sqlc` generates types from the database driver directly, which would mean writing manual conversion code between HTTP request types and raw driver types (e.g. `pgtype.UUID`) at every boundary — for `common.UUID` and `shared.Address`, that conversion is exactly what the project wants to avoid.
- **Mechanism:** [From page] Two override kinds in `sqlc.yaml`: `db_type` overrides apply globally — `db_type: "uuid"` → `common.UUID` means *every* `uuid` column in *every* table gets this type, appropriate for types that should always map the same way. `column` overrides target one specific column — `column: "orders.customers.address"` → `shared.Address`, because the `address` column is stored as `json`, but not every `json` column in the schema is necessarily an address. `sqlc` does not infer these mappings from existing Go code — every custom type needs an explicit override entry.
- **Design Decision:** [From page] `common.UUID` is deliberately the *same* type already used in the OpenAPI-generated HTTP code, so a UUID flows from the HTTP request straight through to the database insert with no manual mapping step. `shared.Address` is *not* shared with the HTTP-layer type this way — the page reasons this is pragmatic because it's a more complex type, and defers the actual HTTP-layer mapping to "the next exercise." The page frames this as a broader tradeoff: share types across layers for stable, universal concepts (UUID, currency); keep separate models with explicit mapping functions for types that evolve differently per layer (see "When to avoid DRY").
- **In Production:** [AI explanation] Because `common.UUID` and `shared.Address` (per the actual code) already implement `driver.Valuer`/`sql.Scanner` (`Value()`/`Scan()`), `pgx/v5` can read and write them directly as column values — the override in `sqlc.yaml` is what tells the *generator* to use these types in generated signatures; the *runtime* database round-trip then works because the types themselves already know how to serialize/deserialize. Whether `sqlc`'s `pgx/v5` code path relies on these exact interfaces or something more pgx-specific isn't confirmed here — worth checking against `sqlc`'s own docs if a generated function's DB round-trip doesn't behave as expected.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Assuming `sqlc` will "figure out" that a `json` column should map to a specific Go struct because that struct already exists in the codebase | `sqlc generate` emits the default driver type (or errors) instead of the intended custom type — silent divergence from what the code actually expects | `db_type`/`column` overrides look declarative enough to seem automatic, but the page is explicit they must be configured manually every time | Add an explicit `column:` (or `db_type:` if truly universal) override for every custom type, every time a new one is introduced |
  | Using a `db_type` override for a column-specific mapping (e.g. mapping *all* `json` columns to `shared.Address`) | Any future unrelated `json` column silently gets mis-typed as `shared.Address` | `db_type` is the simpler-looking option and easy to reach for by default | Use `column:` for anything that isn't universally true across the schema — `db_type` is only for types like UUID that mean the same thing everywhere |

### Running the Generator & Treating Output as Generated

- **Problem:** [From page] Without a fixed, pinned way to run the generator, different contributors could run different `sqlc` versions and get subtly different generated output, or forget to regenerate after changing a query.
- **Mechanism:** [From page] `backend/orders/adapters/db/sqlc.go` holds `//go:generate go tool sqlc generate`. `go tool` (Go 1.24+ tool dependency management) runs the exact `sqlc` version pinned in `go.mod`, so every contributor and CI run the identical generator. Run it via plain `go generate ./...` or, project-wide, `task gen` (which runs codegen across all modules) — the page says to run `task gen` every time a query is added or changed.
- **Design Decision:** [From page] Generated files must never be hand-edited — they're overwritten on the next generation run. `.gitattributes` already marks `dbmodels/**.go` as generated (confirmed present in the repo), so GitHub collapses them in PR review by default — the same mechanism guide 6 covered for `openapi.gen.go`, now extended to `sqlc`'s output directory.
- **In Production:** [AI explanation] Pinning the tool version via `go.mod` (rather than requiring a separately-installed global binary) removes the same class of risk guide 4 already flagged for `oapi-codegen`: a version mismatch between a contributor's machine and CI could otherwise produce a diff that's just generator-version noise, not a real missed regeneration.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Hand-editing a file under `dbmodels/` to fix something quickly | The fix is silently destroyed the next time anyone runs `task gen` for an unrelated reason | Generated code is just Go code, easy to forget it has a "don't touch" convention | Fix the query `.sql` or `sqlc.yaml` config instead, then regenerate |
  | Changing a query in `queries/*.sql` and forgetting to run `task gen` before committing | Code compiles against stale generated types/functions that no longer match the actual query; a teammate's next `task gen` produces an unexpected diff | Easy to edit the "spec" file and mentally treat the change as already applied | Treat `task gen` as mandatory, same discipline as `go build`, after every query file change |

## 3. Plumbing Dissection

**Wiring map (confirmed directly in the current workspace):**

```
backend/orders/adapters/db/sqlc.yaml                (CONFIRMED, current content)
  engine: postgresql, queries: "queries", schema: "migrations"
  gen.go: package "dbmodels", out "dbmodels", sql_package "pgx/v5"
          emit_empty_slices: true, emit_pointers_for_null_types: true
  overrides (more than the page's simplified example):
    ├─ timestamptz (nullable)     → *time.Time
    ├─ timestamptz (non-nullable) → time.Time
    ├─ uuid (non-nullable)        → eats/backend/common.UUID       [db_type override]
    ├─ uuid (nullable)            → eats/backend/common.NullUUID   [db_type override — type does NOT exist yet, see below]
    └─ orders.customers.address   → eats/backend/common/shared.Address  [column override]

backend/orders/adapters/db/sqlc.go                  (CONFIRMED)
  //go:generate go tool sqlc generate

backend/orders/adapters/db/queries/customers.sql    (CONFIRMED)
  "-- todo: implement"   ← this exercise's deliverable; content not guessed at here

backend/orders/adapters/db/migrations/0001_init_orders.up.sql   (CONFIRMED, unchanged since guide 7)
  orders.customers: customer_uuid UUID NOT NULL PRIMARY KEY, name, email,
                     address json NOT NULL, phone_number    ← no nullable uuid column exists yet

backend/orders/adapters/db/dbmodels/               (CONFIRMED: does not exist)
  ← sqlc generate has not been run against real queries yet

backend/common/uuid.go                              (CONFIRMED)
  type UUID [16]byte
  func (u UUID) Value() (driver.Value, error)   ← driver.Valuer
  func (u *UUID) Scan(src any) error            ← sql.Scanner

backend/common/shared/address.go                    (CONFIRMED)
  type Address struct{ Line1, Line2, PostalCode, City string; CountryCode CountryCode }
  func (a *Address) Scan(src any) error         ← unmarshals JSON
  func (a Address) Value() (driver.Value, error) ← marshals JSON

go.mod                                               (CONFIRMED)
  github.com/sqlc-dev/sqlc v1.30.0  (+ github.com/sqlc-dev/sqlc/cmd/sqlc as a go:tool dependency)

Taskfile.yml                                         (CONFIRMED, unchanged)
  gen: go generate ./...  →  task: fmt

.gitattributes                                       (CONFIRMED)
  dbmodels/**.go linguist-generated=true
```

- **Per plumbing piece:** All of the page's described scaffolding (`sqlc.yaml`, `sqlc.go`, the `overrides` mechanism, the `.gitattributes` entry, the `go tool`-pinned version) is already present and matches the page closely. The one piece left unwritten is the actual content of `queries/customers.sql` — currently a placeholder, which is this exercise's real deliverable.
- **Non-obvious lines:** The `uuid, nullable: true → common.NullUUID` override in `sqlc.yaml` points at a type that doesn't exist anywhere in `backend/common` (confirmed by search) — but it's currently *dormant*, not broken: the only table defined so far (`orders.customers`) has no nullable `uuid` column, so this override entry is never actually exercised by `sqlc generate`. The page's own selling point — schema/query mismatches surface "at generation time, not runtime" — implies this would only become a real error the moment a query or schema introduces a nullable `uuid` column that `sqlc` has to resolve against a still-missing `common.NullUUID`. Worth watching, not yet an active bug.
- **What a senior notices:** The real `sqlc.yaml` has more overrides (`timestamptz` handling) than the page's trimmed-down example — a reminder that the page's snippets are teaching simplifications, and the actual config is worth reading in full rather than assuming it matches the page verbatim (the same posture guide 7 took toward `common/migrations.go`'s extra `defer m.Close()` and graceful-shutdown goroutine beyond what its page described).

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a scratch Go module with a local Postgres, reproduce the core `sqlc` mechanism this page describes end to end:
- Write a `migrations/0001_init.sql` creating one table with at least one `uuid` column, one nullable column, and one `json` column.
- Write `queries/items.sql` with one query of each kind: `:exec` (insert), `:one` (get by id, using `RETURNING *`), `:many` (list).
- Write `sqlc.yaml` wiring `schema`/`queries` directories, `sql_package: "pgx/v5"`, and at least one `db_type` override (map `uuid` to a custom named type, even a trivial `[16]byte` wrapper) and one `column` override (map the `json` column to a custom struct).
- Run `sqlc generate` (via `go tool` or the installed binary) and inspect the generated package.

**Acceptance criteria:**
1. The generated `:one` function's return struct uses your custom UUID type and custom JSON struct — not the driver's raw types — proving the overrides took effect.
2. The generated `:many` function returns `[]T{}` (not `nil`) when the query matches zero rows, if `emit_empty_slices: true` is set — confirm by calling it against an empty table.
3. Deliberately reference a column in a query that doesn't exist in your schema, and confirm `sqlc generate` fails at generation time with no database involved (stop Postgres first, if it's running, to prove this).
4. Hand-edit a generated file, then re-run `sqlc generate` and confirm your edit is silently gone.

<details><summary>Hint</summary>`sqlc`'s override `column` field takes the fully-qualified form `schema.table.column` — match the project's own `orders.customers.address` pattern exactly.</details>
<details><summary>Hint</summary>For criterion 3, `sqlc generate` only needs the `migrations`/`schema` directory on disk — no `DATABASE_URL`, no running container — this is the easiest way to directly verify the page's "no running database needed" claim.</details>
<details><summary>Hint</summary>If a custom type doesn't implement `driver.Valuer`/`sql.Scanner`, the generated code will still compile, but a real insert/select against Postgres will fail at runtime — that failure mode is different from (and later than) a `sqlc generate`-time failure, which is worth feeling firsthand.</details>

**Compare step:** Once `queries/customers.sql` is actually written for this exercise, compare your scratch version against it and ask:
1. Does the real exercise use `RETURNING *` or `RETURNING <specific columns>` for its `:one` insert, and why might that choice differ from what you picked?
2. Does the real `customers` table end up needing a nullable `uuid` column anywhere — and if so, does `common.NullUUID` get created to satisfy the dormant override, or does the override get removed instead?

## 5. Before the Exercise

- **Likely task:** [AI explanation — speculation, not derived from `exercise.md`, which hasn't been read] Replace the `-- todo: implement` placeholder in `backend/orders/adapters/db/queries/customers.sql` with annotated queries against `orders.customers` — at minimum an insert (`:exec` or `:one` with `RETURNING`) and a get-by-UUID (`:one`) — then run `task gen` to produce `dbmodels`.
- **Approach:**
  1. Confirm exactly which queries the exercise wants before writing SQL — don't assume beyond insert/get.
  2. Write each query with the correct `-- name: ... :command` annotation matching its real cardinality.
  3. Leave `sqlc.yaml`'s overrides as-is unless a new nullable `uuid` column is introduced (in which case `common.NullUUID` would need to exist first).
  4. Run `task gen` and verify `dbmodels/` now contains generated Go matching `common.UUID`/`shared.Address` in the right fields, not raw driver types.
  5. Confirm the build compiles and `.gitattributes` correctly collapses the new `dbmodels/**.go` files in a diff view.

## 6. Recall Questions

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

*(These questions, with answers, are also appended to `notes/review_queue.md` under today's date.)*

## 7. Cheat Sheet

- `sqlc` parses `migrations` (no live DB needed) + annotated `queries/*.sql` → generates one Go function + matching struct(s) per query, not one shared model per table.
- Annotation cheatsheet: `-- name: X :exec` → `error` only; `-- name: X :one` → `(T, error)`; `-- name: X :many` → `([]T, error)`. Use `RETURNING id` (or `RETURNING *`) with `:one` to get generated values back from an `INSERT`.
- Two override kinds: `db_type` = global, same DB type always maps the same way (e.g. `uuid` → `common.UUID`); `column` = one exact `schema.table.column`, for types that aren't universal across the schema (e.g. `orders.customers.address` → `shared.Address`). `sqlc` never infers either — every custom type needs an explicit entry.
- `sql_package: "pgx/v5"` + custom types work at runtime because those types implement `driver.Valuer`/`sql.Scanner` themselves (`Value()`/`Scan()`) — the override just tells the generator which type to put in the signature.
- Run generation via `go generate ./...` or `task gen` (project-wide) — never a manually-installed `sqlc` binary; `go tool sqlc generate` always resolves the exact version pinned in `go.mod`.
- Never hand-edit anything under `dbmodels/` — it's regenerated wholesale on every run and marked `linguist-generated=true` in `.gitattributes`, same convention as `openapi.gen.go`.
- Decision rule: reach for a `db_type` override when a database type should *always* map to one Go type everywhere; reach for a `column` override the moment "always" isn't true for that DB type across the schema.
- Workspace gap to watch: `common.NullUUID` doesn't exist yet despite being referenced in `sqlc.yaml` — harmless today (no nullable `uuid` column exists), but would need to be created the moment one does.

## 8. FAQ (Beyond `exercise.md` — AI explanation, not from the page)

**Q: Concretely, what manual conversion would I be writing if the `uuid`/`address` overrides didn't exist?**

Without the `db_type: "uuid"` override, `sqlc` would generate query functions using `pgtype.UUID` (pgx's native struct: `{Bytes [16]byte, Valid bool}`), not `common.UUID` (`[16]byte`). Since the HTTP layer hands you `common.UUID`, every call site would need conversion both ways:

```go
// Without the override — tax paid at every call site
pgID := pgtype.UUID{Bytes: req.ID, Valid: true}   // HTTP type → driver type, going in
err := h.queries.InsertCustomer(ctx, pgID)
row, _ := h.queries.GetCustomer(ctx, pgID)
id := common.UUID(row.ID.Bytes)                   // driver type → HTTP type, coming back
```

The override makes `sqlc` generate functions that take/return `common.UUID` directly. That works at runtime because `common.UUID` already implements `Value()`/`Scan()` (`driver.Valuer`/`sql.Scanner`), which `pgx/v5` calls to serialize/deserialize — so the same value flows HTTP → handler → DB and back with zero glue code.

**Q: Why is `shared.Address` "more complex," and why is it pragmatic to not couple it to the HTTP type the way `common.UUID` is?**

Three differences from UUID:
1. **UUID is a dumb value; Address is a validated domain shape.** `common.UUID` is just 16 bytes with one sensible representation. `Address` is a multi-field struct with its own constructor (`NewAddress`) that enforces invariants (non-empty `Line1`/`PostalCode`/`City`, non-zero `CountryCode`) — that's domain logic, not a transport-neutral value.
2. **Storage representation is column-specific, not type-universal.** The `address` column happens to be stored as `json` (hence a `column` override, not a `db_type` override) — unlike `uuid`, which is a native Postgres type that means the same thing on every table.
3. **HTTP shape and domain/DB shape can legitimately diverge.** If the API's address schema ever needs a field the DB doesn't store (or the reverse), sharing one struct across both layers means an API tweak risks breaking DB code and vice versa. UUID never has this problem — a UUID is a UUID everywhere, so reuse is free. Address isn't "free" the same way, so the page's "share stable/universal types, keep separate models for types that evolve differently per layer" rule points at keeping it decoupled, with explicit mapping deferred to the next exercise.

**Q: `github.com/sqlc-dev/sqlc v1.30.0` is marked `// indirect` in `go.mod` — how can the version still be "pinned" if it's indirect?**

"Indirect" and "pinned" are different axes:
- **Pinned** = the exact version `go.mod`/`go.sum` lock you to.
- **Indirect** = whether any compiled Go source in the module does a real `import "..."` of that package.

`sqlc` is invoked as a CLI binary via the `tool (...)` block (`//go:generate go tool sqlc generate`), not imported by any `.go` file — so `go mod tidy` tags it `// indirect` even though it's explicitly declared as a tool dependency. That tag doesn't weaken the pin: `v1.30.0` is still exact, `go.sum` still has the matching checksum, and `go tool sqlc generate` always resolves to that exact version for every contributor and CI. Before Go 1.24's `tool` directive existed, teams faked a direct import (a throwaway `tools.go` with a blank import) just to force a tool into `go.mod` as "direct" — the `tool` block replaces that hack; the honest `indirect` label is just a side effect of there being no real import, not a sign the version floats.

**Q: Concretely, which two layers would diverge, and what would that look like in code?**

1. **HTTP/API layer** — the `Address` shape in the OpenAPI spec (generates request/response structs in `openapi.gen.go`), the public contract with clients.
2. **DB/persistence layer** — `shared.Address`, marshaled to JSON and stored in `orders.customers.address` via sqlc.

They look identical today (`Line1`, `Line2`, `PostalCode`, `City`, `CountryCode`), but nothing forces that to stay true. Dummy scenario: the frontend adds address autocomplete, so the API needs a raw input string and a provider place ID that have no business being persisted:

```go
// HTTP layer (openapi.gen.go) — what a client sends over the wire
type AddressRequest struct {
    RawInput    string  `json:"raw_input"`         // "221B Baker St, London"
    PlaceID     *string `json:"place_id,omitempty"` // autocomplete provider's record id
    Line1       string  `json:"line_1"`
    Line2       string  `json:"line_2,omitempty"`
    PostalCode  string  `json:"postal_code"`
    City        string  `json:"city"`
    CountryCode string  `json:"country_code"`
}
```

```go
// DB layer (shared.Address) — unchanged, this is what's actually stored
type Address struct {
    Line1       string
    Line2       string
    PostalCode  string
    City        string
    CountryCode CountryCode
}
```

`RawInput`/`PlaceID` are useful at the API boundary but shouldn't end up in the stored JSON column — if `Address` were one shared struct, you'd either bloat storage with unused fields or have to remember to strip them ad hoc at every insert call site. Keeping the two types separate forces an explicit, visible mapping step instead:

```go
func toStoredAddress(req AddressRequest) (shared.Address, error) {
    cc, err := shared.ParseCountryCode(req.CountryCode)
    if err != nil {
        return shared.Address{}, err
    }
    return shared.NewAddress(req.Line1, req.Line2, req.PostalCode, req.City, cc)
    // RawInput, PlaceID intentionally dropped — they don't belong in storage
}
```

That mapper is where "HTTP shape vs. DB shape" decisions live visibly — the API can gain fields like `RawInput`/`PlaceID` without touching `shared.Address`, the migration, or any sqlc query.

**Q: Why are there separate `nullable: true` / non-nullable override entries for the same `db_type` (e.g. `timestamptz`, `uuid`), and is the unused `common.NullUUID` override a forward-looking convention sqlc will "pick up automatically" later?**

sqlc matches an override against a column using two keys together: the column's DB type **and** whether it's nullable (`NOT NULL` or not) — so each `(db_type, nullable)` combination needs its own entry, not one rule per type.

The reason you'd want different Go types per nullability: a plain value type often can't represent "no value" safely.
- `time.Time`'s zero value still looks like a real (if weird) timestamp, so nullable `timestamptz` gets `*time.Time` (nil = NULL) instead of `time.Time`.
- `common.UUID` is `[16]byte`; its zero value (`00000000-...-0000`) is itself a *valid-looking* UUID, so it can't double as "NULL." A nullable `uuid` column instead needs a wrapper with an explicit flag, same pattern as `sql.NullString`/`sql.NullTime`:

```go
// What common.NullUUID would look like, if it existed (illustrative, not in the repo)
type NullUUID struct {
    UUID  UUID
    Valid bool // false = column was NULL
}
```

On the "automatic" question — no, there's no magic pickup. At generation time, `sqlc generate` reads the schema, and for each column looks for an override whose `(db_type, nullable)` matches. Only then does it use that override's `go_type`. Right now no table has a nullable `uuid` column, so the `nullable: true` → `common.NullUUID` entry is never matched against anything — it's a pre-configured convention (maybe copied from a template used elsewhere) sitting dormant, not broken.

That cuts both ways: nobody has to remember to add this override later, **but** the type it references (`common.NullUUID`) doesn't exist in `backend/common` yet. The moment a migration adds a nullable `uuid` column, `sqlc generate` will try to emit code referencing `common.NullUUID` and fail (or the build will fail right after) until someone actually writes that type — a landmine already wired, just not stepped on yet.
