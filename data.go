package main

import (
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
