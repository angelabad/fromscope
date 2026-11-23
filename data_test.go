package main

import (
	"encoding/csv"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantColumns []string
		wantRows    int
		wantErr     error // nil = no error expected; checked with errors.Is
		anyErr      bool  // true = any error is fine
	}{
		{
			name:        "valid CSV",
			input:       "repo,branch\napp,master\napp,develop\n",
			wantColumns: []string{"repo", "branch"},
			wantRows:    2,
		},
		{
			name:        "header only",
			input:       "repo,branch\n",
			wantColumns: []string{"repo", "branch"},
			wantRows:    0,
		},
		{
			name:        "trims spaces in header",
			input:       " repo , branch \napp,master\n",
			wantColumns: []string{"repo", "branch"},
			wantRows:    1,
		},
		{
			name:        "windows line endings",
			input:       "repo,branch\r\napp,master\r\n",
			wantColumns: []string{"repo", "branch"},
			wantRows:    1,
		},
		{
			name:   "empty input",
			input:  "",
			anyErr: true,
		},
		{
			name:    "row with extra field",
			input:   "repo,branch\napp,master,extra\n",
			wantErr: csv.ErrFieldCount,
		},
		{
			name:    "row with missing field",
			input:   "repo,branch\napp\n",
			wantErr: csv.ErrFieldCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table, err := ParseCSV(strings.NewReader(tt.input))

			if tt.anyErr || tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(table.Columns, tt.wantColumns) {
				t.Errorf("columns = %q, want %q", table.Columns, tt.wantColumns)
			}
			if len(table.Rows) != tt.wantRows {
				t.Errorf("rows = %d, want %d", len(table.Rows), tt.wantRows)
			}
		})
	}
}

func TestColumnIndex(t *testing.T) {
	table := &Table{Columns: []string{"repo", "branch", "image"}}

	if got := table.ColumnIndex("branch"); got != 1 {
		t.Errorf("ColumnIndex(branch) = %d, want 1", got)
	}
	if got := table.ColumnIndex("missing"); got != -1 {
		t.Errorf("ColumnIndex(missing) = %d, want -1", got)
	}
}

