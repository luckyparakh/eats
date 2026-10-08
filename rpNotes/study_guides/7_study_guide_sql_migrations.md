# Study Guide: SQL Migrations

> **Page:** "SQL Migrations" (versioned schema changes via the `migrate` library, `go:embed`, the Adapters layer, per-module PostgreSQL schemas, and migration safety rules)
> **Track:** backend-masterclass-beta, module `04-database`, exercise `01-add-migrations` (confirmed via `.tdl-exercise`, updated since this guide was first drafted — was previously read ahead of unlock, while `.tdl-exercise` still showed `03-http` / `03-gitattributes`).
> **Previous guide:** `6_study_guide_gitattributes_generated_files.md` — that page was about version-control hygiene for *generated code*. This page shifts to a different kind of versioned, incremental change: *database schema* changes.
> **What this likely unlocks next:** per the page itself, `sqlc` for writing actual queries against the tables this migration creates — the page explicitly defers "how `sqlc` works" and "why the schema needs `CREATE SCHEMA IF NOT EXISTS` twice" follow-up to "the next module."
>
> **Workspace state (refreshed, checked directly — not from `exercise.md`):** The exercise scaffold has landed. `backend/orders/module.go` now has `//go:embed adapters/db/migrations/*.sql` / `var embedMigrations embed.FS` adjacent to each other, and `Init()` calls `common.MigrateDatabaseUp(ctx, string(m.Name()), m.pgxDb, embedMigrations, "adapters/db/migrations")` exactly as the page describes. `backend/common/migrations.go` now exists and implements `MigrateDatabaseUp` — confirmed it: creates the schema via `CREATE SCHEMA IF NOT EXISTS <moduleName>` before configuring `migrate`, wires up `iofs.New(fs, migrationsDir)` as the migration source and `pgxMigrate.WithInstance(...)` (via `github.com/golang-migrate/migrate/v4/database/pgx/v5`) as the destination, and calls `m.Up()` treating `migrate.ErrNoChange` as success. It also does two things beyond what the page mentioned: releases the dedicated pgx connection `pgxMigrate.WithInstance` acquires via a `defer m.Close()` (avoiding a pool leak), and watches `ctx.Done()` in a goroutine to trigger `m.GracefulStop` on interrupt. `go.mod` now includes `github.com/golang-migrate/migrate/v4 v4.19.1`. `backend/orders/adapters/db/migrations/0001_init_orders.up.sql` now exists as a file — **its contents weren't inspected**, since that's the exercise's actual deliverable and reading a student's in-progress solution ahead of them describing it isn't part of this refresh. `project/docker-compose.yaml` (`postgres:17.6-alpine3.22`, port `5432`, db/user/password all `eats`/`user`/`password`) and `Taskfile.yml`'s `up-clean`/`pgcli` tasks are unchanged from the last check.

---

## 1. Snapshot

- **Topic:** How this project applies versioned, ordered SQL schema changes automatically on startup — using the `migrate` library, files embedded into the binary via `go:embed`, one PostgreSQL schema per module, and a fixed set of safety rules (sequential numbering, up-only, explicit transactions).
- **Objective:** After this page, I can explain why migrations must be ordered and applied exactly once, why the SQL files get compiled into the binary instead of shipped alongside it, why `orders.customers` and a hypothetical `restaurants.customers` don't collide, and why `CREATE SCHEMA IF NOT EXISTS orders` has to appear in two different places for two different reasons.
- **Continuity:** The `RegisterCustomer` HTTP handler (built across the last several pages) currently validates a request and does nothing with it. This page is the first step toward actually persisting that data — before you can `INSERT`, the table has to exist, and migrations are how it comes to exist.

## 2. Concepts

### Versioned Migrations & the `migrate` Library

