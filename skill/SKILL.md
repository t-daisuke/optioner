---
name: optioner
description: Use when you need the human to decide something — picking between options, a yes/no call, or a free-form judgement — during planning, design, or implementation. Renders a consistent decision page in the browser instead of ad-hoc HTML, and the human pastes the answers back.
---

# Optioner — ask the human through a consistent decision page

## When to use

Any time you would otherwise write ad-hoc HTML, a long bullet list of options,
or an inline questionnaire: tech choices, design directions, scope decisions.

## How

1. Write a spec JSON file (e.g. `/tmp/optioner-spec.json`):

```json
{
  "optioner": 1,
  "title": "Short decision title",
  "context": "Markdown background. Tables, code and links are fine.",
  "questions": [
    {
      "id": "snake_case_id",
      "type": "single_choice",
      "prompt": "The question, in Markdown",
      "options": [
        {"label": "Option A", "description": "Markdown details", "links": ["https://..."], "recommended": true},
        {"label": "Option B", "description": "Markdown details"}
      ],
      "allow_other": true
    }
  ]
}
```

Question types: `free_text`, `yes_no`, `single_choice`, `multi_choice`.
Choice types need >= 2 options; `allow_other` adds a free input.
`options` and `allow_other` belong to the choice types only — passing them to
`free_text` or `yes_no` is an error.

`optioner` must be the integer `1` (not `1.0`). Every `id` matches
`^[a-z][a-z0-9_]*$` and is unique within the spec. Unknown fields are rejected,
so a typo never gets silently ignored. `context`, `prompt` and option
`description` are all Markdown — but keep `prompt` a short plain question and
put the detail in `context` or in each option's `description`, because the
prompt is repeated verbatim in the summary the human pastes back.

Full schema: `schema/optioner.schema.json` in the optioner repo.

2. Run it **in the background** (it blocks until the human finishes):

```sh
npx @doskoi64/optioner /tmp/optioner-spec.json
```

From a clone of the optioner repo, `go run ./cmd/optioner /tmp/optioner-spec.json`
behaves identically — use it if the npm package is not available to you — except
that `go run` reports usage errors as exit **1** (it collapses the child's exit 2
and prints `exit status 2` as its own text), so with the fallback key on the
stderr message, not the exit code.

Flags go **before** the file — `npx @doskoi64/optioner -no-open /tmp/optioner-spec.json`.
A flag after it is a usage error (exit 2), never a silently ignored flag.

The first stdout line is the URL. The browser opens automatically.
When the process exits, the submitted answers are also echoed to stdout —
reading the background task's output is an alternative to waiting for the paste.

3. Tell the human: "I've opened a decision page — answer there, hit Copy,
   and paste the result back here." Give them the URL too, in case the
   browser did not come up.

4. The pasted text ends with a ` ```json ` block: `{"optioner":1,"title":...,"answers":{...}}`.
   Read answers by question id. Values: string (free_text / choices, including a
   typed "other"), boolean (yes_no), array of strings (multi_choice).
   A question the human skipped is **absent** from `answers` (the Markdown above
   the block shows it as `(no answer)`) — take that as "no opinion", and only
   ask again if you genuinely cannot proceed without it.

5. A rejected spec exits 1 and says why on **stderr** — stdout only ever carries
   the URL and the final answers echo, so read stderr when nothing comes up.
   There are two prefixes, and both are self-repairable:

`spec invalid:` — the JSON parsed, the content is wrong. Every problem is
listed at once, so fix them all in one pass and rerun:

```
spec invalid:
  - optioner: must be 1 (got 2)
  - questions[0].type: must be one of free_text, yes_no, single_choice, multi_choice (got "dropdown")
```

`spec is not valid JSON:` — it never parsed, so nothing was validated yet. A
syntax error, an unknown field (`json: unknown field "titel"` — a typo'd key,
never silently ignored), or a wrong JSON type for a field (`"optioner": 1.0`
is a number with a fraction, not the integer `1`). Fix what the message names
and rerun; expect the `spec invalid:` list next, if anything is still wrong.

## Notes

- Optioner shuts itself down with the page, so there is nothing to clean up
  after a normal answer: the page's **Done** button ends it at once, and a tab
  that was merely closed is noticed when its heartbeat stops
  (`-heartbeat-timeout`, default `2m`).
- A page **nobody ever opens** never starts that clock, so it waits forever:
  if you abandon the question, kill the background task yourself.
- Exit codes: `0` finished (answers echoed if any were submitted), `1` the spec
  was invalid or unreadable, `2` you called it wrong.
