// Fromscope loads a CSV file and serves it on a web page
// with filters and groupings.
package main

import (
	"flag"
	"log"
	"os"
)

var (
	csvPath = flag.String("csv", "", "CSV file to load (required)")
	addr    = flag.String("addr", ":8080", "TCP address to listen on") //nolint:unused // used once the web server lands
)

func main() {
	flag.Parse()
	if *csvPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	table, err := LoadCSV(*csvPath)
	if err != nil {
		log.Fatalf("loading CSV: %v", err)
	}

	dropped, err := table.DropRows(fromColumn, noFromValue)
	if err != nil {
		log.Fatalf("dropping rows: %v", err)
	}

	err = table.DropColumn(fromColumn)
	if err != nil {
		log.Fatalf("dropping column: %v", err)
	}

	log.Printf("loaded %d rows (dropped %d without FROM), columns: %q",
		len(table.Rows), dropped, table.Columns)
}