- **Problem:** [From page] Without a system for this, schema changes happen as ad-hoc manual SQL run by whoever's touching the database that day — different environments (a teammate's laptop, CI, production) drift out of sync with each other, and nobody has a reliable record of what structure any given environment actually has.
- **Mechanism:** [From page] Each migration is a numbered `.sql` file, applied strictly in order. Locally, *all* migrations run from the first one on startup. In production, only migrations that haven't been applied yet run — every environment converges on the same schema state without anyone running manual scripts. The project uses the `migrate` library specifically because it's widely adopted, has a minimal API, and works well with `embed.FS` (see next concept).
- **Design Decision:** [From page] Migrations live at `backend/orders/adapters/db/migrations/` — nested under the module rather than in one repo-wide directory. The page is explicit that this is deliberate for a *modular monolith*, where each module (potentially owned by a different team) needs its migrations isolated from every other module's. It also explicitly says a single top-level directory is a fine choice for smaller, single-schema projects — this isn't a universal rule, it's a fit for this project's architecture.
- **In Production:** [AI explanation] The "run everything locally, only-new in prod" split matters operationally: a fresh clone/CI run starts from zero and needs the full history to reach the current schema, while a long-running production database only ever needs the delta since its last deploy — replaying already-applied migrations there would be redundant at best and, for non-idempotent statements, actively wrong.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Running schema SQL directly against a database instead of writing a migration | That environment's schema is now undocumented and unreproducible elsewhere | Feels faster for a one-off fix | Always express schema changes as a new migration file, even for something trivial |
  | Putting all modules' migrations in one shared top-level directory in a modular monolith meant to scale to multiple teams | Cross-module coupling — one team's migration changes touch a directory another team also owns | Looks simpler at first, and *is* fine for small/single-schema projects | Nest migrations per module (`<module>/adapters/db/migrations/`) once you expect module ownership to diverge |

### `go:embed` — Migrations Baked Into the Binary

- **Problem:** [From page] Migration `.sql` files are separate files on disk during development. Without embedding them, deploying the app means also deploying the migrations directory alongside the binary, in the right relative location, every time — an extra thing that can go missing or get out of sync.
- **Mechanism:** [From page] `//go:embed adapters/db/migrations/*.sql` above `var embedMigrations embed.FS` in `backend/orders/module.go` tells the Go compiler to bundle every matching `.sql` file into the compiled binary at build time. At runtime, `embedMigrations` behaves as a read-only filesystem — reading from it reads bytes baked in at compile time, not from disk. `Init()` passes `embedMigrations` into `common.MigrateDatabaseUp(ctx, string(m.Name()), m.pgxDb, embedMigrations, "adapters/db/migrations")`.
- **Design Decision:** [From page] The direct payoff stated on the page: the app ships as a single binary file, with no separate step to copy `.sql` files around. Adding migration `0002_...up.sql` requires no code change — the `*.sql` glob in the existing directive picks it up automatically on the next build.
- **In Production:** [AI explanation] Because embedding happens at compile time, a stale binary can never silently pick up a migration file that was added to disk after the build — the binary's migration set is exactly whatever existed at the moment `go build` ran, which is precisely the self-contained deployability the page is arguing for.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Adding a new `.sql` migration file but forgetting to rebuild the binary before deploying | The new migration silently never runs in that deployment — no error, just missing schema changes | `go:embed` bundling happens at build time, easy to forget when migrations feel like "just data files" | Treat a new migration file as requiring a rebuild, same as any source change — CI should always build fresh |
  | Expecting to add/modify files in `embedMigrations` at runtime | Compile error / not possible — `embed.FS` has no write operations | Assuming `embed.FS` behaves like a normal mutable filesystem | Remember it's a read-only snapshot from compile time; new migrations require a new build, not a runtime write |

### PostgreSQL Schemas — One Namespace Per Module

