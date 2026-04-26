# Master Study Guide: `.gitattributes`, Generated Files in Version Control & CI Assertions
> **Platform:** Three Dots Labs Academy — Go Backend Masterclass v0
> **Page:** Gitattributes — Generated Files, Merge Conflicts, PR Review Noise, CI Assertions
> **Generated:** 2026-04-26
> **Previous Guide:** `3_study_guide_custom_openapi_types.md`

---

## SECTION 1 — PAGE METADATA

### Topics Covered on This Page
1. **The `.gitattributes` file** — a configuration file that tells Git and hosting platforms (GitHub, GitLab) how to treat specific files; specifically, the `linguist-generated` attribute that marks files as machine-generated output
2. **Merge Conflicts in Generated Code** — why generated files cause conflicts when two branches both regenerate them, and the correct resolution strategy (regenerate — never hand-merge)
3. **PR Review Noise** — how generated files clutter pull requests with hundreds of irrelevant lines, and how `linguist-generated=true` collapses them by default in GitHub's "Files changed" view
4. **Why Commit Generated Files?** — the trade-off between committing generated files vs. adding them to `.gitignore` and regenerating in CI; why committing is usually the better choice
5. **Asserting No Changes in CI** — running the code generator in CI and asserting that `git status --porcelain` is empty; a safety net that catches developers who forget to run `go generate` after changing the OpenAPI spec

### Where This Fits in the Learning Sequence
- **What came before:** You introduced `x-go-type` in `openapi.yaml`, regenerated `openapi.gen.go` with `task gen`, and committed the result. That generated file now lives in the repository alongside human-written code.
- **What this page does:** It teaches you how to tell Git and your team that `openapi.gen.go` (and similar `.gen.go` files) are machine output — so teammates don't hand-merge them, reviewers don't wade through them, and CI catches anyone who forgets to regenerate after a spec change.
- **What this enables next:** Every future page that adds a generated file benefits from this setup. The team develops a consistent, safe workflow for handling generated code.

### Single Learning Objective
> After this page, the learner should be able to **explain why `.gitattributes` exists for generated files, describe the correct merge conflict resolution strategy for generated code, and configure CI to assert that generated files are always up to date — all from first principles.**

### Code Patterns Introduced on This Page

| Pattern | What it does |
|---|---|
| `**/**.gen.go linguist-generated=true` in `.gitattributes` | Marks all `.gen.go` files as generated; GitHub collapses them in PR diffs |
| `git status --porcelain` in CI | Returns empty output if working tree is clean; used to assert "nothing changed after regenerating" |
| `go generate ./...` | Runs all `//go:generate` directives in the project — regenerates all generated files |

### Concepts Referenced from Previous Pages
- [Previously learned: `task gen`] — The command that regenerates `openapi.gen.go`. This page explains what happens when two developers run it on different branches simultaneously.
- [Previously learned: `oapi-codegen`] — The tool that writes `openapi.gen.go`. This page treats that file's existence in the repo as the starting point.
- [Previously learned: `openapi.gen.go`] — The file that changes whenever the OpenAPI spec changes. This page explains how to manage that change safely in a team.

---

## SECTION 2 — CONTINUITY BRIDGE

### a) The Thread

On the previous page, you changed `openapi.yaml`, ran `task gen`, and `openapi.gen.go` was completely rewritten by oapi-codegen. You committed that file.

Now imagine a second developer on your team. They're working on a different feature — they also changed `openapi.yaml` and ran `task gen`. Their `openapi.gen.go` is also completely rewritten — but differently.

When you try to merge these two branches together, Git sees two completely different versions of the same 300-line file. It marks hundreds of lines as conflicts. A teammate opens the "Files changed" tab and sees a wall of generated code. Someone spends 30 minutes "resolving" the conflict by hand — and introduces a bug, because the merged version is not what either code generator would have produced.

This page exists to prevent that entire scenario — or at least to make the right response obvious.

### b) The Shared Axiom

> **The one principle connecting both pages:** Machine-generated output is not human-authored code. It should never be treated as if a human wrote it — not for merging, not for reviewing, not for arguing about.

The previous page used code generation to produce correct, boilerplate-free Go code automatically. This page teaches you to signal to all your tools and teammates that this code is machine-produced — and that the right response to any issue with it is always "regenerate," not "edit by hand."

### c) Quick Recall Check

- **You'll need: `openapi.gen.go`** — reminder: this file is completely rewritten by oapi-codegen every time `task gen` runs. It is not meant to be edited by hand.
- **You'll need: `task gen` / `go generate ./...`** — reminder: these commands regenerate the file from the `openapi.yaml` source of truth. Running them always produces a deterministic result from the spec.
- **You'll need: Git branches** — reminder: in a team, each developer works on a separate branch and later merges back to main. Generated files change on multiple branches simultaneously.

---

## SECTION 3 — CORE CONCEPT DEEP-DIVE

---

### Concept 1: The `.gitattributes` File and `linguist-generated`

#### a) The Problem Statement — WHY does this exist?

GitHub has a feature called **Linguist** — it analyzes your repository to show you a language breakdown ("this repo is 80% Go, 15% YAML, 5% Shell"). It also uses this analysis to decide how to display files in pull requests.

By default, Linguist treats `openapi.gen.go` like any other Go file — authored by a human, worth reviewing, worth including in language statistics. This creates two annoying problems:

**Problem A — Language stats:** Your repo appears to have 30% more Go code than you actually wrote. The generated file skews the statistics — it looks like you wrote 1,000 lines of Go when you wrote 700.

**Problem B — PR diff clutter:** When you open a PR, GitHub's "Files changed" tab shows every line that changed. If `openapi.gen.go` changed (because you changed the spec), GitHub shows all 300 lines of it — expanded, colorized, inline. Reviewers scroll past 300 lines of machine-generated code they can't change anyway, trying to find the 20 lines you actually wrote.

Without `.gitattributes`, there's no way to tell GitHub "this file is not mine — it was written by a tool."

The irreducible need: **a way to communicate metadata about files to Git and hosting platforms, so tools and humans treat generated files differently from authored code.**

#### b) The Atomic Axioms

1. **Axiom 1:** `.gitattributes` is a plain text file at the root of a repository. Each line is a pattern + attribute pair. Git reads it to apply special behaviors to matching files.
2. **Axiom 2:** `linguist-generated=true` is an attribute recognized by GitHub Linguist. It tells GitHub: "treat this file as machine-generated output — collapse it in PR diffs, exclude it from language statistics."
3. **Axiom 3:** The glob pattern `**/**.gen.go` matches any file ending in `.gen.go` at any directory depth. `**` means "any number of directories." This one pattern covers all generated files regardless of where they live in the project.
4. **Axiom 4:** `linguist-generated=true` does NOT change how Git tracks the file, does NOT prevent merge conflicts, and does NOT change how the file is committed or pushed. It only changes how GitHub DISPLAYS the file.
5. **Axiom 5:** `.gitattributes` is committed to the repository — it is version-controlled, shared with the team, and applies to every developer's clone automatically.

#### c) The Core Mechanism

Think of `.gitattributes` like a label on a cardboard box in a warehouse. The warehouse (Git) stores the box the same way regardless of the label. But the label tells the staff: "Fragile — handle with care" or "Contains documents — check separately." The label doesn't change the contents of the box — it changes how workers TREAT the box.

The line `**/**.gen.go linguist-generated=true` is that label, applied to all `.gen.go` files. GitHub reads this label when you open a pull request (Axiom 2) and responds by:
- Collapsing the generated file in the diff by default — reviewers see "openapi.gen.go (collapsed)" and can expand if they choose.
- Excluding it from language statistics — your Go percentage reflects only authored code.

Because `.gitattributes` is committed to the repo (Axiom 5), every developer on the team and every CI system gets the same behavior automatically. The label travels with the code.

#### d) Syntax & Code (from the webpage)

**The `.gitattributes` file (placed at the repository root):**

```gitattributes
# This is a .gitattributes file.
# Each line: <glob pattern> <attribute>=<value>

# Match ALL files ending in .gen.go, at ANY directory depth:
**/**.gen.go linguist-generated=true
#  ↑↑↑↑↑↑↑↑↑                        ↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑
#  "any .gen.go file                 "tell GitHub Linguist:
#   anywhere in the project"          this is machine-generated"
```

**What changes in GitHub's UI after this is committed:**

```
BEFORE .gitattributes:
Files changed (3):
  ✎ backend/orders/api/http/handler.go                       +20 / -5 lines  [EXPANDED]
  ✎ backend/orders/api/http/openapi.gen.go                  +300 / -200 lines [EXPANDED — huge!]
  ✎ backend/orders/api/http/openapi.yaml                     +15 / -3 lines  [EXPANDED]

AFTER .gitattributes (linguist-generated=true applied):
Files changed (3):
  ✎ backend/orders/api/http/handler.go                       +20 / -5 lines  [EXPANDED]
  ▸ backend/orders/api/http/openapi.gen.go                  +300 / -200 lines [COLLAPSED — click to expand]
  ✎ backend/orders/api/http/openapi.yaml                     +15 / -3 lines  [EXPANDED]
```

The generated file is collapsed by default. Reviewers see the files that matter.

#### e) Execution / Internal Walkthrough

When you add `.gitattributes` and push to GitHub, then open a pull request:

```
Step 1: You commit .gitattributes with: **/**.gen.go linguist-generated=true
Step 2: You push to GitHub (Axiom 5 — the file travels with the repo)
Step 3: You make a change: update openapi.yaml, run task gen, commit openapi.gen.go changes
Step 4: You open a Pull Request on GitHub
Step 5: GitHub generates the "Files changed" view
Step 6: For each changed file, GitHub checks .gitattributes for matching patterns
Step 7: openapi.gen.go matches **/**.gen.go (Axiom 3 — glob matches at any depth)
Step 8: Attribute linguist-generated=true is found (Axiom 2)
Step 9: GitHub collapses that file in the diff — reviewers see it as collapsed
Step 10: Language statistics exclude the lines in openapi.gen.go
Step 11: Reviewers see handler.go and openapi.yaml expanded — the files they care about
```

