# Study Guide: Initial Project Setup — Taskfile & Docker Compose

> **Page:** "Initial Project" (Taskfile, running locally with Docker Compose, live reload)
> **Track:** backend-masterclass-beta, module `02-project-setup`, exercise `01-init`
> **Previous guides:** `1`–`5` in this folder belong to a prior/reset curriculum (the repo shows a `deleted` / `initialize backend-masterclass-beta` commit boundary). This guide starts a fresh continuity chain for the current track — there is no direct predecessor to link to yet.

---

## 1. Snapshot

- **Topic:** How the project's local dev environment is wired — `Taskfile.yml` as a command runner, Docker Compose for running the backend + Postgres, and `reflex`-based live reload.
- **Objective:** After this page, I can explain what `task up` actually does end-to-end, what each Docker Compose service/volume is for, and how a saved `.go`/`.mod`/`.sql` file gets from disk to a restarted process without me doing anything.
- **Continuity:** This is the first page of the current track. It unlocks everything after it — every future exercise assumes `task up` gives you a running backend + DB, so this page is infrastructure, not a one-off topic.

## 2. Concepts

### Task / Taskfile

- **Problem:** [From page] Without a task runner, every contributor has to remember and correctly type raw commands (`docker compose up`, `go test ./... -run X`, `golangci-lint run ./...`, `go mod tidy`, `pgcli postgresql://...`). That's error-prone and undiscoverable — a new contributor has no single place to look.
- **Mechanism:** [From code] `Taskfile.yml` (`project/Taskfile.yml`) declares named tasks that wrap shell commands, e.g.:

  ```yaml
  up:
    cmds:
      - docker compose up {{.CLI_ARGS}}
    env:
      COMPOSE_MENU: "false"

  up-clean:
    cmds:
      - task: down
        vars:
          CLI_ARGS: ""
      - task: up
  ```
  `up-clean` composes two other tasks (`down` then `up`) rather than duplicating shell logic — this is Task's built-in task-dependency feature. `{{.CLI_ARGS}}` is a Task template variable that forwards extra CLI args through to the wrapped command.
- **Design Decision:** [AI explanation] The alternative is a Makefile (same idea, `make up` instead of `task up`) or raw shell scripts in a `scripts/` folder. Task's tradeoff is an extra dependency (must be installed, or you fall back to typing the raw command per the page's tip) in exchange for YAML syntax, cross-platform behavior (Makefiles are POSIX-shell-flavored and get awkward on Windows), and first-class task composition (`up-clean` calling `down` then `up`). It would be the wrong choice if the team already has heavy Makefile tooling elsewhere and doesn't want two task-runner conventions in the same repo.
- **In Production:** [AI explanation] This is a dev-ergonomics tool, not a production runtime concern — it doesn't ship. The "failure mode" is entirely local: a missing `task` binary, or a task alias drifting from what CI actually runs (e.g., CI calls `go test ./...` directly while `task test` wraps something slightly different), which quietly lets a local "task test" pass while CI's actual invocation fails.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Letting `Taskfile.yml` tasks drift from what CI actually runs | CI fails on things `task test` never caught locally | Task definitions and CI config are edited independently over time | Have CI call the same `task <name>` invocations, not parallel raw commands |
  | Treating `task` as mandatory | New contributor blocked entirely if they can't install Task | Forgetting the tool is a convenience wrapper, not the source of truth | Document (as this page does) that the underlying raw command always works |

### Local Dev Stack via Docker Compose

