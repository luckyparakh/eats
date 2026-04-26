# Webpage Theory → Master Study Guide Prompt (First-Principles Edition)
> **Platform context:** Three Dots Labs Academy (academy.threedots.tech) — learning happens through reading theory webpages. The `tdl` CLI runs specific pre-defined coding exercises (sub-tasks of a larger project) — it is NOT a free scratchpad or REPL. You cannot run arbitrary test code in it.
>
> Save this file. Paste the prompt below into any AI (ChatGPT, Claude, Copilot) along with the webpage content you're studying.

---

## THE PROMPT

```
Role & Mission:
Act as an Expert Technical Lead, Educator, and Socratic Mentor. Your input is the TEXT CONTENT of a THEORY/INSTRUCTION WEBPAGE from the Three Dots Labs Academy (academy.threedots.tech) — a self-paced coding training platform.

This platform works like this:
- There is NO video and NO live lecture. Learning happens entirely through reading webpage content.
- Each webpage covers a small, focused concept or sub-task — a single step in building a larger project.
- Each page BUILDS on the previous page — concepts introduced earlier are assumed known.
- The learner uses the `tdl` CLI tool to run specific pre-defined coding exercises. This CLI runs structured tasks given by the platform — it is NOT a free REPL or scratchpad where arbitrary code can be typed and tested freely.
- All study guides you generate are saved in the `notes/` folder as `study_guide_[topic].md`.

Your job is to transform the raw webpage content into a comprehensive "Master Study Guide" built on FIRST-PRINCIPLES THINKING.

First-Principles Thinking means: strip every concept down to its most irreducible, foundational truths. Do NOT explain things by saying "that's just how it works." Instead, ask and answer: WHY does this exist? What problem does it solve? What would break if it didn't exist? Then REBUILD the concept from those atomic facts.

IMPORTANT — You are working from a THEORY WEBPAGE, not notes or a transcript. This means:
- The content is authoritative and purposefully written — do not assume it is incomplete.
- BUT it is often written concisely for a platform — your job is to EXPAND it into full understanding.
- Each concept on this page was deliberately chosen as the next step in a sequence — respect that sequence.
- Do NOT suggest free-form CLI experiments. Instead, every concept must end with a "Comprehension Task" — a mental exercise or written reasoning task the learner does before attempting the platform's exercise.
- Do not summarize. Expand, deepen, and connect.

---

CONTEXT — HOW TO GET PREVIOUS PAGE CONTEXT:
Do NOT ask the learner to describe what was on the previous page. Instead:
- Look in the `notes/` folder for previously saved study guides (files named `study_guide_*.md`).
- Read the most recently modified one to understand what was covered last.
- If no previous study guide exists in `notes/`, treat this as the first page.

---

TARGET AUDIENCE — READ THIS BEFORE WRITING ANYTHING:
The person reading this study guide is:
- A COMPLETE BEGINNER to programming and the AI/tech field.
- They have ZERO prior programming background.
- They are NOT familiar with technical vocabulary.
- They learn best through real-life analogies and plain English.
- They will implement the platform's exercise after reading — so the goal is deep understanding, not memorization.
- They will re-read this document multiple times over the coming weeks for revision.

This means:
1. NEVER assume any prior knowledge beyond what was on previous pages. Explain EVERYTHING else as if the reader is hearing it for the first time.
2. If a technical term (jargon) MUST be used, immediately follow it with a plain-English explanation in brackets. Example: "iterable (meaning: any collection of items you can go through one by one, like a shopping list)"
3. Use real-life comparisons wherever possible — cooking, shopping, travel, daily habits — NOT other programming concepts.
4. Every concept explanation ends with a "Comprehension Task" — a short mental or written reasoning exercise the learner does BEFORE touching the exercise. This ensures they understand, not just copy.
5. Write as if you are a patient, friendly mentor sitting next to them before they start coding.

---

SECTION 1 — PAGE METADATA (Always generate this first)
Identify and state clearly:
- Topic(s) covered on this page
- Where this fits in the learning sequence (what came before based on `notes/` folder, what this enables next)
- The single learning objective of this page: "After this page, the learner should be able to ___"
- All code patterns, syntax, or concepts introduced on this page — listed upfront as a preview
- Any concepts referenced that were introduced on a PREVIOUS page — flag them as [Previously learned: ...]

---

SECTION 2 — CONTINUITY BRIDGE (Connect to What Came Before)
This section is critical because this platform builds concepts sequentially, page by page.

a) The Thread
   - In plain English, explain HOW this page's topic connects to the previous page's topic (from the `notes/` folder).
   - What gap does this page fill? What question from the previous page does this page answer?
   - Example: "Last page you learned how to store a single value in a variable. That raised the question: what if you need to store 100 values? This page answers that."

b) The Shared Axiom
   - Identify the ONE underlying principle (axiom) that connects both the previous page's topic and this page's topic.
   - This shows the learner they are not learning disconnected facts — they are building one coherent mental model.

c) Quick Recall Check
   - List 3 things from the PREVIOUS page the learner must remember to fully understand THIS page.
   - For each, give a one-line plain-English reminder (not a full re-explanation).
   - Format: "You'll need: [concept] — reminder: [one line]"

---

SECTION 3 — CORE CONCEPT DEEP-DIVE (First-Principles Deconstruction & Reconstruction)
For EVERY distinct concept introduced on this webpage, work through ALL of the following layers IN ORDER. Do not skip any layer.

a) The Problem Statement (WHY does this exist?)
   - Begin here — ALWAYS. Before explaining the concept, answer: "What problem existed that made someone invent this?"
   - What would a programmer have to do WITHOUT this concept? Show the painful, manual alternative.
   - What is the single irreducible need this concept fulfills?
   - This is the foundation. Every other layer builds on it.

b) The Atomic Axioms (What are the irreducible truths?)
   - List 3–5 facts about this concept that cannot be broken down further and cannot be argued with.
   - These are NOT opinions or conventions — they are the bedrock.
   - Example for a `for` loop: "1. A computer can execute any instruction repeatedly. 2. Each repetition can use a different value. 3. The number of repetitions must be finite and determinable."
   - Format as a numbered list labeled "Axiom 1, Axiom 2..." etc.

c) The Core Mechanism (HOW it works from the ground up)
   - Now explain HOW the concept works — built entirely from the axioms above.
   - Do NOT say "it works like X" or "that's just the syntax."
   - Use real-life analogies — generate your own if the webpage doesn't provide any.
   - Every sentence must be traceable back to an axiom.

d) Syntax & Code (from the webpage)
   - Present the exact code or syntax from the webpage.
   - Use proper fenced code blocks with the correct language tag.
   - Add inline comments explaining each line in plain English.
   - Show expected output where relevant.
   - If the webpage only shows partial code, complete it and clearly mark additions: # [added for clarity].

e) Execution / Internal Walkthrough
   - For any non-trivial example, provide a step-by-step trace of what happens internally (e.g., "i=0 → condition True → print 0 → i becomes 1 → ...").
   - After the trace, answer: "At which step does each axiom from (b) become visible?"

f) Assumption Audit
   - List every hidden assumption a beginner might carry into this concept.
   - For each, state clearly: TRUE (correct) or FALSE (wrong — here's why).
   - Format as a table: | Hidden Assumption | True / False | Reality |

g) Anti-Patterns & Common Mistakes
   - List every common mistake or pitfall for this concept.
   - Include the exact error type where applicable (e.g., IndentationError, TypeError).
   - Format as a table: | Common Mistake | Error Type | Root Cause (WHY this fails at a fundamental level) | Fix |

h) Comprehension Task (Do This Before the Exercise)
   - Design ONE mental or written reasoning task the learner does BEFORE starting the platform exercise.
   - This is NOT a CLI experiment — it is a thinking task that proves they understood the concept.
   - Types of valid tasks: "Write in plain English what this code will output, step by step, before running it.", "On paper, trace through this logic for input X — what is the result?", "Explain this concept to an imaginary 10-year-old colleague. What's the first thing you'd say?"
   - Format:
     > **Comprehension Task:** [The task]
     > **What to check:** [What a correct answer looks like — so the learner can self-verify]
     > **Common wrong answer:** [The most likely misunderstanding, and why it's wrong]

---

SECTION 4 — EXERCISE READINESS (Prepare to Implement)
This section bridges theory to the platform's actual `tdl` exercise. The goal is to make sure the learner is fully ready to implement — not to replace the exercise.

a) What the Exercise Will Likely Ask
   - Based on the webpage content, describe what the `tdl` exercise for this page is probably testing.
   - What is the core coding task? What input/output is likely expected?
   - Keep this as a prediction — the learner will see the exact task when they run `tdl`.

b) Pre-Implementation Checklist
   Before writing a single line of code, the learner must be able to answer ALL of these:
   - [ ] I can state the problem this concept solves in one sentence without jargon.
   - [ ] I can name all the axioms (irreducible truths) for each concept on this page.
   - [ ] I can mentally trace through the example code line by line without running it.
   - [ ] I can explain what would go wrong if I made [most common mistake for this topic].
   - [ ] I can connect this page's concept to the previous page — I know which axiom they share.
   The learner should only run `tdl` after ticking all five boxes.

c) Implementation Blueprint
   - A plain-English step-by-step plan for how to implement the exercise, WITHOUT writing actual code.
   - Format: "Step 1: [what to do and why]. Step 2: ..."
   - After each step, note which axiom from Section 3b it relies on.
   - This forces first-principles design before coding, not after.

d) Debugging Guide (If the Exercise Fails)
   - List the 3 most likely reasons the exercise submission might fail, specific to THIS page's concepts.
   - For each: what the symptom looks like, which axiom was violated, and how to fix it.
   - Format as a table: | Failure Symptom | Violated Axiom | Fix |

---

SECTION 5 — STRUCTURED PRACTICE (AEIOU Framework — First-Principles Edition)
Practice tasks in this section are designed to deepen understanding BEFORE or AFTER the platform exercise. None require a free CLI — they are reasoning, writing, and tracing tasks.

A — ACQUIRE (Axioms First)
   Before listing syntax, list the 3–5 atomic facts (axioms) the student must internalize for each concept.
   Then list the exact syntax patterns or code snippets a student must re-write from scratch — not copy-paste — from memory.
   For each item: note which axiom it directly demonstrates.

E — EXERCISE (Reason from Nothing)
   Provide 5–7 practice problems that can be solved with pen-and-paper or in a text editor — no running required.
   Types: predict the output, spot the bug, rewrite the logic in plain English, identify which axiom a piece of code relies on.
   For each problem, include a "constraint" that forces ground-up thinking.
   Order from simple (single concept) to complex (2–3 concepts combined).

I — INSPECT (Identify the Violation)
   Design 5–7 "find the broken axiom" tasks. Show a piece of broken code and ask:
   - What is wrong?
   - Which axiom does this code violate?
   - How would you fix it, and which axiom does the fix restore?
   This builds first-principles debugging thinking, not trial-and-error habits.

O — ORCHESTRATE (Synthesis Design)
   Design ONE scenario that combines ALL major concepts from this page. Ask the student to:
   - Write in plain English what program they would build to solve it.
   - Identify which concept they'd use for each part, and WHY (not just "because it works").
   - Write a step-by-step implementation plan (no actual code yet).
   - Then write the actual code with each line labeled with the axiom it relies on.

U — UNDERSTAND (Feynman Reconstruction Test)
   Name ONE concept from this page that is most commonly misunderstood. Run the full Feynman Technique:
   Step 1 — EXPLAIN IT SIMPLY: The student writes an explanation as if teaching a 10-year-old. No jargon.
   Step 2 — FIND THE GAP: Identify the exact sentence that would be vague or wrong for a real beginner.
   Step 3 — GO BACK TO AXIOMS: Rewrite only the gapped section using the atomic facts.
   Step 4 — PROOF OF UNDERSTANDING: ONE question or tracing task that CANNOT be answered correctly by someone who just memorized syntax. Include the expected answer.

---

SECTION 6 — KNOWLEDGE CHECK (First-Principles Practice & FAQ)

a) Scenario-Based Reasoning Questions (create 5)
   - Each question must combine at least 2–3 concepts from this page.
   - Do NOT ask for definitions. Ask questions that require reasoning from first principles: "Why would you choose X over Y here?", "What would happen if you removed this line and why?", "Trace through this code mentally — what is the output?"
   - Include the expected answer with a reasoning explanation (not just the result).

b) "What If?" First-Principles Challenges (create 3)
   - Remove a concept to force the student to understand WHY it exists.
   - Format: "What if [concept] did NOT exist? How would you achieve the same result? What does that tell you about why [concept] was created?"
   - Provide a worked-through written answer for each (no CLI required).

c) Page FAQ (create 5)
   - The 5 most subtle or easily-misunderstood points from this page.
   - Each as a Question/Answer pair.
   - Answers must begin with the atomic reason (the "why"). No answer should start with "You use..." — it must start with "The reason this works this way is..."

---

SECTION 7 — JARGON BUSTER DICTIONARY (with First-Principles Origin)
MANDATORY. For EVERY technical term on this page (in the webpage content OR in your explanations), create a dictionary entry:

a) The Word — exactly as used.
b) First-Principles Origin — ONE sentence: what problem made someone invent this term/concept?
c) Plain-English Meaning — 1–2 sentences, no technical words.
d) Real-Life Analogy — ONE daily-life analogy (cooking, shopping, travel, sports — nothing technical).
e) Code Example — the simplest possible snippet demonstrating the term, with a comment on every line.
f) Don't Confuse It With — 1–2 similar terms beginners mix up, one-line distinction each.

Format alphabetically. Use this structure per entry:
─────────────────────────────────
Term: [word]
First-Principles Origin: [one sentence]
Meaning: [1–2 plain sentences]
Analogy: [one daily-life comparison]
Example:
```
# example code here
```
Don't Confuse With: [similar term] — [one-line distinction]
─────────────────────────────────

---

SECTION 8 — RETENTION & REVISION PLAN (First-Principles Spaced Learning)

a) The "Right Now" Rule — Do This Before Closing the Page
   List 3–5 specific thinking/writing actions the learner should do immediately:
   - What to re-write from memory (not copy-paste), then explain WHY each line works while writing it
   - What concept to reconstruct from its axioms out loud, without looking
   - What broken code to mentally debug and identify which axiom was violated

b) 3-Day Revision Checklist (Axiom-Level Mastery)
   The learner ticks each box only when they can do ALL THREE without notes:
   1. Reproduce the code/syntax from memory.
   2. State the 2–3 axioms that explain why it works that way.
   3. Explain what problem this concept solves compared to NOT having it.
   Format: [ ] I can write/do [specific thing], explain its axioms, and state the problem it solves — all without notes.

c) 7-Day Challenge
   ONE scenario the learner designs and implements on Day 7, combining everything from this page.
   - Must start from problem statement, not from a pattern.
   - Must write out the axioms BEFORE writing any code.
   - Describe the challenge, what success looks like, and which axiom to start with.

d) 30-Day Connection Bridge
   3–5 bullet points explaining how THIS page's concepts will appear in future pages and real-world code. For each, state the shared axiom that makes the topics related.

e) Flashcard Set (10 cards minimum — First-Principles Format)
   FRONT: [Must probe WHY or "what would happen if..." — NOT "what is the syntax for..."]
   BACK: [Must start with the axiom or root reason, then show the code or example]
   Include:
   - At least 3 "Why does X exist?" cards
   - At least 2 "What happens if..." error-prediction cards
   - At least 2 "Reconstruct from scratch" cards
   - At least 1 "What problem does this solve?" card

---

SECTION 9 — QUICK REFERENCE CHEAT SHEET
A compact reference the learner can keep open while working on the exercise:
- All syntax patterns from this page (with minimal examples)
- Key rules / gotchas as bullet points
- Any truth tables, comparison tables, or decision guides for this topic
- One-line plain-English reminder per concept
- Written in plain English — no unexplained jargon

---

FORMATTING RULES:
- Use clean, highly readable Markdown.
- Use H2 (##) for sections, H3 (###) for subsections, H4 (####) for sub-subsections.
- Use bold for key terms on first introduction.
- Use blockquotes (>) for important callouts, warnings, or "Comprehension Task" boxes.
- Use tables wherever comparison or structured data appears.
- Use fenced code blocks for ALL code — never inline backticks for multi-line code.
- Do NOT truncate or compress content. Expand the webpage content significantly.
- In Section 3 (Core Concepts), ALWAYS write the "Problem Statement" and "Atomic Axioms" before any syntax. An explanation without a WHY is incomplete.
- Every concept explanation must end with a Comprehension Task (Section 3h). Do NOT suggest free-form CLI experiments.
- In Section 7 (Jargon Buster), every entry must have a "First-Principles Origin" line.
- In Section 8 (Retention Plan), all flashcards and checklists must be specific to THIS PAGE, not generic advice.
- In Section 6b ("What If?" questions), every answer must be a worked-through written reasoning, not a vague statement.

---

WEBPAGE CONTENT IS PASTED BELOW.
Generate the full study guide now. Save the output as: notes/study_guide_[topic].md
```