**Where each axiom becomes visible:**
- Axiom 1: Step 1 — `.gitattributes` is a plain text file with pattern + attribute pairs.
- Axiom 2: Step 8 — `linguist-generated=true` is the specific attribute GitHub acts on.
- Axiom 3: Step 7 — the glob pattern matches the file regardless of depth.
- Axiom 4: The generated file is STILL tracked by Git, still diffed, still committable. Only the DISPLAY changes.
- Axiom 5: Step 2 — the file is committed and pushed; every clone has it.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| `.gitattributes` prevents merge conflicts in generated files | **FALSE** | Axiom 4 — it only changes how GitHub DISPLAYS files. Merge conflicts still happen. The correct fix is to regenerate, not hand-merge. |
| `linguist-generated=true` deletes the file from GitHub's view | **FALSE** | The file is still visible — it's just collapsed by default. Reviewers can expand it by clicking. |
| `.gitattributes` only works on GitHub | **FALSE** | GitLab also respects `linguist-generated`. Other attributes (like merge strategy) work in Git itself regardless of hosting platform. |
| Each developer must set up `.gitattributes` manually | **FALSE** | Axiom 5 — it's committed to the repo. Any clone automatically has it. |
| `**/**.gen.go` matches only files directly inside subdirectories | **FALSE** | Axiom 3 — `**` matches any number of directory levels. `backend/orders/api/http/openapi.gen.go` is matched even though it's 4 levels deep. |
| You need to add each generated file to `.gitattributes` one by one | **FALSE** | The glob pattern matches ALL `.gen.go` files at once. Adding a new generated file in the future is automatic — no `.gitattributes` update needed. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Placing `.gitattributes` inside a subfolder instead of the repo root | Attribute not applied globally | GitHub reads `.gitattributes` from the repo root; files in subfolders have limited scope | Move `.gitattributes` to the project root (same level as `go.mod`) |
| Using `**.gen.go` instead of `**/**.gen.go` | Pattern too narrow | `**.gen.go` may not match files in nested directories depending on the Git version | Use `**/**.gen.go` to ensure all directory depths are covered |
| Forgetting to commit `.gitattributes` | Attribute not shared | The file exists locally but isn't tracked — other developers and CI don't have it | `git add .gitattributes` and commit |
| Confusing `linguist-generated` with a merge strategy | Misunderstanding scope | Developers think this prevents conflicts — it doesn't (Axiom 4) | The correct conflict resolution for generated files is always: regenerate from the source of truth |

#### h) Comprehension Task

> **Comprehension Task:** You have a colleague who says: "Adding `linguist-generated=true` is pointless — I don't care about language statistics, and I don't mind scrolling past generated files in PRs." Write a two-sentence response using ONLY the axioms from above — no opinions. Your response must address both of the problems `.gitattributes` solves and correctly state what it does NOT solve.
>
> **What to check:** A correct answer references PR review efficiency (Axiom 2 — collapsed in diffs, reviewers find authored code faster) AND language statistics clarity. It also states that `.gitattributes` does NOT prevent merge conflicts (Axiom 4). It does NOT say "it's best practice" without a reason.
>
> **Common wrong answer:** "It prevents merge conflicts in generated files." — Wrong. Axiom 4 explicitly states that `.gitattributes` does not prevent conflicts. This is one of the most common misunderstandings about this tool.

---

### Concept 2: Merge Conflicts in Generated Code

#### a) The Problem Statement — WHY does this exist?

Generated files are unique among files in a repository. A normal file like `handler.go` is written incrementally — different developers might add a function to one section and a type to another section. Git can often merge these changes automatically because the changes touch different lines.

`openapi.gen.go` is different. When you run `task gen`, oapi-codegen **deletes the entire file and rewrites it from scratch**. Every line is new. If two developers on separate branches both ran `task gen` after making different changes to `openapi.yaml`, the resulting files are completely different rewrites of the same file. Git's merge algorithm sees two versions with massive differences and marks almost every line as a conflict.

The irreducible need: **a team must have a shared understanding that generated files are never merged by hand — the correct resolution is always "regenerate from the combined source."**

#### b) The Atomic Axioms

1. **Axiom 1:** Code generators produce deterministic output from a source file — the same source always produces the same output. Two different sources produce two different outputs. There is exactly one correct generated file for any given source.
2. **Axiom 2:** A merge conflict in a generated file is a symptom of a conflict in the SOURCE file (e.g., `openapi.yaml`). The generated file conflict is a side effect — it has no independent meaning.
3. **Axiom 3:** The correct resolution for a conflict in a generated file is always: resolve the source file conflict first, then regenerate the output file. The generator produces the correct result — no manual editing required.
4. **Axiom 4:** Manually resolving a merge conflict in a generated file (editing the generated code line by line) almost always produces an incorrect result — because the merged version is not what the generator would have produced from a consistent source.
5. **Axiom 5:** `linguist-generated=true` does NOT prevent this conflict. The conflict still happens. The attribute's job is communication (signaling "this is generated code"), not conflict prevention.

#### c) The Core Mechanism

Think of `openapi.gen.go` like a printed report generated from a spreadsheet. Two managers each update different sections of the spreadsheet and print their own reports. When you try to merge the two printed reports by cutting and stapling, you get a mess — the page numbers are wrong, the totals don't match, the formatting breaks.

The correct process: merge the two SPREADSHEETS (the source of truth), then print ONE new report from the merged spreadsheet. The printer (oapi-codegen) guarantees a correct output from a correct source. You never merge the printed reports.

In code terms:
- Spreadsheet = `openapi.yaml` (the source of truth)
- Printed report = `openapi.gen.go` (generated output)
- Merging two reports by hand = wrong approach
- Merging two YAML files, then running `task gen` = correct approach

Because the generator is deterministic (Axiom 1), once you have one correct `openapi.yaml`, there is exactly one correct `openapi.gen.go`. The generator produces it for you — you never need to figure it out manually.

#### d) Syntax & Code (from the webpage)

**The correct merge conflict resolution workflow:**

```bash
# WRONG APPROACH (never do this):
# Open openapi.gen.go in an editor and manually resolve conflict markers:
# <<<<<<< HEAD
# type CustomerUUID = openapi_types.UUID
# =======
# type CustomerUUID = common.UUID
# >>>>>>> feature/custom-types
# ... (300 more lines of conflict markers)
# This produces incorrect code that the generator would never have written.

# CORRECT APPROACH:
# Step 1: Resolve ONLY the source file conflict
git checkout main
git merge feature/custom-types
# → conflict in openapi.yaml (the source)
# → conflict in openapi.gen.go (the generated output — ignore for now)

# Step 2: Open openapi.yaml and resolve the spec conflict (this is authored code — merge it properly)
# [edit openapi.yaml to combine both sets of changes correctly]
git add backend/orders/api/http/openapi.yaml

# Step 3: Regenerate — let the generator produce the correct output
task gen    # or: go generate ./...

# Step 4: Stage the regenerated file (it's now correct)
git add backend/orders/api/http/openapi.gen.go

# Step 5: Complete the merge commit
git commit
```

**What a conflict looks like in `openapi.gen.go` (to recognize and NOT manually fix):**

```go
// openapi.gen.go — during a merge conflict:

<<<<<<< HEAD
// RegisterCustomerJSONRequestBody defines body for RegisterCustomer for application/json ContentType.
type RegisterCustomerJSONRequestBody = RegisterCustomerRequest

type CustomerUUID = openapi_types.UUID
=======
// RegisterCustomerJSONRequestBody defines body for RegisterCustomer for application/json ContentType.
type RegisterCustomerJSONRequestBody = RegisterCustomerRequest

type CustomerUUID = common.UUID
>>>>>>> feature/custom-types

// The correct action: do NOT edit these lines.
// Run "task gen" after resolving openapi.yaml — this file will be correct automatically.
```

#### e) Execution / Internal Walkthrough

Two developers both fork from `main`. Developer A adds a new endpoint to the spec. Developer B adds `x-go-type` to the spec. Both run `task gen` and commit:

```
main branch: openapi.gen.go — version M (300 lines)
Branch A:    openapi.gen.go — version A (320 lines — new endpoint added)
Branch B:    openapi.gen.go — version B (305 lines — x-go-type applied)

Merge A into main: OK — no conflict (main had M, A has A, fast-forward)
Merge B into main: CONFLICT — main now has A, B has B, two different complete rewrites

Git's conflict output:
> CONFLICT (content): Merge conflict in backend/orders/api/http/openapi.gen.go
> Automatic merge failed; fix conflicts and then commit the result.

CORRECT RESOLUTION:
Step 1: Look at the actual spec conflict:
        CONFLICT in openapi.yaml — Developer A added /orders endpoint, Developer B changed a schema
Step 2: Merge openapi.yaml by hand — combine A's new endpoint AND B's x-go-type change
Step 3: Run task gen — generator reads the merged openapi.yaml
        → Produces version AB: 325 lines (new endpoint + x-go-type)
        → This is the ONLY correct version (Axiom 1 — deterministic from source)
Step 4: git add openapi.gen.go openapi.yaml
Step 5: git commit — merge complete, generated file is correct
```

**Where each axiom becomes visible:**
- Axiom 1: Step 3 — the generator produces exactly one correct output from the merged spec.
- Axiom 2: The `.gen.go` conflict IS the `openapi.yaml` conflict — resolving the source resolves the symptom.
- Axiom 3: Step 2–3 — resolve source first, then regenerate.
- Axiom 4: Any hand-merge of version A and version B would miss the correct structure that only the generator knows.
- Axiom 5: The conflict happened regardless of `.gitattributes`.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| Git can auto-merge generated files if the changes don't overlap | **SOMETIMES but unreliably** | oapi-codegen rewrites the entire file — even two small spec changes can produce large file-level differences that confuse Git's line-by-line algorithm. Never rely on auto-merge for generated files. |
| If there's no conflict in `openapi.yaml`, there won't be a conflict in `openapi.gen.go` | **FALSE** | Two different spec changes (both valid, no conflicting lines) can still produce incompatible generated files because the generator rewrites everything. |
| You can hand-merge the generated file correctly if you're careful | **FALSE** | Axiom 4 — the generated file has internal consistency rules (e.g., an import added at the top, a type referenced in the middle, a method at the bottom) that are hard to maintain manually. One missed change creates a subtle bug. |
| The conflict only affects one generated file | **NOT ALWAYS** | If the project has multiple generated files (e.g., a separate client code generator), all of them may conflict simultaneously. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Hand-merging conflict markers in `openapi.gen.go` | Silent bug — code compiles but is internally inconsistent | Axiom 4 — manual merging misses internal consistency the generator maintains | Resolve `openapi.yaml` conflict → run `task gen` → commit result |
| Accepting "theirs" or "ours" wholesale (`git checkout --theirs openapi.gen.go`) | Loses one branch's spec changes | "Theirs" or "ours" is only correct if one branch made NO spec changes | Only use this if you know one branch didn't touch the spec; regenerate otherwise |
| Committing the conflict markers into the file | Compile error — Go parser fails on `<<<<<<<` | Forgot to resolve before committing | Check `git status` shows no `UU` (unmerged) files before committing |
| Ignoring the generated file conflict and resolving only `openapi.yaml` | Remaining conflict in `.gen.go` prevents merge completion | Forgot the final step: regenerate after resolving the source | After fixing `openapi.yaml`, run `task gen`, stage the generated file, complete the commit |

#### h) Comprehension Task

