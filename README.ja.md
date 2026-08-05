# Optioner

**AI エージェントとの意思決定を、毎回同じページで。**

Claude Code / Codex と作業していると、判断ポイントが絶えず出てきます:
どの DB にするか、どの方針で行くか、出すか待つか。エージェントはこういう場面で
HTML を即席で作りがちで、毎回見た目が違い、ときどき普通に壊れます。

Optioner はこのフォーマットを固定します。エージェントは小さな spec JSON を書くだけ。
Optioner がそれを一貫した意思決定ページとしてブラウザに表示し、あなたは回答して
**Copy** を押し、結果をエージェントに貼り返します。

## 導入

インストールは不要です。npm から直接実行できます(macOS Apple Silicon):

```sh
npx @doskoi64/optioner spec.json
```

30 秒で試すなら:

```sh
curl -sO https://raw.githubusercontent.com/t-daisuke/optioner/main/examples/all-types.json
npx @doskoi64/optioner all-types.json    # ブラウザが開くので、回答 → Copy → Done
```

ソースから実行する場合(Go 1.25+、Go が動く環境ならどこでも):

```sh
git clone https://github.com/t-daisuke/optioner.git && cd optioner
go run ./cmd/optioner examples/db-choice.json
```

**AI エージェントに自動で使わせる:**

- **Claude Code**: スキルをスキルディレクトリへコピー —
  `mkdir -p ~/.claude/skills/optioner && curl -so ~/.claude/skills/optioner/SKILL.md https://raw.githubusercontent.com/t-daisuke/optioner/main/skill/SKILL.md`
- **Codex**: [`skill/codex-prompt.md`](skill/codex-prompt.md) のブロックを `AGENTS.md`(リポジトリ直下 or `~/.codex/AGENTS.md`)に追記

以降、判断が必要になるたびエージェントが自分で Optioner を起動します。

## 使い方

- 空きポートで起動し、ブラウザが開く
- 回答 → **Copy**(Markdown サマリ + 機械可読 JSON)→ **Done**
- 終了時には回答が stdout にもエコーされ、エージェントはそれを直接読める
- サーバはページと運命を共にする: **Done** なら即終了、タブを閉じただけなら
  ハートビート断で検知。一度も開かれなかったページは kill されるまで待つ
- ブラウザ自動オープンは macOS の `open` 頼み。他の環境では表示された URL を
  自分で開く

### フラグ

フラグは spec ファイルより**前**。後置フラグは usage エラー(exit 2)です。

| フラグ | 既定値 | |
|---|---|---|
| `-no-open` | off | ブラウザを開かない。URL は表示される。 |
| `-heartbeat-timeout` | `2m` | ページの死活信号が途絶えてからこの時間で終了。 |
| `-version` | | バージョンを表示して終了。 |

起動時、stdout の 1 行目が URL。エラーは stderr へ。
exit code: `0` 正常終了、`1` spec 不正、`2` usage エラー。

## spec フォーマット

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

質問タイプは 4 つ: `free_text` / `yes_no` / `single_choice` / `multi_choice`。
選択系は 2 つ以上の options と、任意の `allow_other`(自由記述の逃げ道)。
テキストフィールド(`context`、`prompt`、`description`)は Markdown 可。
未知のフィールドは拒否されます。

正式な定義は [`schema/optioner.schema.json`](schema/optioner.schema.json)、
サンプルは [`examples/`](examples/) へ。

## 貼り返すテキスト

````
## Optioner answers: Choose a database

- **Which DB should we use?** — SQLite

```json
{"optioner":1,"title":"Choose a database","answers":{"db":"SQLite"}}
```
````

箇条書きは人間用、JSON ブロックはエージェント用。スキップした質問は
`(no answer)` と表示され、`answers` からは除外されます。

## ライセンス

MIT
