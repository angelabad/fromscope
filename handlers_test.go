package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestTable returns a small table; one cell contains HTML
// to check that templates escape it.
func newTestTable() *Table {
	return &Table{
		Columns: []string{"repo", "image"},
		Rows: [][]string{
			{"app", "alpine:3.20"},
			{"web", "<script>alert(1)</script>"},
			{"api", ""},
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
		">3</span>",                             // row counter
		"&lt;script&gt;alert(1)&lt;/script&gt;", // HTML in cells is escaped
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)") {
		t.Error("body contains unescaped HTML from a cell")
	}
}

func TestHandleIndexEmptyTable(t *testing.T) {
	table := &Table{Columns: []string{"repo", "image"}}

	rec := serve(t, table, http.MethodGet, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "No matching rows") {
		t.Error("body does not contain the empty-state message")
	}
	if !strings.Contains(body, `colspan="2"`) {
		t.Error("empty-state cell does not span every column")
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
