package render

import (
	"strings"
	"testing"
)

func TestMarkdownRendersGFMTable(t *testing.T) {
	got := string(Markdown("| a | b |\n|---|---|\n| 1 | 2 |"))
	if !strings.Contains(got, "<table>") {
		t.Fatalf("expected GFM table, got:\n%s", got)
	}
}

func TestMarkdownKeepsLinksAndCode(t *testing.T) {
	got := string(Markdown("[difit](https://github.com/yoshiko-pg/difit) and `code`"))
	if !strings.Contains(got, `href="https://github.com/yoshiko-pg/difit"`) || !strings.Contains(got, "<code>") {
		t.Fatalf("links/code lost:\n%s", got)
	}
}

func TestMarkdownStripsXSS(t *testing.T) {
	got := string(Markdown(`hello <script>alert(1)</script> <img src=x onerror=alert(1)>`))
	if strings.Contains(got, "<script") || strings.Contains(got, "onerror") {
		t.Fatalf("XSS not sanitized:\n%s", got)
	}
}

// script タグを書かなくても、リンクの href だけで JS は動く。ここを止めて
// いるのは bluemonday の AllowStandardURLs(UGCPolicy に含まれる)ただ 1 つ
// なので、ポリシーを差し替えたときに気づけるよう明示的に固定する。
func TestMarkdownDropsJavaScriptLinks(t *testing.T) {
	got := string(Markdown(`[x](javascript:alert(1))`))
	if strings.Contains(got, "javascript:") {
		t.Fatalf("a javascript: URL must never reach an href:\n%s", got)
	}
}