- **Problem:** [From page] As more modules are added (each potentially with its own `customers`-like table), a single flat namespace means table names collide or require manual disambiguating prefixes (`orders_customers`, `restaurants_customers`) that everyone has to remember and apply consistently.
- **Mechanism:** [From page] A PostgreSQL schema is a namespace inside a database — `orders.customers` is the `customers` table specifically within the `orders` schema. This project gives each module its own schema, so ownership boundaries are explicit and table names never collide across modules; cross-schema joins remain possible when genuinely needed.
- **Design Decision:** [From page] `CREATE SCHEMA IF NOT EXISTS orders` is required in **two** places, for two different consumers, and the page is explicit both are necessary: (1) `MigrateDatabaseUp()` in `backend/common/migrations.go` creates the schema *at runtime, before migrations run*, because `migrate` needs the schema to already exist so it can create its own `schema_migrations` tracking table inside it; (2) the migration file itself needs the same statement because of how `sqlc` works (the page explicitly defers the *why* here to the next module — treat this as a forward reference, not yet explained). Both use `IF NOT EXISTS`, so running both never conflicts.
- **In Production:** [AI explanation] This is a case where the same idempotent statement serves two different tools with two different timing requirements — `migrate`'s tracking table has to exist before any user migration runs, while `sqlc`'s use of the migration file happens independently (likely at code-generation time, not runtime) — so neither location can be dropped in favor of the other without breaking that tool's specific need.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Removing `CREATE SCHEMA IF NOT EXISTS orders` from the migration file since "the Go code already creates it" | Whatever `sqlc` needs the migration file's schema statement for breaks (per the page — exact mechanism not yet explained) | The two statements look redundant at a glance since both are idempotent and target the same schema | Keep both — they serve two different tools/timings, not one duplicated purpose |
  | Creating tables without the module's schema prefix (e.g. plain `CREATE TABLE customers`) | Table lands in the default `public` schema instead of the module's namespace, defeating the isolation | Easy to forget the `orders.` prefix when writing SQL | Always qualify with `<module>.` in `CREATE TABLE` statements |

### Migration Rules — Numbering, Up-Only, and Transactions

- **Problem:** [From page] Migrations are effectively permanent and run against real, persistent data — unlike redeploying application code, a bad migration or an ambiguous execution order can leave production in a state that's hard or risky to reverse.
- **Mechanism:** [From page] Four concrete rules: (1) use sequential numbers (`0001_`, `0002_`, ...), not timestamps — this project explicitly chooses sequential over timestamp-based naming; (2) never edit a migration once merged — create a new one instead; (3) write **up** migrations only, never `down` — "fix forward" with a new migration instead of rolling back; (4) wrap every migration in an explicit `BEGIN`/`COMMIT` transaction, because `migrate` does not auto-wrap files, and Postgres supports transactional schema changes — without it, a multi-statement migration that fails partway leaves the schema partially applied.
- **Design Decision:** [From page] The page directly compares the two migration-numbering strategies rather than presenting sequential numbering as the only option: **timestamps** avoid numbering collisions between parallel PRs but can let two PRs merge in *either* order, risking silent schema drift between production and other environments (unpredictable ordering is the worse failure mode). **Sequential numbers** guarantee deterministic ordering, at the cost of a same-number collision when two PRs add a migration concurrently — resolved with a trivial rename after one PR merges. The page frames the rename as "a minor inconvenience" next to the risk of production ordering unpredictability, which is why this project picks sequential.
- **In Production:** [AI explanation] The up-only/fix-forward rule and the transaction-wrapping rule address different failure axes: fix-forward avoids the data-compatibility risk of reversing a schema change *after* new data has been written under it, while transaction-wrapping avoids a *single* migration leaving the schema in a half-applied state if one of several statements in the same file fails.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Editing a migration file after it has already been applied somewhere (even just locally) | Different environments end up disagreeing about what "migration 0001" actually contains | The file is easy to just fix in place, especially before it's merged/deployed anywhere else | Once applied anywhere, treat it as immutable — express the correction as a new, later-numbered migration |
  | Omitting `BEGIN`/`COMMIT` around a migration with multiple statements, even a "simple" one | A failure partway through leaves some statements committed and others not — an inconsistent, hard-to-diagnose schema state | `migrate` doesn't wrap files in a transaction automatically, so it's easy to assume atomicity that isn't actually there | Wrap every migration file in an explicit transaction as a default habit, even single-statement ones (the page notes it "costs nothing") |

## 3. Plumbing Dissection

