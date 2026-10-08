# Study Guide: Gitattributes & Generated Files

> **Page:** "Gitattributes" (why generated `.gen.go` files need special handling in version control — merge conflicts and PR review noise)
> **Track:** backend-masterclass-beta, module `03-http`, exercise `03-gitattributes`
> **Previous guide:** `5_study_guide_custom_openapi_types.md` — that page was about *what* gets generated (custom types via `x-go-type`). This page is entirely about the *consequences* of committing that generated output to Git: merge conflicts and reviewable PRs.
> **What this likely unlocks next:** nothing structurally new in the codebase — this is a process/tooling page, not a new runtime concept. It closes out the "generated code" thread that's run through the last two guides.
>
> **Workspace state:** `project/.gitattributes` already exists, is already committed to git, and matches the page's example exactly. `openapi.gen.go` is confirmed tracked in git (not gitignored). There is **no CI configuration in this repo** (no `.github/workflows`, no `.gitlab-ci.yml`) — the page's "assert nothing changed in CI" recommendation is `[From page]` only; nothing here verifies it's actually enforced anywhere in this project.

---

## 1. Snapshot

- **Topic:** How to handle machine-generated `.gen.go` files in Git — why merge conflicts in generated code should be regenerated rather than hand-resolved, and how `.gitattributes`' `linguist-generated=true` keeps generated output out of PR reviewers' way.
- **Objective:** After this page, I can explain why `.gitattributes` doesn't prevent merge conflicts (a common misconception), state the correct workflow for resolving a conflict in a generated file, and justify why this project commits generated files instead of `.gitignore`-ing and regenerating in CI.
- **Continuity:** Directly follows from the last two guides' subject (`openapi.gen.go`) — this page is about the version-control hygiene around the exact file those guides were dissecting.

## 2. Concepts

### Merge Conflicts in Generated Code

- **Problem:** [From page] Two developers on separate branches both edit the OpenAPI spec and run `go generate ./...`. Both branches now have a fully-rewritten, different version of the same `openapi.gen.go`. Merging surfaces dozens of conflicting lines — in code neither of them actually hand-wrote.
- **Mechanism:** [From page] The prescribed workflow: don't resolve the conflict by hand. Merge the OpenAPI *specs* (the actual source of truth, hand-written and small), then re-run `go generate ./...`, then commit the freshly regenerated output. The generated file is never merged as text — it's regenerated from already-merged inputs.
- **Design Decision:** [From page] The alternative (rejected) is manually resolving each conflicting hunk in the generated file. [AI explanation] Hand-merging generated code is fragile and pointless: the file's content is a deterministic function of the spec, so any manual edit that doesn't also match what `oapi-codegen` would produce from the merged spec is just a new source of drift, waiting to be silently overwritten the next time someone regenerates.
- **In Production:** [AI explanation] The failure mode of hand-merging is invisible until it bites: a manually patched-together generated file might compile and even pass tests, but diverge from what regeneration would produce — meaning the next `task gen` run (for an unrelated change) silently reverts the hand-merge, resurrecting whatever bug it was fixing.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Manually resolving conflicting hunks inside `openapi.gen.go` | The hand-resolved version drifts from what regeneration would produce; a future `task gen` run silently discards the manual fix | The conflict markers look like any other merge conflict, so it's tempting to resolve them the normal way | Merge the `.yaml` spec files, regenerate, commit the fresh output — never hand-edit the conflict markers in a `.gen.go` file |
  | Assuming `.gitattributes` prevents this conflict from happening at all | Confusion when the conflict still shows up despite `linguist-generated=true` being set | `linguist-generated` sounds like it could be a merge strategy | Understand `linguist-generated=true` as a **display/review** hint only — it changes nothing about Git's merge behavior |

### `.gitattributes` and PR Review Noise

