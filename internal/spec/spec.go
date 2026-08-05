// Package spec defines the Optioner spec format and its validation.
// Validation error messages are part of the product: agents read them
// to self-repair their spec JSON, so treat wording changes as spec changes.
package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Question types accepted in the "type" field of a question.
const (
	TypeFreeText     = "free_text"
	TypeYesNo        = "yes_no"
	TypeSingleChoice = "single_choice"
	TypeMultiChoice  = "multi_choice"
)

const (
	// version is the only spec format version this build understands.
	version = 1
	// minChoiceOptions is the smallest number of options that makes a choice a choice.
	minChoiceOptions = 2
)

// validTypes lists the question types in the order used by error messages.
var validTypes = []string{TypeFreeText, TypeYesNo, TypeSingleChoice, TypeMultiChoice}

// idPattern constrains question ids so they are safe to use as form field names
// and as keys in the answer output.
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Spec is one round of questions to put in front of a human.
type Spec struct {
	Optioner  int        `json:"optioner"`
	Title     string     `json:"title"`
	Context   string     `json:"context,omitempty"`
	Questions []Question `json:"questions"`
}

// Question is a single question, rendered according to its Type.
type Question struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Prompt     string   `json:"prompt"`
	Options    []Option `json:"options,omitempty"`
	AllowOther bool     `json:"allow_other,omitempty"`
}

// Option is one selectable answer of a choice question.
type Option struct {
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Links       []string `json:"links,omitempty"`
	Recommended bool     `json:"recommended,omitempty"`
}

// ValidationError reports every problem found in a spec at once, so an agent
// can repair its JSON in a single pass instead of one round trip per mistake.
type ValidationError struct{ Problems []string }

func (e *ValidationError) Error() string {
	return "spec invalid:\n  - " + strings.Join(e.Problems, "\n  - ")
}

// Load parses and validates a spec. It fails on malformed JSON, on unknown
// fields (a typo must not be silently ignored), and on any validation problem.
func Load(data []byte) (*Spec, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s Spec
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("spec is not valid JSON: %w", err)
	}
	if problems := s.validate(); len(problems) > 0 {
		return nil, &ValidationError{Problems: problems}
	}
	return &s, nil
}

// problems accumulates validation messages in the order they are found.
type problems []string

func (p *problems) addf(format string, args ...any) {
	*p = append(*p, fmt.Sprintf(format, args...))
}

// validate returns every problem in the spec, top level first, then questions
// in document order.
func (s *Spec) validate() []string {
	var p problems
	if s.Optioner != version {
		p.addf("optioner: must be %d (got %d)", version, s.Optioner)
	}
	if s.Title == "" {
		p.addf("title: required")
	}
	if len(s.Questions) == 0 {
		p.addf("questions: at least one question is required")
	}
	seenIDs := make(map[string]bool, len(s.Questions))
	for i, q := range s.Questions {
		q.validate(fmt.Sprintf("questions[%d]", i), seenIDs, &p)
	}
	return p
}

// validate appends the question's problems to p, prefixing each message with
// path (for example "questions[0]") so the agent knows what to fix. It marks
// the question's id as seen in seenIDs to detect later duplicates.
func (q Question) validate(path string, seenIDs map[string]bool, p *problems) {
	at := func(field, msg string) { p.addf("%s.%s: %s", path, field, msg) }

	switch {
	case q.ID == "":
		at("id", "required")
	case !idPattern.MatchString(q.ID):
		at("id", fmt.Sprintf("must match %s (got %q)", idPattern, q.ID))
	case seenIDs[q.ID]:
		at("id", fmt.Sprintf("duplicate id %q", q.ID))
	default:
		seenIDs[q.ID] = true
	}
	if q.Prompt == "" {
		at("prompt", "required")
	}

	switch q.Type {
	case TypeSingleChoice, TypeMultiChoice:
		if len(q.Options) < minChoiceOptions {
			at("options", fmt.Sprintf("%s requires at least %d options", q.Type, minChoiceOptions))
		}
		for j, o := range q.Options {
			if o.Label == "" {
				p.addf("%s.options[%d].label: required", path, j)
			}
		}
	case TypeFreeText, TypeYesNo:
		if len(q.Options) > 0 {
			at("options", "not allowed for type "+q.Type)
		}
		if q.AllowOther {
			at("allow_other", "not allowed for type "+q.Type)
		}
	default:
		at("type", fmt.Sprintf("must be one of %s (got %q)", strings.Join(validTypes, ", "), q.Type))
	}
}
