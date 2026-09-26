# Master Study Guide: SQL Migrations, `go:embed`, PostgreSQL Schemas & Migration Rules
> **Platform:** Three Dots Labs Academy — Go Backend Masterclass v0
> **Page:** SQL Migrations — How Migrations Work, `go:embed`, PostgreSQL Schemas, Migration Rules
> **Generated:** 2026-04-26
> **Previous Guide:** `4_study_guide_gitattributes_generated_files.md`

---

## SECTION 1 — PAGE METADATA

### Topics Covered on This Page
1. **SQL Migrations** — what they are, why they exist, how they work as versioned, ordered SQL files that keep every environment's database structure in sync
2. **The `migrate` library** — the Go library used to apply migrations; how it tracks which have already run using a `schema_migrations` table
3. **`go:embed`** — a Go compiler directive that bundles files (like `.sql` migration files) directly into the compiled application binary at build time
4. **The Adapters layer** — the architectural concept of a layer dedicated to code that talks to external systems (databases, APIs, filesystems); where migrations live in the project
5. **PostgreSQL Schemas** — namespaces within a database that isolate each module's tables; why `orders.customers` is different from `restaurants.customers`
6. **Migration Rules** — sequential numbering, "up only" strategy, wrapping in explicit transactions (`BEGIN / COMMIT`), the rule against editing merged migrations
7. **`CREATE SCHEMA IF NOT EXISTS`** — why it appears in TWO places (Go code and migration file) and why both are needed

### Where This Fits in the Learning Sequence
- **What came before:** You built the `RegisterCustomer` HTTP endpoint with a validated request body (using custom types, OpenAPI, and the Enum pattern). The handler receives valid data — but currently throws it away. Nothing is saved.
- **What this page does:** It sets up the database layer — specifically the tool for creating and evolving the database's structure. Before you can store a customer record, you need a `customers` table. Before you can have a table, you need a migration. This page teaches you how migrations work.
- **What this enables next:** With migrations in place, the next step is writing actual database queries (using `sqlc` or a similar library) to INSERT and SELECT customer records.

### Single Learning Objective
> After this page, the learner should be able to **create a numbered SQL migration file, understand how it gets embedded into the Go binary and applied on startup, explain why PostgreSQL schemas provide module isolation, and state the rules that make migrations safe in a team environment — all from first principles.**

### Code Patterns Introduced on This Page

| Pattern | What it does |
|---|---|
| `//go:embed adapters/db/migrations/*.sql` | Bundles all `.sql` files from that path into the compiled binary at build time |
| `var embedMigrations embed.FS` | Declares the variable that holds the embedded files; acts as a read-only filesystem at runtime |
| `common.MigrateDatabaseUp(ctx, moduleName, pool, embedMigrations, "adapters/db/migrations")` | Runs all pending migrations against the database on application startup |
| `BEGIN; ... COMMIT;` in `.sql` files | Wraps migration SQL in an explicit transaction so failures don't leave the schema in a partial state |
| `0001_init_orders.up.sql` filename pattern | Sequential number + descriptive name + `.up.sql` extension — the naming convention for migration files |
| `CREATE SCHEMA IF NOT EXISTS orders;` | Creates the PostgreSQL namespace for the orders module (appears in both Go code and the migration file) |

### Concepts Referenced from Previous Pages
- [Previously learned: `go.embed` directive — the `//go:generate` comment pattern] — from the `.gitattributes` page; `//go:embed` follows the same Go directive comment syntax.
- [Previously learned: Module structure] — from the HTTP handler page; `orders.Module` and its `Init()` method are where `MigrateDatabaseUp` is called.
- [Previously learned: `module.Name()`] — the module name (`"orders"`) is reused as the PostgreSQL schema name.

---

## SECTION 2 — CONTINUITY BRIDGE

### a) The Thread

On the previous pages, you built the entire HTTP side of the `RegisterCustomer` endpoint. A request comes in, gets validated, passes through your handler — and then... nothing. The handler returns a UUID but doesn't save anything. The customer record exists for a fraction of a second and is then forgotten.

This page is the first step in fixing that. To store data, you need a database. To use a database, you need TABLES — structured containers where rows of data live. To create tables in a consistent, teamwork-safe way, you use MIGRATIONS.

Think of it this way: the endpoint is a form at a government office. The previous pages taught you how to design the form and validate what people write on it. This page teaches you how to create the filing cabinet where completed forms are stored. Before you can file anything, the cabinet must exist — and the process of building the cabinet is a migration.

### b) The Shared Axiom

> **The one principle connecting both pages:** Changes that affect shared, persistent systems (the Git repository, the database) must be versioned, ordered, and automated — never manual, never one-off, never dependent on someone remembering.

The previous page versioned generated code changes (`.gitattributes`, CI assertions). This page versions DATABASE STRUCTURE changes (SQL migrations). In both cases, the principle is identical: if the change matters enough to deploy, it must be tracked, reproducible, and verifiable by every environment automatically.

### c) Quick Recall Check

- **You'll need: Module `Init()` method** — reminder: this is the function called once when the application starts. It's where database setup (including running migrations) belongs.
- **You'll need: `//go:generate` directive syntax** — reminder: Go directive comments start with `//go:` and are read by the Go toolchain. `//go:embed` follows the same pattern as `//go:generate` — compiler-read instructions as code comments.
- **You'll need: The project module structure** — reminder: `backend/orders/` is the orders module; it owns its own adapters, API, and now migrations. Each module is self-contained.

---

## SECTION 3 — CORE CONCEPT DEEP-DIVE

---

### Concept 1: SQL Migrations — What They Are and Why They Exist

#### a) The Problem Statement — WHY does this exist?

Imagine your application has been running in production for 6 months with a `customers` table. A developer needs to add a `phone_number` column. They open a SQL editor, type `ALTER TABLE customers ADD COLUMN phone_number TEXT`, press enter — and it works. Done.

Now a second developer on a different computer needs to work on the same feature. They clone the repo and start their database from scratch. They have NO `phone_number` column — because the first developer ran the SQL manually and nobody wrote it down. The second developer's app crashes.

Now imagine production, staging, QA, and 5 developer laptops — each in a different state because different people ran different SQL commands at different times, in different orders, with different typos. Nobody can reproduce a bug reliably because nobody knows exactly what database structure any environment has.

The irreducible need: **a system where every change to the database structure is written down, versioned, ordered, and executed automatically so that every environment's database is always in a known, reproducible state.**

#### b) The Atomic Axioms

1. **Axiom 1:** A database's structure (tables, columns, indexes) is separate from the data it contains. Structure changes (like adding a table) are schema changes. Migrations are the mechanism for applying schema changes.
2. **Axiom 2:** Migrations are applied in ORDER. Migration 0001 always runs before 0002. The database after applying 0001 and 0002 is always the same regardless of when or where the migrations are run.
3. **Axiom 3:** Each migration is applied ONCE per database. The `migrate` library records which migrations have already been applied in a tracking table (`schema_migrations`). It skips migrations that have already run.
4. **Axiom 4:** A migration that has been merged and applied in any environment MUST NOT be changed. If it needs to be undone or modified, a NEW migration is created. This preserves the historical record of every structural change.
5. **Axiom 5:** Migrations run automatically on application startup — no manual SQL scripts, no separate deployment step, no sidecar containers. Every environment (local, CI, production) gets the same migrations applied in the same order automatically.

#### c) The Core Mechanism

Think of a building under construction. A migration is like a work order from the construction company. Work Order #1: "Pour the foundation." Work Order #2: "Build the first floor." Work Order #3: "Add the roof."

