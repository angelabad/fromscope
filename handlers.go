package main

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
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
// Every field is set by newServer and only read afterwards,
// so it is safe for concurrent requests.
type server struct {
	table   *Table
	options map[string][]string // dropdown values per column
}

// newServer returns a server that serves the given table.
// It computes the dropdown values once, since the table never changes.
func newServer(t *Table) *server {
	options := make(map[string][]string)

	for _, col := range dropdownColumns {
		if values := t.Values(col); values != nil {
			options[col] = values
		}
	}

	return &server{table: t, options: options}
}

// routes returns the HTTP handler with every route registered.
func (s *server) routes() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("GET /{$}", s.handleIndex) // {$} matches "/" only, anything else is a 404
	router.HandleFunc("GET /rows", s.handleRows)
	router.Handle("GET /static/", http.FileServer(http.FS(assets)))

	return router
}

// handleIndex renders the full page: every row of the table plus the
// search box and the column filters.
func (s *server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	render(w, "index.html", view{
		Columns: s.table.Columns,
		Rows:    s.table.Rows,
		Total:   len(s.table.Rows),
		Options: s.options,
	})
}

// handleRows renders only the rows that match the query parameters,
// plus the row counter as an out-of-band swap.
func (s *server) handleRows(w http.ResponseWriter, r *http.Request) {
	f := filterFromQuery(r.URL.Query(), s.table.Columns)

	filtered := s.table.Filter(f)

	render(w, "results.html", view{
		Columns: filtered.Columns,
		Rows:    filtered.Rows,
		Total:   len(s.table.Rows),
	})
}

// filterFromQuery builds a Filter from the query parameters: q is the
// global search, and every other parameter named after a column filters it.
func filterFromQuery(q url.Values, columns []string) Filter {
	f := Filter{
		Query:    strings.TrimSpace(q.Get("q")),
		Contains: make(map[string]string),
		Equals:   make(map[string]string),
	}

	for _, col := range columns {
		v := strings.TrimSpace(q.Get(col))

		switch {
		case v == "":
			// not set: no filter for this column.
		case slices.Contains(dropdownColumns, col):
			f.Equals[col] = v
		default:
			f.Contains[col] = v
		}
	}

	return f
}

// view is the data passed to the templates.
type view struct {
	Columns []string
	Rows    [][]string
	Total   int                 // rows before filtering
	Options map[string][]string // dropdown values per column; only the full page uses it
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
