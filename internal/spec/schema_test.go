package spec

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compileSchema loads the published JSON Schema. It resolves the 2020-12
// metaschema from the library's embedded copy, so the tests stay offline.
func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	sch, err := c.Compile("../../schema/optioner.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

// agree fails unless the published schema and the Go validator both return the
// verdict the caller expects. Either one disagreeing is the bug this guards.
func agree(t *testing.T, sch *jsonschema.Schema, label string, data []byte, wantValid bool) {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	schemaErr := sch.Validate(inst)
	_, loadErr := Load(data)
	if (schemaErr == nil) != wantValid || (loadErr == nil) != wantValid {
		t.Fatalf("schema/validator disagree for %s: schemaErr=%v loadErr=%v wantValid=%v",
			label, schemaErr, loadErr, wantValid)
	}
}

// specWith wraps one question in an otherwise minimal valid spec.
func specWith(question string) []byte {
	return []byte(`{"optioner":1,"title":"t","questions":[` + question + `]}`)
}

// 公開 JSON Schema と Go バリデータが同じ合否を返すことを保証する。
// 既知の差: 質問 id の一意性は JSON Schema で表現できないため Go 側だけが弾く
// (schema/optioner.schema.json の $comment 参照)。ここで使う不正データは
// 型ベースなので両者とも invalid と判定する。
func TestSchemaAndValidatorAgree(t *testing.T) {
	sch := compileSchema(t)
	checkFile := func(t *testing.T, path string, wantValid bool) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		agree(t, sch, path, data, wantValid)
	}
	valid, err := filepath.Glob("../../examples/*.json")
	if err != nil || len(valid) < 2 {
		t.Fatalf("expected example files, got %v (%v)", valid, err)
	}
	for _, f := range valid {
		t.Run("valid/"+filepath.Base(f), func(t *testing.T) { checkFile(t, f, true) })
	}
	checkFile(t, "../../e2e/testdata/invalid-type.json", false)
}

// タイプ別ルール(スキーマの if/then 分岐)は examples が触らないので、
// インラインの spec で両層の合否を固定する。特に「明示的なゼロ値」は
// Go 側が未指定と区別できないため、スキーマも受け入れなければならない。
func TestSchemaAndValidatorAgreeOnTypeRules(t *testing.T) {
	sch := compileSchema(t)
	cases := []struct {
		name      string
		question  string
		wantValid bool
	}{
		{"free_text with explicit empty options",
			`{"id":"a","type":"free_text","prompt":"?","options":[]}`, true},
		{"yes_no with explicit allow_other false",
			`{"id":"a","type":"yes_no","prompt":"?","allow_other":false}`, true},
		{"free_text with options",
			`{"id":"a","type":"free_text","prompt":"?","options":[{"label":"x"},{"label":"y"}]}`, false},
		{"yes_no with allow_other",
			`{"id":"a","type":"yes_no","prompt":"?","allow_other":true}`, false},
		{"single_choice with two options",
			`{"id":"a","type":"single_choice","prompt":"?","options":[{"label":"x"},{"label":"y"}]}`, true},
		{"single_choice with one option",
			`{"id":"a","type":"single_choice","prompt":"?","options":[{"label":"x"}]}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			agree(t, sch, c.name, specWith(c.question), c.wantValid)
		})
	}
}

// 明示的な JSON null は「省略」と同じ意味になる。encoding/json は null を
// 代入なしとして読み飛ばすので Go 側は素通しする。ゼロ値を省略しない
// ジェネレータ(例: Python の Optional[...] = None)が吐く spec が公開
// スキーマで弾かれないよう、任意フィールドの null を両層で受け入れる。
// ただし必須フィールドと choice の options は null を許さない。
func TestSchemaAndValidatorAgreeOnExplicitNulls(t *testing.T) {
	sch := compileSchema(t)
	cases := []struct {
		name      string
		json      []byte
		wantValid bool
	}{
		{"context null",
			[]byte(`{"optioner":1,"title":"t","context":null,"questions":[{"id":"a","type":"free_text","prompt":"?"}]}`), true},
		{"free_text options null",
			specWith(`{"id":"a","type":"free_text","prompt":"?","options":null}`), true},
		{"yes_no allow_other null",
			specWith(`{"id":"a","type":"yes_no","prompt":"?","allow_other":null}`), true},
		{"option description null",
			specWith(`{"id":"a","type":"single_choice","prompt":"?","options":[{"label":"x","description":null},{"label":"y"}]}`), true},
		{"option links null",
			specWith(`{"id":"a","type":"single_choice","prompt":"?","options":[{"label":"x","links":null},{"label":"y"}]}`), true},
		{"option recommended null",
			specWith(`{"id":"a","type":"single_choice","prompt":"?","options":[{"label":"x","recommended":null},{"label":"y"}]}`), true},
		{"single_choice options null",
			specWith(`{"id":"a","type":"single_choice","prompt":"?","options":null}`), false},
		{"multi_choice options null",
			specWith(`{"id":"a","type":"multi_choice","prompt":"?","options":null}`), false},
		{"title null",
			[]byte(`{"optioner":1,"title":null,"questions":[{"id":"a","type":"free_text","prompt":"?"}]}`), false},
		{"option label null",
			specWith(`{"id":"a","type":"single_choice","prompt":"?","options":[{"label":null},{"label":"y"}]}`), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			agree(t, sch, c.name, c.json, c.wantValid)
		})
	}
}
