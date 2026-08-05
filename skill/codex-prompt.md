# Optioner instructions for Codex (AGENTS.md block)

Codex has no skill system; it reads `AGENTS.md` (repo root, or `~/.codex/AGENTS.md`
for all projects). Append the block below to whichever file fits your setup.
It is the same contract as `skill/SKILL.md`, condensed for a system-prompt context.

---

```markdown
## Getting human decisions (Optioner)

When you need the human to decide something — picking between options, a
yes/no call, a scope/tech/design judgement — do NOT present a long bullet
list or ad-hoc HTML. Use Optioner: write a spec JSON and serve it as a
consistent decision page in their browser.

1. Write `/tmp/optioner-spec.json`:

   {
     "optioner": 1,
     "title": "Short decision title",
     "context": "Markdown background. Tables, code, links are fine.",
     "questions": [
       {"id": "db", "type": "single_choice", "prompt": "Which DB?",
        "options": [
          {"label": "SQLite", "description": "Markdown details", "recommended": true},
          {"label": "Postgres", "description": "Markdown details"}
        ],
        "allow_other": true}
     ]
   }

   Types: free_text, yes_no, single_choice, multi_choice. Choice types need
   >= 2 options; options/allow_other are for choice types only. "optioner"
   is the integer 1 (not 1.0). ids match ^[a-z][a-z0-9_]*$ and are unique.
   Unknown fields are rejected. Keep "prompt" short and plain; put detail
   in "context" and option "description" (both Markdown).

2. Launch it in the background — it blocks until the human finishes:

   npx @doskoi64/optioner /tmp/optioner-spec.json
   # or, from a clone of the optioner repo:
   go run ./cmd/optioner /tmp/optioner-spec.json

   Flags go BEFORE the file (a trailing flag is a usage error). The first
   stdout line is the URL; the browser opens automatically. Tell the human
   to answer, hit Copy, and paste the result back — and give them the URL.

3. Read the result: the pasted text (and the process's stdout on exit)
   ends with a ```json block {"optioner":1,...,"answers":{...}}. Values by
   question id: string, boolean (yes_no), or string array (multi_choice).
   A skipped question is absent from "answers" — treat as "no opinion".

4. A rejected spec exits 1 with the reason on stderr; fix exactly what the
   message says and rerun. Two prefixes, both self-repairable:
   "spec invalid:" (content — all problems listed at once) and
   "spec is not valid JSON:" (syntax / unknown field / 1.0-style number).

5. Optioner shuts down with the page (Done = immediate; closed tab =
   heartbeat loss, default 2m). A page nobody ever opens waits forever —
   kill the background process if you abandon the question.
```
