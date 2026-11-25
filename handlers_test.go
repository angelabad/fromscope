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
		`name="by" value="repo"`,                // sort button per column
		`name="by" value="image"`,               // sort button per column
		`<button type="submit" hidden>`,         // default button for Enter
		"alpine:3.20",                           // a cell value
		">3</span> rows",                        // row counter, unfiltered
		"&lt;script&gt;alert(1)&lt;/script&gt;", // HTML in cells is escaped
		`hx-get="/rows"`,                        // the form asks for filtered rows
		`name="q"`,                              // global search
		`<input type="search" name="repo"`,      // text filter for a regular column
		`<select name="rama"`,                   // dropdown for a dropdown column
		`<input type="checkbox" name="group"`,   // group branches switch
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
	if strings.Contains(body, "aria-sort") {
		t.Error("full page marks a column as sorted, want CSV order")
	}
	if strings.Contains(body, "checked") || strings.Contains(body, "groups from") {
		t.Error("full page is grouped, want every row")
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
			if !strings.Contains(body, `id="head" hx-swap-oob="innerHTML"`) {
				t.Error("body does not update the header row out of band")
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

func TestHandleRowsSorted(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		wantRepo []string // repo of every row, in order
		wantHead []string // what the header row must contain
	}{
		{
			name:     "no sort keeps the CSV order",
			target:   "/rows",
			wantRepo: []string{"app", "web", "api"},
		},
		{
			name:     "click sorts ascending",
			target:   "/rows?by=repo",
			wantRepo: []string{"api", "app", "web"},
			wantHead: []string{
				`aria-sort="ascending"`,
				`<input type="hidden" name="sort" value="repo">`,
				`<input type="hidden" name="dir" value="asc">`,
			},
		},
		{
			name:     "second click sorts descending",
			target:   "/rows?sort=repo&dir=asc&by=repo",
			wantRepo: []string{"web", "app", "api"},
			wantHead: []string{
				`aria-sort="descending"`,
				`<input type="hidden" name="dir" value="desc">`,
			},
		},
		{
			name:     "filtering keeps the order",
			target:   "/rows?sort=repo&dir=desc&q=a",
			wantRepo: []string{"web", "app", "api"},
			wantHead: []string{`aria-sort="descending"`},
		},
		{
			name:     "sorting keeps the filters",
			target:   "/rows?rama=master&by=repo",
			wantRepo: []string{"app"},
			wantHead: []string{`aria-sort="ascending"`},
		},
		{
			name:     "unknown column keeps the CSV order",
			target:   "/rows?by=nope",
			wantRepo: []string{"app", "web", "api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, newTestTable(), http.MethodGet, tt.target)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()

			// Each wanted repo must appear after the previous one.
			rest := body
			for _, repo := range tt.wantRepo {
				i := strings.Index(rest, ">"+repo+"</td>")
				if i == -1 {
					t.Fatalf("rows are not in order %q: %q not found where expected", tt.wantRepo, repo)
				}
				rest = rest[i+1:]
			}
			if got := strings.Count(body, "<tr class="); got != len(tt.wantRepo) {
				t.Errorf("got %d rows, want %d", got, len(tt.wantRepo))
			}

			for _, want := range tt.wantHead {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
			if len(tt.wantHead) == 0 && strings.Contains(body, "aria-sort") {
				t.Error("a column is marked as sorted, want none")
			}
		})
	}
}

// newGroupedTestTable returns a table where the two "app" rows differ
// only in rama, so they are merged when grouping.
func newGroupedTestTable() *Table {
	return &Table{
		Columns: []string{"repo", "rama", "image"},
		Rows: [][]string{
			{"app", "master", "alpine:3.20"},
			{"web", "master", "nginx:1.27"},
			{"app", "develop", "alpine:3.20"},
		},
	}
}

func TestHandleRowsGrouped(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		wantRows []string // "repo rama" of every row, in order
		want     []string
		notWant  []string
	}{
		{
			name:     "without group every row is shown",
			target:   "/rows",
			wantRows: []string{"app master", "web master", "app develop"},
			want:     []string{">3</span> rows"},
			notWant:  []string{"groups from"},
		},
		{
			name:     "group merges the branches",
			target:   "/rows?group=on",
			wantRows: []string{"app develop, master", "web master"},
			want:     []string{">2</span> groups from ", ">3</span> rows"},
		},
		{
			name:     "group is applied after the filters",
			target:   "/rows?group=on&rama=develop",
			wantRows: []string{"app develop"},
			want:     []string{">1</span> groups from ", ">1</span> of ", ">3</span> rows"},
		},
		{
			name:     "group is applied before sorting",
			target:   "/rows?group=on&sort=repo&dir=desc",
			wantRows: []string{"web master", "app develop, master"},
			want:     []string{">2</span> groups from ", `aria-sort="descending"`},
		},
		{
			name:     "group can be sorted by the merged column",
			target:   "/rows?group=on&by=rama",
			wantRows: []string{"app develop, master", "web master"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, newGroupedTestTable(), http.MethodGet, tt.target)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()

			// Each wanted cell must appear after the previous one.
			rest := body
			for _, row := range tt.wantRows {
				repo, rama, _ := strings.Cut(row, " ")
				for _, cell := range []string{repo, rama} {
					i := strings.Index(rest, ">"+cell+"</td>")
					if i == -1 {
						t.Fatalf("rows are not in order %q: %q not found where expected", tt.wantRows, row)
					}
					rest = rest[i+1:]
				}
			}
			if got := strings.Count(body, "<tr class="); got != len(tt.wantRows) {
				t.Errorf("got %d rows, want %d", got, len(tt.wantRows))
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

func TestSortFromQuery(t *testing.T) {
	columns := []string{"repo", "rama", "image"}

	tests := []struct {
		name  string
		query url.Values
		want  Sort
	}{
		{
			name:  "no parameters",
			query: url.Values{},
			want:  Sort{},
		},
		{
			name:  "current order, ascending",
			query: url.Values{"sort": {"repo"}},
			want:  Sort{Column: "repo"},
		},
		{
			name:  "current order, descending",
			query: url.Values{"sort": {"repo"}, "dir": {"desc"}},
			want:  Sort{Column: "repo", Desc: true},
		},
		{
			name:  "unknown dir is ascending",
			query: url.Values{"sort": {"repo"}, "dir": {"DESC"}},
			want:  Sort{Column: "repo"},
		},
		{
			name:  "click without a current order sorts ascending",
			query: url.Values{"by": {"rama"}},
			want:  Sort{Column: "rama"},
		},
		{
			name:  "click on the ascending column sorts descending",
			query: url.Values{"sort": {"rama"}, "by": {"rama"}},
			want:  Sort{Column: "rama", Desc: true},
		},
		{
			name:  "click on the descending column sorts ascending",
			query: url.Values{"sort": {"rama"}, "dir": {"desc"}, "by": {"rama"}},
			want:  Sort{Column: "rama"},
		},
		{
			name:  "click on another column sorts it ascending",
			query: url.Values{"sort": {"rama"}, "dir": {"desc"}, "by": {"repo"}},
			want:  Sort{Column: "repo"},
		},
		{
			name:  "unknown current column is ignored",
			query: url.Values{"sort": {"nope"}, "dir": {"desc"}},
			want:  Sort{},
		},
		{
			name:  "click on an unknown column is ignored",
			query: url.Values{"sort": {"repo"}, "by": {"nope"}},
			want:  Sort{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortFromQuery(tt.query, columns)

			if got != tt.want {
				t.Errorf("sortFromQuery() = %+v, want %+v", got, tt.want)
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
