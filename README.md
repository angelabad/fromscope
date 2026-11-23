# fromscope

A small Go tool to load a specific kind of CSV file and display it on a web
page with filters and groupings.

## Scope

This is, for now, a **very specific project**: it is built around one
particular CSV format (repository/branch listings with a `tiene_from` column)
and the views needed to explore it. It is not meant to be a general-purpose
CSV viewer yet.

Over time it will probably evolve into a **universal CSV viewer**, able to
load any CSV and offer filtering and grouping on arbitrary columns.

## Status

Early development. What works today:

- Loading a CSV file with a header row into memory.
- Dropping rows whose `tiene_from` column is `no`, then hiding that column.
- Serving the resulting table as a web page on `-addr` (default `:8080`).
- Filtering the table from the page, without reloading it (see [Filtering](#filtering)).
- Sorting the table by any column (see [Sorting](#sorting)).

Planned:

- Groupings by column.
- Support for arbitrary CSV formats.

## Expected CSV format

- First line is a header with the column names (surrounding spaces are trimmed).
- Every row must have the same number of fields as the header.
- A `tiene_from` column must exist; rows where it equals `no` are discarded,
  and the column itself is not shown in the web view.

Example:

```csv
repo,branch,tiene_from
app,master,si
app,develop,no
```

## Usage

Requires Go 1.27 or later.

```sh
go install github.com/angelabad/fromscope@latest
fromscope -csv path/to/file.csv
```

Or from a clone of the repository:

```sh
go build
./fromscope -csv path/to/file.csv
```

Then open <http://localhost:8080> in your browser.

### Flags

| Flag    | Default | Description                         |
|---------|---------|-------------------------------------|
| `-csv`  | —       | CSV file to load (required)         |
| `-addr` | `:8080` | TCP address to listen on            |

### Filtering

The page has a global search box and a filter under each column header.
Results update as you type, and the row counter shows how many rows match
out of the total. A row is shown only if it matches every filter:

- **Search box**: case-insensitive substring of any cell in the row.
- **Column filters**: case-insensitive substring of that column.
- **`rama` column**: a dropdown with its distinct values, matched exactly.

The filtered rows are served by `GET /rows` as an HTML fragment, which the
page calls through htmx. It takes the same criteria as query parameters (`q`
for the search, and one parameter per column name), so it can also be used
directly:

```sh
curl 'http://localhost:8080/rows?q=alpine&rama=master'
```

Empty parameters and unknown column names are ignored.

### Sorting

Rows are shown in CSV order until you click a column header, which sorts the
table by that column in ascending order. Clicking the same header again
toggles between ascending and descending; an arrow marks the active column
and its direction. The order is kept while you filter, and filters are kept
when you change the order.

Values are compared as plain text, byte by byte: uppercase sorts before
lowercase and `10` before `9`. Rows with equal values keep their CSV order.

`GET /rows` takes the order as query parameters: `sort` for the column and
`dir` (`asc` or `desc`) for the direction. For example:

```sh
curl 'http://localhost:8080/rows?rama=master&sort=repositorio&dir=desc'
```

An unknown column keeps the CSV order.

## Development

Linting requires [golangci-lint](https://golangci-lint.run/docs/welcome/install/)
in your `PATH`.

```sh
make help    # list the available targets
make fmt     # format the code (fixes most lint issues)
make check   # run every quality check, the same as CI
make build   # build the fromscope binary
make css     # rebuild static/app.css after changing classes in templates/
```

The stylesheet is built with the [Tailwind CSS](https://tailwindcss.com/docs/installation/tailwind-cli)
v4 standalone CLI (override the binary with `make css TAILWIND=/path/to/tailwindcss`).
The generated `static/app.css` is committed, so building the Go binary does not
require Tailwind. CI runs `make css-check` to make sure it matches the templates;
use the same Tailwind version as CI (pinned in `.github/workflows/ci.yml`), since
the output is compared byte for byte.

[htmx](https://htmx.org/) 4.0.2 is vendored in `static/htmx.min.js` (the
minified file does not include its version). To update it, replace the file
with the `dist/htmx.min.js` of the new release and change the version here.

## License

[MIT](LICENSE)