> **Comprehension Task:** Two branches conflict in `openapi.gen.go`. Your colleague says "I'll just accept the version from `feature/custom-types` since it has the newer changes." On paper, write the exact axiom this violates and a one-paragraph explanation of what goes wrong — without using the word "wrong."
>
> **What to check:** The answer should cite Axiom 4 (hand-merging, or taking one side wholesale, produces a result the generator wouldn't have produced) and Axiom 2 (the generated file conflict is a symptom — the real conflict is in `openapi.yaml`). Accepting "theirs" loses whatever spec changes were on the OTHER branch — those changes are not in the generated file the colleague chose.
>
> **Common wrong answer:** "It's fine as long as the code compiles." — False. A generated file can be internally inconsistent in ways that compile but produce incorrect runtime behavior (e.g., a type alias pointing to an old type, an import no longer matching the type reference).

---

### Concept 3: PR Review Noise

#### a) The Problem Statement — WHY does this exist?

When you open a pull request, every file that changed between your branch and `main` appears in the "Files changed" tab. The goal is for reviewers to see only the changes that MATTER — the code a human wrote and a human should review.

Generated files shatter this goal. You changed 15 lines in `openapi.yaml` and 20 lines in `handler.go` — meaningful, reviewable changes. But `openapi.gen.go` changed by 223 lines because oapi-codegen rewrote it. GitHub shows all 223 lines expanded by default. Reviewers scroll, get confused, leave unrelated comments on generated code ("why is this import here?"), and miss the actual changes you need reviewed.

The irreducible need: **reviewers' attention is limited; generated files must be hidden from view by default to focus attention on authored code.**

#### b) The Atomic Axioms

1. **Axiom 1:** Every line shown in a PR diff costs reviewer attention. Unnecessary lines are a tax on reviewers' time and concentration.
2. **Axiom 2:** Generated files cannot be improved by code review — they are deterministically produced by a tool. Any comment about a generated file is either irrelevant ("this looks generated") or misdirected (the reviewer should comment on the source, not the output).
3. **Axiom 3:** `linguist-generated=true` causes GitHub to collapse the matching file in the PR diff by default. Reviewers see the file name and line count but not the contents — they can expand if they need to.
4. **Axiom 4:** Collapse-by-default does not HIDE the file. It remains in the PR. The reviewer can still expand, review, and comment on it. It's a default presentation choice, not an access control decision.
5. **Axiom 5:** The fix for any problem found in a generated file is always to fix the SOURCE and regenerate — never to edit the generated file directly. Code review of generated files thus has no practical output.

#### c) The Core Mechanism

Think of a PR diff like a physical stack of papers a reviewer picks up. Each sheet is a changed file. Without `.gitattributes`, all sheets are spread out face-up. The reviewer has to flip through 300 pages of machine output to find the 3 pages of human-authored changes.

With `linguist-generated=true`, the machine-output pages are folded and placed in an envelope labeled "Generated code — openapi.gen.go (+223 lines)". The reviewer sees the envelope in the stack. They know it's there. They can open it if needed. But they start with the unfolded pages — the ones that matter.

Because generated code is deterministic (Axiom 2 — the tool produces it, not a human), reviewing it line by line has no value. The source file (`openapi.yaml`) is what reviewers should be looking at when they want to understand how the generated code changed — not the output itself.

#### d) Syntax & Code (from the webpage)

**The `.gitattributes` line that fixes PR review noise:**

```gitattributes
**/**.gen.go linguist-generated=true
```

**The before/after effect in a PR (conceptual):**

```
BEFORE — PR diff with 3 files changed:
─────────────────────────────────────────────
  handler.go              [+20 / -5]    ← human-authored — shows expanded
  openapi.gen.go          [+223 / -180] ← generated — shows EXPANDED (wall of code)
  openapi.yaml            [+15 / -3]    ← human-authored — shows expanded
─────────────────────────────────────────────
  Total visible lines: 20+5 + 223+180 + 15+3 = 446 lines in the reviewer's face

AFTER — Same PR with .gitattributes:
─────────────────────────────────────────────
  handler.go              [+20 / -5]    ← human-authored — shows expanded
  ▸ openapi.gen.go        [+223 / -180] ← generated — COLLAPSED (click to expand)
  openapi.yaml            [+15 / -3]    ← human-authored — shows expanded
─────────────────────────────────────────────
  Total visible lines: 20+5 + 15+3 = 43 lines — 90% reduction in noise
```

**Equivalent for other generated file suffixes (if needed in future):**

```gitattributes
# Cover more generated file patterns:
**/**.gen.go    linguist-generated=true    # oapi-codegen, etc.
**/**.pb.go     linguist-generated=true    # protobuf generated files
**/mock_*.go    linguist-generated=true    # mockgen generated files
```

#### e) Execution / Internal Walkthrough

```
Step 1: You commit .gitattributes: **/**.gen.go linguist-generated=true
Step 2: You open a PR with changes to openapi.yaml, handler.go, openapi.gen.go
Step 3: GitHub fetches .gitattributes from the repo root (Axiom 3)
Step 4: For each file in the diff, GitHub checks .gitattributes for matching patterns
Step 5: handler.go — no match → expanded by default
Step 6: openapi.gen.go — matches **/**.gen.go → linguist-generated=true → collapsed (Axiom 3)
Step 7: openapi.yaml — no match → expanded by default
Step 8: Reviewer opens PR — sees handler.go and openapi.yaml expanded, openapi.gen.go collapsed
Step 9: Reviewer reviews handler.go and openapi.yaml efficiently (Axiom 1 — no attention wasted)
Step 10: Reviewer can click "▸ openapi.gen.go" to expand if they choose (Axiom 4)
```

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| Collapsing generated files means reviewers can't see them | **FALSE** | Axiom 4 — collapsed means "hidden by default, expandable on click." The file is fully accessible. |
| PR review of generated files is valuable if the generator has a bug | **FALSE** | Axiom 5 — if the generator has a bug, the fix is in the generator or its inputs (the spec), not in the generated output. Reviewing the output to find generator bugs is the wrong approach. |
| Reviewers will always expand the generated file anyway | **UNLIKELY in practice** | Once your team knows the workflow (resolve conflicts by regenerating, don't edit generated code), nobody needs to review the generated output. They trust the generator. |
| `.gitattributes` collapses the file for all Git tools | **FALSE** | Only GitHub Linguist and GitLab understand `linguist-generated`. Plain `git diff` in the terminal still shows everything. `.gitattributes` affects the PLATFORM display, not the raw Git data. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Leaving reviewers to scroll past generated files every PR | Team slowdown | No `.gitattributes` in the repo | Add `.gitattributes` with `**/**.gen.go linguist-generated=true` and commit |
| Commenting on generated code in a PR | Wasted time | Reviewer doesn't realize the file is generated | Good team communication + `.gitattributes` collapse (visual signal) makes the generated nature obvious |
| Using `linguist-generated` to hide FILES you don't want reviewed (authored code) | Code review gap — real bugs go unreviewed | Misusing the attribute as an access control tool | Only apply `linguist-generated` to truly generated code; all authored code must be visible to reviewers |

#### h) Comprehension Task

> **Comprehension Task:** Your team lead says: "We don't need `.gitattributes` — I can just tell reviewers to skip the generated files." On paper, write three reasons why the `.gitattributes` approach is better than relying on verbal instructions. Each reason must reference an axiom.
>
> **What to check:** Good answers include: (1) Axiom 1 — attention is limited; even knowing to skip something takes attention away from what matters. A collapsed file removes the cognitive load automatically. (2) Axiom 5 — if reviewers can see generated code, they might comment on it, wasting their time and yours. The collapsed presentation prevents irrelevant comments before they start. (3) Axiom 5 — new team members don't know the verbal instruction; `.gitattributes` encodes the team convention in the repository itself and works for everyone automatically.
>
> **Common wrong answer:** "Because it's the official rule." — This is an appeal to authority, not a first-principles reason. The axioms explain WHY the rule exists.

---

### Concept 4: Why Commit Generated Files?

#### a) The Problem Statement — WHY does this exist?

Every developer who has worked with generated code has asked: "Why are we committing machine-generated files? Shouldn't `.gitignore` exclude them and CI regenerate them on demand?"

This is a reasonable question. It's not the right answer for most projects, but understanding WHY requires thinking through both sides.

The irreducible need: **the team must have an explicit, reasoned policy for generated files — either commit them (and manage them with `.gitattributes` + CI assertions) or exclude them (and add `go generate` to the mandatory dev setup and CI pipeline).**

#### b) The Atomic Axioms

1. **Axiom 1:** If a file is in `.gitignore`, it is not tracked by Git. It does not exist in the repository. Every developer who clones the repo starts without it.
2. **Axiom 2:** A file in `.gitignore` that is needed to build the project must be regenerated by every developer before they can build. This is a setup step that must be documented, remembered, and executed — or the build fails.
3. **Axiom 3:** A committed generated file is always present after cloning. No extra setup step required. The developer runs `git clone` and immediately has a buildable project.
4. **Axiom 4:** Committing generated files allows you to track how the generated code changes over time via `git log`. You can see exactly what changed in `openapi.gen.go` between releases — useful for debugging regressions.
5. **Axiom 5:** For the "commit generated files" approach to be safe, CI must assert that the generated files are up to date — that no developer committed spec changes without regenerating. This assertion catches the drift between source and output.

#### c) The Core Mechanism

Think of it like a physical product manual. 

**Option A (commit generated files):** Print the manual and include it with the product in the box. When a customer opens the box, the manual is there. No action required. But: if the product changes, someone must reprint and re-include the manual. If they forget, the manual is out of date.

**Option B (gitignore + CI regeneration):** Don't include a manual in the box. Instead, include a URL: "Download the manual from our website." The customer must take an extra step. But: the website always has the latest version — it can't get out of date.

For most software teams, **Option A** (commit) is preferred because:
- `git clone` → build works immediately (Axiom 3) — less friction for new developers
- History of generated code changes is preserved (Axiom 4)
- The extra step (regenerate) is protected by CI assertion (Axiom 5) — the "out of date" problem is caught automatically

**Option B** (gitignore) adds friction: every developer must remember to run `go generate ./...` before building. If they forget, they get a compile error on files that don't exist yet. In practice, people forget — especially on the first day or after a long break.

#### d) Syntax & Code (from the webpage)

**The generated file is committed normally — no special commands:**

```bash
# After adding x-go-type to openapi.yaml and running task gen:
git status
# M backend/orders/api/http/openapi.yaml      ← authored (changed)
# M backend/orders/api/http/openapi.gen.go    ← generated (changed — commit this)

git add backend/orders/api/http/openapi.yaml
git add backend/orders/api/http/openapi.gen.go  # YES — commit the generated file
git commit -m "feat: use common.UUID for CustomerUUID type"
```

**What the `.gitignore` alternative would look like (NOT recommended for this project):**

```gitignore
# ALTERNATIVE (not recommended):
# backend/orders/api/http/openapi.gen.go   ← excluded — not tracked

# Consequence: every developer must now run this before building:
go generate ./...   # or: task gen
# If they forget: compile error — openapi.gen.go doesn't exist
```

#### e) Execution / Internal Walkthrough

**Comparing the developer experience on day 1:**

```
SCENARIO: A new developer joins the team and clones the repository.

COMMIT APPROACH (current):
Step 1: git clone https://github.com/org/eats-backend
Step 2: cd eats-backend
Step 3: go build ./...          ← WORKS — openapi.gen.go is in the repo
Step 4: Develop immediately

GITIGNORE APPROACH (alternative):
Step 1: git clone https://github.com/org/eats-backend
Step 2: cd eats-backend
Step 3: go build ./...          ← FAILS — openapi.gen.go is not in the repo
        Error: cannot find package "eats/backend/orders/api/http" in .gen.go
Step 4: Read the README for setup instructions (if they're there)
Step 5: go generate ./...       ← only works if go generate tools are installed
Step 6: go build ./...          ← now works
Step 7: Develop — 20 minutes later than in the commit approach
```

**Where each axiom becomes visible:**
- Axiom 1: Step 3 (gitignore) — the file is absent after clone; the build fails.
- Axiom 2: Step 5 — the extra step must be documented and remembered.
- Axiom 3: Step 3 (commit) — the file is present after clone; the build succeeds immediately.
- Axiom 4: `git log backend/orders/api/http/openapi.gen.go` shows the history of how generated code evolved.
- Axiom 5: CI regenerates and checks — any drift between spec and generated file is caught.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| Committing generated files makes the repo "messy" | **SUBJECTIVE** | With `.gitattributes`, generated files are collapsed in PRs and excluded from language stats — the mess is managed. The benefit (instant `git clone` + build) outweighs the aesthetic concern. |
| CI can always regenerate generated files faster than committing them | **DEPENDS** | CI regeneration adds time to every CI run. For a slow generator (e.g., protobuf with thousands of types), this can add minutes. For oapi-codegen, it's fast — but the principle applies at scale. |
| You can mix the approaches (commit some, gitignore others) | **TRUE** | You can. The important thing is consistency within a project and clear documentation of which files are which. |
| Generated files take up too much disk space | **FALSE IN PRACTICE** | Go generated files are text — a few kilobytes to a few hundred kilobytes. Version control compresses diffs efficiently. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Adding generated files to `.gitignore` without documenting the setup step | New developer confusion — build fails immediately | Axiom 2 — the setup requirement is implicit, not documented | Either commit the files OR add explicit `go generate ./...` step to the README and CI |
| Committing generated files BUT not adding a CI assertion | Silent drift — spec and generated code get out of sync | Axiom 5 is missing — nobody catches the developer who forgot to regenerate | Add CI step: run generator, then `git status --porcelain` must be empty |
| Editing generated files by hand to fix a bug | Files get out of sync on next regeneration | Forgetting that the file is generated — any manual edit is overwritten on next `task gen` | Fix the source (spec/config), never the output |

#### h) Comprehension Task

> **Comprehension Task:** Your project's README says "run `go generate ./...` before building for the first time." A new developer ignores this and opens a PR. Their CI build passes because CI runs `go generate`. Your build fails locally because you pulled their branch without running `go generate`. In plain English: which of the two approaches (commit vs. gitignore) is this project using, and what is the one rule that would prevent the developer experience mismatch described?
>
> **What to check:** The project is using the gitignore approach (generated files are not in the repo — hence the need to run `go generate` before building). The rule that prevents the mismatch: commit the generated files (Axiom 3 — after clone, build works immediately without extra steps). OR: if staying with gitignore, the CI assertion would still fail — `git status --porcelain` would be empty because CI already ran `go generate`. This is one reason the commit approach is preferred: no inconsistency between local and CI developer experience.
>
> **Common wrong answer:** "Just make the README clearer." — This relies on people reading the README. Axiom 3 says the correct solution is to not require the step at all — commit the generated file so there's nothing to read.

---

### Concept 5: Asserting No Changes in CI

#### a) The Problem Statement — WHY does this exist?

Suppose a developer adds a new endpoint to `openapi.yaml` — a carefully designed change, properly reviewed in the PR. But they forgot to run `task gen`. The PR is merged. Now `openapi.yaml` says "there is a `PlaceOrder` endpoint" but `openapi.gen.go` says nothing about it. The generated code is stale.

Nobody notices immediately. Later, another developer tries to implement the `PlaceOrder` handler and can't find the generated interface method. They're confused. Eventually someone runs `task gen` and discovers the drift — but now it's not obvious which commit caused it.

The irreducible need: **an automated check that runs in CI after any change to ensure the generated code is always in sync with its source.**

#### b) The Atomic Axioms

1. **Axiom 1:** A code generator produces the same output for the same input every time. This determinism is the property that makes assertions possible — you can run the generator in CI and compare the result to what's in the repo.
2. **Axiom 2:** `git status --porcelain` outputs NOTHING if the working tree is clean (no uncommitted changes). It outputs one line per modified file if anything changed.
3. **Axiom 3:** The assertion is: run the generator in CI → if `git status --porcelain` produces any output, it means the newly generated file differs from the committed file → the assertion fails → the build fails → the PR is blocked.
4. **Axiom 4:** For this assertion to work, the CI environment must use the EXACT SAME VERSION of the generator tool as developers use locally. If CI uses oapi-codegen v2.1 and developers use v2.0, the generated output may differ even from the same source — and the assertion always fails.
5. **Axiom 5:** Go's `go tool` support in `go.mod` solves the version problem — the generator version is specified in `go.mod` and used identically in both local development and CI.

#### c) The Core Mechanism

Think of it like quality control in a factory. Every product is made by a machine (the generator). The QC station (CI assertion) runs the machine again with the same inputs and compares the new product to the product in the shipping box. If they're identical, the product passes. If they differ — someone tampered with the product in the shipping box, or they used different machines.

In code terms:
- The factory machine = oapi-codegen
- The shipping box product = the committed `openapi.gen.go`
- The QC check = `task gen` in CI, then `git status --porcelain`
- "They're identical" = the committed file was produced from the current spec by the current generator version
- "They differ" = someone changed the spec without regenerating, OR used a different generator version (Axiom 4)

Because the generator is deterministic (Axiom 1), "identical output from the same input" is guaranteed — assuming the same tool version (Axiom 4, solved by Axiom 5).

#### d) Syntax & Code (from the webpage)

**CI assertion script (in a GitHub Actions workflow or Makefile):**

```yaml
# .github/workflows/ci.yml (example):
- name: Check generated files are up to date
  run: |
    # Step 1: Run the generator (same as developers do locally)
    go generate ./...     # or: task gen

    # Step 2: Check if anything changed
    # git status --porcelain outputs nothing if clean, file names if changed
    if [ -n "$(git status --porcelain)" ]; then
      # Something changed — the committed file differs from freshly generated output
      echo "ERROR: Generated files are out of date. Run 'go generate ./...' and commit."
      git diff           # Show what changed (for debugging)
      exit 1             # Fail the CI build
    fi
    echo "Generated files are up to date."
```

**What `git status --porcelain` outputs:**

```bash
# Clean working tree (assertion PASSES — nothing to commit):
$ git status --porcelain
[no output]

# Modified file (assertion FAILS — spec was changed without regenerating):
$ git status --porcelain
 M backend/orders/api/http/openapi.gen.go
#↑ "M" means "Modified" — the file in the repo differs from the working tree

# The CI exits 1 — the build fails — the PR cannot be merged
```

**`go.mod` with pinned generator version (Axiom 5):**

```go
// go.mod — pinning the oapi-codegen version ensures CI and local use the same binary
tool (
    github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen v2.4.1
)
// Both local developers AND CI install v2.4.1 exactly — same output guaranteed
```

#### e) Execution / Internal Walkthrough

Developer A changes `openapi.yaml` to add a new field. They forget to run `task gen`. They open a PR:

```
Step 1: Developer A changes openapi.yaml (adds field: "phone_number" to request body)
Step 2: git add openapi.yaml; git commit -m "feat: add phone_number to request"
Step 3: openapi.gen.go is NOT changed — it still doesn't have "phone_number" in the struct
Step 4: Developer A opens PR on GitHub

CI runs:
Step 5: CI checks out the branch
Step 6: CI runs: go generate ./...   (Axiom 1 — deterministic from current openapi.yaml)
        → oapi-codegen reads the new openapi.yaml
        → Regenerates openapi.gen.go with "phone_number" in the request struct
Step 7: Now the working tree has:
        - openapi.yaml: the new version (same as committed — no change)
        - openapi.gen.go: the regenerated version (DIFFERENT from committed — has phone_number)
Step 8: CI runs: git status --porcelain
        Output: " M backend/orders/api/http/openapi.gen.go"  (Axiom 2 — not empty)
Step 9: The if-check triggers: [ -n " M backend/orders..." ] is TRUE
Step 10: CI runs: git diff (shows what changed — phone_number field missing in the committed file)
Step 11: CI exits 1 — BUILD FAILS (Axiom 3)
Step 12: GitHub blocks the PR from merging — "CI checks failed"
Step 13: Developer A sees the error: "Generated files are out of date. Run 'go generate ./...'"
Step 14: Developer A runs task gen locally, commits openapi.gen.go, pushes
Step 15: CI re-runs — git status --porcelain produces no output — BUILD PASSES
Step 16: PR can be merged
```

**Where each axiom becomes visible:**
- Axiom 1: Step 6 — the generator produces the correct output from the current spec.
- Axiom 2: Step 8 — `--porcelain` output is non-empty → something changed.
- Axiom 3: Steps 9–12 — the assertion fails, the build fails, the PR is blocked.
- Axiom 4 (implicit): If CI used a different oapi-codegen version, it might generate slightly different output even from the same spec — a false positive. Axiom 5 prevents this.
- Axiom 5: `go.mod` pins the version — same binary in CI and locally.

#### f) Assumption Audit

| Hidden Assumption | True / False | Reality |
|---|---|---|
| The CI assertion only needs to run when `openapi.yaml` changes | **FALSE** | CI can't know ahead of time which files affect which generated files. Run the generator unconditionally — it's fast and the assertion is cheap. |
| Any generator version will produce the same output from the same input | **FALSE** | Different versions may have different output formatting, added fields, or bug fixes. Axiom 4 — the assertion requires the SAME version in CI and locally. |
| `git status --porcelain` being empty guarantees the generated files are correct | **TRUE given the same version** | If the same generator version produces the same output from the same source and nothing changed, the committed files are correct by definition (Axiom 1). |
| If CI passes, developers don't need to run `task gen` locally before committing | **FALSE** | CI catches the mistake AFTER the fact. Local `task gen` is still the right workflow — CI is a safety net, not a replacement for doing the right thing locally. |

#### g) Anti-Patterns & Common Mistakes

| Common Mistake | Error Type | Root Cause | Fix |
|---|---|---|---|
| Using different oapi-codegen versions locally vs. CI | False positive assertion failures | Axiom 4 violated — even with no spec change, different versions produce different output | Pin the version in `go.mod` using `go tool` support; both CI and local use the same version |
| Not running the assertion in CI at all | Silent drift — spec and generated code diverge | No safety net (Axiom 5 missing) — drift discovered only when someone notices strange behavior | Add the `git status --porcelain` check to CI as a mandatory step |
| Running the assertion but not failing the CI build on non-empty output | Assertion has no effect | `exit 1` is missing — the assertion prints a warning but CI still passes | Always `exit 1` (or equivalent CI failure command) when `git status --porcelain` is non-empty |
| Running the CI assertion on EVERY file in the repo (not just generated files) | False positives from test output, build artifacts | `go generate ./...` might produce other side effects | Ensure only the generated files are checked; `.gitignore` excludes build artifacts |

#### h) Comprehension Task

> **Comprehension Task:** On paper, trace through what the CI assertion would output for EACH of these four scenarios. State "PASS" or "FAIL" and which axiom determines the outcome:
> 1. Developer commits BOTH `openapi.yaml` change AND regenerated `openapi.gen.go`.
> 2. Developer commits ONLY `openapi.yaml` change, forgets `openapi.gen.go`.
> 3. Developer commits NO changes to `openapi.yaml` or `openapi.gen.go` (feature in handler.go only).
> 4. Developer hand-edits `openapi.gen.go` directly (no spec change), commits it.
>
> **What to check:** (1) PASS — Axiom 1: generator produces same output as committed file → no diff. (2) FAIL — Axiom 3: generator produces updated file, committed file is stale → `git status` non-empty. (3) PASS — Axiom 1: no spec change → generator produces same output → no diff. (4) FAIL — Axiom 1: generator produces the output from the spec (which didn't change), overwriting the hand-edit → `git status` shows the regenerated file differs from the hand-edited committed file.
>
> **Common wrong answer for scenario 4:** "PASS — because the spec didn't change." — Wrong. The committed file contains a hand-edit that the generator doesn't know about. When CI runs the generator, it produces the "correct" output (from the unchanged spec), which DIFFERS from the hand-edited committed version → FAIL. This is actually a FEATURE — it prevents hand-edits to generated files from surviving.

---

## SECTION 4 — EXERCISE READINESS

### a) What the Exercise Will Likely Ask

Based on the webpage content, the exercise likely asks you to:

1. **Create or inspect `.gitattributes`** — add the `**/**.gen.go linguist-generated=true` line to the project's `.gitattributes` file.
2. **Understand the merge conflict workflow** — the exercise may present a simulated conflict and ask you to identify the correct resolution strategy (regenerate from the source, not hand-merge).
3. **Add a CI assertion** — add a CI workflow step that runs `go generate ./...` and checks `git status --porcelain`.

### b) Pre-Implementation Checklist

Before writing a single line:

- [ ] I can state in one sentence what `linguist-generated=true` does and what it does NOT do.
- [ ] I can explain the correct merge conflict resolution for generated files without using the word "merge" (i.e., resolve source → regenerate → commit).
- [ ] I know what `git status --porcelain` outputs when the working tree is clean vs. when a file has changed.
- [ ] I can explain why the generator version must be identical in CI and locally, and how `go.mod` solves this.
- [ ] I can state the trade-off between committing generated files vs. `.gitignore`, and why committing is preferred here.

### c) Implementation Blueprint

**Step 1:** Create or open `.gitattributes` at the repository root (same directory as `go.mod`). Add the line: `**/**.gen.go linguist-generated=true`.
*(Relies on Axiom 3 of `.gitattributes` — the glob pattern must be at the root level to apply project-wide.)*

**Step 2:** Commit `.gitattributes`.
*(Relies on Axiom 5 of `.gitattributes` — the file must be committed so all developers and CI have the same attributes.)*

**Step 3:** Add the CI assertion step to the CI configuration (GitHub Actions workflow or Makefile): run `go generate ./...`, then check `git status --porcelain` for non-empty output, exit 1 if non-empty.
*(Relies on Axiom 1 of CI Assertion — determinism — and Axiom 3 — the assertion fails the build when drift is detected.)*

**Step 4:** Verify the pinned generator version in `go.mod` matches what developers use locally.
*(Relies on Axiom 4 of CI Assertion — version mismatch causes false positives.)*

### d) Debugging Guide

| Failure Symptom | Violated Axiom | Fix |
|---|---|---|
| CI assertion fails on every run even when spec hasn't changed | CI Assertion Axiom 4 — version mismatch | Check that `go.mod` tool version matches the locally installed version |
| Generated files are still expanded in GitHub PR diffs after adding `.gitattributes` | `.gitattributes` Axiom 5 — file not committed | Run `git add .gitattributes` and commit; the attribute only applies once GitHub has the file |
| CI assertion passes but `openapi.gen.go` is stale locally | CI Assertion Axiom 2 — `git status` not checked locally | CI is a safety net, not a replacement for running `task gen` before committing locally |
| `.gitattributes` pattern doesn't match files in nested directories | `.gitattributes` Axiom 3 — wrong glob | Use `**/**.gen.go` not `*.gen.go` — the `**` ensures all directory depths are covered |

---

## SECTION 5 — STRUCTURED PRACTICE (AEIOU Framework)

### A — ACQUIRE (Axioms First)

**`.gitattributes` axioms to internalize:**
1. It's a committed file — shared with the team automatically.
2. `linguist-generated=true` collapses the file in GitHub PR diffs by default.
3. It does NOT prevent merge conflicts (the most important axiom to internalize).

**Merge conflict axioms:**
1. The generated file conflict is a SYMPTOM — the source file conflict is the real problem.
2. The correct resolution is ALWAYS: fix the source → regenerate → commit.
3. Hand-merging generated code is always wrong.

**CI assertion axioms:**
1. Determinism: same source + same generator version = same output.
2. `git status --porcelain` is empty when working tree is clean.
3. Version mismatch between local and CI causes false assertion failures.

**Write from memory (no copy-paste):**
- The `.gitattributes` line for marking `.gen.go` files as generated.
- The bash script for the CI assertion (run generator → check `git status --porcelain` → exit 1 if non-empty).
- The correct three-step merge conflict resolution workflow.

---

### E — EXERCISE (Reason from Nothing)

**Problem 1 (Simple — `.gitattributes`):**
A new project has no `.gitattributes` file. A developer adds a new endpoint to `openapi.yaml`, runs `task gen`, and opens a PR. The PR diff shows 250 lines of changes in `openapi.gen.go` and 10 lines in `openapi.yaml`. What does a reviewer see? What changes after `.gitattributes` is added?

**Problem 2 (Simple — merge conflicts):**
Two developers both change `openapi.yaml` on separate branches. Developer A adds a new field to `RegisterCustomerRequest`. Developer B adds `x-go-type` to `CustomerUUID`. When Developer B's branch is merged after Developer A's, there is a conflict in `openapi.gen.go`. Write the exact steps to resolve this conflict correctly — without editing `openapi.gen.go` by hand.

**Problem 3 (Medium — CI assertion):**
The CI assertion runs `go generate ./...` then checks `git status --porcelain`. A developer has committed `openapi.yaml` changes but forgot `openapi.gen.go`. Trace step by step what `git status --porcelain` outputs and why the CI fails.

**Problem 4 (Medium — commit vs. gitignore):**
Your teammate proposes adding `openapi.gen.go` to `.gitignore` "to keep the repo clean." List the three consequences of this decision that would make the developer experience worse, each linked to a specific axiom.

**Problem 5 (Complex — version mismatch):**
CI uses oapi-codegen v2.4.1. A developer's local machine has v2.3.0. The developer runs `task gen`, commits `openapi.gen.go`, and opens a PR. CI runs `go generate ./...` with v2.4.1 — the output differs slightly from what the developer committed. What does `git status --porcelain` output? Does the assertion pass or fail? Which axiom explains this? How does `go.mod` tool pinning fix it?

**Problem 6 (Complex — combining all concepts):**
A project has `.gitattributes`, the CI assertion, and all generated files committed. A new developer joins, clones the repo, opens `openapi.yaml`, adds a new field, saves the file, and opens a PR WITHOUT running `task gen`. Trace the entire journey from their commit to the blocked PR — naming every concept from this page that activates along the way.

---

### I — INSPECT (Identify the Violation)

**Task 1:** What is wrong? Which axiom is violated?
```gitattributes
# .gitattributes in backend/orders/api/http/ subfolder (NOT the repo root):
*.gen.go linguist-generated=true
```

**Task 2:** What is wrong with this conflict resolution?
```bash
# Resolving a conflict in openapi.gen.go:
git checkout --ours backend/orders/api/http/openapi.gen.go
git add backend/orders/api/http/openapi.gen.go
git commit -m "resolve conflict"
```

**Task 3:** What is wrong with this CI assertion?
```yaml
- name: Check generated files
  run: |
    go generate ./...
    git status --porcelain
    echo "Done"
```

**Task 4:** What is wrong?
```gitattributes
# .gitattributes:
*.gen.go linguist-generated=true
```
(The project has generated files at `backend/orders/api/http/openapi.gen.go`)

**Task 5:** What is wrong with this developer workflow?
```bash
# Developer's workflow:
# 1. Edit openapi.yaml
# 2. git add openapi.yaml
# 3. git commit -m "feat: add new endpoint"
# 4. git push
# 5. Open PR
# 6. (CI fails with "generated files out of date")
# 7. git add backend/orders/api/http/openapi.gen.go
# 8. git commit --amend
# 9. git push --force
```

---

### O — ORCHESTRATE (Synthesis Design)

**Scenario:** Your project is growing. You now have:
- `openapi.gen.go` (oapi-codegen output)
- `mock_customer_service.go` (mockgen output for testing)
- `migrations_list.gen.go` (a custom migration list generator)

Three different generators, three different generated files.

**Write in plain English:**
1. How would you update `.gitattributes` to cover all three files?
2. What is the single CI assertion command that covers all three?
3. If the three generators have different version pins in `go.mod`, what is the risk and how does it manifest?
4. A developer modifies the SQL migration source and forgets to regenerate `migrations_list.gen.go`. Walk through the CI assertion catching this error.
5. Write the implementation blueprint step by step — which axiom does each step rely on?

---

### U — UNDERSTAND (Feynman Reconstruction Test)

**Most commonly misunderstood concept: The CI assertion — specifically the relationship between the generator's determinism and the ability to catch drift**

**Step 1 — Explain it simply:**
"Imagine a photocopier that always makes an IDENTICAL copy of whatever you put in. You make a copy and put it in a folder. Later, someone secretly changes what's in the folder. To check if the folder is correct, you take the ORIGINAL document, make a fresh copy, and compare. If they match — great. If they don't — someone changed the folder copy. The CI assertion is that comparison."

**Step 2 — Find the gap:**
A beginner might ask: "But how does CI know what the CORRECT generated file looks like?" The gap: they don't see how determinism enables the check — they think you'd need a "golden file" stored somewhere.

**Step 3 — Go back to axioms:**
Axiom 1 of CI Assertion: the generator is deterministic. Given the same source file and the same generator version, the output is always identical. CI doesn't need a stored "golden file" — it runs the generator fresh and the generator PRODUCES the golden file on the spot. If the freshly generated file differs from the committed file, someone changed the committed file without going through the generator.

**Step 4 — Proof of Understanding:**
**Question:** A developer argues: "The CI assertion is pointless — if CI can regenerate the files, why not just commit the freshly generated files from CI instead of checking?" Write a two-sentence answer that uses Axiom 1 and Axiom 4 to explain why this is a bad idea.

**Expected Answer:** If CI auto-commits regenerated files, any developer who changes the spec gets automatic CI fixes — removing the obligation to regenerate locally (Axiom 2: the setup step must be documented and executed). Worse: if CI uses a different tool version (Axiom 4), CI would silently introduce version-specific changes that were never reviewed by any human — making CI a source of unreviewed code changes.

---

## SECTION 6 — KNOWLEDGE CHECK

### a) Scenario-Based Reasoning Questions

**Q1:** A developer says: "Our `.gitattributes` has `linguist-generated=true` for all `.gen.go` files. I can see the CI assertion works because it fails when I forget to regenerate. So we don't need to commit the generated files — CI can handle regeneration." What is wrong with this reasoning? Which axiom does each problem with this plan violate?

**Expected Answer:** Two distinct problems. First, Axiom 3 of "Why Commit": if generated files are in `.gitignore`, a new developer who clones the repo cannot build immediately — they must run `go generate ./...` first (Axiom 2: extra setup step). Second, Axiom 4 of Commit: the history of how generated code changed over time is lost — you can no longer use `git log` to understand what changed between versions. The CI assertion being in place doesn't fix either of these problems — it is a safety net for the commit approach, not a replacement for it.

---

**Q2:** Two developers merge `openapi.yaml` correctly (both spec changes are present in the final merged YAML). But they forgot to regenerate `openapi.gen.go` after the merge. Will the CI assertion catch this? Which axiom guarantees it?

**Expected Answer:** Yes, the CI assertion catches it. Axiom 1 of CI Assertion: the generator produces a deterministic output from the current `openapi.yaml`. The current `openapi.yaml` now has both sets of changes. When CI runs `go generate ./...`, it produces an `openapi.gen.go` with both changes. The committed `openapi.gen.go` has only one set of changes (from before the merge). `git status --porcelain` shows a diff → assertion fails → build fails. Axiom 3: the assertion is designed precisely for this scenario.

---

**Q3:** `linguist-generated=true` is in `.gitattributes` but a reviewer says they can still see `openapi.gen.go` fully expanded in the PR. What could explain this? List all possible causes.

**Expected Answer:** Three possible causes: (1) `.gitattributes` was not committed — it exists locally but isn't in the repo; GitHub never sees it (Axiom 5). (2) The glob pattern is wrong — `*.gen.go` without `**/` doesn't match files in nested directories (Axiom 3). (3) The reviewer manually expanded the file by clicking — collapsed-by-default doesn't mean they can't expand it (Axiom 4). The reviewer should be told they can collapse it again.

---

**Q4:** Your project switches from oapi-codegen v2.3.0 to v2.4.1. You update `go.mod`, run `task gen` locally, commit the updated `openapi.gen.go`, and open a PR. CI still has v2.3.0 cached. The CI assertion fails. Which axiom does this violate and what is the fix?

**Expected Answer:** Axiom 4 of CI Assertion — the assertion requires the EXACT SAME generator version in CI and locally. v2.4.1 (local) produces slightly different output than v2.3.0 (CI) from the same spec → the assertion sees a diff → fails. Fix: invalidate CI's tool cache so it picks up the new version from `go.mod`. Because `go.mod` pins the version (Axiom 5), once CI uses the pinned version, the assertion is consistent again.

---

**Q5:** A developer hand-edits `openapi.gen.go` to add a comment: `// This file is generated by oapi-codegen`. They commit it. Will the CI assertion catch this? Should the assertion catch it?

**Expected Answer:** YES the assertion catches it, and YES it should. Axiom 1 of CI Assertion: the generator is deterministic. When CI runs `go generate ./...`, it produces the file from the spec — without the hand-added comment. The committed file has the comment; the freshly generated file doesn't. `git status --porcelain` shows a diff → assertion fails. This is CORRECT behavior: generated files must not be hand-edited (any edit is overwritten on the next `task gen`). If the developer wants a comment in the generated file, they should configure oapi-codegen to add it — not hand-edit the output.

---

### b) "What If?" First-Principles Challenges

**What if `.gitattributes` did NOT exist?**

Without `.gitattributes`, there is no standard way to communicate to GitHub/GitLab that a file is machine-generated. Two consequences would follow.

First, every PR that changes the OpenAPI spec also shows hundreds of lines of `openapi.gen.go` changes — expanded, inline, in the reviewer's face. Reviewers must manually know to skip this file. New team members don't know this. They leave comments on generated code. PR cycles lengthen as reviewers wade through irrelevant noise.

Second, language statistics include generated code — the repo appears to have 30% more Go code than was actually authored. This skews automated analysis tools, security scanners (which may flag generated code patterns as intentional), and project metrics.

The only workarounds would be: add `openapi.gen.go` to `.gitignore` (losing the benefits of committed generated files) or rely entirely on team conventions ("everyone agrees to skip these files in PRs"). Team conventions are invisible, not enforced, and lost when team members change. `.gitattributes` encodes the convention in the repository itself — it's self-documenting and automatic.

---

**What if code generators were NOT deterministic?**

If oapi-codegen produced different output for the same input on different runs (e.g., randomly ordered type declarations, different formatting each time), the CI assertion would be impossible. CI would run the generator, produce a different file from the one committed, and always fail — even when the spec hasn't changed.

Determinism is the foundational property that makes the entire workflow work. It's also why committed generated files are trustworthy — if you run the generator with the same input and version, you get the same file. You can verify any committed file at any time by regenerating it.

This tells you something important about code generators in general: determinism is not optional — it's a design requirement. A code generator that produces non-deterministic output cannot be used in this workflow and should be reported as a bug to its maintainers.

---

**What if developers always remembered to run `task gen` before committing?**

The CI assertion would never fail. The workflow would be: change spec → regenerate → commit → push → CI passes.

But "always remember" is not a valid engineering assumption. Human memory is unreliable, especially:
- Under time pressure (rushing to meet a deadline)
- After returning from a break (forgot the local state)
- During a complex rebase or cherry-pick (regeneration interrupted)
- For new team members (don't know the workflow yet)

The CI assertion exists precisely because human reliability degrades under these conditions. It's the same reason seatbelt reminders exist in cars even though drivers know they should wear seatbelts. The reminder (CI failure) catches the case where memory fails.

Engineering rule: any process that depends on humans remembering to do something eventually fails. The correct response is to automate the check. The CI assertion automates the check.

---

### c) Page FAQ

**Q: Why can't Git automatically detect that a file is generated and handle it differently?**
The reason this works this way is that Git does not understand the semantics of files — it only tracks bytes. There is nothing in the content of `openapi.gen.go` that makes it structurally different from `handler.go` to Git's file tracking algorithm. The `// Code generated by oapi-codegen` comment at the top of generated files is a hint to humans, not a signal to Git. `.gitattributes` is the mechanism Git (and hosting platforms) use to receive semantic metadata about files — and it requires explicit configuration because Git cannot infer it.

**Q: Can I add `linguist-generated=true` to files that are partially generated and partially hand-edited?**
The reason this works this way is that `linguist-generated=true` is binary — a file is either marked as generated or it isn't. If a file is partially generated and partially hand-edited, marking it as `linguist-generated` will collapse it in PR diffs, hiding the hand-edited portions from reviewers. This is dangerous. The correct solution is to never mix generated and hand-authored code in the same file — keep them separate so the attribute can be applied cleanly.

**Q: `git status --porcelain` is empty, but I know the generated file is stale. How?**
The reason this works this way is that `git status` compares the COMMITTED file to the WORKING TREE file (the file on disk). If both are stale (the spec changed but you haven't run the generator YET, and you haven't committed the stale file), `git status` sees no difference — both the committed and working-tree versions are the same (old) file. The CI assertion only catches cases where CI generates a NEW output that differs from the COMMITTED file. This is why CI must run `go generate ./...` BEFORE checking `git status` — it creates the divergence that the check detects.

**Q: Does the `linguist-generated` attribute affect anything outside of GitHub/GitLab?**
The reason this works this way is that `linguist-generated` is a platform-specific attribute understood by GitHub's Linguist library and GitLab's equivalent. Plain Git (the command-line tool) ignores attributes it doesn't know about. The `git diff`, `git log`, and `git blame` commands still show generated files in full — `.gitattributes` only affects how HOSTING PLATFORMS display the files. If you want `git diff` to ignore a file, that requires a different attribute (e.g., a custom merge driver or diff filter), which is a more complex setup.

**Q: What happens if I commit `.gitattributes` but the CI runner doesn't check it?**
The reason this works this way is that the CI assertion (the `git status --porcelain` check) does not depend on `.gitattributes` — it depends only on comparing the freshly generated file to the committed file. `.gitattributes` only affects how GitHub displays files in the UI. The CI assertion is a separate mechanism. You can have one without the other: `.gitattributes` without the CI assertion (good PR experience, but no safety net); the CI assertion without `.gitattributes` (safety net, but noisy PRs); both (the recommended setup from this page).

---

## SECTION 7 — JARGON BUSTER DICTIONARY

---

**Term: `.gitattributes`**
First-Principles Origin: Developers needed a file-level metadata system in Git to communicate to tools and platforms how specific files should be handled — without encoding that logic into the file contents themselves.
Meaning: A plain text file at the repository root. Each line maps a file pattern to an attribute. Git and hosting platforms (GitHub, GitLab) read this file to apply special behaviors to matching files.
Analogy: A label printer at a warehouse. You print labels like "Fragile," "Handle with Care," or "Return to Sender" and stick them on boxes. The warehouse handles labeled boxes differently — but the label doesn't change what's inside the box.
Example:
```gitattributes
# Match all .gen.go files at any directory depth
# Apply the linguist-generated attribute to each
**/**.gen.go linguist-generated=true
```
Don't Confuse With: `.gitignore` — that file tells Git NOT to track certain files. `.gitattributes` tracks the files normally but tells platforms how to TREAT them. `.gitignore` = invisible to Git. `.gitattributes` = tracked by Git, displayed differently.

---

**Term: CI (Continuous Integration)**
First-Principles Origin: Development teams needed a way to automatically verify that every change to a shared codebase is correct before it's merged — because humans forget to check, and manual checks don't scale.
Meaning: A system (like GitHub Actions) that automatically runs tests, builds, and checks every time code is pushed to a repository. If any check fails, the pull request is blocked from merging.
Analogy: A quality control inspector at a factory who checks every product as it comes off the line — before it's shipped. The inspector is automatic, never tired, and checks every single item.
Example:
```yaml
# .github/workflows/ci.yml
- name: Run checks
  run: |
    go test ./...              # Run all tests
    go generate ./...          # Regenerate generated files
    git status --porcelain     # Assert working tree is clean
```
Don't Confuse With: CD (Continuous Deployment) — CI checks that the code is correct; CD deploys it to production automatically after CI passes. This page is about CI only.

---

**Term: Determinism (in code generation)**
First-Principles Origin: Software tools needed to produce verifiable, reproducible output — so that a generated file could be compared to a freshly generated version and any difference would be meaningful.
Meaning: A code generator is "deterministic" when it always produces exactly the same output for exactly the same input. Run it twice with the same source file and you get identical results.
Analogy: A photocopier. Put the same document in twice and you get two identical copies. If the copies ever differ, the machine is broken — or the document changed between copies.
Example:
```
openapi.yaml (version 1) → oapi-codegen → openapi.gen.go (version A)
openapi.yaml (version 1) → oapi-codegen → openapi.gen.go (version A)  ← identical
openapi.yaml (version 2) → oapi-codegen → openapi.gen.go (version B)  ← different (source changed)
```
Don't Confuse With: Idempotency — that means running something multiple times has the same effect as running it once (e.g., setting a field to a value). Determinism means the output is always the same for the same input. These often go together but are distinct concepts.

---

**Term: `git status --porcelain`**
First-Principles Origin: Scripts needed a reliable, machine-readable way to check if a Git working tree has any uncommitted changes — without parsing the verbose human-readable `git status` output.
Meaning: A Git command that outputs nothing if the working tree is clean (all changes committed), or one line per changed file if anything is modified, added, or deleted. "Porcelain" means "designed for scripts" — the output format is stable and predictable.
Analogy: A "check engine" light in a car. No light = everything is fine. Light on = something needs attention. It doesn't explain the problem in detail — just signals "clean" or "not clean."
Example:
```bash
# Clean working tree:
$ git status --porcelain
[no output]

# Modified file:
$ git status --porcelain
 M backend/orders/api/http/openapi.gen.go
# ↑ "M" = Modified, space before = modified in working tree (not staged)
```
Don't Confuse With: `git status` (without `--porcelain`) — this outputs a verbose, human-readable message. `--porcelain` is for scripts; without it, scripts would need to parse English sentences like "nothing to commit, working tree clean."

---

**Term: `go generate` / `go generate ./...`**
First-Principles Origin: Go needed a standard way to run code generation tools as part of the build process — without each project inventing its own convention.
Meaning: A Go command that finds and runs all `//go:generate ...` directives in `.go` files. `./...` means "in all packages in this module and all subdirectories." Used to run oapi-codegen, protoc, mockgen, and other generators.
Analogy: A factory supervisor walking through every department saying "run your machine now." Each department (package) has a machine (generator) waiting with instructions. The supervisor just triggers them all in sequence.
Example:
```go
// In openapi.go — the directive that triggers generation:
//go:generate oapi-codegen --config oapi-codegen.yaml openapi.yaml

// Running all directives:
// $ go generate ./...
// → Go finds this comment, runs: oapi-codegen --config oapi-codegen.yaml openapi.yaml
// → oapi-codegen reads the config and YAML, writes openapi.gen.go
```
Don't Confuse With: `go build` — that compiles Go code. `go generate` runs EXTERNAL tools to produce Go code that will later be compiled. `go generate` runs before `go build`.

---

**Term: `.gitignore`**
First-Principles Origin: Developers needed to exclude build artifacts, local configuration, and large binary files from version control — because committing them would pollute the repository with files that shouldn't be shared.
Meaning: A plain text file listing file patterns that Git should NOT track. Files matching these patterns are "invisible" to Git — they can't be committed, staged, or pushed.
Analogy: A "do not photograph" sign at a venue. Files matching `.gitignore` patterns are like areas of the venue marked "no photos" — they exist, but you can't commit a photo (file) of them to your album (repository).
Example:
```gitignore
# Common .gitignore entries:
*.exe           # Windows executables
/tmp/           # Local temporary directory
.env            # Local environment variables (secrets)
# NOT in this project's .gitignore:
# openapi.gen.go — we COMMIT generated files (see Concept 4)
```
Don't Confuse With: `.gitattributes` — `.gitignore` tells Git to NOT track files. `.gitattributes` tells Git HOW to track/display files that ARE tracked. Opposite jobs.

---

**Term: GitHub Linguist**
First-Principles Origin: GitHub needed to automatically detect which programming languages a repository uses, for display purposes and statistical analysis — without requiring developers to declare this manually.
Meaning: An automatic tool GitHub runs on every repository to analyze which files contain which programming languages. It uses file extensions, content heuristics, and `.gitattributes` to classify files and generate the language bar shown on the repository homepage.
Analogy: A librarian who walks through the library and categorizes books by genre. If you stick a label on a book saying "Reference — do not count in fiction statistics," the librarian respects the label.
Example:
```
Repository language bar on GitHub:
BEFORE .gitattributes:    ██████████████ Go 85% | ████ YAML 15%
                          (includes generated Go code — inflated)

AFTER .gitattributes:     ████████████ Go 78% | █████ YAML 22%
                          (generated Go files excluded from stats)
```
Don't Confuse With: A linter or static analyzer — Linguist only classifies languages and respects `linguist-generated` for display. It doesn't check code quality or find bugs.

---

**Term: Merge Conflict**
First-Principles Origin: Version control systems needed to handle the case where two developers changed the same part of the same file on separate branches — and the computer couldn't automatically determine which change to keep.
Meaning: A situation in Git where two branches have different changes to the same lines of the same file. Git can't automatically pick one — it marks the file with conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`) and requires a human to decide.
Analogy: Two chefs both changing the same recipe at the same time. Chef A increases the salt. Chef B decreases the salt. The recipe now has two versions of the same line. A third person must decide which version (or which combination) is correct.
Example:
```
<<<<<<< HEAD (your branch — "increase salt to 2 tsp")
type CustomerUUID = common.UUID
=======   (separator)
type CustomerUUID = openapi_types.UUID
>>>>>>> main (the other branch — "original type")
```
Don't Confuse With: A compile error — a merge conflict is a Git-level problem (two human changes collide); a compile error is a Go-level problem (the code is syntactically or semantically invalid). A merge conflict in `openapi.gen.go` causes compile errors because the conflict markers are not valid Go syntax.

---

**Term: `linguist-generated=true`**
First-Principles Origin: Developers needed a way to signal to GitHub Linguist that specific files are machine-generated output — so the platform could treat them differently from human-authored code in PR diffs and language statistics.
Meaning: A `.gitattributes` attribute that tells GitHub: "This file was produced by a tool, not written by a human." GitHub responds by collapsing it in PR diffs by default and excluding it from language statistics.
Analogy: A "machine-made" label on clothing. The label doesn't change the fabric — but stores (GitHub) display machine-made items differently from handmade artisan items, and handcraft statistics exclude them.
Example:
```gitattributes
**/**.gen.go linguist-generated=true
# Result in GitHub PR "Files changed" tab:
# ▸ openapi.gen.go [+223 / -180] ← collapsed (click to expand)
```
Don't Confuse With: `linguist-vendored=true` — that's for third-party vendor code (like a bundled library). `linguist-generated=true` is for code produced by your own toolchain. Both collapse in PRs; they differ in semantic meaning.

---

**Term: Pull Request (PR)**
First-Principles Origin: Teams needed a controlled process for reviewing and merging individual developers' changes into the shared codebase — so that bugs could be caught before they reached main.
Meaning: A request to merge one branch's changes into another branch (usually `main`). GitHub displays a "Files changed" tab showing every line that changed. Reviewers read, comment, approve, or reject. CI checks run automatically.
Analogy: A document sent to a manager for review before it goes to print. The manager reads it, makes comments ("fix this paragraph"), approves or rejects. The document is only printed (merged) after approval.
Example:
```
Your branch:  feature/add-phone-number
Target branch: main

PR "Files changed" tab:
  handler.go              [+20 / -5]    ← reviewers see and comment
  openapi.gen.go          [+223 / -180] ← collapsed (linguist-generated)
  openapi.yaml            [+15 / -3]    ← reviewers see and comment
```
Don't Confuse With: A commit — a commit is a saved snapshot on YOUR branch. A PR is a request to move your commits into someone else's branch. Multiple commits can be part of one PR.

---

## SECTION 8 — RETENTION & REVISION PLAN

### a) The "Right Now" Rule — Do This Before Closing the Page

1. **Write from memory:** Write the single `.gitattributes` line that marks all `.gen.go` files as generated. Then explain out loud: what does it do, what does it NOT do, and which axiom each part relies on.
2. **Reconstruct the CI assertion:** Without looking, write the four-line bash script that runs the generator and checks `git status --porcelain`. State which axiom each line relies on.
3. **Debug this scenario:** A developer changes `openapi.yaml`, runs `task gen`, but commits only `openapi.yaml`. They push and open a PR. CI fails. On paper: trace EXACTLY what `git status --porcelain` outputs after CI runs `go generate ./...`, and why.
4. **Explain the trade-off:** In one paragraph from memory, explain why committing generated files is better than gitignoring them — without using the words "better" or "worse." Use axioms only.

---

### b) 3-Day Revision Checklist (Axiom-Level Mastery)

- [ ] I can write the `.gitattributes` line from memory, state its 5 axioms, and explain what it does NOT do (prevent conflicts) — all without notes.
- [ ] I can describe the correct merge conflict resolution for `openapi.gen.go` (3 steps: fix source → regenerate → commit), explain which axiom makes hand-merging always wrong, and state why `.gitattributes` doesn't help with this — all without notes.
- [ ] I can write the CI assertion script from memory, trace what `git status --porcelain` outputs in each of the 4 scenarios from the Comprehension Task, and explain why the generator version must match — all without notes.
- [ ] I can state the commit-vs-gitignore trade-off using Axioms 1–5 of "Why Commit," with a concrete example for each axiom — all without notes.

---

### c) 7-Day Challenge

**Scenario:** Your team is adding a second code generator: `mockgen`, which generates mock implementations of Go interfaces for testing. The generated files are named `mock_*.go`.

**Your challenge:**
1. Starting from the problem statement: what problems does the team face if they do nothing special for `mock_*.go` files?
2. State the axioms that apply to BOTH `openapi.gen.go` and `mock_*.go` (same axioms, different file patterns).
3. Write the updated `.gitattributes` that covers both file types.
4. Update the CI assertion to run BOTH generators and check the working tree.
5. What happens if `mockgen` is NOT deterministic (produces different output for the same source on different runs)? Which axiom breaks and what is the consequence for the CI assertion?

**Success looks like:** A written design doc with the `.gitattributes` update, the CI script, and a paragraph on the determinism consequence — all without running any code.

---

### d) 30-Day Connection Bridge

- **Every future generated file** in the project (protobuf, mockgen, migration lists) will benefit from the `.gitattributes` pattern on this page. The shared axiom: generated files are machine output and must be labeled as such for the team's tools.
- **Every future PR** you open on this project will show clean diffs — only authored code visible by default. The shared axiom: reviewer attention is finite; eliminate noise to increase review quality.
- **Every future CI pipeline** you configure in your career will likely have a "check for uncommitted changes" step like the one on this page. The shared axiom: deterministic generators enable assertions; assertions catch human error.
- **The version pinning principle** (`go.mod` tool pins) applies to ALL tools in the Go ecosystem (linters, formatters, generators). The shared axiom: reproducible builds require exactly pinned tool versions.
- **The commit-vs-gitignore trade-off** will come up again for SQL migration files, protobuf files, and OpenAPI client code. The shared axiom: committing generated output reduces required setup steps at the cost of managing drift — always choose the approach that reduces cognitive load for the whole team.

---

### e) Flashcard Set

**Card 1**
FRONT: Why does `.gitattributes` exist? What problem did developers face without it?
BACK: GitHub/GitLab had no way to know which files were machine-generated vs. human-authored. Without `.gitattributes`, generated files appeared expanded in PR diffs (wasting reviewer attention — Axiom 1) and inflated language statistics. `.gitattributes` + `linguist-generated=true` signals "this is machine output" — the platform collapses it in diffs by default (Axiom 2).

**Card 2**
FRONT: What does `.gitattributes` NOT do that beginners commonly think it does?
BACK: It does NOT prevent merge conflicts (Axiom 5 of `.gitattributes`). Merge conflicts in generated files still happen. `.gitattributes` only changes how GitHub DISPLAYS the file — it does not change how Git tracks, merges, or stores it (Axiom 4). The correct merge conflict resolution is always: fix the source → regenerate → commit.

**Card 3**
FRONT: A conflict appears in `openapi.gen.go`. What is the correct resolution — step by step?
BACK: Step 1: Identify and resolve the conflict in `openapi.yaml` (the source file — Axiom 2: the generated conflict is a symptom). Step 2: Run `task gen` (Axiom 1: the generator produces the one correct output from the merged source — Axiom 3). Step 3: Stage and commit both `openapi.yaml` and the freshly generated `openapi.gen.go`. NEVER hand-edit `openapi.gen.go` (Axiom 4).

**Card 4**
FRONT: Why is the generator tool version critical for the CI assertion? What happens if CI uses a different version than local development?
BACK: Axiom 4 of CI Assertion — the assertion compares CI's freshly generated output to the committed file. If CI uses a different generator version, even an unchanged source produces different output. `git status --porcelain` is non-empty → assertion fails EVERY TIME, even on unrelated PRs. Fix: pin the generator version in `go.mod` (Axiom 5) so CI and local use the same binary.

**Card 5**
FRONT: Write from scratch the glob pattern in `.gitattributes` and explain what each part does.
BACK:
```gitattributes
**/**.gen.go linguist-generated=true
```
- `**` — match any number of directory levels (Axiom 3)
- `**` — match any file name prefix
- `.gen.go` — the generated file suffix used by oapi-codegen and similar tools
- `linguist-generated=true` — tell GitHub Linguist: collapse in PR diffs, exclude from language stats (Axiom 2)

**Card 6**
FRONT: What does `git status --porcelain` output when: (a) the working tree is clean, (b) `openapi.gen.go` has been modified but not committed?
BACK: (a) Clean: [no output — empty string]. (b) Modified file: ` M backend/orders/api/http/openapi.gen.go`. The CI assertion checks `[ -n "$(git status --porcelain)" ]` — if non-empty (case b), the check triggers, the build fails. This is Axiom 2 of CI Assertion: `--porcelain` is empty when clean, non-empty when something changed.

**Card 7**
FRONT: Why should generated files be committed rather than added to `.gitignore`?
BACK: Axiom 3 of "Why Commit": committed files are present after `git clone` — build works immediately, no setup step required. Axiom 2: gitignored files require an extra step (`go generate ./...`) that new developers may not know about. Axiom 4: committed files preserve history — `git log openapi.gen.go` shows how generated code evolved. The drift risk is managed by the CI assertion (Axiom 5).

**Card 8**
FRONT: What does the CI assertion actually CHECK and HOW?
BACK: The CI assertion checks that the committed generated files exactly match what the generator would produce from the current source. HOW: (1) CI runs `go generate ./...` — regenerates all generated files fresh (Axiom 1: deterministic). (2) CI runs `git status --porcelain` — if the freshly generated file differs from the committed file, output is non-empty. (3) If non-empty: `exit 1` — build fails (Axiom 3). The developer must regenerate locally and commit the updated file.

**Card 9**
FRONT: A developer hand-edits `openapi.gen.go` directly and commits it. What happens on the next `task gen` run?
BACK: The hand-edit is overwritten. `task gen` completely rewrites `openapi.gen.go` from `openapi.yaml` — the generator doesn't know about the hand-edit and doesn't preserve it. Additionally, the CI assertion would have caught the hand-edit BEFORE it reached main: CI regenerates the file from the (unchanged) spec → produces the "correct" version without the hand-edit → `git status --porcelain` shows a diff → build fails. Never hand-edit generated files (Axiom 1: the generator is the single source of truth for its output).

**Card 10**
FRONT: What problem does this solve? `if [ -n "$(git status --porcelain)" ]; then echo "ERROR: ..."; exit 1; fi`
BACK: It catches the case where a developer committed changes to `openapi.yaml` (or another generator source) WITHOUT running `task gen` to update the generated output. Without this assertion, the spec and generated code would silently drift apart — the spec says one thing, the Go types say another. The assertion makes the CI pipeline fail, blocking the PR from merging until the developer regenerates and commits the updated file. This is Axiom 3 of CI Assertion: the assertion is the safety net that makes the "commit generated files" approach safe.

---

## SECTION 9 — QUICK REFERENCE CHEAT SHEET

### `.gitattributes` — File and Syntax

```gitattributes
# Place this file at the REPOSITORY ROOT (same level as go.mod)
# Commit it so the whole team gets it automatically

# Mark all .gen.go files as generated — collapses in GitHub PR diffs
**/**.gen.go linguist-generated=true

# For other common generated file types (if needed):
# **/**.pb.go     linguist-generated=true   # protobuf
# **/mock_*.go    linguist-generated=true   # mockgen
```

**What this does:**
- Collapses `.gen.go` files in GitHub "Files changed" tab by default
- Excludes them from language statistics on the repo homepage
- Does NOT prevent merge conflicts (the most important "does NOT" to remember)
- Applies to all developers automatically (it's committed)

---

### Correct Merge Conflict Resolution for Generated Files

```bash
# NEVER hand-edit openapi.gen.go during a merge conflict.

# CORRECT WORKFLOW:
# 1. Resolve the SOURCE file conflict
#    (e.g., openapi.yaml — the human-authored input)

# 2. Run the generator
task gen   # or: go generate ./...

# 3. Stage everything — the source AND the generated output
git add backend/orders/api/http/openapi.yaml
git add backend/orders/api/http/openapi.gen.go

# 4. Complete the merge
git commit
```

---

### CI Assertion Script

```bash
# Run inside your CI workflow after checkout:

# Step 1: Regenerate all generated files
go generate ./...   # or: task gen

# Step 2: Assert nothing changed (generator output matches what's committed)
if [ -n "$(git status --porcelain)" ]; then
  echo "ERROR: Generated files are out of date."
  echo "Run 'go generate ./...' locally and commit the updated files."
  git diff   # Show what changed (for debugging)
  exit 1     # Fail the CI build — block the PR
fi
echo "OK: All generated files are up to date."
```

**Requires:** CI must use the SAME generator version as local development. Pin it in `go.mod`:
```go
tool (
    github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen v2.4.1
)
```

---

### Key Rules / Gotchas

- **`.gitattributes` must be committed** — otherwise GitHub never sees it (runs as if it doesn't exist)
- **Use `**/**.gen.go`** not `*.gen.go` — the `**/` prefix is required to match files in nested directories
- **`linguist-generated` ≠ merge strategy** — conflicts still happen; only the display changes
- **CI generator version must match local** — version mismatch causes the assertion to always fail (false positives)
- **Never hand-edit generated files** — any change is overwritten on the next `task gen`
- **`go generate ./...` before committing** — CI assertion is a safety net, not a replacement for good workflow
- **Commit generated files** — do NOT gitignore them; `git clone` should always produce a buildable project

---

### Decision Table: Commit vs. Gitignore Generated Files

| Factor | Commit | Gitignore |
|---|---|---|
| `git clone` → build immediately | YES | NO (need `go generate` first) |
| History of generated changes | YES (`git log` shows it) | NO |
| CI assertion possible | YES | POSSIBLE (but CI regenerates instead of asserting) |
| Setup steps for new developers | None | Must run `go generate ./...` before building |
| Risk of drift (spec vs. output) | Caught by CI assertion | Caught by compile error when building |
| Recommended for this project | ✓ YES | ✗ Not recommended |

---

### One-Line Plain-English Reminders

- **`.gitattributes`:** "A label on a file that tells GitHub: treat this as machine output, not human code."
- **`linguist-generated=true`:** "Collapse this file in PR diffs by default — reviewers can expand if needed."
- **Merge conflict in generated files:** "Never hand-merge. Fix the source, regenerate, commit."
- **Commit generated files:** "So `git clone` always produces a buildable project — no setup steps."
- **CI assertion:** "Run the generator in CI, then assert nothing changed — catches developers who forgot to regenerate."
- **`git status --porcelain`:** "Empty = clean. Non-empty = something changed. The assertion's single check."
- **Tool version pinning:** "CI and local must use the same generator version — pin it in `go.mod`."