func TestDropRows(t *testing.T) {
	newTable := func() *Table {
		return &Table{
			Columns: []string{"repo", "has_from"},
			Rows: [][]string{
				{"a", "yes"},
				{"b", "no"},
				{"c", "yes"},
				{"d", "no"},
			},
		}
	}

	t.Run("removes matching rows", func(t *testing.T) {
		table := newTable()

		dropped, err := table.DropRows("has_from", "no")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dropped != 2 {
			t.Errorf("dropped = %d, want 2", dropped)
		}

		want := [][]string{{"a", "yes"}, {"c", "yes"}}
		if !slices.EqualFunc(table.Rows, want, slices.Equal) {
			t.Errorf("rows = %q, want %q", table.Rows, want)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		table := newTable()

		dropped, err := table.DropRows("has_from", "maybe")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dropped != 0 || len(table.Rows) != 4 {
			t.Errorf("dropped = %d, rows = %d; want 0 and 4", dropped, len(table.Rows))
		}
	})

	t.Run("unknown column", func(t *testing.T) {
		table := newTable()

		if _, err := table.DropRows("missing", "no"); err == nil {
			t.Fatal("expected an error, got nil")
		}
		if len(table.Rows) != 4 {
			t.Errorf("rows = %d, want 4 (table must not change)", len(table.Rows))
		}
	})
}

func TestDropColumn(t *testing.T) {
	newTable := func() *Table {
		return &Table{
			Columns: []string{"repo", "has_from", "image"},
			Rows: [][]string{
				{"a", "yes", "alpine"},
				{"b", "no", "debian"},
			},
		}
	}

	tests := []struct {
		name        string
		column      string
		wantColumns []string
		wantRows    [][]string
	}{
		{
			name:        "middle column",
			column:      "has_from",
			wantColumns: []string{"repo", "image"},
			wantRows:    [][]string{{"a", "alpine"}, {"b", "debian"}},
		},
		{
			name:        "first column",
			column:      "repo",
			wantColumns: []string{"has_from", "image"},
			wantRows:    [][]string{{"yes", "alpine"}, {"no", "debian"}},
		},
		{
			name:        "last column",
			column:      "image",
			wantColumns: []string{"repo", "has_from"},
			wantRows:    [][]string{{"a", "yes"}, {"b", "no"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := newTable()

			if err := table.DropColumn(tt.column); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(table.Columns, tt.wantColumns) {
				t.Errorf("columns = %q, want %q", table.Columns, tt.wantColumns)
			}
			if !slices.EqualFunc(table.Rows, tt.wantRows, slices.Equal) {
				t.Errorf("rows = %q, want %q", table.Rows, tt.wantRows)
			}
		})
	}

	t.Run("unknown column", func(t *testing.T) {
		table := newTable()

		if err := table.DropColumn("missing"); err == nil {
			t.Fatal("expected an error, got nil")
		}
		if len(table.Columns) != 3 || len(table.Rows[0]) != 3 {
			t.Errorf("table changed: columns = %q, rows = %q", table.Columns, table.Rows)
		}
	})
}

// newFilterTable returns a table for the Filter and Values tests.
// "develop" and "development" check that Equals is an exact match.
func newFilterTable() *Table {
	return &Table{
		Columns: []string{"repo", "rama", "imagen"},
		Rows: [][]string{
			{"aem-atc", "master", "cypress/included:13.14.1"},
			{"web", "develop", "node:20-alpine"},
			{"api", "development", "Alpine:3.20"},
			{"api", "master", ""},
		},
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		want   []string // "repo/rama" of the expected rows, in order
	}{
		{
			name:   "empty filter keeps every row",
			filter: Filter{},
			want:   []string{"aem-atc/master", "web/develop", "api/development", "api/master"},
		},
		{
			name:   "query is case-insensitive",
			filter: Filter{Query: "ALPINE"},
			want:   []string{"web/develop", "api/development"},
		},
		{
			name:   "query matches any column",
			filter: Filter{Query: "master"},
			want:   []string{"aem-atc/master", "api/master"},
		},
		{
			name:   "contains is case-insensitive",
			filter: Filter{Contains: map[string]string{"imagen": "alpine"}},
			want:   []string{"web/develop", "api/development"},
		},
		{
			name:   "contains only looks at its column",
			filter: Filter{Contains: map[string]string{"repo": "master"}},
			want:   []string{},
		},
		{
			name:   "equals is an exact match",
			filter: Filter{Equals: map[string]string{"rama": "develop"}},
			want:   []string{"web/develop"},
		},
		{
			name:   "equals is case-sensitive",
			filter: Filter{Equals: map[string]string{"rama": "Master"}},
			want:   []string{},
		},
		{
			name: "every criterion must match",
			filter: Filter{
				Query:    "alpine",
				Contains: map[string]string{"repo": "api"},
				Equals:   map[string]string{"rama": "development"},
			},
			want: []string{"api/development"},
		},
		{
			name: "empty values are ignored",
			filter: Filter{
				Contains: map[string]string{"repo": ""},
				Equals:   map[string]string{"rama": ""},
			},
			want: []string{"aem-atc/master", "web/develop", "api/development", "api/master"},
		},
		{
			name: "unknown columns are ignored",
			filter: Filter{
				Contains: map[string]string{"nope": "x"},
				Equals:   map[string]string{"nope": "x"},
			},
			want: []string{"aem-atc/master", "web/develop", "api/development", "api/master"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := newFilterTable()

			got := table.Filter(tt.filter)

			if !slices.Equal(got.Columns, table.Columns) {
				t.Errorf("Columns = %q, want %q", got.Columns, table.Columns)
			}
			keys := []string{}
			for _, row := range got.Rows {
				keys = append(keys, row[0]+"/"+row[1])
			}
			if !slices.Equal(keys, tt.want) {
				t.Errorf("rows = %q, want %q", keys, tt.want)
			}
		})
	}
}

func TestFilterDoesNotModifyTable(t *testing.T) {
	table := newFilterTable()
	want := newFilterTable()

	got := table.Filter(Filter{Equals: map[string]string{"rama": "master"}})

	if len(got.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(got.Rows))
	}
	if !slices.EqualFunc(table.Rows, want.Rows, slices.Equal) {
		t.Errorf("Filter modified the table rows: %q", table.Rows)
	}
	if !slices.Equal(table.Columns, want.Columns) {
		t.Errorf("Filter modified the table columns: %q", table.Columns)
	}
}

func TestValues(t *testing.T) {
	tests := []struct {
		column string
		want   []string
	}{
		{"rama", []string{"develop", "development", "master"}},                            // sorted and distinct
		{"imagen", []string{"Alpine:3.20", "cypress/included:13.14.1", "node:20-alpine"}}, // no empty value
		{"nope", nil},
	}

	for _, tt := range tests {
		t.Run(tt.column, func(t *testing.T) {
			got := newFilterTable().Values(tt.column)
			if !slices.Equal(got, tt.want) || (tt.want == nil) != (got == nil) {
				t.Errorf("Values(%q) = %#v, want %#v", tt.column, got, tt.want)
			}
		})
	}
}

func TestSort(t *testing.T) {
	tests := []struct {
		name string
		sort Sort
		want []string // "repo/rama" of the expected rows, in order
	}{
		{
			name: "zero value keeps the CSV order",
			sort: Sort{},
			want: []string{"aem-atc/master", "web/develop", "api/development", "api/master"},
		},
		{
			name: "unknown column keeps the CSV order",
			sort: Sort{Column: "nope"},
			want: []string{"aem-atc/master", "web/develop", "api/development", "api/master"},
		},
		{
			name: "ascending, ties keep the CSV order",
			sort: Sort{Column: "repo"},
			want: []string{"aem-atc/master", "api/development", "api/master", "web/develop"},
		},
		{
			name: "descending, ties keep the CSV order",
			sort: Sort{Column: "repo", Desc: true},
			want: []string{"web/develop", "api/development", "api/master", "aem-atc/master"},
		},
		{
			name: "empty values first",
			sort: Sort{Column: "imagen"},
			want: []string{"api/master", "api/development", "aem-atc/master", "web/develop"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := newFilterTable()

			got := table.Sort(tt.sort)

			if !slices.Equal(got.Columns, table.Columns) {
				t.Errorf("Columns = %q, want %q", got.Columns, table.Columns)
			}
			keys := []string{}
			for _, row := range got.Rows {
				keys = append(keys, row[0]+"/"+row[1])
			}
			if !slices.Equal(keys, tt.want) {
				t.Errorf("rows = %q, want %q", keys, tt.want)
			}
		})
	}
}

func TestSortDoesNotModifyTable(t *testing.T) {
	table := newFilterTable()
	want := newFilterTable()

	got := table.Sort(Sort{Column: "repo", Desc: true})

	if got.Rows[0][0] != "web" {
		t.Fatalf("got first repo %q, want web", got.Rows[0][0])
	}
	if !slices.EqualFunc(table.Rows, want.Rows, slices.Equal) {
		t.Errorf("Sort modified the table rows: %q", table.Rows)
	}
	if !slices.Equal(table.Columns, want.Columns) {
		t.Errorf("Sort modified the table columns: %q", table.Columns)
	}
}

func TestSortWithoutColumnReturnsCopy(t *testing.T) {
	for _, s := range []Sort{{}, {Column: "nope"}} {
		table := newFilterTable()

		got := table.Sort(s)

		if got == table || &got.Rows[0] == &table.Rows[0] {
			t.Errorf("Sort(%+v) shares the receiver's rows, want a copy", s)
		}
	}
}
