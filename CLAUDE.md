# Optioner

AI エージェント(Claude Code / Codex)との「選択肢提示 → 人間の判断」を、毎回同じ UI・同じ出力で行う difit ライクな CLI。Go 製、`npx optioner` で配布。

**作業前に必ず読む**: 本ファイルと README.md(設計ルールはここが正)。

## 開発ワークフロー(必須)

- **outside-in TDD** で進める。E2E(`e2e/`)を最初に整備してあり、**E2E が green になったら v1 完成**という進捗メーターとして使う。E2E は CLI 本実装タスクが終わるまで red なのが正常
- 各タスクは **superpowers:test-driven-development** に従う:失敗するテストを書く → 失敗を確認 → 最小実装 → green を確認 → コミット
- プランの実行は **superpowers:subagent-driven-development**(または executing-plans)で、タスク単位に進める
- 完了を主張する前に **superpowers:verification-before-completion**:検証コマンドを実際に実行し、出力を見てから言う
- 内側の開発ループは `go test ./internal/...`、締めに `go test ./...`(E2E 込み)

## コマンド

```sh
go test ./internal/...                            # 内側ループ(速い)
go test ./...                                     # 全テスト(E2E 込み)
go run ./cmd/optioner examples/db-choice.json     # 手動起動(ブラウザが開く)
go build -o bin/optioner ./cmd/optioner           # ビルド
```

## 構成と制約

- リポジトリは **Go 一色**。JS ツールチェーン禁止(UI は `html/template` + 素の JS/CSS を `go:embed`)
- ランタイム依存は goldmark v1.8.5(CVE-2026-5160 修正込み)/ bluemonday v1.0.27 の 2 つのみ。テスト専用依存は go-cmp と santhosh-tekuri/jsonschema/v6。runn は依存爆発(総計 239)のため不採用(E2E も stdlib)。これ以外を足すときは理由を本ファイルに記録してから
- サーバの Host 検証 + `http.CrossOriginProtection`(Go 1.25)を外さない(127.0.0.1 で POST を受けるための DNS リバインディング / CSRF 対策)
- ユーザー向け文字列(UI・エラーメッセージ)は英語。例・ドキュメントは日本語可
- バリデーションのエラーメッセージは製品仕様そのもの(エージェントの自己修復ループの核)。文面を変えるときはテストも意図的に更新する
- **ゾンビサーバを絶対に残さない**(D4 ルール: プロセスはブラウザタブより長生きしない)。ライフサイクルに触る変更は必ず E2E の lifecycle テストで検証する
- リポジトリルートがプロジェクトルート。コマンドはすべてここから実行する
