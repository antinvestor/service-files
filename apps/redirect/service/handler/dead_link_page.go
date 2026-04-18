package handler

import (
	_ "embed"
	"html/template"
	"net/http"
)

// deadLinkHTML is the markup for /r/{slug} hits that land on an
// inactive link. Kept in a sibling .html file so the project's
// en-GB spell-checker can't rewrite CSS property names inside a Go
// string literal, which would produce invalid CSS. Embedding is
// zero-cost at runtime.
//
//go:embed dead_link.html
var deadLinkHTML string

var deadLinkTemplate = template.Must(template.New("dead-link").Parse(deadLinkHTML))

type deadLinkData struct {
	Title       string
	JobsBaseURL string
}

// renderDeadLinkPage emits the dead-link HTML with an HTTP 410 Gone
// status. 410 is the semantically correct code — "the resource used
// to be here, do not retry" — and search engines honour it better
// than 404 for deindexing. X-Robots-Tag: noindex belts-and-braces the
// <meta> tag so crawlers that didn't render the HTML still get the
// signal.
func renderDeadLinkPage(w http.ResponseWriter, data deadLinkData) {
	if data.JobsBaseURL == "" {
		data.JobsBaseURL = "https://jobs.stawi.org/"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.WriteHeader(http.StatusGone)
	_ = deadLinkTemplate.Execute(w, data)
}
