package main

import (
	"cmp"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// Table holds the CSV contents in memory.
type Table struct {
	Columns []string   // column names taken from the CSV header
	Rows    [][]string // each row has len(Columns) cells
}

// Rows whose fromColumn equals noFromValue are dropped at load time.
const (
	fromColumn  = "tiene_from"
	noFromValue = "no"
)

// dropdownColumns are filtered with a dropdown of their distinct values
// and an exact match, instead of a free-text "contains" search.
var dropdownColumns = []string{"rama"}

// ParseCSV reads a CSV with a header line from r.
// Every row must have the same number of fields as the header.
func ParseCSV(r io.Reader) (*Table, error) {
	reader := csv.NewReader(r)

	headers, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return nil, errors.New("CSV is empty")
	}
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}

	table := &Table{Columns: headers}

	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading rows: %w", err)
		}
		table.Rows = append(table.Rows, row)
	}

	return table, nil
}

// LoadCSV opens the file at path and parses it with ParseCSV.
func LoadCSV(path string) (*Table, error) {
	file, err := os.Open(path) //nolint:gosec // G304: path comes from the -csv flag, set by the user running the CLI
	if err != nil {
		return nil, err // os.Open already includes the path in the message
	}
	defer file.Close()

	return ParseCSV(file)
}

// ColumnIndex returns the index of the named column, or -1 if it doesn't exist.
func (t *Table) ColumnIndex(name string) int {
	return slices.Index(t.Columns, name)
}

// DropRows removes the rows whose value in column equals value
// and returns how many rows were removed.
func (t *Table) DropRows(column, value string) (int, error) {
	idx := t.ColumnIndex(column)
	if idx == -1 {
		return 0, fmt.Errorf("column %q not found", column)
	}

	before := len(t.Rows)
	t.Rows = slices.DeleteFunc(t.Rows, func(row []string) bool {
		return row[idx] == value
	})

	return before - len(t.Rows), nil
}

// DropColumn removes the named column from Columns and from every row.
// It returns an error, leaving the table unchanged, if the column doesn't exist.
func (t *Table) DropColumn(column string) error {
	idx := t.ColumnIndex(column)
	if idx == -1 {
		return fmt.Errorf("column %q not found", column)
	}

	t.Columns = slices.Delete(t.Columns, idx, idx+1)

	for i, row := range t.Rows {
		t.Rows[i] = slices.Delete(row, idx, idx+1)
	}

	return nil
}

// Filter holds the criteria used to select rows. A row is kept only if
// it matches every criterion; empty values and unknown columns are ignored.
type Filter struct {
	Query    string            // case-insensitive substring of any cell
	Contains map[string]string // column name -> case-insensitive substring
	Equals   map[string]string // column name -> exact value
}

// Filter returns a new table with the rows that match f.
// The receiver is never modified.
func (t *Table) Filter(f Filter) *Table {
	var conds []condition

	for col, v := range f.Contains {
		idx := t.ColumnIndex(col)
		if idx == -1 || v == "" {
			continue
		}

		conds = append(conds, condition{idx: idx, value: strings.ToLower(v), exact: false})
	}

	for col, v := range f.Equals {
		idx := t.ColumnIndex(col)
		if idx == -1 || v == "" {
			continue
		}

		conds = append(conds, condition{idx: idx, value: v, exact: true})
	}

	query := strings.ToLower(f.Query)

	rows := make([][]string, 0, len(t.Rows))

	for _, row := range t.Rows {
		if matchRow(row, query, conds) {
			rows = append(rows, row)
		}
	}

	return &Table{Columns: t.Columns, Rows: rows}
}

// condition is a Filter criterion resolved against a table's columns.
type condition struct {
	idx   int    // column index in each row
	value string // lowercased when exact is false
	exact bool   // true: cell == value; false: lowercased cell contains value
}

// matchRow reports whether row meets every condition and, if query
// is not empty, has a cell that contains it.
func matchRow(row []string, query string, conds []condition) bool {
	for _, c := range conds {
		cell := row[c.idx]
		if c.exact {
			if cell != c.value {
				return false
			}
		} else if !strings.Contains(strings.ToLower(cell), c.value) {
			return false
		}
	}

	if query == "" {
		return true
	}

	return slices.ContainsFunc(row, func(cell string) bool {
		return strings.Contains(strings.ToLower(cell), query)
	})
}

// Sort holds a sort order. The zero value keeps the CSV order.
type Sort struct {
	Column string // column name; empty or unknown keeps the CSV order
	Desc   bool   // descending instead of ascending
}

// Sort returns a new table with the rows sorted by s.
// The receiver is never modified.
func (t *Table) Sort(s Sort) *Table {
	ordered := slices.Clone(t.Rows)

	idx := t.ColumnIndex(s.Column)
	if idx == -1 {
		return &Table{Columns: t.Columns, Rows: ordered}
	}

	slices.SortStableFunc(ordered, func(a, b []string) int {
		c := cmp.Compare(a[idx], b[idx])
		if s.Desc {
			return -c
		}
		return c
	})

	return &Table{
		Columns: t.Columns,
		Rows:    ordered,
	}
}

// Values returns the distinct non-empty values of column, sorted.
// It returns nil if the column doesn't exist.
func (t *Table) Values(column string) []string {
	idx := t.ColumnIndex(column)
	if idx == -1 {
		return nil
	}

	var values []string

	for _, row := range t.Rows {
		if row[idx] != "" {
			values = append(values, row[idx])
		}
	}

	slices.Sort(values)

	return slices.Compact(values)
}
