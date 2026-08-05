// Package answers turns submitted answers into the clipboard text:
// a human-readable Markdown summary followed by a machine-readable
// JSON block the agent can parse without guessing.
package answers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/t-daisuke/optioner/internal/spec"
)

// outputVersion is the format version of the machine-readable JSON block.
// It travels with the text so an agent can tell how to parse it; it is pinned
// to the block's own shape, not copied from the spec that was answered.
const outputVersion = 1

// Answers maps question id to its value: string, bool, or []any of strings.
type Answers map[string]any

// FormatClipboard renders the answers as the two-layer text the human pastes
// back to the agent. The spec drives the order and the wording: questions the
// human skipped are still listed (as "(no answer)") so the agent sees what was
// asked, but they are left out of the JSON block, and answers whose id is not
// in the spec are ignored entirely. Skipped means blank as well as absent, see
// isBlank.
func FormatClipboard(s *spec.Spec, a Answers) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Optioner answers: %s\n\n", s.Title)

	answered := map[string]any{}
	for _, q := range s.Questions {
		v, ok := a[q.ID]
		if !ok || isBlank(v) {
			fmt.Fprintf(&b, "- **%s** — (no answer)\n", q.Prompt)
			continue
		}
		answered[q.ID] = v
		fmt.Fprintf(&b, "- **%s** — %s\n", q.Prompt, display(v))
	}

	// The error is ignored on purpose: every value here came out of the
	// browser's JSON, so it marshals back by construction.
	machine, _ := json.Marshal(struct {
		Optioner int            `json:"optioner"`
		Title    string         `json:"title"`
		Answers  map[string]any `json:"answers"`
	}{outputVersion, s.Title, answered})
	fmt.Fprintf(&b, "\n```json\n%s\n```\n", machine)
	return b.String()
}

// isBlank reports whether a value carries no answer. The browser posts a value
// for every field it rendered, so an untouched free_text arrives as "", an
// unchecked multi_choice as [], and either can arrive as null. Treating those
// as answers would put a dangling bullet or a "<nil>" into the text the human
// pastes back, so they are held to the same "(no answer)" contract as a
// missing key. A deliberate false (yes_no) is an answer and is not blank.
func isBlank(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	default:
		return false
	}
}

// display renders one answer value for the human-readable layer.
func display(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "yes"
		}
		return "no"
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, fmt.Sprint(e))
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprint(v)
	}
}