The construction company (the `migrate` library) keeps a logbook (the `schema_migrations` table). Every completed work order is recorded: "Order #1 done on Jan 15." When a new building site starts (a new developer's database), the company checks the logbook — the site is empty, so ALL orders must be executed from #1. On the production site that's been running for 6 months, the logbook says orders 1–10 are done — only order 11 (the new one) is executed.

Because orders are numbered (Axiom 2), there's never confusion about sequence. Because each order is only done once (Axiom 3), you never pour the foundation twice. Because you can't erase old orders (Axiom 4), you always know exactly what was done and in what order.

The result: every site (every environment) always ends up with the same structure — even if they were built at different times, because the orders are always applied in the same order.

#### d) Syntax & Code (from the webpage)

**Migration file naming convention:**

```
backend/orders/adapters/db/migrations/
  0001_init_orders.up.sql      ← first migration: sets up initial tables
  0002_add_phone_number.up.sql ← second migration: adds a column (hypothetical)
  0003_add_orders_table.up.sql ← third migration: adds orders table (hypothetical)

# Naming pattern: [4-digit number]_[descriptive_name].up.sql
# The number controls ORDER of execution (Axiom 2)
# The .up means this is an "apply" migration (not a rollback)
```

**Contents of a migration file — with explicit transaction wrapping:**

```sql
-- 0001_init_orders.up.sql
-- Every migration is wrapped in BEGIN/COMMIT (Migration Rule: explicit transactions)

BEGIN;    -- Start the transaction — all changes happen together or not at all

CREATE SCHEMA IF NOT EXISTS orders;    -- Create the PostgreSQL namespace for this module
                                        -- (explained further in Concept 5)

CREATE TABLE IF NOT EXISTS orders.customers (
    id          UUID        PRIMARY KEY,    -- The customer's unique ID
    name        TEXT        NOT NULL,       -- The customer's name — required
    email       TEXT        NOT NULL,       -- The customer's email — required
    created_at  TIMESTAMPTZ DEFAULT NOW()   -- When this record was created
);

COMMIT;   -- End the transaction — commit all the above changes to the database
```

**The `migrate` tracking table (created automatically, you never touch it):**

```sql
-- In schema: orders (the module's schema)
-- Table: schema_migrations (created by the migrate library)
-- This is what tracks which migrations have run:

-- orders.schema_migrations:
-- | version | dirty |
-- | 1       | false |   ← migration 0001 was applied successfully
-- | 2       | false |   ← migration 0002 was applied successfully
-- When the app starts again, migrate reads this and skips 0001 and 0002 (Axiom 3)
```

#### e) Execution / Internal Walkthrough

The application starts for the first time on a fresh database:

```
Step 1: Module.Init() is called during application startup
Step 2: common.MigrateDatabaseUp(...) is called
Step 3: migrate library checks: does the orders.schema_migrations table exist?
        → NO (fresh database) → creates it
Step 4: migrate library reads the list of available migrations from embedMigrations:
        → [0001_init_orders.up.sql]
Step 5: migrate library reads schema_migrations:
        → Table is empty — no migrations have been applied yet
Step 6: migrate applies 0001_init_orders.up.sql:
        → Executes the SQL: BEGIN; CREATE SCHEMA... CREATE TABLE... COMMIT;
        → Records in schema_migrations: version=1, dirty=false
Step 7: No more migration files → migration complete
Step 8: Application startup continues — database is ready

The application starts AGAIN (second run, same database):
Step 3: schema_migrations exists
Step 4: Available: [0001_init_orders.up.sql]
Step 5: schema_migrations shows: version=1 already applied
Step 6: No new migrations to apply → migrate logs: "no change"
Step 7: Application startup continues immediately (Axiom 3 — skip applied migrations)

A developer adds 0002_add_phone_number.up.sql and restarts:
Step 4: Available: [0001_init_orders.up.sql, 0002_add_phone_number.up.sql]
Step 5: schema_migrations shows: version=1 applied
Step 6: Only 0002 is new → apply it (Axiom 2 — in order, Axiom 3 — skip applied)
Step 7: Records version=2
```

**Where each axiom becomes visible:**
- Axiom 1: Step 6 — the SQL in the migration file makes structural changes (CREATE TABLE).
- Axiom 2: Steps 4, 6 (second run) — `0001` always runs before `0002`; numbering enforces order.
- Axiom 3: Steps 5–6 (second run) — migration 0001 is already in `schema_migrations`; it's skipped.
- Axiom 4: Implicit — if someone edits `0001_init_orders.up.sql` after it's been applied, the applied database state no longer matches the file. Migrate will detect a checksum mismatch and refuse to run.
- Axiom 5: Step 1 — `Init()` runs automatically on startup, no manual SQL needed.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| Migrations run every time the application starts | **FALSE** | Axiom 3 — each migration runs ONCE per database. The `schema_migrations` table tracks applied migrations. Already-applied ones are skipped. |
| You can edit a migration file after it's been committed and applied | **FALSE** | Axiom 4 — the `migrate` library records a checksum of each applied migration. Editing the file changes the checksum → migrate detects the mismatch → refuses to proceed. Fix forward with a new migration. |
| Migrations are only for adding tables | **FALSE** | Axiom 1 — any schema change (add table, add column, rename column, add index, drop column) is a migration. |
| Down migrations (rollbacks) are required | **FALSE** | The page explicitly recommends UP migrations only. Down migrations are complex in production. "Fix forward" with a new migration is safer. |
| Migrations run even if there's nothing new | **FALSE** | Axiom 3 — if all available migrations are already in `schema_migrations`, `migrate` returns `ErrNoChange` and the app starts normally. |
| Two developers can add migration 0002 independently without conflict | **FALSE** | Sequential numbering means both would name their file `0002_...`. Git will flag this as a conflict (two new files with the same name is a real conflict). One developer must rename to `0003_...` after the other merges. This is the known trade-off of sequential numbers. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Editing a migration file after it's been applied | `migrate` checksum error — app refuses to start | Axiom 4 violated — the database has state that was produced by the OLD file; the new file disagrees | Never edit committed migrations. Create a new migration file with the corrective change. |
| Forgetting `BEGIN; ... COMMIT;` in a multi-statement migration | Partial migration on failure — database in broken state | No transaction means each statement commits independently. A failure midway leaves some statements applied and others not. | Always wrap migration SQL in `BEGIN; ... COMMIT;`. Even for single statements — it's a safe default. |
| Using timestamp-based numbering when parallel PRs are common | Schema drift in production — hard to debug | Two PRs merged in different orders produce different migration sequences in different environments | Use sequential numbers for predictable, deterministic ordering (the page's recommendation). |
| Running SQL directly against the database instead of creating a migration | Environment drift — your database is different from everyone else's | The change exists only on one machine, not tracked in code | ALWAYS create a migration file for schema changes. Never run schema SQL directly. |
| Creating a new migration file with a skipped number (e.g., 0003 when 0002 exists) | migrate applies migrations in order — a gap could indicate a missing file | Gap in numbering looks like a missing migration | Number sequentially without gaps. If you need to "skip" a migration, create it as a no-op (just a comment). |

#### h) Comprehension Task

> **Comprehension Task:** On paper, trace through what happens when the application starts for the THIRD time on a database that already has migrations 0001 and 0002 applied, and now you've added migration 0003. Write out: (1) what `schema_migrations` contains at the start, (2) which migrations the `migrate` library looks at, (3) which migrations it applies and why, (4) what `schema_migrations` contains at the end.
>
> **What to check:** (1) `schema_migrations`: versions 1 and 2, both `dirty=false`. (2) `migrate` reads all three files: 0001, 0002, 0003. (3) It applies ONLY 0003 — because 1 and 2 are already recorded (Axiom 3). It applies 0003 in order (Axiom 2). (4) `schema_migrations`: versions 1, 2, and 3, all `dirty=false`.
>
> **Common wrong answer:** "It applies all three migrations again." — Wrong. Axiom 3: each migration runs ONCE per database. The `schema_migrations` table is the record that prevents re-running already-applied migrations.

---

### Concept 2: `go:embed` — Bundling Files Into the Binary

#### a) The Problem Statement — WHY does this exist?

Your application is compiled into a single binary file — say, `eats-backend`. You deploy this file to a server. But your migration `.sql` files are separate files in a directory on your DEVELOPMENT MACHINE. When the binary runs on the server, those `.sql` files don't exist there — the binary would try to read files that aren't present, and migration would fail.

Without `go:embed`, you'd have to deploy BOTH the binary AND the migrations directory, keep them in the same relative path, write deployment scripts that copy files correctly, and hope nobody deletes the migrations folder. Every container image, every deployment package, every developer machine would need the SQL files in just the right place.

The irreducible need: **a way to bake auxiliary files (like SQL migrations) directly into the compiled binary so the binary is completely self-contained — deployable as a single file with no external dependencies.**

#### b) The Atomic Axioms

1. **Axiom 1:** The Go compiler can, at build time, read files from the local filesystem and embed their contents directly into the compiled binary. This happens during compilation — not at runtime.
2. **Axiom 2:** The `//go:embed` directive is a comment that the Go compiler reads BEFORE compilation begins. The pattern after `//go:embed` specifies which files to embed.
3. **Axiom 3:** The embedded files are accessed through a variable of type `embed.FS` (a read-only filesystem). At runtime, reading from `embed.FS` reads the bytes that were baked into the binary — not from disk.
4. **Axiom 4:** `embed.FS` is read-only. You cannot create, modify, or delete files in it. It is a snapshot of the files as they existed at compile time.
5. **Axiom 5:** The glob pattern in `//go:embed` must be relative to the directory containing the `.go` file with the directive. `adapters/db/migrations/*.sql` means: "all `.sql` files inside the `adapters/db/migrations/` subdirectory relative to this file's location."

#### c) The Core Mechanism

Think of `go:embed` like a publisher who prints a textbook. While printing, they include an appendix: "Appendix A: SQL Scripts" — and they TYPESET the actual SQL text directly into the book's pages. When a student buys the book, the appendix is IN the book — they don't need a separate booklet called "Appendix A" shipped separately. The book is self-contained.

`go:embed` is the instruction to the printer (Go compiler): "When you print (compile) this Go program, typeset (embed) the contents of these files directly into the binary." When the program runs, the `embed.FS` variable is like the appendix — the data is already there, in the binary, without needing to read any files from disk (Axiom 3).

Because this happens at COMPILE TIME (Axiom 1), there is nothing on the server or in any deployment that needs to know where the SQL files live. The binary is the only thing that matters.

#### d) Syntax & Code (from the webpage)

**In `backend/orders/module.go`:**

```go
package orders

import (
    "embed"   // Required import — the embed package enables embed.FS type
)

// The directive and variable MUST be adjacent — no blank line between them.
// The Go compiler reads the directive comment and associates it with the next var.

//go:embed adapters/db/migrations/*.sql
//  ↑ Go compiler directive (Axiom 2) — not a regular comment
//         ↑ path relative to THIS file's location (Axiom 5)
//                              ↑ glob pattern: all .sql files in that directory
var embedMigrations embed.FS
//  ↑ The variable that holds the embedded files (Axiom 3)
//                   ↑ embed.FS = read-only embedded filesystem (Axiom 4)
```

**Internally, what happens to the binary:**

```
BEFORE go:embed (binary is just compiled Go code):
  eats-backend [binary]
    → handler.go code
    → module.go code
    → common/ code
    (NO SQL files)

AFTER go:embed (SQL files baked in at compile time — Axiom 1):
  eats-backend [binary]
    → handler.go code
    → module.go code
    → common/ code
    → embedMigrations (embedded filesystem):
        → adapters/db/migrations/0001_init_orders.up.sql: "BEGIN; CREATE TABLE..."
        → adapters/db/migrations/0002_add_phone.up.sql:   "BEGIN; ALTER TABLE..."
```

**How `embedMigrations` is passed to `MigrateDatabaseUp`:**

```go
// In module.go Init():
if err := common.MigrateDatabaseUp(
    ctx,
    string(m.Name()),  // "orders" — used as PostgreSQL schema name
    m.pgxDb,           // The database connection pool
    embedMigrations,   // The embedded .sql files (Axiom 3 — in-binary filesystem)
    "adapters/db/migrations",  // The path prefix within the embedded FS (Axiom 5)
); err != nil {
    return err
}
```

**In `common/migrations.go` — how the embedded FS is used:**

```go
// The iofs.New call creates a migration source from the embedded filesystem:
d, err := iofs.New(fs, migrationsDir)
// fs = embedMigrations (the embed.FS)
// migrationsDir = "adapters/db/migrations"
// Result: migrate can read migration files from the binary's embedded data
```

#### e) Execution / Internal Walkthrough

At compile time (when you run `go build ./...` or `task`):

```
Step 1: Go compiler reads module.go
Step 2: It finds the directive: //go:embed adapters/db/migrations/*.sql
Step 3: Compiler looks in the filesystem at: [location of module.go]/adapters/db/migrations/
Step 4: Glob *.sql matches: [0001_init_orders.up.sql]
Step 5: Compiler reads the file contents: "BEGIN;\nCREATE SCHEMA..."
Step 6: Compiler bakes these bytes directly into the binary as the value of embedMigrations
Step 7: Binary is produced — it contains the SQL text internally (Axiom 1)

At runtime (when the binary is executed on ANY machine):
Step 8: embedMigrations is available as an embed.FS variable
Step 9: iofs.New(embedMigrations, "adapters/db/migrations") creates a reader
Step 10: The reader reads bytes from embedMigrations — FROM THE BINARY (Axiom 3)
Step 11: No filesystem access — the SQL content came from inside the binary itself
Step 12: The migrate library uses these bytes as the migration SQL
```

**Adding a new migration file:**

```
Step 1: You create: adapters/db/migrations/0002_add_customers_index.up.sql
Step 2: The embed glob *.sql matches it automatically (Axiom 5)
Step 3: Next time you run go build, the new file is also baked in
Step 4: No .gitattributes, no CI changes, no extra configuration needed
```

**Where each axiom becomes visible:**
- Axiom 1: Step 6 — the file content is baked in DURING COMPILATION.
- Axiom 2: Step 2 — the `//go:embed` comment is read by the compiler, not at runtime.
- Axiom 3: Step 10 — at runtime, `embed.FS` reads bytes from the binary, not from disk.
- Axiom 4: You can only read from `embedMigrations`, never write to it or add files at runtime.
- Axiom 5: Step 3 — the path is relative to `module.go`'s location.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| `go:embed` copies the files to a special folder next to the binary | **FALSE** | Axiom 1 — the files are embedded INSIDE the binary. No separate files exist. The binary is completely self-contained. |
| You can read and write to `embed.FS` at runtime | **FALSE** | Axiom 4 — `embed.FS` is read-only. It's a snapshot from compile time. To add a file, recompile. |
| You need to update something when adding a new `.sql` file | **FALSE** | Axiom 5 — the `*.sql` glob matches ALL `.sql` files in the directory. Adding a new file is automatic. |
| `//go:embed` works for any file type | **TRUE** | You can embed `.html`, `.json`, `.png`, `.sql`, or any file type. The content is stored as raw bytes. |
| There must be a blank line between `//go:embed` and `var embedMigrations embed.FS` | **FALSE** | The directive and variable must be ADJACENT (no blank line). A blank line breaks the association — the compiler won't apply the directive to the variable. |
| `embed.FS` acts like a real OS filesystem | **PARTLY** | It implements the `fs.FS` interface (the standard Go filesystem abstraction), so any library that accepts `fs.FS` works with it. But it has no write operations (Axiom 4). |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Blank line between `//go:embed` and `var embedMigrations embed.FS` | Compile error: `go:embed cannot apply to var` | Axiom 2 — the directive must be immediately before the variable declaration | Remove the blank line — directive and var must be adjacent |
| Using an absolute path in `//go:embed` | Compile error: path not found | Axiom 5 — paths are relative to the file containing the directive | Use relative paths only: `adapters/db/migrations/*.sql` |
| Forgetting to import `"embed"` | Compile error: `embed` not imported | The `embed.FS` type requires the `embed` package | Add `import "embed"` even if you don't use any function from it (the import is needed for the type) |
| Trying to write to `embedMigrations` at runtime | Compile error — no Write method exists | Axiom 4 — `embed.FS` is read-only | Use a real filesystem (`os.DirFS`) if you need write access |
| Putting the `//go:embed` directive in a test file to embed test fixtures | This works but has quirks | Test files (`_test.go`) can use `go:embed`, but the path is still relative to the test file's location | Works correctly as long as you understand the path is relative to the test file |

#### h) Comprehension Task

> **Comprehension Task:** You deploy the `eats-backend` binary to a brand new server. The server has NO `/adapters/db/migrations/` directory anywhere on it — just the single binary file. The application starts and migrations run successfully. On paper, explain: (1) where the SQL was read from during migration, (2) which step in the build process put it there, (3) what `embed.FS` returns when migration code calls `iofs.New(embedMigrations, "adapters/db/migrations")`.
>
> **What to check:** (1) The SQL was read from INSIDE the binary — from the bytes baked in during compilation (Axiom 3). (2) The `go build` step — when the compiler processed `//go:embed adapters/db/migrations/*.sql`, it embedded the file contents (Axiom 1). (3) `iofs.New` creates a reader that reads from those embedded bytes — it acts like a filesystem where `"adapters/db/migrations/0001_init_orders.up.sql"` is a readable file, but the data comes from inside the binary.
>
> **Common wrong answer:** "The application must have copied the SQL files somewhere during startup." — Wrong. `embed.FS` is a READ-ONLY snapshot from compile time (Axiom 4). Nothing is copied at runtime. The data was baked in at build time (Axiom 1).

---

### Concept 3: The Adapters Layer

#### a) The Problem Statement — WHY does this exist?

Your business logic — "create a customer with this name and email" — should not care whether that customer is stored in PostgreSQL, MySQL, a JSON file on disk, or an in-memory map. Business logic should be about WHAT to do, not WHERE to put it.

Without an Adapters layer, your handler code might call `db.Exec("INSERT INTO customers...")` directly. Now your business logic is entangled with PostgreSQL. If you ever want to switch databases, run tests without a real database, or add a second storage system, you must rewrite the handler code — which is supposed to contain business logic, not database driver calls.

The irreducible need: **a dedicated layer that contains all code responsible for translating between your application's domain language and external systems (databases, HTTP APIs, filesystems) — so the rest of the application is insulated from those external details.**

#### b) The Atomic Axioms

1. **Axiom 1:** External systems (databases, third-party APIs, filesystems) have their own languages, protocols, and quirks. Code that speaks those languages is inherently different from code that implements business rules.
2. **Axiom 2:** Mixing "talk to the database" code with "apply business logic" code makes both harder to change — changing the database requires changing the business logic code, and vice versa.
3. **Axiom 3:** The Adapters layer is a physical separation — a distinct directory (`adapters/`) — that enforces the conceptual separation. Code in `adapters/` is the only code allowed to talk directly to PostgreSQL, Redis, external HTTP APIs, etc.
4. **Axiom 4:** Each module owns its own adapters. `orders/adapters/db/` is for orders-related database code only. `restaurants/adapters/db/` would be for restaurants-related database code. No sharing of adapter code between modules.
5. **Axiom 5:** Migrations are part of the database adapter — they define the database structure that the adapter's queries rely on. Migrations live IN `adapters/db/migrations/` because they are a database concern, not a business logic concern.

#### c) The Core Mechanism

Think of a large hotel. The guests (business logic) speak English — they ask for towels, room service, a wake-up call. The hotel has staff who speak the guests' language AND the "back-of-house" languages: kitchen jargon, laundry procedures, maintenance terminology. The front-desk staff (adapters) translate between guests and back-of-house.

The guests don't know or care whether laundry uses a conveyor belt or hand-washing. They just ask for clean towels. The front-desk staff know the laundry system — that's their job. If the hotel switches from a conveyor belt to hand-washing, only the front-desk-to-laundry translation changes — guests don't notice.

In code: the business logic (handler) asks for "save this customer." The adapter translates that into `INSERT INTO orders.customers (...)`. If the database changes from PostgreSQL to DynamoDB, only the adapter changes — the handler is untouched.

#### d) Syntax & Code (from the webpage)

**The directory structure:**

```
backend/
  orders/
    adapters/               ← the Adapters layer for the orders module
      db/                   ← database adapter (PostgreSQL)
        migrations/         ← database structure changes (schema evolution)
          0001_init_orders.up.sql
    api/                    ← HTTP adapter (request/response handling)
      http/
        handler.go
        openapi.gen.go
    module.go               ← wires everything together
```

**The architectural boundary (conceptual — no new syntax):**

```
┌─────────────────────────────────────────────────────┐
│                  orders module                       │
│                                                     │
│  api/http/handler.go  ←  business logic layer       │
│        │                 (what to do)               │
│        ↓                                            │
│  adapters/db/         ←  database adapter layer     │
│        │                 (how to store it)          │
│        ↓                                            │
│  [PostgreSQL database] ←  external system           │
│                           (the actual database)     │
└─────────────────────────────────────────────────────┘
```

#### e) Execution / Internal Walkthrough

```
Request: POST /orders/register-customer {"name": "Alice", ...}

Step 1: HTTP adapter (api/http/handler.go) receives the request
Step 2: Handler validates request, creates common.UUID via common.NewUUIDv7()
Step 3: Handler calls... [not yet implemented — next pages will add this]
        → This would be: ordersAdapter.SaveCustomer(ctx, customer)
Step 4: Database adapter (adapters/db/) translates to SQL:
        INSERT INTO orders.customers (id, name, email) VALUES ($1, $2, $3)
Step 5: PostgreSQL executes the INSERT
Step 6: Adapter returns success to handler
Step 7: Handler returns RegisterCustomer201JSONResponse to the HTTP adapter
Step 8: HTTP adapter sends response to the client

The business logic (Step 2–3) never touches SQL.
The adapter (Step 4) never knows the full business context.
They communicate through a well-defined interface. (Axiom 2 — separation)
```

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| The handler should contain the SQL INSERT statement | **FALSE** | Axiom 2 — mixing business logic with database code. SQL belongs in `adapters/db/`, not `api/http/handler.go`. |
| Adapters are only for databases | **FALSE** | Axiom 1 — adapters translate between your application and ANY external system: PostgreSQL, Redis, external HTTP APIs, message queues, email providers. |
| Multiple modules can share the same database adapter | **FALSE** | Axiom 4 — each module owns its own adapters. Sharing adapters creates coupling between modules. |
| The adapters layer is complex and should be avoided for simple projects | **OPINION** | For small, single-purpose projects, the separation may not be worth the overhead. But for a modular system that will grow, the separation pays off over time. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Writing SQL directly in handler.go | Architecture violation — coupling business logic to database | Axiom 2 violated — handler now depends on PostgreSQL directly | Move all SQL to adapter files in `adapters/db/` |
| Creating a single shared `adapters/db/` at the project level that all modules use | Architecture violation — cross-module coupling | Axiom 4 violated — a change to the shared adapter affects all modules | Each module has its own adapter; use shared infrastructure (like `common/`) only for truly cross-cutting concerns |

#### h) Comprehension Task

> **Comprehension Task:** Someone argues: "The adapters layer is unnecessary complexity. Just put the SQL directly in the handler — it's simpler." Write a response using only the axioms from this concept. Your response must answer: what specific problem does separating the adapter layer prevent? What would the developer have to change if they later needed to switch from PostgreSQL to a different database?
>
> **What to check:** Good answer: Axiom 2 — mixing SQL in the handler couples business logic to PostgreSQL. If you switch databases, you must rewrite the handler — which means changing business logic code just because the storage system changed. With the adapter layer, only the adapter changes; the handler is untouched. The business logic (what to do) is independent of the storage mechanism (where to put it).
>
> **Common wrong answer:** "It's just best practice." — This is not a first-principles answer. The first-principles reason is Axiom 2: mixing the two types of code causes them to change together even when only one needs to change.

---

### Concept 4: PostgreSQL Schemas — Module Namespaces

#### a) The Problem Statement — WHY does this exist?

As the project grows, you'll have an `orders` module, a `restaurants` module, a `couriers` module, a `payments` module. Each module needs a `customers` table (or something similar). In a single flat database, two tables can't have the same name: you can't have both `orders.customers` and `restaurants.customers` — you'd have to name them `orders_customers` and `restaurants_customers` and invent a naming convention and remember it everywhere.

Worse, if different teams work on different modules, they might accidentally create tables with the same name. The last team to run their migration overwrites the first team's table. Data loss, chaos, late nights.

The irreducible need: **a way to give each module its own isolated namespace within the same database, so table names don't collide, ownership is clear, and each module's data is separate by default.**

#### b) The Atomic Axioms

1. **Axiom 1:** A PostgreSQL schema is a namespace — a container within a database. Tables, sequences, and indexes inside a schema are referenced as `schema_name.table_name` (e.g., `orders.customers`).
2. **Axiom 2:** The same table name can exist in multiple schemas without conflict: `orders.customers` and `restaurants.customers` are completely separate tables.
3. **Axiom 3:** Each module in this project gets its own schema named after the module. `orders` module → `orders` schema. `restaurants` module → `restaurants` schema.
4. **Axiom 4:** `CREATE SCHEMA IF NOT EXISTS orders;` appears in TWO places for two different reasons: in `MigrateDatabaseUp()` (so the migration tracking table can be created in the right schema before any migrations run) AND in the migration file itself (so `sqlc` — the query library — can find the schema definition when generating query code).
5. **Axiom 5:** You CAN query across schemas with explicit schema prefixes: `SELECT o.id, r.name FROM orders.customers o JOIN restaurants.menu r ON ...`. Isolation is the DEFAULT; cross-schema access is opt-in.

#### c) The Core Mechanism

Think of a large company headquarters with multiple departments: Sales, Engineering, HR, Finance. Each department has its own filing system — its own folders, its own naming conventions. The Sales folder labeled "Q3 Report" is different from the Finance folder labeled "Q3 Report." The labels can be the same because the DEPARTMENTS (namespaces) are different.

A PostgreSQL schema is like a department's filing cabinet. `orders.customers` is the "customers" file in the "orders department cabinet." `restaurants.customers` is the "customers" file in the "restaurants department cabinet." Same name, completely different file, never confused.

The `orders` module creates its schema (`CREATE SCHEMA IF NOT EXISTS orders`) — it's claiming its own cabinet. Every table it creates inside that schema is its own, under its own department prefix. No other module touches the `orders` schema.

Because the schema is created in TWO places (Axiom 4): once in Go code (before migrations run, so the `schema_migrations` tracking table can be placed in the right schema) and once in the migration file (so query generation tools like `sqlc` can see the schema definition). Both are idempotent (`IF NOT EXISTS`) — running both never causes a conflict.

#### d) Syntax & Code (from the webpage)

**In `common/migrations.go` — creating the schema before migrations run:**

```go
// This runs BEFORE any migration files are applied:
if _, err := db.ExecContext(
    ctx,
    "CREATE SCHEMA IF NOT EXISTS "+string(moduleName),
    // moduleName = "orders" → executes: CREATE SCHEMA IF NOT EXISTS orders
); err != nil {
    return fmt.Errorf("could not create schema %s: %w", moduleName, err)
}
// WHY here? The migrate library needs to create schema_migrations TABLE in this schema.
// The schema must exist before migrate can create that table. (Axiom 4)

// Then migrate is configured to use this schema:
migDb, err := pgxMigrate.WithInstance(db, &pgxMigrate.Config{
    SchemaName:      string(moduleName),     // "orders" schema
    MigrationsTable: "schema_migrations",    // → orders.schema_migrations
})
```

**In the migration file — creating the schema as part of the migration:**

```sql
-- 0001_init_orders.up.sql
BEGIN;

CREATE SCHEMA IF NOT EXISTS orders;
-- WHY here? sqlc reads migration files to understand the database schema.
-- If the schema isn't defined in migration files, sqlc doesn't know it exists.
-- (Axiom 4 — the migration file is for sqlc's benefit)

CREATE TABLE orders.customers (
    id          UUID        PRIMARY KEY,
    name        TEXT        NOT NULL,
    email       TEXT        NOT NULL
);
-- Note: explicitly qualified as orders.customers (Axiom 1 — schema.table notation)

COMMIT;
```

**How table names look across modules:**

```sql
-- orders module tables (in orders schema):
orders.customers
orders.orders
orders.order_items

-- restaurants module tables (in restaurants schema — hypothetical):
restaurants.customers     ← SAME name as orders.customers, but different table (Axiom 2)
restaurants.menus
restaurants.menu_items

-- Cross-schema query (Axiom 5 — opt-in cross-schema access):
SELECT oc.name, rm.item_name
FROM orders.customers oc
JOIN restaurants.menus rm ON oc.id = rm.customer_id
```

#### e) Execution / Internal Walkthrough

`common.MigrateDatabaseUp` is called with `moduleName = "orders"`:

```
Step 1: Execute: CREATE SCHEMA IF NOT EXISTS orders
        → PostgreSQL creates the orders namespace (if it doesn't exist yet)
        → If it already exists (app restarted), IF NOT EXISTS makes this a no-op
Step 2: Configure migrate to use schema "orders" and table "schema_migrations"
        → migrate will place its tracking table at orders.schema_migrations
Step 3: migrate checks orders.schema_migrations for applied migrations
Step 4: migrate reads migration file: 0001_init_orders.up.sql
Step 5: Migration executes:
        BEGIN;
        CREATE SCHEMA IF NOT EXISTS orders;   ← no-op (already created in Step 1)
        CREATE TABLE orders.customers (...);
        COMMIT;
Step 6: migrate records version=1 in orders.schema_migrations
Step 7: Done

Result in PostgreSQL:
  Schema: orders
    Table: customers        (orders.customers)
    Table: schema_migrations (orders.schema_migrations — migrate's tracking table)
```

**Where each axiom becomes visible:**
- Axiom 1: Step 5 — `orders.customers` uses `schema.table` notation.
- Axiom 2: If a `restaurants` module ran similarly, `restaurants.customers` would be a completely separate table.
- Axiom 3: The schema name comes from `m.Name()` — the module name IS the schema name.
- Axiom 4: Step 1 (Go code) AND Step 5 (migration file) both run `CREATE SCHEMA IF NOT EXISTS orders` — both are idempotent.
- Axiom 5: Implicit — cross-schema queries are possible but not used here.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| All modules share one global schema (no prefixes) | **FALSE** | Axiom 3 — each module gets its own schema. `orders.customers` is NOT in the default `public` schema. |
| `CREATE SCHEMA IF NOT EXISTS` in the migration file is a duplicate and should be removed | **FALSE** | Axiom 4 — it's there for `sqlc`. The Go code creates the schema for `migrate`. Both have different consumers and both are needed. |
| You can have `orders.customers` and `restaurants.customers` in the same database | **TRUE** | Axiom 2 — schemas are namespaces; same table name, different schema, no conflict. |
| PostgreSQL schemas are the same as databases | **FALSE** | A database can contain MANY schemas. A schema is a NAMESPACE within a database — not a separate database. You can't query across databases, but you can query across schemas within the same database (Axiom 5). |
| The orders module can access `restaurants.customers` directly | **TRUE technically, but wrong architecturally** | Axiom 5 — cross-schema access is possible. But in the modular architecture, each module should only access its own schema. Cross-module data sharing goes through module contracts/interfaces, not direct SQL joins. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Creating tables without a schema prefix (e.g., `CREATE TABLE customers`) | Tables land in the default `public` schema — not in the module's schema | Forgetting to prefix with `orders.` in the SQL | Always use `schema_name.table_name` notation: `orders.customers` |
| Using the same schema name for two different modules | PostgreSQL error or data corruption — tables collide | Forgetting that schema = module name | Each module uses a unique schema name (its module name) |
| Removing `CREATE SCHEMA IF NOT EXISTS` from the migration file | `sqlc` can't find the schema definition — query generation fails | Misunderstanding Axiom 4 — the migration version exists for `sqlc`'s benefit | Keep `CREATE SCHEMA IF NOT EXISTS` in BOTH the Go code and the migration file |

#### h) Comprehension Task

> **Comprehension Task:** You're adding a new `restaurants` module to the project. On paper, answer: (1) What PostgreSQL schema will it use? (2) Where does `CREATE SCHEMA IF NOT EXISTS restaurants` appear, and how many times? (3) If both modules have a `customers` table, what are their full names? (4) The `orders` module calls `MigrateDatabaseUp` with `moduleName = "orders"`. The `restaurants` module will call it with what `moduleName`?
>
> **What to check:** (1) `restaurants` schema. (2) TWICE — once in `common.MigrateDatabaseUp()` (in Go code) and once in the first migration file for restaurants (e.g., `0001_init_restaurants.up.sql`). (3) `orders.customers` and `restaurants.customers` — different tables, no collision (Axiom 2). (4) `moduleName = "restaurants"` — the module's `Name()` method returns `"restaurants"`, and the schema is named after the module (Axiom 3).
>
> **Common wrong answer:** "The restaurants module also uses the `orders` schema." — Wrong. Axiom 3 and Axiom 4 — each module gets its own schema, named after itself. The `orders` schema belongs to the orders module exclusively.

---

### Concept 5: Migration Rules

#### a) The Problem Statement — WHY do these rules exist?

Migrations are permanent. When migration 0001 runs in production against real data, it changes the database forever. If it's wrong, it might corrupt data. If it runs partially (half the statements succeeded), the database is in an inconsistent state. If two people add migration files with the same number, the production database gets one version but another developer's local environment gets a different version.

Unlike application code where you can "just redeploy," a botched migration can require emergency database surgery at 2 AM while customers can't check out.

The irreducible need: **a set of non-negotiable rules that make migrations safe in production — correct ordering, atomic execution, and an explicit "no rollbacks" policy that encourages careful forward progress.**

#### b) The Atomic Axioms

1. **Axiom 1:** Sequential numbers (0001, 0002, 0003) guarantee a deterministic, globally agreed-upon execution order. Any machine running any version of the codebase will always apply migrations in the same order.
2. **Axiom 2:** A migration that has been applied to ANY environment (even one developer's local database) must be treated as immutable. Changing it after application creates a split-brain: some environments ran the old version, others run the new version — and there's no automated way to know which is which.
3. **Axiom 3:** PostgreSQL supports transactional DDL (Data Definition Language — structural changes like `CREATE TABLE`). Wrapping a migration in `BEGIN; ... COMMIT;` means ALL statements succeed together or ALL fail together. A failed migration with a transaction leaves the database UNCHANGED. Without a transaction, a partial failure leaves some statements applied and others not.
4. **Axiom 4:** Down migrations (rollbacks) in production are dangerous because: data may have been written under the new schema that is incompatible with the old schema; the rollback migration itself may have bugs; coordination with application deployment is complex. "Fix forward" (a new migration that undoes the change) is safer.
5. **Axiom 5:** IF NOT EXISTS in structural SQL (`CREATE TABLE IF NOT EXISTS`, `CREATE SCHEMA IF NOT EXISTS`) makes migrations idempotent — running them a second time has no effect. This protects against edge cases where a migration's status is uncertain.

#### c) The Core Mechanism

Think of migration rules like surgery protocols. Surgeons have strict protocols not because they enjoy bureaucracy, but because the cost of getting it wrong is catastrophic and irreversible. "Always use sequential numbering" is like "always label instruments before an operation." "Never edit a merged migration" is like "never modify the operating plan mid-surgery without a formal change order." "Wrap in a transaction" is like "have a crash cart ready."

The rules exist not to slow developers down but to protect them (and their data) from the consequences of mistakes that would be unrecoverable in production.

**Sequential numbering (Axiom 1):** Two developers can't be confused about order. `0003` ALWAYS runs after `0002` — on every machine, in every environment, for the lifetime of the project.

**Immutability (Axiom 2):** Once a migration runs, it's in the logbook. If you change the file, the `migrate` library detects the checksum mismatch and refuses to run — protecting you from a situation where different environments have different schemas from "the same migration."

**Transactions (Axiom 3):** If your migration file has 5 statements and statement 4 fails, the transaction rolls back ALL 5 — the database is as if the migration never ran. You can fix the migration, try again from a clean state.

#### d) Syntax & Code (from the webpage)

**Rule 1 — Sequential numbering:**

```
CORRECT:
  0001_init_orders.up.sql
  0002_add_customers_index.up.sql
  0003_add_delivery_address.up.sql

WRONG (timestamp — unpredictable ordering when two PRs land):
  20260218120000_init_orders.up.sql
  20260219093000_add_index.up.sql
  20260219094500_add_address.up.sql
  ↑ If two PRs add migrations at the same time, order depends on timestamp precision.
    Two developers both working at 9:45 AM on the same day would conflict.
```

**Rule 2 — Never edit a merged migration (instead, create a new one):**

```sql
-- WRONG: editing 0001 after it's been applied somewhere:
-- 0001_init_orders.up.sql (EDITED — DON'T DO THIS)
BEGIN;
CREATE TABLE orders.customers (
    id    UUID PRIMARY KEY,
    name  TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT          ← ADDED AFTER MERGE — migrate will detect checksum mismatch
);
COMMIT;

-- CORRECT: create a new migration for the change:
-- 0002_add_phone_to_customers.up.sql (NEW FILE)
BEGIN;
ALTER TABLE orders.customers ADD COLUMN phone TEXT;
COMMIT;
```

**Rule 3 — Wrap in explicit `BEGIN; ... COMMIT;` transaction:**

```sql
-- CORRECT — all-or-nothing execution (Axiom 3):
BEGIN;
CREATE TABLE orders.customers (id UUID PRIMARY KEY, ...);
CREATE INDEX idx_customers_email ON orders.customers(email);
ALTER TABLE orders.customers ADD CONSTRAINT uk_email UNIQUE(email);
COMMIT;
-- If the UNIQUE CONSTRAINT fails (email column already has duplicates),
-- ALL three statements are rolled back — database is unchanged.

-- WRONG — no transaction:
CREATE TABLE orders.customers (id UUID PRIMARY KEY, ...);
CREATE INDEX idx_customers_email ON orders.customers(email);
ALTER TABLE orders.customers ADD CONSTRAINT uk_email UNIQUE(email);
-- ↑ If line 3 fails, lines 1 and 2 are already committed.
--   Database has a partial migration — table exists, constraint doesn't.
--   Migration state is unclear and hard to recover from.
```

**Rule 4 — Up migrations only (no down migrations):**

```sql
-- We write ONLY this (up migration):
-- 0003_add_delivery_address.up.sql
BEGIN;
ALTER TABLE orders.customers ADD COLUMN delivery_address_line1 TEXT;
ALTER TABLE orders.customers ADD COLUMN delivery_address_city TEXT;
COMMIT;

-- We do NOT write this (down migration — not used in this project):
-- 0003_add_delivery_address.down.sql  ← DO NOT CREATE
-- BEGIN;
-- ALTER TABLE orders.customers DROP COLUMN delivery_address_line1;
-- ALTER TABLE orders.customers DROP COLUMN delivery_address_city;
-- COMMIT;
```

#### e) Execution / Internal Walkthrough

A migration fails midway (without a transaction — showing WHY transactions are required):

```
WITHOUT TRANSACTION (wrong approach):
Step 1: CREATE TABLE orders.customers — SUCCESS (committed immediately)
Step 2: CREATE INDEX idx_email ON orders.customers(email) — SUCCESS (committed)
Step 3: ALTER TABLE orders.customers ADD CONSTRAINT uk_email UNIQUE(email)
        → FAILS: ERROR — column email has duplicate values
Step 4: Database state: table EXISTS, index EXISTS, but constraint DOES NOT EXIST
Step 5: If the migration is retried without fixing the data issue:
        → Step 1 FAILS: "relation orders.customers already exists"
        → The retry can't succeed because the partial state blocks it
Step 6: Manual intervention required — partial cleanup needed
        → Time, risk, and confusion

WITH TRANSACTION (correct approach):
Step 1: BEGIN — start transaction
Step 2: CREATE TABLE orders.customers — part of transaction (not committed yet)
Step 3: CREATE INDEX — part of transaction
Step 4: ADD CONSTRAINT — FAILS
Step 5: PostgreSQL rolls back the ENTIRE transaction
Step 6: Database is exactly as it was before the migration started
Step 7: Developer fixes the data issue, retries — migration succeeds from a clean state
```

**Where each axiom becomes visible:**
- Axiom 1: Sequential numbers in the walkthrough — `0001` always comes before `0002`.
- Axiom 2: The "WRONG approach" code comment shows what happens if a merged migration is edited.
- Axiom 3: The `WITHOUT TRANSACTION` vs `WITH TRANSACTION` comparison shows Axiom 3 in action.
- Axiom 4: No `.down.sql` file is created in this project.
- Axiom 5: `CREATE SCHEMA IF NOT EXISTS` and `CREATE TABLE IF NOT EXISTS` make migrations safe to retry.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| Down migrations provide a safe rollback option for production | **FALSE** | Axiom 4 — data written under the new schema may be incompatible with the old schema. Rollback can corrupt data. Fix forward. |
| `BEGIN; ... COMMIT;` makes the migration slower | **NEGLIGIBLE** | A transaction around schema changes has near-zero performance overhead. The correctness benefit (Axiom 3) vastly outweighs any theoretical cost. |
| You can use timestamp-based naming if you're careful about naming conflicts | **TRUE but risky at scale** | With a small team, timestamps work fine. With parallel development (the norm in growing projects), sequential numbers are safer. The page recommends sequential numbers for this reason. |
| Sequential numbering means migrations can never be merged in parallel | **PARTLY TRUE** | Two people CAN develop migrations in parallel — they just pick the same number and ONE must rename after the other merges. This is a minor inconvenience vs. the unpredictability of timestamp ordering. |
| A failed transaction leaves the database in an error state | **FALSE** | Axiom 3 — PostgreSQL atomically rolls back ALL statements in a failed transaction. The database returns to its exact state before the transaction started. No error state remains. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| No `BEGIN; ... COMMIT;` in a multi-statement migration | Partial migration on failure — database in inconsistent state | Axiom 3 violated — statements commit individually; failure leaves partial state | Always wrap ALL migration SQL in `BEGIN; ... COMMIT;` |
| Editing a committed migration to fix a typo | `migrate` checksum error on next startup | Axiom 2 violated — any applied migration is immutable | Create a new migration with the correction; never edit applied migrations |
| Using `CREATE TABLE` without `IF NOT EXISTS` | Migration fails with "relation already exists" on retry after partial failure | No idempotency — running the migration twice fails on the second run | Use `CREATE TABLE IF NOT EXISTS` (Axiom 5 — idempotency) |
| Naming a new migration with the same number as an existing one | Git conflict (two new files with same name) or silent overwrites | Not following sequential numbering (Axiom 1) | Check the highest existing number, use the next sequential number |
| Writing a down migration "just in case" | Wasted effort + false sense of safety | Misunderstanding Axiom 4 — down migrations are rarely safe in production | Skip down migrations entirely; fix forward with a new migration |

#### h) Comprehension Task

> **Comprehension Task:** Below is a migration file. On paper, identify EVERY migration rule violation and state which axiom each violation breaks:
>
> ```sql
> -- File: 0003_update_customers.up.sql
> -- (Note: 0002_something.up.sql was merged last week)
>
> CREATE TABLE orders.customers (
>     id UUID PRIMARY KEY,
>     name TEXT NOT NULL,
>     email TEXT NOT NULL
> );
>
> ALTER TABLE orders.customers ADD COLUMN phone TEXT NOT NULL;
> ```
>
> **What to check:** (1) No `BEGIN; ... COMMIT;` — Axiom 3 violated. If `ALTER TABLE` fails (e.g., because `orders.customers` already exists from an earlier migration), the `CREATE TABLE` is already committed — partial state, hard to recover from. (2) `CREATE TABLE` without `IF NOT EXISTS` — Axiom 5 (idempotency) violated; running this migration twice causes "relation already exists" error. (3) Possible Axiom 2 violation — if this file is meant to UPDATE an existing `customers` table (i.e., 0001 or 0002 already created it), then `CREATE TABLE` should be `ALTER TABLE ADD COLUMN` — re-creating a table that already exists overwrites existing data.
>
> **Common wrong answer:** "The migration looks fine — it has CREATE TABLE and ALTER TABLE which are valid SQL." — Wrong. SQL validity is not the same as migration correctness. The migration is technically valid SQL but violates safety rules that prevent production disasters.

---

## SECTION 4 — EXERCISE READINESS

### a) What the Exercise Will Likely Ask

Based on the webpage content, the exercise likely asks you to:

1. **Create the first migration file** — `0001_init_orders.up.sql` in `backend/orders/adapters/db/migrations/` (the file currently contains just `-- todo: implement`).
2. **Write the SQL** — `CREATE SCHEMA IF NOT EXISTS orders;` and `CREATE TABLE orders.customers (...)` with appropriate columns, wrapped in `BEGIN; ... COMMIT;`.
3. **Verify migrations run** — start the application (or run a test) and confirm the `orders.customers` table is created by the migration.

### b) Pre-Implementation Checklist

- [ ] I can explain what a SQL migration is in one sentence without using the word "migration."
- [ ] I can trace through what happens on the first vs. second application startup with respect to `schema_migrations`.
- [ ] I can state the five migration rules (sequential numbers, never edit merged, use transactions, up-only, IF NOT EXISTS) and the axiom behind each.
- [ ] I can explain why `CREATE SCHEMA IF NOT EXISTS orders` appears TWICE (in Go code AND in the migration file) without calling it a duplicate.
- [ ] I know exactly what the `//go:embed` directive does and which step in the build process it acts on.

### c) Implementation Blueprint

**Step 1:** Open `backend/orders/adapters/db/migrations/0001_init_orders.up.sql`. This is where the migration SQL goes.
*(The file already exists with `-- todo: implement`. The embed glob `*.sql` will pick it up automatically — Axiom 5 of `go:embed`.)*

**Step 2:** Start with `BEGIN;` on the first line and `COMMIT;` on the last line — always.
*(Relies on Migration Rule Axiom 3 — all statements must be wrapped in an explicit transaction.)*

**Step 3:** Add `CREATE SCHEMA IF NOT EXISTS orders;` as the first statement inside the transaction.
*(Relies on PostgreSQL Schema Axiom 4 — the migration file must declare the schema for `sqlc` to find it. `IF NOT EXISTS` makes it idempotent — Axiom 5.)*

**Step 4:** Add `CREATE TABLE IF NOT EXISTS orders.customers (...)` with at minimum: `id UUID PRIMARY KEY`, `name TEXT NOT NULL`, `email TEXT NOT NULL`.
*(Relies on PostgreSQL Schema Axiom 1 — use `schema.table` notation. `IF NOT EXISTS` — Axiom 5.)*

**Step 5:** Save the file. The `//go:embed adapters/db/migrations/*.sql` glob picks it up on the next `go build` automatically.
*(Relies on `go:embed` Axiom 5 — glob pattern matches all `.sql` files.)*

**Step 6:** Start the application (via `task` or docker-compose) and check the logs for "migration up" output. If no error, the `orders.customers` table now exists in PostgreSQL.
*(Relies on SQL Migration Axiom 5 — migrations run automatically on startup via `Module.Init()`.)*

### d) Debugging Guide

| Failure Symptom | Violated Axiom | Fix |
|---|---|---|
| `migrate` checksum error on startup | Migration Axiom 2 — a migration file was edited after being applied | Never edit a migration that's already been applied. For a local dev database, you can drop it and start fresh. For production, create a new migration. |
| `relation "orders.customers" already exists` error in migration | Migration Axiom 5 — missing `IF NOT EXISTS` | Add `IF NOT EXISTS` to `CREATE TABLE` |
| Migration SQL runs partially — table created but some columns missing | Migration Axiom 3 — missing `BEGIN; ... COMMIT;` | Wrap all SQL in explicit transaction; partially applied migrations need database cleanup first |
| `go:embed` directive causes compile error "no matching files" | `go:embed` Axiom 5 — no `.sql` files in the directory | Ensure at least one `.sql` file exists in `adapters/db/migrations/` |

---

## SECTION 5 — STRUCTURED PRACTICE (AEIOU Framework)

### A — ACQUIRE (Axioms First)

**SQL Migration axioms to internalize:**
1. Applied ONCE per database — `schema_migrations` tracks which have run.
2. Applied in ORDER — sequential numbers guarantee this.
3. Never edit a merged migration — create a new one instead.
4. Always wrap in `BEGIN; ... COMMIT;` — all-or-nothing execution.
5. Up migrations only — fix forward, never rollback.

**`go:embed` axioms:**
1. Files are embedded at COMPILE TIME — not at runtime.
2. The directive is a comment read by the compiler — not a function call.
3. `embed.FS` is read-only — no runtime writes.

**PostgreSQL schema axioms:**
1. Schema = namespace — same table name in different schemas = different tables.
2. Each module owns its own schema (named after the module).
3. `CREATE SCHEMA IF NOT EXISTS` appears TWICE — Go code (for migrate) and migration file (for sqlc).

**Write from memory (no copy-paste):**
- The `//go:embed` directive and `var embedMigrations embed.FS` declaration.
- A complete migration file (BEGIN/COMMIT, CREATE SCHEMA IF NOT EXISTS, CREATE TABLE).
- The `common.MigrateDatabaseUp(...)` function call from `module.go`.

---

### E — EXERCISE (Reason from Nothing)

**Problem 1 (Simple — migration ordering):**
A project has migration files: `0001_init.up.sql`, `0002_add_index.up.sql`, `0003_add_column.up.sql`. The database has `schema_migrations` showing versions 1 and 2. The app starts. Which migrations run, in what order, and why?

**Problem 2 (Simple — `go:embed`):**
You add a new file `0004_add_table.up.sql` to `adapters/db/migrations/`. You do NOT change any Go code. After running `go build`, is the new file embedded in the binary? Why? Which axiom guarantees this?

**Problem 3 (Medium — transaction rules):**
A migration file has three statements: `CREATE TABLE`, `CREATE INDEX`, `ADD CONSTRAINT`. The `ADD CONSTRAINT` fails. Describe what the database state looks like WITH and WITHOUT `BEGIN; ... COMMIT;`. Which situation is easier to recover from and why?

**Problem 4 (Medium — schema isolation):**
Two modules both have a migration that creates a `customers` table. The `orders` module creates `orders.customers` and the `restaurants` module creates `restaurants.customers`. Explain why these don't conflict, using PostgreSQL Schema Axioms.

**Problem 5 (Complex — immutability rule):**
You merged migration `0002_add_column.up.sql` last week. It created a column called `phone_numbre` (typo). You want to fix the typo. On paper, write the two approaches: (1) editing the existing file, (2) creating a new file. For each approach, describe what happens when the app starts, which axiom it relies on or violates, and which approach is correct.

**Problem 6 (Complex — combining all concepts):**
The `embed.FS` variable `embedMigrations` is passed to `common.MigrateDatabaseUp`. Inside that function, `iofs.New(embedMigrations, "adapters/db/migrations")` is called. Trace what happens from binary startup to the first SQL statement executing in the database. Name every concept from this page that activates at each step.

---

### I — INSPECT (Identify the Violation)

**Task 1:** What is wrong? Which axiom is violated?
```sql
-- 0002_add_phone.up.sql
ALTER TABLE orders.customers ADD COLUMN phone TEXT;
```

**Task 2:** What is wrong?
```go
//go:embed adapters/db/migrations/*.sql

var embedMigrations embed.FS
```

**Task 3:** What is wrong?
```sql
-- 0003_add_address.up.sql
BEGIN;
CREATE TABLE orders.customers (
    delivery_address TEXT
);
COMMIT;
```
*(Note: 0001 already created `orders.customers`)*

**Task 4:** What is wrong?
```go
//go:embed /home/developer/project/backend/orders/adapters/db/migrations/*.sql
var embedMigrations embed.FS
```

**Task 5:** What is wrong?
```sql
-- Developer edited 0001_init_orders.up.sql to add a missing column, 
-- then ran the migration in their local dev environment.
-- They pushed this edit to git and opened a PR.
BEGIN;
CREATE SCHEMA IF NOT EXISTS orders;
CREATE TABLE orders.customers (
    id    UUID PRIMARY KEY,
    name  TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT           ← added after the fact
);
COMMIT;
```

---

### O — ORCHESTRATE (Synthesis Design)

**Scenario:** You're adding the `restaurants` module to the project. It needs:
- Its own PostgreSQL schema: `restaurants`
- A `menus` table: `id UUID PRIMARY KEY`, `restaurant_id UUID NOT NULL`, `name TEXT NOT NULL`
- A `menu_items` table: `id UUID PRIMARY KEY`, `menu_id UUID NOT NULL`, `name TEXT NOT NULL`, `price_cents INT NOT NULL`
- The migrations to be embedded in the `restaurants` module binary
- The migrations to run automatically on startup

**Write in plain English:**
1. What files do you create and where?
2. What SQL goes in the first migration file?
3. Where does the `//go:embed` directive go and what pattern does it use?
4. What `Init()` method change calls `MigrateDatabaseUp`?
5. What is the module name passed to `MigrateDatabaseUp` and why does it matter for schema names?

**Write the implementation blueprint step by step** — which axiom does each step rely on?

---

### U — UNDERSTAND (Feynman Reconstruction Test)

**Most commonly misunderstood concept: `go:embed` — specifically the difference between compile time and runtime**

**Step 1 — Explain it simply:**
"Imagine you're baking a birthday cake, and the recipe is written on a notecard. When you put the cake in a box to give to someone, you GLUE the notecard to the inside of the box. The person who receives the cake has the recipe right there — they don't need to find the notecard separately. `go:embed` is like gluing the notecard (SQL files) inside the box (binary) when baking (compiling) the cake (program)."

**Step 2 — Find the gap:**
A beginner might say: "So `go:embed` runs when the program starts and copies the files into the binary." The gap: they think it happens AT RUNTIME — they don't understand that "gluing the notecard" happens during BAKING (compilation), not when the recipient opens the box.

**Step 3 — Go back to axioms:**
Axiom 1: The embedding happens at COMPILE TIME — when you run `go build`. The Go compiler reads the files from your disk and bakes their contents into the binary. By the time the binary runs on any machine, the files are already inside it. The binary doesn't need any files to exist on disk at runtime.

**Step 4 — Proof of Understanding:**
**Question:** You compile the binary on your laptop. You send only the binary file (not the SQL directory) to a colleague. Your colleague runs the binary on their machine which has never had any SQL files. Will migrations run? Which axiom explains why?

**Expected Answer:** YES, migrations run successfully. Axiom 1 — the SQL file contents were embedded INTO the binary during compilation on your laptop. Your colleague's machine never needs the SQL files — the binary already contains them. `embed.FS` reads from the binary's internal data, not from the filesystem (Axiom 3).

---

## SECTION 6 — KNOWLEDGE CHECK

### a) Scenario-Based Reasoning Questions

**Q1:** The app is deployed to production. Migration 0001 has been applied and 10,000 customer rows have been inserted. You notice that you need to add a `created_at TIMESTAMPTZ DEFAULT NOW()` column to `orders.customers`. What is the correct approach? Walk through EXACTLY what file you create and what SQL it contains.

**Expected Answer:** Create a NEW migration file: `0002_add_created_at_to_customers.up.sql`. Contents:
```sql
BEGIN;
ALTER TABLE orders.customers ADD COLUMN created_at TIMESTAMPTZ DEFAULT NOW();
COMMIT;
```
Axiom 2: migration 0001 is immutable — don't edit it. Axiom 4: don't write a down migration. Axiom 3: wrap in transaction. Using `ALTER TABLE ADD COLUMN` with a DEFAULT means existing 10,000 rows get `NOW()` as their `created_at` value — safe in PostgreSQL. When the app next starts, `migrate` sees version 2 is new, applies it (Axiom 3 — tracking), and the column exists.

---

**Q2:** A developer on your team claims: "I run `CREATE TABLE` directly in my database client tool instead of creating migration files — it's faster." Explain, using axioms, exactly what problems this creates for the rest of the team.

**Expected Answer:** Axiom 5 of SQL Migration — migrations run automatically on startup across ALL environments. The developer's direct SQL only runs on their machine. Every other developer's database is missing the table — their app crashes when code tries to use it. CI and production are also missing the change — the build may pass locally but fail in CI. There's no record in `schema_migrations`, so when proper migrations are later applied, `migrate` doesn't know about the manual change and may conflict with it. The fix: always create a migration file; never run schema SQL directly.

---

**Q3:** Your project's `//go:embed adapters/db/migrations/*.sql` directive currently embeds one file. A new developer adds `0002_add_column.up.sql` to the directory. They rebuild and redeploy. Does the new migration get embedded and applied? Which axioms explain each part?

**Expected Answer:** YES, on both counts. `go:embed` Axiom 5 — the glob `*.sql` matches ALL `.sql` files in the directory, including newly added ones. When the developer ran `go build`, the new file was automatically included in the binary (Axiom 1 — embed happens at compile time). When the app starts, `migrate` checks `schema_migrations`, finds version 2 is missing, applies `0002_add_column.up.sql` (SQL Migration Axiom 3 — each migration runs once), records version 2 in `schema_migrations`.

---

**Q4:** A CI job runs `go test ./...`. The tests connect to a test PostgreSQL database that has never had migrations applied. The test setup calls `Module.Init()`. What happens to the test database, and what state is it in after `Init()` completes?

**Expected Answer:** Migration Axiom 5 — `Module.Init()` calls `MigrateDatabaseUp` automatically. `MigrateDatabaseUp` creates the `orders` schema, creates `schema_migrations`, finds all migration files in `embedMigrations` (via `go:embed`), and applies them all in order (Axiom 2 — in sequence). After `Init()` completes: the test database has the `orders` schema, the `orders.customers` table (from 0001), all subsequent migration tables/columns, and `schema_migrations` showing all versions as applied. The tests can now INSERT and SELECT from the database normally.

---

**Q5:** You're explaining the `CREATE SCHEMA IF NOT EXISTS orders` duplication to a new teammate. They ask: "Can't we just put it in the Go code and remove it from the migration file?" What happens if you remove it from the migration file?

**Expected Answer:** PostgreSQL Schema Axiom 4 — the migration file version exists specifically for `sqlc` (the query generation library, introduced in the next page). `sqlc` reads the migration files to understand the database schema structure — it needs to see `CREATE SCHEMA` to know that the `orders` schema exists. Without it in the migration file, `sqlc` wouldn't recognize `orders.customers` as a valid table reference when generating Go database query code. The Go code version is for `migrate` (so it can place `schema_migrations` in the right schema). Both serve different tools; both are needed.

---

### b) "What If?" First-Principles Challenges

**What if migrations were NOT embedded in the binary (no `go:embed`)?**

Without `go:embed`, the migration `.sql` files would be external to the binary. Deploying the application would require deploying TWO things: the binary AND the migrations directory. Every deployment system (Docker image, server copy, CI pipeline) would need to know: "migration files must be at `adapters/db/migrations/` relative to the binary."

Consequences:
1. Developers frequently forget to copy the SQL files when testing locally → migrations don't run → app crashes with "table does not exist."
2. Container images must include the SQL files, adding complexity to the Dockerfile.
3. Any reorganization of the directory structure (moving SQL files) breaks production deployments silently — the binary looks for files that are no longer there.
4. The single-binary deployment advantage (one file, deploy anywhere) is lost.

`go:embed` exists because the irreducible need is: the binary must be self-contained. Axiom 1: embed at compile time, so no runtime file dependencies exist.

---

**What if migrations had NO transaction wrapping?**

Without `BEGIN; ... COMMIT;`, each SQL statement in a migration commits immediately and independently. Consider a migration with three statements: `CREATE TABLE`, `CREATE INDEX`, `ALTER TABLE ADD CONSTRAINT`.

If the third statement fails (say, a unique constraint on a column that already has duplicate values), the outcome is:
- Statement 1: `CREATE TABLE` — COMMITTED (permanent)
- Statement 2: `CREATE INDEX` — COMMITTED (permanent)
- Statement 3: `ALTER TABLE ADD CONSTRAINT` — FAILED (not applied)

The `migrate` library marks this migration as `dirty=true` in `schema_migrations` — meaning it partially applied. To recover: someone must manually connect to the database and clean up the partial state (drop the index, drop the table) before the migration can be retried.

In production, this might mean a deployment that fails with a partial database change, manual database surgery, and a period where the application can't start. With `BEGIN; ... COMMIT;`, the failure rolls back ALL three statements atomically — the database is untouched, and the migration can be fixed and retried from a clean state.

---

**What if migrations used TIMESTAMPS instead of sequential numbers?**

With timestamps (e.g., `20260218120000_add_index.up.sql`), each migration's order is determined by the timestamp in its filename. Two developers both add migrations at the same time:

- Developer A creates: `20260219093045_add_customer_index.up.sql`
- Developer B creates: `20260219093052_add_order_table.up.sql`

These can be merged in any order. But now: Developer A's migration lands in production first. Developer B's migration runs after. This is fine.

But next month:
- Developer C's PR is approved first and merged: `20260315100000_add_payments_table.up.sql`
- Developer D's PR, which was started before C's, has a migration with an earlier timestamp: `20260314080000_modify_customers.up.sql`

Developer D's migration has an EARLIER timestamp — it should run BEFORE Developer C's. But Developer C's is already in production as `version_applied: 20260315100000`. Now Developer D's migration from `20260314` tries to run AFTER Developer C's `20260315` migration. The `migrate` library applies them in timestamp order — but the production database has already processed a later-timestamp migration. The state is undefined.

With sequential numbers: Developer C's migration is `0005` (the next after 4 existing ones). Developer D must rename their migration to `0006` after C merges. The order is always explicit, always deterministic, always agreed upon before any migration runs in production.

---

### c) Page FAQ

**Q: The migration file already exists with `-- todo: implement`. Do I need to create a new file?**
The reason this works this way is that the file exists because the project's exercise system created it as a placeholder for you to fill in. The `//go:embed` glob requires at least one `.sql` file to exist in the directory at compile time — otherwise the embed directive fails. The `-- todo: implement` placeholder ensures the embed works even before you write the actual migration. Your task is to replace the placeholder content with real SQL.

**Q: Why does `common.MigrateDatabaseUp` use `pgxpool.Pool` but then convert it to a `*sql.DB`?**
The reason this works this way is that the `golang-migrate` library's PostgreSQL driver (`pgxMigrate.WithInstance`) uses Go's standard `database/sql` interface, not the newer `pgx`-native interface. `stdlib.OpenDBFromPool(pool)` creates a `*sql.DB` that wraps the existing pgx connection pool — no new connections are established. The same connection pool serves both `pgx`-native queries (for your application code) and the `database/sql`-compatible migrate library. Two different library interfaces, one underlying connection pool.

**Q: What does `dirty=true` in `schema_migrations` mean and how do you fix it?**
The reason this works this way is that `dirty=true` means a migration started applying but didn't finish (or failed without a transaction rollback). The `migrate` library sets `dirty=true` before applying a migration and only sets it back to `false` after success. If the process crashes mid-migration, `dirty=true` remains. Fix: (1) connect to the database, (2) manually clean up any partial changes the migration made, (3) run `UPDATE schema_migrations SET dirty=false WHERE version=X`. Then the migration can be retried. This is exactly why wrapping in `BEGIN; ... COMMIT;` is critical — a transaction failure leaves `dirty=false` and the schema unchanged, making recovery trivial.

**Q: Can migrations run in parallel (multiple app instances starting simultaneously)?**
The reason this works this way is that the `golang-migrate` library uses a database-level advisory lock when applying migrations. Only one instance can hold the lock at a time — all other instances wait. When the first instance finishes and releases the lock, the next instance checks `schema_migrations`, finds all migrations already applied (Axiom 3), and skips them. Multiple app instances starting simultaneously (common in Kubernetes deployments) is safe — migrations apply exactly once.

**Q: Why does the `Module.Name()` method return `"orders"` as a string? Couldn't you just hardcode `"orders"` in `MigrateDatabaseUp`?**
The reason this works this way is that `MigrateDatabaseUp` is defined in `common/migrations.go` — it's shared infrastructure used by EVERY module. It accepts the schema name as a parameter so it works correctly for `orders`, `restaurants`, `couriers`, and any future module without modification. If `"orders"` were hardcoded inside the function, every module would migrate into the `orders` schema — a catastrophic schema collision (violating PostgreSQL Schema Axiom 3). Using `string(m.Name())` means the schema name comes from the module itself — each module's `Name()` method returns the right value for that module.

---

## SECTION 7 — JARGON BUSTER DICTIONARY

---

**Term: Adapter (in software architecture)**
First-Principles Origin: Systems needed a way to separate "what the application does" from "how it stores and retrieves data" — so either side could change without affecting the other.
Meaning: A software component that translates between your application's internal language and an external system's language. It sits between your business logic and the outside world (database, API, filesystem).
Analogy: A universal power adapter for travel. Your laptop (business logic) needs electricity. The wall socket in France has a different shape than in the US. The adapter translates between the two — your laptop doesn't care about the socket format; the adapter handles the difference.
Example:
```
// The adapter lives in:
backend/orders/adapters/db/

// Inside: SQL queries that translate Go structs to database rows and back.
// The handler calls: adapter.SaveCustomer(ctx, customer)
// The adapter executes: INSERT INTO orders.customers (id, name, email) VALUES ($1, $2, $3)
```
Don't Confuse With: A handler — the handler processes HTTP requests and applies business logic. The adapter handles database/external system communication. Handler = what to do. Adapter = how to store it.

---

**Term: `BEGIN; ... COMMIT;` (SQL Transaction)**
First-Principles Origin: Databases needed a way to group multiple SQL statements so they either ALL succeed together or ALL fail together — preventing partial changes that leave data in an inconsistent state.
Meaning: A transaction is a group of SQL statements that run as one atomic unit. `BEGIN` starts the group; `COMMIT` applies all changes permanently. If any statement fails, `ROLLBACK` (or automatic rollback) undoes ALL changes back to the state before `BEGIN`.
Analogy: A bank transfer. Moving money from Account A to Account B requires two steps: debit A, credit B. A transaction ensures both happen or neither happens. Without a transaction, if the system crashes after debiting A but before crediting B, money disappears. The transaction makes both steps atomic — one inseparable unit.
Example:
```sql
BEGIN;                                          -- Start the atomic group
CREATE TABLE orders.customers (id UUID, ...);  -- Step 1
CREATE INDEX idx_email ON orders.customers;     -- Step 2
COMMIT;                                         -- ALL changes applied permanently
-- If Step 2 fails: BOTH changes are rolled back.
-- Database is unchanged. Safe to fix and retry.
```
Don't Confuse With: A savepoint — that's a "partial rollback" marker within a transaction. `BEGIN/COMMIT` is the outer boundary. For migrations, always use `BEGIN/COMMIT` without savepoints.

---

**Term: DDL (Data Definition Language)**
First-Principles Origin: SQL needed a category for commands that change the DATABASE STRUCTURE (as opposed to the data inside it) — so developers could reason about which commands affect schemas and which affect rows.
Meaning: SQL commands that create, modify, or delete structural elements of a database: tables, schemas, indexes, constraints. Examples: `CREATE TABLE`, `ALTER TABLE`, `DROP TABLE`, `CREATE INDEX`, `CREATE SCHEMA`.
Analogy: Building construction vs. moving furniture. DDL is like building a room (adding walls, doors, windows — structural). DML (Data Manipulation Language: INSERT, UPDATE, DELETE) is like arranging furniture inside a room that already exists.
Example:
```sql
-- DDL — changes the structure:
CREATE TABLE orders.customers (id UUID PRIMARY KEY, name TEXT);
ALTER TABLE orders.customers ADD COLUMN phone TEXT;
DROP TABLE orders.customers;

-- NOT DDL (DML — changes the data):
INSERT INTO orders.customers VALUES ('uuid-here', 'Alice');
UPDATE orders.customers SET name = 'Bob' WHERE id = 'uuid-here';
```
Don't Confuse With: DML (Data Manipulation Language) — INSERT, UPDATE, DELETE, SELECT. DML changes the DATA inside tables. DDL changes the STRUCTURE of tables. Migrations use DDL (creating tables and schemas) plus DML only when needed for data migrations.

---

**Term: `embed.FS`**
First-Principles Origin: Go needed a type to represent the embedded files baked into a binary, compatible with the standard filesystem interface so existing libraries could read from it without modification.
Meaning: A Go type that holds files embedded at compile time by the `//go:embed` directive. It behaves like a read-only filesystem — you can read files from it, list directories, but you can never write to it. It's a snapshot of files frozen at the moment of compilation.
Analogy: A photo album vs. a camera. A camera (regular filesystem) takes new photos. A photo album (`embed.FS`) contains photos that were printed at a specific moment. You can look at and share the photos, but the album itself is sealed — you can't take new photos with it or remove existing ones.
Example:
```go
//go:embed templates/*.html          // Bake all .html files into the binary
var webTemplates embed.FS            // Access them through this variable

// Reading an embedded file at runtime:
data, err := webTemplates.ReadFile("templates/index.html")
// ↑ Reads from the binary's internal data — no disk access
```
Don't Confuse With: `os.DirFS` — that reads files from the actual disk at runtime. `embed.FS` reads from the binary's internal snapshot (compile-time). Use `os.DirFS` when you need files to be changeable at runtime; use `embed.FS` for files that are fixed at build time.

---

**Term: `go:embed`**
First-Principles Origin: Go needed a standard, built-in mechanism to include arbitrary files in a compiled binary so binaries could be self-contained without requiring external file dependencies at runtime.
Meaning: A Go compiler directive (a special comment starting with `//go:`) that tells the Go compiler to bundle specific files from your filesystem into the compiled binary. The files are accessible at runtime through an `embed.FS` variable.
Analogy: Packing your lunch in the morning (compile time). You prepare your sandwich and put it in your backpack. During the day (runtime), when you're hungry, you eat the sandwich from your backpack — you don't go back to the kitchen. `go:embed` prepares and packs the files; the running program eats from the backpack.
Example:
```go
package orders

import "embed"

//go:embed adapters/db/migrations/*.sql  // directive — tell compiler to embed these files
var embedMigrations embed.FS             // variable — access embedded files here at runtime
```
Don't Confuse With: `//go:generate` — that runs an EXTERNAL TOOL (like oapi-codegen) during development. `//go:embed` bundles FILES into the binary during compilation. Different directive, different purpose.

---

**Term: Idempotent / Idempotency**
First-Principles Origin: Systems needed operations that could safely be repeated without causing errors or unintended side effects — because in distributed systems, operations sometimes run twice due to retries or failures.
Meaning: An operation is idempotent if running it multiple times produces the same result as running it once. "Do this thing, but only if it hasn't been done already."
Analogy: Pressing the elevator button. The elevator is called whether you press the button once or ten times — pressing it more than once has no additional effect. Pressing "light on" on a light that's already on — same result as pressing it once.
Example:
```sql
-- Idempotent (safe to run multiple times):
CREATE TABLE IF NOT EXISTS orders.customers (...);
CREATE SCHEMA IF NOT EXISTS orders;

-- NOT idempotent (fails on second run):
CREATE TABLE orders.customers (...);   -- Error: relation already exists
```
Don't Confuse With: Deterministic — that means "same input, same output." Idempotent means "running multiple times is the same as running once." A delete operation can be deterministic but NOT idempotent (deleting once succeeds; deleting a second time when nothing's there may error or produce a different state).

---

**Term: `migrate` library (golang-migrate)**
First-Principles Origin: Go applications needed a reliable, versioned way to apply SQL schema changes to databases — handling tracking, ordering, and error recovery automatically.
Meaning: A Go library (`github.com/golang-migrate/migrate`) that manages SQL migration files. It tracks which migrations have been applied in a `schema_migrations` table, applies new ones in order, and handles errors. Used in this project via `common.MigrateDatabaseUp`.
Analogy: A project management system with a checklist. The checklist (schema_migrations) records which tasks (migrations) are done. When you start a work session, you check the list — only the unchecked items need to be done. Completed items are never repeated.
Example:
```go
// From common/migrations.go:
m, err := migrate.NewWithInstance("iofs", d, "pgx", migDb)
// d = source of migration files (from embed.FS)
// migDb = database to apply migrations to

if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
    return fmt.Errorf("migration up failed: %w", err)
}
// m.Up() applies all unapplied migrations in order.
// ErrNoChange means "all migrations already applied" — not an error.
```
Don't Confuse With: `sqlc` — that generates Go query code from SQL files. `migrate` applies schema CHANGES to the database. `migrate` changes the database structure; `sqlc` generates Go code to QUERY that structure.

---

**Term: Migration (SQL)**
First-Principles Origin: Software teams needed a versioned, auditable, automated way to evolve database schemas across multiple environments — instead of running ad-hoc SQL commands that are easily lost or forgotten.
Meaning: A numbered SQL file containing one increment of database structure change. Each migration is applied exactly once per database. Together, migrations form the complete history of how a database schema evolved over time.
Analogy: A building's renovation history — documented work orders. Work Order 1: "Install plumbing." Work Order 2: "Build kitchen." Work Order 3: "Add second bathroom." Each work order is done once, in order, on every building. No work order is ever undone except by a new work order.
Example:
```
backend/orders/adapters/db/migrations/
  0001_init_orders.up.sql    ← "Install the foundation" — creates initial tables
  0002_add_index.up.sql      ← "Add faster search" — adds a database index
  0003_add_column.up.sql     ← "Extend the structure" — adds a new column
```
Don't Confuse With: A seed file — a seed file INSERTS initial data (like sample categories). A migration changes the STRUCTURE (tables, columns). Seeds change data; migrations change schema.

---

**Term: `pgxpool.Pool`**
First-Principles Origin: Database connections are expensive to create — each one requires a network handshake. Applications needed a way to reuse existing connections instead of creating a new one for every query.
Meaning: A connection pool for the `pgx` PostgreSQL driver. Instead of opening a new database connection for every operation, the pool maintains a set of pre-opened connections that are reused. `pgxpool.Pool` manages the lifecycle of these connections.
Analogy: A taxi dispatch service. Instead of buying a new car every time someone needs a ride, the dispatch service maintains a fleet of cars. A car picks up a passenger (handles a query), drops them off, and returns to the fleet — ready for the next passenger. No new car is needed for every ride.
Example:
```go
// In module.go:
type Module struct {
    pgxDb *pgxpool.Pool   // The pool — pre-opened database connections
}
// When MigrateDatabaseUp needs a connection, it borrows one from the pool.
// When done, the connection returns to the pool for reuse.
```
Don't Confuse With: A single `*sql.DB` connection — that's also a pool but uses the older `database/sql` interface. `pgxpool.Pool` uses the newer, PostgreSQL-specific `pgx` interface with better type support and performance.

---

**Term: PostgreSQL Schema (namespace)**
First-Principles Origin: Databases serving multiple applications or teams needed a way to organize tables into separate namespaces — preventing name collisions and providing clear ownership.
Meaning: A named container within a PostgreSQL database. Tables inside a schema are referenced as `schema_name.table_name`. Two schemas can have tables with the same name without any conflict. In this project, each module gets its own schema.
Analogy: Folders in a file system. Two files can have the same name if they're in different folders: `/documents/report.pdf` and `/downloads/report.pdf` are completely different files. PostgreSQL schemas are like folders — `orders.customers` and `restaurants.customers` are different tables even though both are called `customers`.
Example:
```sql
-- Create the namespace:
CREATE SCHEMA IF NOT EXISTS orders;

-- Create a table inside it:
CREATE TABLE orders.customers (id UUID PRIMARY KEY, name TEXT);
-- Full name: schema_name.table_name

-- Another schema, same table name — no conflict:
CREATE SCHEMA IF NOT EXISTS restaurants;
CREATE TABLE restaurants.customers (id UUID PRIMARY KEY, name TEXT);
```
Don't Confuse With: A PostgreSQL database — a database is the top-level container. A schema is a namespace WITHIN a database. One database can have many schemas. `orders` and `restaurants` are both schemas inside the single `eats` database.

---

**Term: `schema_migrations` table**
First-Principles Origin: Migration libraries needed a persistent record of which migrations had been applied to a database — so they could determine which ones to skip and which ones to run on the next startup.
Meaning: A table automatically created and managed by the `migrate` library. It stores one row per applied migration (the version number and a `dirty` flag). The library reads this table on every startup to decide which migrations need to run.
Meaning: Think of it as the library's own internal logbook — a record of completed work.
Analogy: A completed checkboxes list in a home renovation project binder. Each checkbox represents a task (migration). Completed tasks are checked off. When a new contractor (app instance) starts work, they look at the binder — anything checked off is already done. Only unchecked items need attention.
Example:
```sql
-- orders.schema_migrations (created automatically by the migrate library):
-- | version | dirty |
-- | 1       | false |   ← migration 0001 applied successfully
-- | 2       | false |   ← migration 0002 applied successfully
-- dirty=true would mean a migration started but didn't finish cleanly
```
Don't Confuse With: Your own migration files — `schema_migrations` is the LIBRARY's tracking table. Your migration files (`0001_init_orders.up.sql`) are the actual SQL to run. The library uses `schema_migrations` to know which of YOUR files have already been run.

---

**Term: Sequential Numbering (for migrations)**
First-Principles Origin: Teams needed a way to guarantee that migration files always execute in the same deterministic order on every machine, regardless of when they were created or merged.
Meaning: Using a fixed-length number prefix (`0001`, `0002`, `0003`) to name migration files. The number determines the order of execution. The next migration always gets the next number in the sequence.
Analogy: Pages in a recipe book. Page 1 is always step 1. Page 2 is always step 2. When a new recipe is added to the book, it gets the next page number — pages don't reorder themselves based on when they were added.
Example:
```
Correct (sequential, deterministic):
0001_init.up.sql → 0002_add_index.up.sql → 0003_add_column.up.sql
Always runs in this exact order on every machine.

Problematic (timestamp, order depends on creation time):
20260218_init.up.sql → 20260219_add_index.up.sql → 20260219_add_column.up.sql
If two migrations have the same timestamp prefix, order may be ambiguous.
```
Don't Confuse With: Timestamp-based naming — both are valid strategies. Sequential numbers guarantee strict, explicit ordering at the cost of needing to rename when parallel PRs conflict. Timestamps avoid renaming but can produce ordering surprises in production.

---

## SECTION 8 — RETENTION & REVISION PLAN

### a) The "Right Now" Rule — Do This Before Closing the Page

1. **Write from memory:** Write a complete, correct migration file for `0001_init_orders.up.sql` — starting from `BEGIN;`, including the `CREATE SCHEMA`, the `CREATE TABLE orders.customers`, and ending with `COMMIT;`. While writing each line, state out loud which axiom it relies on.
2. **Reconstruct the `go:embed` block:** Without looking, write the two-line declaration (`//go:embed` directive + `var embedMigrations embed.FS`). State: at which step in the build process do the files get baked in? What type is `embedMigrations`? Can you write to it at runtime?
3. **Explain out loud the "two places" fact:** Why does `CREATE SCHEMA IF NOT EXISTS orders` appear in BOTH `common/migrations.go` (Go code) AND `0001_init_orders.up.sql` (SQL file)? State the axiom and the specific tool each location serves.
4. **Debug this broken migration on paper:**
   ```sql
   CREATE TABLE orders.customers (id UUID, name TEXT);
   ALTER TABLE orders.customers ADD COLUMN email TEXT NOT NULL;
   ```
   The `ALTER TABLE` fails because NOT NULL requires a default for existing rows. Which migration rules does this violate? What is the database state after the failure?

---

### b) 3-Day Revision Checklist (Axiom-Level Mastery)

- [ ] I can write a complete, correct migration file from memory (BEGIN, CREATE SCHEMA IF NOT EXISTS, CREATE TABLE with schema prefix, COMMIT), state all 5 migration rule axioms, and explain what problem SQL migrations solve — all without notes.
- [ ] I can write the `//go:embed` directive and `var embedMigrations embed.FS` declaration, explain that embedding happens at compile time (not runtime), state that `embed.FS` is read-only, and describe what happens when a new `.sql` file is added — all without notes.
- [ ] I can explain PostgreSQL schemas (namespace, same table name in two schemas = different tables), state WHY `CREATE SCHEMA IF NOT EXISTS` appears twice (migrate needs it in Go code; sqlc needs it in migration file), and give an example of a full table reference with schema prefix — all without notes.

---

### c) 7-Day Challenge

**Scenario:** You're adding database support to a new `payments` module. It needs:
- A PostgreSQL schema: `payments`
- A `payment_transactions` table: `id UUID PRIMARY KEY`, `order_id UUID NOT NULL`, `amount_cents BIGINT NOT NULL`, `currency TEXT NOT NULL`, `status TEXT NOT NULL`, `created_at TIMESTAMPTZ DEFAULT NOW()`

**Starting from problem statement:**
1. What problem does this migration solve? (State it without using the word "migration.")
2. State the 5 axioms BEFORE writing any SQL.
3. Write the migration file from scratch — file path, file name, complete SQL.
4. Write the `//go:embed` directive and variable declaration for the payments module.
5. Write the `MigrateDatabaseUp` call that would go in the payments `Init()` method.

**Success looks like:** A complete, correct migration file that follows all 5 rules, a correct `go:embed` declaration, and a working `Init()` call — all written from memory, all traced back to axioms.

---

### d) 30-Day Connection Bridge

- **The next page (`sqlc`)** will read the migration files you wrote to generate Go query code. `sqlc` reads `0001_init_orders.up.sql` and generates a `SaveCustomer(ctx, customer)` function in Go. The shared axiom: the migration file is the source of truth for the database schema — both `migrate` (runtime) and `sqlc` (compile-time code generation) consume it.
- **Every future module** will follow this same pattern: `//go:embed` + `adapters/db/migrations/` + `MigrateDatabaseUp` in `Init()`. The shared axiom: each module is self-contained — its database structure lives with its code, not in a global migrations folder.
- **In real-world Go backends**, embedded migrations are the standard approach for deploying self-contained binaries. You'll see `//go:embed` used for HTML templates, JSON configs, and SQL files across the Go ecosystem. The shared axiom: compile-time embedding eliminates runtime file dependencies.
- **CI assertions for migrations** (not on this page, but coming) will verify that every code change that modifies migration files also passes migration-specific tests. The shared axiom from the previous page: automated assertions catch drift between what's written and what's expected.
- **The `schema_migrations` table** will appear in your PostgreSQL client when you inspect the database. Understanding it helps you debug "why did this migration not run?" questions in production. The shared axiom: the tracking table IS the state — always check it first when migrations behave unexpectedly.

---

### e) Flashcard Set

**Card 1**
FRONT: Why do SQL migrations exist? What problem did teams face before them?
BACK: Without migrations, database schema changes were applied manually — different developers ran different SQL at different times on different machines. Environments diverged: production had one schema, staging had another, local development had a third. Bugs were non-reproducible. Axiom 1: migrations are versioned, ordered, automated schema changes applied identically in every environment.

**Card 2**
FRONT: Why does `go:embed` exist? What does the binary look like without it?
BACK: Without `go:embed`, the binary depends on SQL files existing at a specific path on the machine where it runs. Deploy the binary without the files → migrations fail → app crashes. With `go:embed` (Axiom 1 of embed), the SQL is baked INTO the binary at compile time. The binary is self-contained: one file, deployable anywhere.

**Card 3**
FRONT: What happens when the app starts for the SECOND time on a database that already has all migrations applied?
BACK: Axiom 3 of SQL Migration: `migrate` checks `schema_migrations`, finds all migrations already applied, returns `migrate.ErrNoChange`. The app treats `ErrNoChange` as "nothing to do" (not an error — see `!errors.Is(err, migrate.ErrNoChange)` in `migrations.go`). Application startup continues immediately. No SQL is executed.

**Card 4**
FRONT: A developer edits `0001_init_orders.up.sql` to fix a typo after it's been applied in production. What happens on next startup?
BACK: Axiom 2 of SQL Migration (immutability). The `migrate` library stores a checksum of each applied migration in `schema_migrations`. The edited file has a different checksum than what's stored. `migrate` detects "dirty" — a migration that was applied but whose file has since changed. It refuses to proceed (application startup fails with a checksum error). Fix: revert the edit; create `0002_fix_typo.up.sql` with a corrective change.

**Card 5**
FRONT: Reconstruct from memory: the `//go:embed` directive and `var embedMigrations embed.FS` declaration as they appear in `orders/module.go`.
BACK:
```go
//go:embed adapters/db/migrations/*.sql
var embedMigrations embed.FS
```
Key facts: (1) No blank line between directive and var — they must be adjacent. (2) Path is RELATIVE to module.go's location (Axiom 5 of embed). (3) `embed.FS` is read-only (Axiom 4 of embed). (4) The glob `*.sql` matches all future `.sql` files automatically.

**Card 6**
FRONT: Why does `CREATE SCHEMA IF NOT EXISTS orders` appear in both Go code (`common/migrations.go`) and the SQL migration file (`0001_init_orders.up.sql`)? Isn't it a duplicate?
BACK: Axiom 4 of PostgreSQL Schemas — both are needed for different reasons. The Go code version runs before any migrations execute, so the `migrate` library can create the `schema_migrations` TABLE in the correct schema (`orders.schema_migrations`). The migration file version is for `sqlc` — the query generator that reads migration files to understand database schema. Both use `IF NOT EXISTS` — running both is idempotent, never a conflict.

**Card 7**
FRONT: A migration file has three SQL statements and NO `BEGIN; ... COMMIT;`. The third statement fails. What is the database state?
BACK: Axiom 3 of Migration Rules (transactions). Without a transaction, statements 1 and 2 committed immediately and permanently. Statement 3 failed. The database has a PARTIAL migration: some structural changes applied, others not. The `migrate` library sets `dirty=true` in `schema_migrations`. Recovery requires manual database cleanup. With `BEGIN; ... COMMIT;`, a failed statement triggers automatic rollback — ALL three changes are undone, database is unchanged, migration can be retried cleanly.

**Card 8**
FRONT: What problem does the Adapters layer solve? Where do migrations live in the project and why?
BACK: Axiom 2 of Adapters — mixing "talk to the database" code with "apply business logic" code makes both harder to change independently. The Adapters layer (`adapters/`) is the physical boundary that separates database communication from business logic. Migrations live at `orders/adapters/db/migrations/` because they are a database concern (defining the structure the database adapter's queries rely on). They belong with the database adapter, not with the business logic (Axiom 5 of Adapters).

**Card 9**
FRONT: Why does each module use its own PostgreSQL schema? What would happen without schemas?
BACK: Axiom 2 of PostgreSQL Schemas — the same table name can exist in multiple schemas without conflict. Without schemas, `orders.customers` and `restaurants.customers` would both try to be named `customers` in the flat `public` schema — one would need to be renamed (e.g., `order_customers`), requiring team-wide naming conventions, preventing table name collision detection, and creating potential data confusion. Schemas give each module its own namespace — clear ownership, zero collision risk (Axiom 3: each module's name IS its schema name).

**Card 10**
FRONT: What problem does this solve, and which axiom does it rely on? `if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) { return err }`
BACK: This pattern prevents the "no new migrations" case from being treated as an error. When all migrations have already been applied, `m.Up()` returns `migrate.ErrNoChange` — a sentinel value that means "nothing to do." Without the `!errors.Is(err, migrate.ErrNoChange)` check, the application startup would fail every time after the first run (treating "nothing to do" as an error). SQL Migration Axiom 3 (each migration runs once) means "no new migrations" is the NORMAL steady state — not an error condition.

---

## SECTION 9 — QUICK REFERENCE CHEAT SHEET

### Migration File — Template

```sql
-- File: backend/orders/adapters/db/migrations/0001_init_orders.up.sql
-- Naming: [4-digit number]_[descriptive_name].up.sql

BEGIN;   -- ALWAYS wrap in a transaction (Migration Rule)

CREATE SCHEMA IF NOT EXISTS orders;    -- Module's PostgreSQL namespace (for sqlc)

CREATE TABLE IF NOT EXISTS orders.customers (
    id          UUID        PRIMARY KEY,    -- UUID from common.NewUUIDv7()
    name        TEXT        NOT NULL,
    email       TEXT        NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

COMMIT;  -- Commit all statements atomically
```

---

### `go:embed` — Key Syntax

```go
// In backend/orders/module.go:
import "embed"   // Required — even if no functions from embed are used

//go:embed adapters/db/migrations/*.sql   // No blank line before the var!
var embedMigrations embed.FS              // Read-only embedded filesystem
```

**Key rules:**
- No blank line between `//go:embed` and `var` — they must be adjacent
- Path is relative to the file containing the directive
- `*.sql` glob matches ALL `.sql` files — new files are embedded automatically on next build
- `embed.FS` is READ-ONLY — no runtime writes possible
- Embedding happens at COMPILE TIME — not at runtime

---

### `MigrateDatabaseUp` Call

```go
// In orders/module.go Init():
if err := common.MigrateDatabaseUp(
    ctx,
    string(m.Name()),          // "orders" — becomes the PostgreSQL schema name
    m.pgxDb,                   // Connection pool
    embedMigrations,           // The embedded .sql files
    "adapters/db/migrations",  // Path prefix within the embedded FS
); err != nil {
    return err
}
```

---

### Migration Rules — Summary

| Rule | Why |
|---|---|
| Sequential numbers (0001, 0002, ...) | Deterministic, globally agreed execution order (Axiom 1) |
| Never edit a merged migration | Checksum mismatch — migrate refuses to run; environments diverge (Axiom 2) |
| Always wrap in `BEGIN; ... COMMIT;` | Atomic execution — failure leaves database unchanged (Axiom 3) |
| Up migrations only | Down migrations risky in production; fix forward instead (Axiom 4) |
| Use `IF NOT EXISTS` | Idempotent — safe to run multiple times without error (Axiom 5) |

---

### PostgreSQL Schema Key Facts

| Concept | Details |
|---|---|
| Schema = namespace | `orders.customers` ≠ `restaurants.customers` — completely different tables |
| Schema = module name | `orders` module → `orders` schema; `restaurants` module → `restaurants` schema |
| `CREATE SCHEMA` twice | Go code: for `migrate`'s tracking table | Migration file: for `sqlc`'s schema awareness |
| Schema in table names | Always use `schema.table` format: `orders.customers`, not just `customers` |
| Cross-schema queries | Possible with explicit prefix: `SELECT * FROM orders.customers JOIN restaurants.menus` |

---

### One-Line Plain-English Reminders

- **SQL Migration:** "A numbered SQL file that changes the database structure — applied once, in order, automatically on startup."
- **`schema_migrations` table:** "The library's logbook — records which migrations have already run so they're never repeated."
- **`go:embed`:** "Bake files into the binary at compile time — so the binary runs anywhere without needing files on disk."
- **`embed.FS`:** "Read-only snapshot of files, frozen at build time — lives inside the binary."
- **Adapters layer:** "All code that talks to external systems (database, APIs) lives here — never in business logic."
- **PostgreSQL schema:** "A namespace inside the database — each module gets its own, named after itself."
- **`BEGIN; ... COMMIT;`:** "All-or-nothing — all statements succeed together or all fail together."
- **Never edit a merged migration:** "It's in the logbook. Change the logbook = confusion everywhere. Create a new entry instead."
