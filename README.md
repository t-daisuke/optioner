# Optioner

**Consistent decision pages for AI agent sessions.**

Working with Claude Code / Codex, you constantly hit decision points:
which DB, which approach, ship or wait. Agents love to improvise HTML
for these — different every time, and sometimes just broken.

Optioner fixes the format. The agent writes a small spec JSON; Optioner
renders it as a consistent decision page in your browser; you answer,
hit **Copy**, and paste the result back to your agent.

## Install

Nothing to install — run it straight from npm (macOS Apple Silicon):

```sh
npx @doskoi64/optioner spec.json
```

Try it in 30 seconds:

```sh
curl -sO https://raw.githubusercontent.com/t-daisuke/optioner/main/examples/all-types.json
npx @doskoi64/optioner all-types.json    # a browser tab opens — answer, Copy, Done
```

From source, on any platform Go runs on (Go 1.25+):

```sh
git clone https://github.com/t-daisuke/optioner.git && cd optioner
go run ./cmd/optioner examples/db-choice.json
```

**Let your AI agent use it by itself:**

- **Claude Code**: copy the skill into your skills directory —
  `mkdir -p ~/.claude/skills/optioner && curl -so ~/.claude/skills/optioner/SKILL.md https://raw.githubusercontent.com/t-daisuke/optioner/main/skill/SKILL.md`
- **Codex**: append the block in [`skill/codex-prompt.md`](skill/codex-prompt.md) to your `AGENTS.md` (repo-local or `~/.codex/AGENTS.md`)

After that, the agent reaches for Optioner on its own whenever a
decision comes up.

## Usage

- Starts on a random free port and opens your browser
- Answer, **Copy** (Markdown summary + machine-readable JSON), **Done**
- On exit the answers are also echoed to stdout, so agents can read them directly
- The server dies with the page: **Done** ends it immediately; a closed
  tab is detected by heartbeat loss. A page that was never opened waits
  until you kill it
- Auto-opening the browser uses macOS `open`; elsewhere, open the
  printed URL yourself

### Flags

Flags go **before** the spec file; a trailing flag is a usage error (exit 2).

| Flag | Default | |
|---|---|---|
| `-no-open` | off | Don't open a browser. The URL is still printed. |
| `-heartbeat-timeout` | `2m` | Shut down this long after the page stops checking in. |
| `-version` | | Print the version and exit. |

On start, stdout line 1 is the URL; errors go to stderr.
Exit codes: `0` done, `1` invalid spec, `2` usage error.

## Spec format

```json
{
  "optioner": 1,
  "title": "Choose a database",
  "context": "## Background\nWe need to persist user data for the new feature.",
  "questions": [
    {
      "id": "db",
      "type": "single_choice",
      "prompt": "Which DB should we use?",
      "options": [
        {"label": "SQLite", "description": "**Zero config.** Single file, easy backup.", "links": ["https://www.sqlite.org/"], "recommended": true},
        {"label": "Postgres", "description": "Production grade, needs a server process."}
      ],
      "allow_other": true
    }
  ]
}
```

Four question types: `free_text`, `yes_no`, `single_choice`, `multi_choice`.
Choice types take >= 2 options and an optional `allow_other` (a free-text
escape hatch). Text fields (`context`, `prompt`, `description`) accept Markdown.
Unknown fields are rejected.

Full definition in [`schema/optioner.schema.json`](schema/optioner.schema.json),
more samples in [`examples/`](examples/).

## What you paste back

````
## Optioner answers: Choose a database

- **Which DB should we use?** — SQLite

```json
{"optioner":1,"title":"Choose a database","answers":{"db":"SQLite"}}
```
````

The bullets are for you; the JSON block is for your agent. Skipped
questions show as `(no answer)` and are omitted from `answers`.

## License

MIT
