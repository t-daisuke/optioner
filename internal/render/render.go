// Package render converts AI-authored Markdown into sanitized HTML.
// Everything that reaches the page goes through bluemonday: the spec
// is written by an LLM and must be treated as untrusted input.
package render

import (
	"bytes"
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	policy = bluemonday.UGCPolicy()
	md     = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(html.WithUnsafe()), // sanitizer below is the gate
	)
)

// Markdown renders src as GFM and returns HTML that is safe to drop into a
// page. Raw HTML in src is passed through by goldmark on purpose so that
// bluemonday — not goldmark's escaping — is the single place where the policy
// lives. If rendering fails the source is escaped and shown as plain text
// rather than dropped, so the human still sees what the agent wrote.
func Markdown(src string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return template.HTML("<p>" + template.HTMLEscapeString(src) + "</p>")
	}
	return template.HTML(policy.SanitizeBytes(buf.Bytes()))
}
