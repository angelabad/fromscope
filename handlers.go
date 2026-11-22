package main

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"
)

// assets holds the HTML templates and the static files (CSS, JS),
// embedded in the binary so it runs without any files next to it.
//
//go:embed templates static
var assets embed.FS

// tmpl holds every page template, parsed once at startup.
// Templates are named after their file, e.g. "index.html".
var tmpl = template.Must(template.ParseFS(assets, "templates/*.html"))

// server holds the state shared by the HTTP handlers.
type server struct {
	table *Table // read-only after startup, safe for concurrent requests
}

// newServer returns a server that serves the given table.
func newServer(t *Table) *server {
	return &server{t}
}

// routes returns the HTTP handler with every route registered.
func (s *server) routes() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("GET /{$}", s.handleIndex) // {$} matches "/" only, anything else is a 404
	router.Handle("GET /static/", http.FileServer(http.FS(assets)))

	return router
}

// handleIndex renders the full page with every row of the table.
func (s *server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	render(w, "index.html", s.table)
}

// render executes the named template with data and writes it to w.
// The output is buffered first, so a template error replies with a
// clean 500 instead of a half-written page with a 200 status.
func render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer

	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("rendering %s: %v", name, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write(buf.Bytes())
	if err != nil {
		// Headers are already sent, so all we can do is log it
		// (usually the client closed the connection).
		log.Printf("writing response: %v", err)
	}
}
