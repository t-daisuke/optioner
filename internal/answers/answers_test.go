package answers

import (
	"strings"
	"testing"

	"github.com/t-daisuke/optioner/internal/spec"
)

func fixtureSpec(t *testing.T) *spec.Spec {
	t.Helper()
	s, err := spec.Load([]byte(`{
		"optioner": 1,
		"title": "Choose a database",
		"questions": [
			{"id": "db", "type": "single_choice", "prompt": "Which DB?",
			 "options": [{"label": "SQLite"}, {"label": "Postgres"}], "allow_other": true},
			{"id": "flags", "type": "multi_choice", "prompt": "Which flags?",
			 "options": [{"label": "wal"}, {"label": "fts"}]},
			{"id": "ship", "type": "yes_no", "prompt": "Ship this week?"},
			{"id": "notes", "type": "free_text", "prompt": "Notes?"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFormatClipboardTwoLayers(t *testing.T) {
	got := FormatClipboard(fixtureSpec(t), Answers{
		"db":    "SQLite",
		"flags": []any{"wal", "fts"},
		"ship":  true,
		"notes": "keep it simple",
	})
	wantParts := []string{
		"## Optioner answers: Choose a database",
		"- **Which DB?** — SQLite",
		"- **Which flags?** — wal, fts",
		"- **Ship this week?** — yes",
		"- **Notes?** — keep it simple",
		"```json",
		`"answers":{"db":"SQLite","flags":["wal","fts"],"notes":"keep it simple","ship":true}`,
	}
	for _, w := range wantParts {
		if !strings.Contains(got, w) {
			t.Fatalf("clipboard text missing %q:\n%s", w, got)
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "```") {
		t.Fatalf("machine-readable JSON block must close the text:\n%s", got)
	}
}

func TestFormatClipboardUnanswered(t *testing.T) {
	got := FormatClipboard(fixtureSpec(t), Answers{"db": "SQLite"})
	if !strings.Contains(got, "- **Notes?** — (no answer)") {
		t.Fatalf("unanswered question should read (no answer):\n%s", got)
	}
	if strings.Contains(got, `"notes"`) {
		t.Fatalf("unanswered question must be omitted from JSON block:\n%s", got)
	}
}

// The browser sends a value for every field it rendered, so an untouched
// free_text arrives as "" and an unchecked multi_choice as []. Those are not
// answers, and neither is a null: they must read the same as a missing key.
func TestFormatClipboardEmptyValuesCountAsUnanswered(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{"empty string", ""},
		{"empty slice", []any{}},
		{"null", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FormatClipboard(fixtureSpec(t), Answers{"db": "SQLite", "notes": c.value, "flags": c.value})
			for _, want := range []string{"- **Notes?** — (no answer)", "- **Which flags?** — (no answer)"} {
				if !strings.Contains(got, want) {
					t.Fatalf("empty value should read %q:\n%s", want, got)
				}
			}
			if strings.Contains(got, `"notes"`) || strings.Contains(got, `"flags"`) {
				t.Fatalf("empty value must be omitted from JSON block:\n%s", got)
			}
			if strings.Contains(got, "<nil>") {
				t.Fatalf("nil must never reach the pasted text:\n%s", got)
			}
		})
	}
}

func TestFormatClipboardKeepsFalseAndZero(t *testing.T) {
	got := FormatClipboard(fixtureSpec(t), Answers{"ship": false, "notes": "0"})
	if !strings.Contains(got, "- **Ship this week?** — no") {
		t.Fatalf("a deliberate no is an answer:\n%s", got)
	}
	if !strings.Contains(got, `"ship":false`) || !strings.Contains(got, `"notes":"0"`) {
		t.Fatalf("false and \"0\" must survive into the JSON block:\n%s", got)
	}
}

func TestFormatClipboardIgnoresUnknownIDs(t *testing.T) {
	got := FormatClipboard(fixtureSpec(t), Answers{"db": "SQLite", "bogus": "x"})
	if strings.Contains(got, "bogus") {
		t.Fatalf("unknown answer ids must be ignored:\n%s", got)
	}
}