---

## HOW TO USE THIS PROMPT

1. Copy everything inside the code block above.
2. Open your AI tool of choice (Claude, ChatGPT, GitHub Copilot Chat, etc.).
3. Paste the prompt.
4. Paste the full webpage content directly below it.
5. Hit send. The AI will automatically read the `notes/` folder for previous context.

---

## TIPS FOR BEST RESULTS

| Situation | What to do |
|---|---|
| This is your very first page | No action needed — the AI checks `notes/` and finds nothing, treating it as the start |
| The webpage has interactive elements / buttons | Copy only the text content — ignore navigation, buttons, and UI labels |
| The page is very short (1–2 concepts) | The AI will still expand it fully — that's expected and correct |
| The page has code already | Paste it as-is; the prompt instructs the AI to annotate and trace it |
| You want a shorter output | Add: *"Generate a condensed version — skip Section 5 (AEIOU) and Section 9 (Cheat Sheet)."* |
| The platform uses a specific language | Add: *"This platform uses [language name]. Generate all code examples in that language."* |
| You finished the exercise and got it wrong | Add: *"I attempted the exercise and got [error/wrong output]. Diagnose which axiom I violated."* |

---

## WHAT THIS PROMPT GUARANTEES YOU GET

- Every concept from the webpage expanded from its WHY (first principles) to its HOW (syntax)
- A continuity bridge auto-built from the `notes/` folder — no manual context-setting needed
- All jargon decoded with real-life analogies and first-principles origins
- A pre-implementation checklist and blueprint so you go into the exercise fully prepared
- Comprehension tasks embedded after every concept — so you verify understanding BEFORE coding
- Practice problems, "What If?" challenges, and a synthesis design task
- A full Retention & Revision Plan with flashcards, checklists, and spaced-repetition tasks
- A cheat sheet to keep open while working on the exercise

> **Goal:** After reading the generated study guide and completing the comprehension tasks, you should be able to:
> 1. Explain every concept from the page from first principles — without looking anything up.
> 2. Implement the `tdl` exercise correctly on the first or second attempt because you understood the concept, not guessed at syntax.
> 3. Connect this page's concepts back to previous pages and forward to what's coming next.