> Unlike guide 6, this page's "plumbing" **does not exist in the codebase yet** — verified directly, not assumed:

**Wiring map (refreshed — scaffold has landed, cross-checked against current workspace; migration file's own SQL content intentionally not inspected):**

```
backend/orders/module.go                         (CONFIRMED, current content)
  //go:embed adapters/db/migrations/*.sql
  var embedMigrations embed.FS                    ← adjacent, no blank line, exactly as the page requires
        │
        └─ Init() calls:
              common.MigrateDatabaseUp(ctx, string(m.Name()), m.pgxDb, embedMigrations, "adapters/db/migrations")

backend/common/migrations.go                      (CONFIRMED, exists and implements MigrateDatabaseUp)
  MigrateDatabaseUp(ctx, moduleName, pool, fs, migrationsDir):
        ├─ stdlib.OpenDBFromPool(pool)                  → adapts the pgxpool.Pool to a database/sql *DB
        ├─ iofs.New(fs, migrationsDir)                  → wraps embedMigrations as a migrate source
        ├─ CREATE SCHEMA IF NOT EXISTS <moduleName>      → runtime schema creation, BEFORE migrate configured
        ├─ pgxMigrate.WithInstance(db, {SchemaName, MigrationsTable: "schema_migrations"})
        ├─ migrate.NewWithInstance("iofs", d, "pgx", migDb)
        ├─ defer m.Close()                               → releases the dedicated pgx connection (not mentioned on the page — prevents a pool leak)
        ├─ goroutine watching ctx.Done() → m.GracefulStop  → not mentioned on the page — clean shutdown on interrupt
        └─ m.Up(), treating migrate.ErrNoChange as success

backend/orders/adapters/db/migrations/            (CONFIRMED: directory + 0001_init_orders.up.sql now exist)
  0001_init_orders.up.sql                          ← exists; content not read (this is the exercise's own deliverable)

go.mod                                            (CONFIRMED, refreshed)
  github.com/jackc/pgx/v5 v5.10.0                  ← already present
  github.com/sqlc-dev/sqlc v1.30.0 (indirect/tool) ← already present, unused by orders yet
  github.com/golang-migrate/migrate/v4 v4.19.1     ← NOW present (was missing on first read)

docker-compose.yaml                               (CONFIRMED, unchanged)
  postgres:17.6-alpine3.22, port 5432, db "eats", user/password "user"/"password"

Taskfile.yml                                       (CONFIRMED, unchanged, matches page's tip exactly)
  up-clean  → down, then up  (clean local DB)
  pgcli     → pgcli postgresql://user:password@localhost:5432/eats
```

- **Per plumbing piece:** All of the scaffolding the page describes (`module.go`'s `go:embed`, `common/migrations.go`'s `MigrateDatabaseUp`) is now confirmed present and matches the page's description closely — as anticipated, it arrived with the exercise scaffold rather than needing to be hand-written. The one deliverable left to write (or already written, unverified here) is the migration file's actual SQL content.
- **Non-obvious lines:** `common/migrations.go` does two things beyond what the page's simplified snippet shows: it releases the dedicated connection `pgxMigrate.WithInstance` acquires (`defer m.Close()` — without it, every `MigrateDatabaseUp` call would leak one pool connection) and it wires `ctx.Done()` to `m.GracefulStop` so a shutdown signal mid-migration stops cleanly instead of leaving `migrate` running unattended. Neither detail is mentioned on the page — both are things a senior would notice reading the real implementation that a simplified teaching snippet naturally omits.
- **What a senior notices:** `golang-migrate/migrate` has since been added to `go.mod` (`v4.19.1`) — confirming the earlier open question (whether it'd be added by the exercise scaffold or need to be added manually) resolved itself: it arrived with the scaffold, same as the rest of this wiring.

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a scratch Go project with a local Postgres (or even SQLite, if you want to skip Docker), reproduce the core mechanism this page describes end to end:
- Create a `migrations/` directory with `0001_init.up.sql` that creates a schema and one table, wrapped in `BEGIN`/`COMMIT`.
- Use `//go:embed migrations/*.sql` + `embed.FS` to bundle it, and use the `golang-migrate/migrate` library (with its `iofs` source driver) to apply it against your database.
- Add `0002_...up.sql` that alters the table, and confirm on a second run only `0002` applies (not `0001` again).

**Acceptance criteria:**
1. Deleting the migrations directory from disk after building the binary still lets migrations run successfully — proving the SQL is actually coming from inside the binary, not disk.
2. Running the binary twice against the same database logs "no change" (or equivalent) the second time — confirming `migrate` tracked what already ran.
3. Deliberately break `0002` into two statements without a transaction, force the second to fail (e.g. a bad constraint on data you seed first), and confirm the schema is left partially applied — then fix it by wrapping in `BEGIN`/`COMMIT` and confirm a failure now leaves the schema completely unchanged.
4. Rename `0002` to reuse `0001`'s number and confirm `migrate` either errors or behaves unpredictably — demonstrating why sequential numbers must stay unique and ordered.

<details><summary>Hint</summary>The `golang-migrate/migrate` library's `iofs` source driver (`github.com/golang-migrate/migrate/v4/source/iofs`) is exactly what turns an `embed.FS` into something the `migrate` library can read migrations from — same shape as this project's likely wiring.</details>
<details><summary>Hint</summary>For criterion 1, build the binary, then literally `rm -rf` the migrations source directory before running it — if migrations still apply, embedding is proven, not assumed.</details>
<details><summary>Hint</summary>For criterion 3, an easy way to force a mid-migration failure is a second statement that violates a constraint your first statement's inserted data already breaks (e.g. add a `UNIQUE` constraint on a column that already has duplicate seed values).</details>

**Compare step:** Once the actual exercise scaffold unlocks in this repo, compare your version against the real `backend/common/migrations.go` and `backend/orders/module.go` wiring, and ask:
1. Does the real project's `MigrateDatabaseUp` create the schema at a different point relative to configuring `migrate` than you did?
2. Does your `0001` migration's schema-creation statement match the "needed twice, for two different consumers" reasoning from the page, or did you only create the schema once?

## 5. Before the Exercise

- **Likely task:** [AI explanation — speculation, not derived from `exercise.md`, which hasn't been read] Write `backend/orders/adapters/db/migrations/0001_init_orders.up.sql` creating the `orders` schema and an `orders.customers` table with the columns the page lists (`customer_uuid uuid NOT NULL PRIMARY KEY`, `name varchar(255) NOT NULL`, `email varchar(255) NOT NULL`, `address json NOT NULL`, `phone_number varchar(50) NOT NULL`), wrapped in `BEGIN`/`COMMIT`. Given `common/migrations.go` and the `module.go` wiring don't exist in the workspace yet, the exercise scaffold will likely add that plumbing — but that's inferred from the current gap, not confirmed.
- **Approach:**
  1. Confirm what scaffold code actually ships with the exercise once it unlocks, rather than assuming today's page snippets are all pre-existing.
  2. Write the migration file exactly matching the column table from the page — note `customer_uuid`, not a bare `id`, per the code-review discussion embedded in the page (self-documenting joins, clean `USING (...)` clauses).
  3. Remember both required schema-creation statements if you end up touching `common/migrations.go` too: one at runtime (for `migrate`'s own tracking table), one inside the migration file (for `sqlc`, mechanism TBD next module).
  4. Verify locally with `task up-clean` then `task pgcli` (or another client) against `postgres://user:password@localhost:5432/eats`, remembering `SET search_path TO 'orders';` to query the module's schema directly.

## 6. Recall Questions

1. Why does a fresh/local environment run *every* migration on startup, while production only runs the ones that haven't been applied yet?
   <details><summary>Answer</summary>The `migrate` library tracks which migrations have already run (via its own tracking table, scoped to the module's schema). A fresh database has no record of anything having run, so every migration file is "new" and applies in order. A long-running production database already has a record of everything applied through the last deploy — only the migrations added since then are actually new, so only those run. Both cases follow the same rule ("apply what hasn't run yet, in order"); they just start from different existing states.</details>

2. What specific problem does compiling migration `.sql` files into the binary via `go:embed` solve, versus just deploying the `.sql` files alongside the binary in the right directory?
   <details><summary>Answer</summary>It removes an entire class of deployment mistakes: keeping two things (the binary and a directory of files) in sync, in the correct relative path, on every environment. With `go:embed`, the binary is the single artifact — there's nothing else to copy, nothing else that can go missing or drift out of the expected location.</details>

3. Why does `CREATE SCHEMA IF NOT EXISTS orders` need to appear both in the Go code (`MigrateDatabaseUp`) and inside the migration file itself, rather than just once?
   <details><summary>Answer</summary>They serve two different consumers with two different timing needs: the Go-code call creates the schema at runtime, before `migrate` runs, because `migrate` needs the schema to already exist so it can create its own tracking table inside it. The statement inside the migration file is there because of how `sqlc` works (the page defers the exact mechanism to the next module). Both use `IF NOT EXISTS`, so neither call conflicts with the other — but dropping either one breaks the tool that specifically needs it.</details>

4. The page picks sequential migration numbers over timestamp-based ones for this project. What's the actual tradeoff being made, and why does the project accept the downside of sequential numbering?
   <details><summary>Answer</summary>Timestamp-based numbering avoids same-number collisions when two PRs add migrations concurrently, but if two PRs merge in a different order than their timestamps implied, the migration order becomes unpredictable across environments — production, CI, and a given developer's machine can end up applying migrations in different sequences, causing silent schema drift. Sequential numbers guarantee one deterministic order everywhere, at the cost of an occasional same-number collision between two concurrent PRs — which is resolved with a simple rename after one merges. The project accepts that minor renaming inconvenience specifically to avoid the worse failure mode: unpredictable production ordering.</details>

5. Why does this project write only "up" migrations and never "down" (rollback) migrations?
   <details><summary>Answer</summary>Using down migrations in production is described as tricky and risky — reversing a schema change after new data has already been written under the new schema can be unsafe or lossy. The safer path is "fixing forward": if a migration was wrong, write a new up migration that corrects it, rather than trying to undo the old one. Locally, the page notes it's simpler to just reset the database from scratch than to figure out which version to roll back to — so writing down migrations that would rarely if ever be used isn't worth the effort.</details>

*(These questions, with answers, should also be appended to `notes/review_queue.md` under today's date, per the guides' convention — not yet automated here.)*

## 7. Cheat Sheet

- Migrations are numbered, ordered, applied-once `.sql` files; `migrate` tracks what's already run so fresh environments apply everything and long-running ones only apply what's new.
- `//go:embed adapters/db/migrations/*.sql` + `embed.FS` bakes migration files directly into the compiled binary — no separate files to deploy or keep in sync; adding a new migration just needs a rebuild, nothing else.
- One PostgreSQL schema per module (`orders.customers`, hypothetically `restaurants.customers`) — same table name, different schema, zero collision; cross-schema joins remain possible but aren't the default.
- `CREATE SCHEMA IF NOT EXISTS <module>` is needed in **two** places: at runtime in Go (before `migrate` creates its tracking table) and inside the migration file itself (for `sqlc`, detail deferred to the next module) — both idempotent, neither redundant.
- Sequential numbering > timestamps here specifically because unpredictable production ordering (timestamps' failure mode) is worse than an occasional rename (sequential numbers' failure mode).
- Never edit a merged migration — write a new one. Never write down migrations — fix forward instead. Always wrap a migration in `BEGIN`/`COMMIT`, even a single statement, since `migrate` won't do it for you and a partial failure without one leaves the schema in an inconsistent state.
- Workspace gap to watch: as of this reading, `backend/orders/adapters/db/`, `backend/common/migrations.go`, and the `go:embed`/`MigrateDatabaseUp` wiring in `module.go` do not exist yet — this page previews the target shape, the exercise itself will confirm what's scaffolded vs. what you write.