- **Problem:** [From page] Without Docker Compose, running the training locally would mean manually installing and managing a matching Postgres version, plus running the Go binary yourself, and keeping both in sync across machines.
- **Mechanism:** [From page + code] `task up` runs `docker compose up`, which starts two services (`project/docker-compose.yaml`):

  ```yaml
  services:
    backend:
      build:
        context: .
        dockerfile: backend/Dockerfile
      ports:
        - 8080:8080
      environment:
        POSTGRES_URL: "postgres://user:password@postgres:5432/eats?sslmode=disable"
      depends_on:
        postgres:
          condition: service_healthy
      volumes:
        - ./:/src
        - go_pkg:/go/pkg
        - go_cache:/go-cache

    postgres:
      image: postgres:17.6-alpine3.22
      environment:
        POSTGRES_USER: user
        POSTGRES_PASSWORD: password
        POSTGRES_DB: eats
      healthcheck:
        test: pg_isready -U user -d eats
        interval: 1s
        retries: 120
        start_period: 2s
      ports:
        - "5432:5432"

  volumes:
    go_pkg:
    go_cache:
  ```
  `depends_on: condition: service_healthy` means `backend` won't start until Postgres's `healthcheck` (`pg_isready`) passes — not just "container started," but "actually accepting connections." `POSTGRES_URL` inside the container points at hostname `postgres` (the Compose service name, resolved via Docker's internal DNS), which is why it differs from the `POSTGRES_URL` in `Taskfile.yml`'s `env:` block, which uses `localhost` (for host-machine tools like `pgcli` run outside the container).
- **Design Decision:** [AI explanation] The alternative is a devcontainer/Nix-based reproducible shell, or requiring a locally-installed Postgres. Compose's tradeoff: it needs Docker, and every container restart pays image-pull/build cost, but in exchange you get an isolated, disposable, version-pinned Postgres (`17.6-alpine3.22`) that can't drift from what CI or a teammate uses. It would be the wrong choice for a single-binary CLI tool with no external dependencies — Compose orchestration would be pure overhead there.
- **In Production:** [AI explanation] This compose file is dev-only (note `restart: unless-stopped` and bind-mounting the whole repo into `/src` — you'd never bind-mount source in production). In a real incident, you'd see this fail as "backend container restart-looping" if Postgres's healthcheck never passes (bad credentials, DB not accepting connections in time) — visible via `docker compose ps` showing `postgres` stuck `starting` past its `retries * interval` budget (120 × 1s ≈ 2 minutes here).
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Using `localhost` for `POSTGRES_URL` inside the `backend` container | Connection refused — `localhost` inside a container is the container itself, not the host or the `postgres` service | Confusing host-machine networking with Docker's per-container network namespace | Use the Compose service name (`postgres`) as the hostname inside container-to-container config |
  | Dropping `depends_on: condition: service_healthy` in favor of plain `depends_on: [postgres]` | Backend starts before Postgres accepts connections, first request/migration fails intermittently | Plain `depends_on` only waits for container start, not readiness | Always gate on a real healthcheck for stateful dependencies |
  | Running `task down-volumes` when you meant `task down` | Postgres data wiped (`down -v` removes named volumes) | The two tasks look similar and the volume flag is easy to miss | Read the Taskfile task name carefully — `down-volumes` is explicitly destructive |

### Live Reload with `reflex`

- **Problem:** [From page] Without auto-reload, every code change requires manually stopping and restarting the container/process — slow feedback loop, exactly what the page opens by warning against.
- **Mechanism:** [From code] The backend image (`backend/Dockerfile`) installs `reflex` and runs it as the container's entrypoint:

  ```dockerfile
  FROM golang:1.26-alpine3.22
  WORKDIR /src/backend
  RUN go install github.com/cespare/reflex@v0.3.1
  CMD ["reflex", "-c", "reflex.conf"]
  ```
  `reflex.conf` (`backend/reflex.conf`) drives it:

  ```
  -r '\.(go|mod|sql)$' -R '_test\.go$' -R '^(vendor|testdata)/' -s -- sh -c 'go run ./cmd/'
  ```
  `-r` is the watch regex (`.go`, `.mod`, `.sql` files — [From page] this is exactly why `.sql` migration files are watched too, they're part of the same reload trigger set); `-R` are exclude regexes (test files, `vendor/`, `testdata/`); `-s` means restart the previous run instead of letting multiple overlap; the command re-executed on every matching change is `go run ./cmd/`. The `./:/src` bind mount in `docker-compose.yaml` is what makes this work at all — edits made on the host filesystem appear inside the container instantly, where `reflex` is watching.
- **Design Decision:** [AI explanation] The alternative is `air` (a similarly popular Go live-reload tool) or no reload at all (manual `docker compose restart backend`). `reflex`'s tradeoff here is generic regex-based watching (simple, but you must get the include/exclude regexes right yourself) versus `air`'s more Go-specific config. Skipping live reload entirely would be the wrong choice for exactly the reason the page states: it's a rapid-iteration training exercise, and slow restarts compound over hundreds of small edits.
- **In Production:** [AI explanation] Never ships to production — it's a dev-loop tool. If it "fails" locally, symptoms are: saving a file and seeing stale behavior (regex didn't match the changed file — e.g. editing a `.yaml` config wouldn't retrigger it, since `.yaml` isn't in the watch list), or seeing two overlapping processes if `-s` were removed.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Editing a file type not in `reflex.conf`'s `-r` pattern and expecting a reload | Silently stale running process, confusing debugging session | Watch regex only covers `.go`, `.mod`, `.sql` | Know the watch list; restart manually for anything else, or extend the regex |
  | Not bind-mounting source (`./:/src`) | `reflex` inside the container never sees host edits at all | Assuming the container filesystem is the same as the host's | Keep the bind mount for dev; it's intentionally different from a production image which would `COPY` source in instead |

## 3. Plumbing Dissection

**Wiring map:**

```
task up  (Taskfile.yml)
   └─ docker compose up
        ├─ postgres service  → image postgres:17.6-alpine3.22, healthcheck pg_isready
        │                        (must report healthy before backend starts)
        └─ backend service   → builds backend/Dockerfile
                                  ├─ installs reflex
                                  ├─ bind-mounts repo root → /src
                                  └─ CMD: reflex -c reflex.conf
                                        └─ on watched-file change: `go run ./cmd/`
                                              └─ backend/cmd/main.go : func main()
```

- **Per plumbing piece:**
  - `Taskfile.yml` → just a thin alias layer; nothing to break here beyond a missing `task` binary (the page explicitly says it's optional).
  - `docker-compose.yaml`'s `depends_on.condition: service_healthy` → without it, `backend` would race Postgres startup; the design choice embedded here is "correctness over speed" — it's fine to wait up to ~2 minutes (120 retries × 1s) for Postgres rather than fail fast, because in dev, waiting is cheap and flaky startup races are expensive to debug.
  - `go_pkg` / `go_cache` named volumes → without these, every rebuild/rerun would re-download Go modules and lose the build cache, since the module cache and build cache would otherwise live inside the ephemeral container filesystem (or get invalidated each time `go.mod`/`.go` files change and reflex reruns `go run`). This is a caching design choice specific to the fast-iteration goal stated on the page.
  - `backend/Dockerfile`'s `CMD ["reflex", "-c", "reflex.conf"]` → without this, the container would need a fixed `CMD ["go", "run", "./cmd/"]`, which works once but never picks up subsequent edits — reflex is the entire mechanism that turns a static container into a live-reload dev loop.
  - `reflex.conf`'s exclude patterns (`-R '_test\.go$'`) → without this, saving a test file would restart the running server unnecessarily (tests don't affect runtime behavior of `main`), wasting the exact iteration time the page says this setup is designed to save.
  - `backend/cmd/main.go` → currently a placeholder (`fmt.Println("Hello, World!")`); [From code] this confirms the exercise (`02-project-setup/01-init`) is genuinely the first scaffold — there's no HTTP server or DB wiring here yet for this plumbing to connect to.

- **Non-obvious lines:**
  - `condition: service_healthy` under `backend.depends_on.postgres` — easy to skim as "backend depends on postgres" without noticing it's gated on the healthcheck outcome specifically, not just container-started.
  - `-s` in `reflex.conf` — a single flag easy to miss, but it's what prevents overlapping/duplicate `go run` processes on rapid saves.
  - `go_pkg:/go/pkg` and `go_cache:/go-cache` volume mounts — easy to read as "just more source mounts" when they're actually caches with a completely different lifecycle (`task down-volumes` wipes them; `task down` doesn't).

- **What a senior notices:**
  - The separation between the `localhost`-based `POSTGRES_URL` in `Taskfile.yml` (for host tools like `pgcli`) and the `postgres`-hostname one in `docker-compose.yaml` (for container-to-container) is a small but real "two networks" detail that trips people up constantly; it's handled correctly here but not called out anywhere in config — worth a comment if this repo grows past a training project.
  - The healthcheck retry budget (`120 retries * 1s interval` ≈ 2 minutes) is generous for local dev; at scale/CI this exact pattern is fine, but a senior would flag that this number is currently untuned — nobody has had to think about whether 2 minutes is actually the right timeout versus just "generous enough that it never matters."
  - The Dockerfile has no multi-stage build and no distinction between a dev image (this one, with `reflex`) and a prod image — reasonable for a training repo where only local dev matters, but it's the kind of thing you'd deliberately split before this became a real service.

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a blank scratch Go module, rebuild a minimal version of this dev loop:
- A `Dockerfile` that installs and runs `reflex` against a Go entrypoint.
- A `docker-compose.yaml` with one Go service and one Postgres service, where the Go service only starts after Postgres reports healthy (not merely "started").
- A bind mount so edits to a host `.go` file are visible inside the running container.
- A `reflex.conf` that reruns `go run ./cmd/` on `.go` changes but ignores `_test.go` files.

**Acceptance criteria:**
1. Editing and saving `main.go` while the stack is running causes new output to appear without you running any command yourself.
2. Editing a `_test.go` file does **not** trigger a restart.
3. Killing/misconfiguring the Postgres container (e.g., wrong `POSTGRES_PASSWORD` so `pg_isready` never succeeds) keeps the Go service from starting at all — verify via `docker compose ps` showing it never leaves `created`/waiting state.
4. Removing the source bind mount causes edits to stop having any effect (proves the mount, not magic, is what makes reload work).
5. Stopping the stack and removing only non-volume state (`down`, not `down -v`) preserves Postgres data on the next `up`; using `down -v` does not.

<details><summary>Hint</summary>Start with just the Postgres service and its healthcheck — get `pg_isready` returning healthy before adding the Go service at all.</details>
<details><summary>Hint</summary><code>depends_on</code> needs the long-form map syntax (<code>condition: service_healthy</code>), not the short list syntax, to gate on health rather than just container start.</details>
<details><summary>Hint</summary>reflex's <code>-r</code>/<code>-R</code> flags are both regexes matched against the changed file path — test yours against a string like <code>cmd/main_test.go</code> before wiring it into Docker.</details>

**Compare step:** After finishing, diff against `project/docker-compose.yaml`, `project/backend/Dockerfile`, and `project/backend/reflex.conf`, and ask yourself:
1. Did I gate readiness on the same signal (`pg_isready`) or something weaker (e.g., a fixed sleep)?
2. Did I separate the module/build cache from the source bind mount, or did I let Go re-download dependencies on every container rebuild?
3. Does my watch regex correctly exclude test files and `vendor/`/`testdata/`, or did I only handle the happy path?

## 5. Before the Exercise

- **Likely task:** [AI explanation — a guess, not derived from the exercise itself] Given the scaffold (`main.go` is a placeholder, module name `02-project-setup/01-init`), the exercise likely asks you to get the stack running via `task up` (or raw `docker compose up`) and confirm the backend container builds, starts after Postgres is healthy, and reflex's live reload actually restarts the process on a save.
- **Approach:**
  1. Run `task up` (or `docker compose up` if Task isn't installed) and watch the prefixed log output for both services.
  2. Confirm Postgres reports healthy before the backend container starts (per `docker compose ps` or the log ordering).
  3. Make a trivial edit to `backend/cmd/main.go` and save it; watch the logs to confirm `reflex` detects the change and reruns `go run ./cmd/`.
  4. Use `task pgcli` (or `psql`) to confirm you can reach the `eats` database directly from the host.
  5. Use `task down` vs `task down-volumes` deliberately at least once each, to feel the difference before you rely on either.

## 6. Recall Questions

1. Why does `docker-compose.yaml` use `postgres` as the hostname in `backend`'s `POSTGRES_URL`, but `Taskfile.yml`'s `pgcli` task uses `localhost`?
   <details><summary>Answer</summary>They're resolved from two different network namespaces. Inside the `backend` container, Docker Compose's internal DNS resolves service names, so `postgres` reaches the Postgres container directly. `pgcli` in the `Taskfile.yml` task runs on the host machine (not inside a container), where the only route to the exposed Postgres port is via `localhost:5432`, thanks to the `ports: ["5432:5432"]` mapping.</details>

2. What would happen if `depends_on.postgres.condition` were changed from `service_healthy` to the default (just `depends_on: [postgres]`)?
   <details><summary>Answer</summary>The backend container would start as soon as the Postgres *container* starts, not when Postgres is actually ready to accept connections. Since Postgres takes a moment to initialize before `pg_isready` succeeds, the backend could attempt a DB connection during that window and fail — an intermittent race rather than a deterministic failure, which is worse to debug.</details>

3. If you deleted the `go_pkg` and `go_cache` volumes from `docker-compose.yaml` (and the corresponding mounts), what would you observe on the next few `reflex`-triggered reruns, and why?
   <details><summary>Answer</summary>Each `go run ./cmd/` invocation would potentially need to re-resolve/rebuild dependencies from scratch instead of reusing a persisted module and build cache, since that cache would no longer survive between container restarts — it would live only inside the ephemeral container filesystem. You'd see slower reload cycles, directly undermining the fast-feedback goal the page opens with.</details>

4. Why does `reflex.conf` explicitly exclude `_test\.go$` files from its watch pattern instead of just watching `\.go$` for everything?
   <details><summary>Answer</summary>Because saving a test file doesn't change the running server's behavior (`go run ./cmd/` doesn't execute tests) — reloading on a test-file save would restart the process for no behavioral reason, just wasted iteration time. The exclude is a deliberate optimization matching the "don't restart unless it matters" goal of live reload.</details>

5. What is the practical difference between `task down` and `task down-volumes`, and which one would you use to reset your local Postgres state versus just stopping the stack for the night?
   <details><summary>Answer</summary>`task down` runs `docker compose down` — stops and removes containers/networks but leaves named volumes (`go_pkg`, `go_cache`, and Postgres's own data volume) intact. `task down-volumes` runs `docker compose down -v`, which additionally deletes named volumes — including your Postgres data. Use plain `down` to just stop for the night (keep your data); use `down-volumes` only when you deliberately want a clean-slate database.</details>

*(These questions, with answers, have also been appended to `notes/review_queue.md` under today's date.)*

## 7. Cheat Sheet

- `task up` → `docker compose up`; `task up-clean` → `down` then `up` (fresh state). Use `up-clean` when you suspect stale container state; avoid it if you specifically want to keep Postgres data.
- `task down` (keeps volumes) vs `task down-volumes` (`docker compose down -v`, wipes them) — pick based on whether you want to keep the Postgres data.
- Compose health gating: `depends_on: { <service>: { condition: service_healthy } }` — use whenever a dependency needs to be *ready*, not just *started*; avoid plain `depends_on` for anything stateful.
- `reflex -r '<include-regex>' -R '<exclude-regex>' -s -- <command>` — `-s` serializes restarts (no overlap); use broad include regexes plus explicit test/vendor excludes, not the other way around.
- Gotcha: container-internal hostnames (Compose service names) and host-machine `localhost` are not interchangeable — pick based on where the calling process actually runs.
- Gotcha: bind-mounting source (`./:/src`) is a dev-only pattern; it's exactly what a production Dockerfile would replace with a `COPY`.
