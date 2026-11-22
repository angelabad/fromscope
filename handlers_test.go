package main

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newTestTable returns a small table; one cell contains HTML
// to check that templates escape it. "rama" is a dropdown column,
// and "develop"/"development" check that it is an exact match.
func newTestTable() *Table {
	return &Table{
		Columns: []string{"repo", "rama", "image"},
		Rows: [][]string{
			{"app", "master", "alpine:3.20"},
			{"web", "develop", "<script>alert(1)</script>"},
			{"api", "development", ""},
		},
	}
}

// serve sends a request through the full router and returns the recorded response.
func serve(t *testing.T, table *Table, method, target string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
	rec := httptest.NewRecorder()
	newServer(table).routes().ServeHTTP(rec, req)

	return rec
}

func TestHandleIndex(t *testing.T) {
	rec := serve(t, newTestTable(), http.MethodGet, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", got)
	}

	body := rec.Body.String()
	for _, want := range []string{
		">repo</th>",                            // header from Columns
		">image</th>",                           // header from Columns
		"alpine:3.20",                           // a cell value
		">3</span> rows",                        // row counter, unfiltered
		"&lt;script&gt;alert(1)&lt;/script&gt;", // HTML in cells is escaped
		`hx-get="/rows"`,                        // the form asks for filtered rows
		`name="q"`,                              // global search
		`<input type="search" name="repo"`,      // text filter for a regular column
		`<select name="rama"`,                   // dropdown for a dropdown column
		"<option>develop</option>",              // dropdown values
		"<option>development</option>",          // dropdown values
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)") {
		t.Error("body contains unescaped HTML from a cell")
	}
	if strings.Contains(body, "hx-swap-oob") {
		t.Error("full page contains an out-of-band swap")
	}
}

func TestHandleIndexEmptyTable(t *testing.T) {
	table := &Table{Columns: []string{"repo", "rama", "image"}}

	rec := serve(t, table, http.MethodGet, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "No matching rows") {
		t.Error("body does not contain the empty-state message")
	}
	if !strings.Contains(body, `colspan="3"`) {
		t.Error("empty-state cell does not span every column")
	}
}

func TestHandleRows(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		want    []string
		notWant []string
	}{
		{
			name:   "no filters returns every row",
			target: "/rows",
			want:   []string{">app</td>", ">web</td>", ">api</td>", ">3</span> rows"},
		},
		{
			name:    "global search is case-insensitive",
			target:  "/rows?q=ALPINE",
			want:    []string{">app</td>", ">1</span> of ", ">3</span> rows"},
			notWant: []string{">web</td>", ">api</td>"},
		},
		{
			name:    "dropdown column is an exact match",
			target:  "/rows?rama=develop",
			want:    []string{">web</td>"},
			notWant: []string{">app</td>", ">api</td>"},
		},
		{
			name:    "text column is a trimmed contains match",
			target:  "/rows?repo=+AP+",
			want:    []string{">app</td>", ">api</td>"},
			notWant: []string{">web</td>"},
		},
		{
			name:    "filters are combined",
			target:  "/rows?q=a&rama=master",
			want:    []string{">app</td>"},
			notWant: []string{">web</td>", ">api</td>"},
		},
		{
			name:   "unknown parameters are ignored",
			target: "/rows?nope=x",
			want:   []string{">app</td>", ">web</td>", ">api</td>"},
		},
		{
			name:    "no matches shows the empty state",
			target:  "/rows?q=nothing",
			want:    []string{"No matching rows", ">0</span> of "},
			notWant: []string{">app</td>"},
		},
		{
			name:    "cells are escaped",
			target:  "/rows?q=script",
			want:    []string{"&lt;script&gt;alert(1)&lt;/script&gt;"},
			notWant: []string{"<script>alert(1)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, newTestTable(), http.MethodGet, tt.target)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			if strings.Contains(body, "<html") {
				t.Error("body contains the full page, want only the rows")
			}
			if !strings.Contains(body, `id="count" hx-swap-oob="innerHTML"`) {
				t.Error("body does not update the counter out of band")
			}
			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(body, notWant) {
					t.Errorf("body contains %q", notWant)
				}
			}
		})
	}
}

func TestFilterFromQuery(t *testing.T) {
	columns := []string{"repo", "rama", "image"}

	tests := []struct {
		name  string
		query url.Values
		want  Filter
	}{
		{
			name:  "no parameters",
			query: url.Values{},
			want:  Filter{},
		},
		{
			name: "every kind of parameter",
			query: url.Values{
				"q":     {" alpine "},
				"repo":  {"app"},
				"rama":  {"master"},
				"image": {"  "}, // blank values are skipped
				"nope":  {"x"},  // not a column
			},
			want: Filter{
				Query:    "alpine",
				Contains: map[string]string{"repo": "app"},
				Equals:   map[string]string{"rama": "master"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterFromQuery(tt.query, columns)

			if got.Query != tt.want.Query {
				t.Errorf("Query = %q, want %q", got.Query, tt.want.Query)
			}
			if !maps.Equal(got.Contains, tt.want.Contains) {
				t.Errorf("Contains = %q, want %q", got.Contains, tt.want.Contains)
			}
			if !maps.Equal(got.Equals, tt.want.Equals) {
				t.Errorf("Equals = %q, want %q", got.Equals, tt.want.Equals)
			}
		})
	}
}

func TestRoutes(t *testing.T) {
	tests := []struct {
		method string
		target string
		want   int
	}{
		{http.MethodGet, "/", http.StatusOK},
		{http.MethodHead, "/", http.StatusOK},
		{http.MethodPost, "/", http.StatusMethodNotAllowed},
		{http.MethodGet, "/nope", http.StatusNotFound},
		{http.MethodGet, "/rows", http.StatusOK},
		{http.MethodPost, "/rows", http.StatusMethodNotAllowed},
		{http.MethodGet, "/rows/", http.StatusNotFound},
		{http.MethodGet, "/static/htmx.min.js", http.StatusOK},
		{http.MethodGet, "/static/app.css", http.StatusOK},
		{http.MethodGet, "/static/missing.js", http.StatusNotFound},
		{http.MethodGet, "/templates/index.html", http.StatusNotFound}, // templates are not served
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			rec := serve(t, newTestTable(), tt.method, tt.target)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestRenderError(t *testing.T) {
	rec := httptest.NewRecorder()

	// struct{}{} has no Columns/Rows fields, so executing the template fails.
	render(rec, "index.html", struct{}{})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "<html") {
		t.Error("body contains partial HTML; render must buffer before writing")
	}
}
