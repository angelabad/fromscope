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
- Dropping rows whose `tiene_from` column is `no`.

Planned:

- Web interface to browse the data (served on `-addr`, default `:8080`).
- Filters and groupings by column.
- Support for arbitrary CSV formats.

## Expected CSV format

- First line is a header with the column names (surrounding spaces are trimmed).
- Every row must have the same number of fields as the header.
- A `tiene_from` column must exist; rows where it equals `no` are discarded.

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

### Flags

| Flag    | Default | Description                         |
|---------|---------|-------------------------------------|
| `-csv`  | —       | CSV file to load (required)         |
| `-addr` | `:8080` | TCP address to listen on (planned)  |

## Development

```sh
go vet ./...
go test ./...
```

## License

[MIT](LICENSE)
