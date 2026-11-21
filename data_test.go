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
