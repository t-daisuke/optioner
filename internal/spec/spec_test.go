package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidSpec(t *testing.T) {
	data := []byte(`{
		"optioner": 1,
		"title": "Choose a database",
		"context": "## Background",
		"questions": [
			{"id": "db", "type": "single_choice", "prompt": "Which?",
			 "options": [{"label": "SQLite", "recommended": true}, {"label": "Postgres"}],
			 "allow_other": true},
			{"id": "notes", "type": "free_text", "prompt": "Anything else?"}
		]
	}`)
	s, err := Load(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Title != "Choose a database" || len(s.Questions) != 2 {
		t.Fatalf("spec not loaded correctly: %+v", s)
	}
	if !s.Questions[0].Options[0].Recommended {
		t.Fatal("recommended flag lost")
	}
}

func TestLoadRejectsInvalidSpecs(t *testing.T) {
	cases := []struct {
		name, json, wantMsg string
	}{
		{"bad version", `{"optioner": 2, "title": "t", "questions": [{"id":"a","type":"free_text","prompt":"?"}]}`,
			`optioner: must be 1 (got 2)`},
		{"missing title", `{"optioner": 1, "questions": [{"id":"a","type":"free_text","prompt":"?"}]}`,
			`title: required`},
		{"no questions", `{"optioner": 1, "title": "t", "questions": []}`,
			`questions: at least one question is required`},
		{"missing id", `{"optioner": 1, "title": "t", "questions": [{"type":"free_text","prompt":"?"}]}`,
			`questions[0].id: required`},
		{"bad id", `{"optioner": 1, "title": "t", "questions": [{"id":"DB!","type":"free_text","prompt":"?"}]}`,
			`questions[0].id: must match ^[a-z][a-z0-9_]*$ (got "DB!")`},
		{"duplicate id", `{"optioner": 1, "title": "t", "questions": [{"id":"a","type":"free_text","prompt":"?"},{"id":"a","type":"free_text","prompt":"?"}]}`,
			`questions[1].id: duplicate id "a"`},
		{"bad type", `{"optioner": 1, "title": "t", "questions": [{"id":"a","type":"dropdown","prompt":"?"}]}`,
			`questions[0].type: must be one of free_text, yes_no, single_choice, multi_choice (got "dropdown")`},
		{"missing prompt", `{"optioner": 1, "title": "t", "questions": [{"id":"a","type":"free_text"}]}`,
			`questions[0].prompt: required`},
		{"choice needs options", `{"optioner": 1, "title": "t", "questions": [{"id":"a","type":"single_choice","prompt":"?"}]}`,
			`questions[0].options: single_choice requires at least 2 options`},
		{"one option is not a choice", `{"optioner": 1, "title": "t", "questions": [{"id":"a","type":"multi_choice","prompt":"?","options":[{"label":"x"}]}]}`,
			`questions[0].options: multi_choice requires at least 2 options`},
		{"option needs label", `{"optioner": 1, "title": "t", "questions": [{"id":"a","type":"single_choice","prompt":"?","options":[{"label":"x"},{"description":"y"}]}]}`,
			`questions[0].options[1].label: required`},
		{"free_text must not have options", `{"optioner": 1, "title": "t", "questions": [{"id":"a","type":"free_text","prompt":"?","options":[{"label":"x"},{"label":"y"}]}]}`,
			`questions[0].options: not allowed for type free_text`},
		{"yes_no must not allow_other", `{"optioner": 1, "title": "t", "questions": [{"id":"a","type":"yes_no","prompt":"?","allow_other":true}]}`,
			`questions[0].allow_other: not allowed for type yes_no`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load([]byte(c.json))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.HasPrefix(err.Error(), "spec invalid:") {
				t.Fatalf("error should start with 'spec invalid:', got %q", err.Error())
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Fatalf("expected message to contain %q, got:\n%s", c.wantMsg, err.Error())
			}
		})
	}
}

// 個々の文面は上のテーブルが見ているが、エージェントが実際に読むのは
// 「1 回の実行で返ってくる 1 本の文字列」。ここだけは組み立て後の全文を
// 固定して、(a) 問題を 1 つ見つけた時点で打ち切らないこと、(b) 見出し
// "spec invalid:" + "\n  - " 区切りという、自己修復ループが当てにしている
// 形そのものを守る。
func TestValidationReportsEveryProblemInOneMessage(t *testing.T) {
	_, err := Load([]byte(`{"optioner": 2, "questions": [{"id": "A!", "type": "dropdown", "prompt": "?"}]}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "spec invalid:\n" +
		"  - optioner: must be 1 (got 2)\n" +
		"  - title: required\n" +
		"  - questions[0].id: must match ^[a-z][a-z0-9_]*$ (got \"A!\")\n" +
		"  - questions[0].type: must be one of free_text, yes_no, single_choice, multi_choice (got \"dropdown\")"
	if err.Error() != want {
		t.Fatalf("the error text is the product spec (agents parse it).\nwant:\n%s\n\ngot:\n%s", want, err.Error())
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	_, err := Load([]byte(`{"optioner": 1, "titel": "typo", "questions": []}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestLoadRejectsBrokenJSON(t *testing.T) {
	_, err := Load([]byte(`{not json`))
	if err == nil || !strings.Contains(err.Error(), "spec is not valid JSON") {
		t.Fatalf("expected JSON error, got %v", err)
	}
}

// The shipped examples double as documentation, so a broken one is a broken doc.
func TestExamplesAreValid(t *testing.T) {
	files, err := filepath.Glob("../../examples/*.json")
	if err != nil || len(files) < 2 {
		t.Fatalf("expected example files, got %v (%v)", files, err)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Load(data); err != nil {
				t.Fatalf("example must be valid: %v", err)
			}
		})
	}
}