- **Problem:** [From page] A PR that changes 15 lines in a spec and 20 lines in a handler also drags in 223 lines of regenerated boilerplate in its diff. Reviewers must scroll past machine output to find the actual change.
- **Mechanism:** [From code, `project/.gitattributes` — confirmed, matches the page exactly]:

  ```
  **/**.gen.go linguist-generated=true
  ```
  `**/**.gen.go` is a glob matching any `.gen.go` file at any directory depth (confirmed: this repo's only such file today is `project/backend/orders/api/http/openapi.gen.go`, which this pattern does match). `linguist-generated=true` is read by GitHub's Linguist (and GitLab's equivalent) to mark the file as machine output: GitHub collapses it by default in the "Files changed" tab of a PR, and excludes it from the repo's language statistics. Reviewers can still expand it manually if they need to check the generated output itself.
- **Design Decision:** [From page] Explicitly stated: "`.gitattributes` doesn't prevent this merge conflict. It still happens. The `linguist-generated` attribute is not a merge strategy." Its entire job is communicating intent to tooling and teammates — "this is machine output, don't review or hand-merge it line by line" — not changing Git's actual merge mechanics.
- **In Production:** [AI explanation] Without this attribute, every PR touching a spec-driven endpoint carries disproportionate diff noise relative to the actual hand-written change, which in a real team setting degrades review quality over time (reviewers skim faster, or skip, when diffs are mostly unreviewable boilerplate) — a social/process cost more than a technical one.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Believing `linguist-generated=true` is a Git merge strategy | Surprise when a `.gen.go` merge conflict still has to be handled (via regeneration) despite the attribute being set | The name and the "resolves conflicts" hope are easy to conflate | Treat it purely as a GitHub/GitLab *review-UI* signal — merge behavior is unaffected |
  | Forgetting the glob needs to match the actual generated-file naming convention | A generated file exists but isn't collapsed/excluded, because its name/path doesn't match the pattern | Copy-pasting a `.gitattributes` line from another project without checking the local naming convention | Confirm the glob (`**/**.gen.go` here) actually matches every generated file's real name and location in *this* repo |

### Why Commit Generated Files (vs. `.gitignore` + Regenerate)

- **Problem:** [From page] There's a real alternative to committing `.gen.go` files: `.gitignore` them and regenerate during CI/build instead. The page argues this is worse for most projects.
- **Mechanism:** [From code, confirmed] `project/.gitignore` does **not** exclude `.gen.go` files, and `openapi.gen.go` is confirmed tracked in git (`git ls-files` lists it) — this project has made the "commit it" choice concretely, not just in prose.
- **Design Decision:** [From page] The stated tradeoffs of the `.gitignore` + regenerate-in-CI alternative: it complicates developer experience (you must always run `go generate` before building locally, or the code simply won't be there), CI jobs take longer (regeneration on every run), and you lose the ability to track how generated code changes over time (no history/diff of the generated output itself). The page's conclusion: "For most projects, committing generated files and marking them with `.gitattributes` is the best approach."
- **In Production:** [AI explanation] Committing generated files means a fresh clone or CI checkout is immediately buildable without a generation step — a real operational simplicity win — at the cost of the exact PR-noise problem `.gitattributes` exists to mitigate. The two concepts (commit generated files, mark them with `.gitattributes`) are a matched pair: one creates the noise, the other manages it.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Committing generated files without also configuring `.gitattributes` | Full PR review noise with none of the mitigation — every generated-code change clutters every diff | `.gitattributes` is an easy step to skip since the project builds/works fine without it | Always pair "commit generated files" with a `linguist-generated=true` (or equivalent) `.gitattributes` entry |
  | `.gitignore`-ing generated files without a reliable, fast regeneration step in CI *and* locally | Fresh checkouts fail to build until someone remembers to run the generator; onboarding friction | Seems cleaner ("no generated code in the repo") without weighing the operational cost | If choosing this path, make regeneration a mandatory, automatic, fast part of both local setup and CI — otherwise prefer committing generated output |

### Asserting No Changes in CI

- **Problem:** [From page] Someone changes the OpenAPI spec but forgets to run `go generate` before committing — the spec and the generated code silently drift apart, and nothing catches it.
- **Mechanism:** [From page — not present in this repo; no CI config exists here] Run the code generators in CI, then assert the working tree is clean afterward — e.g., check that `git status --porcelain` produces empty output. If regenerating produces any diff, someone committed source changes without regenerating, and CI should fail.
- **Design Decision:** [From page] This only works reliably if CI uses the *exact same* generator version everyone uses locally — otherwise a version mismatch could produce spurious diffs (or mask real ones) independent of anyone's actual mistake. The page notes this is now straightforward because of `go tool` support in `go.mod` (covered in the HTTP Handler guide) — the generator version is pinned in the module itself, so CI and every local dev environment resolve to the identical tool version automatically.
- **In Production:** [AI explanation] Without this check, spec/generated-code drift is a silent, deferred bug: the mismatch might not surface until someone else regenerates for an unrelated reason and suddenly sees an unexpected diff, with no easy way to tell which change introduced the drift.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Not asserting regeneration produces zero diff in CI | Spec/code drift goes undetected until it surfaces confusingly later, possibly attributed to the wrong commit | Requires an explicit CI step most starter setups don't include by default | Add a CI step that runs the generator and fails the build if `git status --porcelain` (or equivalent) is non-empty afterward |
  | Using a different generator version in CI than what's pinned locally | False-positive or false-negative diffs unrelated to any real spec/code mismatch | CI environments sometimes install tools independently of the project's own pinned versions | Rely on `go tool`'s `go.mod`-pinned version in both places, rather than a separately-installed CI tool |

### Line Endings & `eol=` Normalization (general reference — not present in this repo's `.gitattributes`)

> The project's actual `project/.gitattributes` (confirmed, one line: `**/**.gen.go linguist-generated=true`) does **not** contain any `text=`/`eol=` rules. Everything below is [AI explanation] from a separate, general discussion about a *different example* `.gitattributes` file (one using `text=auto` and per-extension `eol=crlf`/`eol=lf` rules), kept here for reference since it's the same config file/mechanism, not because this repo uses it.

- **Problem:** [AI explanation] Every line in a text file ends with an invisible byte sequence — LF (`\n`, Unix/macOS) or CRLF (`\r\n`, Windows). Without an enforced rule, a cross-platform team produces a mix depending on each contributor's OS/editor, causing two concrete failures: (1) scripts break — a `.sh` file with CRLF turns the shebang line `#!/bin/bash` into `#!/bin/bash\r`, which the kernel reads as a nonexistent interpreter path (`bad interpreter` error); (2) every line in a diff can show as "changed" purely from an endings mismatch, even with zero real content change, polluting diffs/blame/merges.
- **Mechanism:** [AI explanation] The example file used two mechanisms together:
  - `* text=auto` — a baseline: let Git auto-detect text files and normalize them to LF *inside the repository* (the stored blob), converting to the OS-native ending on checkout unless overridden.
  - Per-extension overrides that force a specific ending regardless of platform or a contributor's local `core.autocrlf`: `*.sh text eol=lf` and `*.go text eol=lf` (Unix scripts/Go toolchain expect LF), vs. `*.cmd`/`*.bat`/`*.ics` `text eol=crlf` (Windows batch scripts and RFC 5545 calendar files require CRLF).
  - Two conversion points, both local to whichever machine is acting, never a push/pull-time step: **checkout** (repo blob → working directory) applies `eol=` to write the OS/tool-appropriate ending to disk; **commit** (working directory → blob) converts back to the `eol=`-mandated ending before storing. The repository itself always holds the "canonical" ending dictated by the rule; push/pull just transfers that already-normalized blob unchanged.
- **Design Decision:** [AI explanation] Enforcing this via `.gitattributes` (committed, shared, applies to every clone/CI runner automatically) rather than relying on each developer's local `core.autocrlf` setting (which varies by value — `true`/`input`/`false` — and only works if every individual remembers to set it) makes line-ending behavior a property of the repository, not of each person's machine.
- **In Production:** [AI explanation] Concretely: `*.go text eol=lf` means a Go file checked out on Windows still gets LF on disk — not CRLF — because the toolchain (`gofmt`, build tooling) expects LF and mixing endings across a team would reproduce the same reformat-everything diff noise this whole mechanism exists to prevent.
- **Mistakes:**

  | Mistake | Consequence in production | Why it happens | Correct approach |
  |---|---|---|---|
  | Relying only on each developer's local `core.autocrlf` instead of `.gitattributes` | Inconsistent line endings across the team depending on who forgot to configure it, or configured it differently | `core.autocrlf` feels like "the" line-ending setting since it's the one Git surfaces in `git config`, but it's per-machine, not per-repo | Put `eol=`/`text=` rules in a committed `.gitattributes` so the rule travels with the repo, not with each person's local config |
  | Assuming `*.ext` patterns only match files in the root directory | A rule silently fails to apply to files nested in subdirectories, since the author didn't realize plain `*.ext` (no slash) already applies at every depth | Confusing gitattributes/gitignore glob semantics with typical shell globbing, where `*` doesn't cross directories | Remember: a pattern with **no `/` anywhere in it** is applied at every directory level automatically; a `**/` prefix is only needed once the pattern itself contains a `/` (e.g. `docs/*.md` vs `docs/**/*.md`) |

## 3. Plumbing Dissection

> This page's "plumbing" is a single, tiny, already-committed config file plus a workflow convention — not application code. Verified against the actual repo state:

**Wiring map (verified):**

```
project/.gitattributes                          (tracked, already committed)
  **/**.gen.go linguist-generated=true
        │
        ├─ matches: project/backend/orders/api/http/openapi.gen.go  (confirmed tracked in git)
        │
        └─ read by: GitHub Linguist / GitLab equivalent
              ├─ collapses this file by default in "Files changed" (PR UI)
              └─ excludes it from repo language statistics

project/.gitignore                               (confirmed: does NOT exclude *.gen.go)
        → generated files are committed, not regenerated-on-checkout

CI (page-recommended, NOT present in this repo):
  run generator → git status --porcelain → assert empty
```

- **Per plumbing piece:** The entire mechanism is declarative config (`.gitattributes`) plus a human workflow convention (regenerate-don't-hand-merge, and eventually a CI assertion) — there's no runtime code path here at all, unlike every previous guide in this module.
- **Non-obvious lines:** The glob `**/**.gen.go` (not just `*.gen.go` or `**/*.gen.go`) — the doubled `**/**` is what makes it match at *any* depth, including the file's actual location three directories deep (`backend/orders/api/http/openapi.gen.go`); a simpler `*.gen.go` would only match files directly in the repo root.
- **What a senior notices:** This repo has fully implemented the "commit generated files + `.gitattributes`" half of the page's recommendation, but **not** the "assert no diff in CI" half — there's no CI configuration at all yet. That's a real, current gap between what the page recommends as complete practice and what this project actually enforces today; worth flagging rather than assuming CI catches spec/code drift just because the page describes it as a good idea.

## 4. Rebuild Challenge

**Spec:** Outside `tdl`, in a scratch git repo, simulate and correctly resolve the exact merge-conflict scenario this page describes:
- Initialize a repo with a small "spec" file and a "generated" file that's a deterministic function of it (a simple script standing in for `oapi-codegen` is fine — e.g., a script that reads the spec and writes a generated file based on its content).
- Create two branches, each changing the spec differently, each regenerating the "generated" file on their branch.
- Merge one branch into the other and observe the conflict in the generated file.
- Resolve it the *correct* way: merge the spec files, regenerate, commit — never hand-editing the generated file's conflict markers.

**Acceptance criteria:**
1. The merge produces real conflict markers in your generated file — confirm you can see them before "fixing" anything.
2. Your resolution never touches a conflict marker directly inside the generated file; it only touches the spec file, followed by regeneration.
3. Add a `.gitattributes` entry marking your generated file's pattern as `linguist-generated=true`, and confirm (via `git check-attr linguist-generated -- <path>`) that Git recognizes the attribute is applied to the right file.
4. Write a tiny script that regenerates and then checks `git status --porcelain` — confirm it exits non-zero when you manually edit the spec without regenerating, and exits zero right after you do regenerate.
5. (Failure case) Deliberately hand-resolve a generated-file conflict once, incorrectly, then run your regeneration script from criterion 4 — confirm it detects and reports the drift.

<details><summary>Hint</summary>Your "generator" doesn't need to be real `oapi-codegen` — even a shell one-liner that copies/transforms the spec file's content deterministically is enough to reproduce the conflict dynamics.</details>
<details><summary>Hint</summary><code>git check-attr &lt;attribute&gt; -- &lt;path&gt;</code> tells you exactly what Git resolved a given attribute to for a given path — the fastest way to confirm your glob pattern actually matches what you think it matches.</details>
<details><summary>Hint</summary>For criterion 4, `git status --porcelain` outputs nothing (empty string) when the tree is clean — checking `[ -z "$(git status --porcelain)" ]` in a shell script is the whole assertion.</details>

**Compare step:** Compare your `.gitattributes` glob and workflow against `project/.gitattributes`'s real (one-line) content, and ask yourself:
1. Does your glob pattern match generated files at every depth they could realistically appear at, the way `**/**.gen.go` does?
2. Did your regeneration-assertion script rely on any tool version that isn't pinned the same way in every environment it runs in?
3. Would a teammate, seeing your `.gitattributes` file for the first time, understand from it alone that hand-merging is never the right move — or does that convention live only in your head/README?

## 5. Before the Exercise

- **Likely task:** [AI explanation — a guess, not derived from the exercise itself] Since `project/.gitattributes` already exists in the workspace with the exact content the page shows, and `openapi.gen.go` is already tracked in git, the "configuration" half of this page may already be done for you. Given there's no CI setup in this repo, the exercise might instead focus on the *workflow* understanding (regenerate-don't-hand-merge) rather than new config — but this is speculation; I haven't read `exercise.md`.
- **Approach:**
  1. Confirm `.gitattributes`' exact content and location (`project/.gitattributes`) and that it matches what the page shows character-for-character.
  2. Run `git check-attr linguist-generated -- backend/orders/api/http/openapi.gen.go` (from inside `project/`) to see the attribute actually resolve for the real generated file in this repo.
  3. If you have another generated file anywhere else in the project, confirm the glob catches it too — don't assume a single successful match proves the pattern is fully correct.
  4. If asked to add a CI check, focus on the "regenerate, then assert clean tree" pattern from the page rather than inventing a different verification approach.

## 6. Recall Questions

1. Why doesn't `.gitattributes`' `linguist-generated=true` prevent the merge conflict scenario the page opens with?
   <details><summary>Answer</summary>`linguist-generated=true` is read by GitHub/GitLab's UI tooling to change how a file is *displayed* during review (collapsed by default, excluded from language stats) — it says nothing to Git's actual merge algorithm. Git still performs a normal line-based merge on the file's content regardless of this attribute, so two branches with different regenerated output still produce real conflict markers. The attribute is a review-noise fix, not a merge-conflict fix.</details>

2. When a merge conflict appears inside a `.gen.go` file, why is hand-resolving the conflict markers the wrong move, even if the result compiles?
   <details><summary>Answer</summary>The generated file's content is a deterministic function of the (already hand-written, small) spec files. A manually resolved version might compile and even look correct, but if it doesn't exactly match what running the generator against the merged spec would produce, it has silently drifted from its source of truth. The next time anyone regenerates the file for an unrelated reason, that manual fix gets silently overwritten — so the correct workflow is merging the spec, regenerating, and committing the fresh output instead.</details>

3. What does the glob pattern `**/**.gen.go` do differently from a plain `*.gen.go` in `.gitattributes`?
   <details><summary>Answer</summary>`*.gen.go` alone would only match `.gen.go` files sitting directly in whatever directory the pattern is evaluated relative to (typically the repo root). `**/**.gen.go` matches at any directory depth, which is necessary here since the real generated file (`backend/orders/api/http/openapi.gen.go`) is nested three directories deep, not in the repo root.</details>

4. According to the page, what three costs does the `.gitignore` + regenerate-in-CI alternative impose, that committing generated files avoids?
   <details><summary>Answer</summary>It complicates developer experience (you must always remember to run the generator before building locally, since the file won't exist otherwise), it makes CI jobs slower (regeneration has to happen on every run instead of being a checked-out file), and it loses the ability to track how the generated code changes over time in git history, since the file is never actually committed.</details>

5. Why does asserting "no changes after running the generator in CI" (`git status --porcelain` is empty) only work reliably if CI uses the exact same tool version as local development?
   <details><summary>Answer</summary>If the generator version differs between CI and local dev, CI could produce a diff purely from version differences in output format/behavior — a false positive unrelated to anyone forgetting to regenerate — or, in the opposite case, mask a real drift if the differing version happens to produce similarly-shaped output. `go tool`'s `go.mod`-pinned version (covered in the HTTP Handler guide) removes this risk, since every environment resolves to the identical generator version automatically, making a detected diff a reliable signal of an actual missed regeneration.</details>

*(These questions, with answers, have also been appended to `notes/review_queue.md` under today's date.)*

## 7. Cheat Sheet

- `.gitattributes`' `linguist-generated=true` is a **review-UI signal only** — it does not prevent, resolve, or reduce merge conflicts in any way. Merge conflicts in generated files still happen exactly as normal.
- Correct workflow for a generated-file merge conflict: merge the *source* (spec), regenerate, commit the fresh output. Never hand-edit conflict markers inside a generated file.
- Glob depth matters: use `**/**.gen.go` (or equivalent), not `*.gen.go`, if generated files live in nested directories — verify with `git check-attr <attr> -- <path>`.
- Commit-generated-files tradeoff: (+) simpler dev experience, faster CI, trackable history of generated output; (–) PR diff noise — mitigated specifically by `.gitattributes`, not eliminated by it.
- CI hygiene (not yet present in this repo): regenerate in CI, then assert `git status --porcelain` is empty — catches spec/code drift automatically, but only reliable if the generator version is pinned identically everywhere (`go tool` in `go.mod` gives you this for free).
- Decision rule: commit generated files + `.gitattributes` when you want simple local/CI builds and don't mind mitigated (not eliminated) review noise; `.gitignore` + regenerate-in-CI only if you're willing to pay the dev-experience and CI-speed cost everywhere, every time.
- *(General reference, not this repo's config)* `text=auto` normalizes stored line endings to LF; per-type `eol=lf`/`eol=crlf` overrides force a specific ending on checkout regardless of platform (e.g. `*.sh`/`*.go` → LF even on Windows; `*.cmd`/`*.bat`/`*.ics` → CRLF). Conversion happens locally at checkout and commit, not during push/pull — the repo always stores the canonical ending.
- *(General reference)* A glob with no `/` in it (like `*.go`) already applies at every directory depth in `.gitattributes`/`.gitignore` — no `**/` prefix needed unless the pattern itself contains a `/`.
