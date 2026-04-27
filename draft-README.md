# qql

[![test](https://github.com/siadat/qql/actions/workflows/test.yml/badge.svg)](https://github.com/siadat/qql/actions/workflows/test.yml)

> Query structured YAML and JSON configs and data files

qql is a lightweight and extendable command-line data processor akin to `jq` for working with YAML and other data sources.

## Install

```
go install github.com/siadat/qql@latest
```

## Example

Given `testdata/servers.yaml` (included in the repo):

```yaml
web1: {cpu: 8, ram: 32, status: up, role: web}
web2: {cpu: 8, ram: 32, status: down, role: web}
web3: {cpu: 16, ram: 64, status: up, role: web}
db1: {cpu: 32, ram: 128, status: up, role: db}
cache1: {cpu: 4, ram: 16, status: up, role: cache}
```

Run:

```
$ qql "SELECT key, cpu, ram WHERE status = 'up' AND cpu >= 16" testdata/servers.yaml
```
Output:
```
key   cpu  ram
----  ---  ---
web3  16   64
db1   32   128
```

## Query syntax

A query is a sequence of clauses written in this fixed order. Every clause is optional:

- `SELECT <projection>`
- `FROM <source>`
- `WHERE <predicate>`
- `ORDER BY <terms>`
- `LIMIT <N>`
- `OFFSET <M>`
- `WITH <key> = '<value>' [, ...]`

Keywords are case-insensitive. String literals use single or double quotes (no escapes: bytes are taken verbatim, which keeps regex and path-glob values readable). Numbers may be integer or floating-point. The keywords `true`, `false`, and `null` carry their usual meaning.

### SELECT

`SELECT *` keeps every column. A comma-separated list of identifiers projects exactly those columns. Identifiers may contain dot paths like `address.city` or `tags.0`. `SELECT COUNT(*)` collapses the post-WHERE result to a single row with one `count` column and cannot be combined with regular projections. Only the bare `COUNT(*)` form is supported (no `COUNT(col)`, no `COUNT(DISTINCT ...)`).

### FROM

A path to a YAML or JSON file. When absent, positional CLI arguments supply the sources. When present, the FROM source becomes the first positional path and CLI arguments are appended.

### WHERE

- A boolean expression over column references and literals. Comparison operators are `=`, `!=`, `<`, `<=`, `>`, `>=`.
- The pattern operator `MATCHES '<regex>'` runs a Go regular expression against the value. `NOT MATCHES '<regex>'` negates it.
- Logical connectives are `AND`, `OR`, `NOT`, with parentheses for grouping.
- Type mismatches between operands evaluate to false, except with `= null` and `!= null`.

### ORDER BY

One or more `<column> [ASC|DESC]` terms separated by commas. `ASC` is the default. The sort is stable, and types compare as `null < bool < number < string` so heterogeneous values still produce a deterministic order.

### LIMIT and OFFSET

`LIMIT N` caps the number of rows after ORDER BY. `OFFSET M` skips the first M. The two combine as `LIMIT N OFFSET M` for top-N pagination, and `OFFSET` may appear without `LIMIT` to skip a prefix and return the rest. Both N and M are non-negative integers. `LIMIT 0` is valid and returns zero rows.

### WITH

Trailing configuration as a comma-separated list of `<key> = '<value>'` pairs. Recognized keys:

- `prefix = '<glob>'`: extract rows from a nested dot-path. A `*` segment matches any map key and captures it. Literal segments descend without capturing. Non-matching branches are silently skipped. The rightmost `*` becomes the `key` column and carries the full path from the root (e.g. `*.servers` yields `key = "region-a.servers"`). Earlier `*`s become `key_capture_1`, `key_capture_2`, … and carry just the matched key, useful for `WHERE` filtering.
- `provider = 'git:<repo>'`: read commit rows from the given repository (use `git:.` for the current directory). Columns: `commit`, `author`, `email`, `time`, `subject`. `FROM` and positional paths are ignored.
- `provider = 'external:<script>'`: replace built-in dispatch with a user-supplied script (see "External providers").

## External providers

You can plug in any executable as a row source with `WITH provider = 'external:<path>'`. qql execs the script once per query, hands it a JSON request on stdin, and reads JSONL envelopes from stdout. WHERE/ORDER BY are re-applied by qql, so the script is free to ignore them: the script doesn't need to understand qql's predicate grammar to participate.

The bundled `examples/providers/fs.py` walks directories and emits one row per file:

```
$ qql "SELECT key, name, size
       WHERE size > 100
       ORDER BY size DESC
       WITH provider = 'external:./examples/providers/fs.py'" ./testdata/
```

The bundled `examples/providers/yaml_file_reader.py` reads YAML files and emits each document as a `tree`. qql unfolds it with the same dot-glob prefix used by built-in YAML/JSON loaders:

```
$ qql "SELECT key, cpu
       WITH provider = 'external:./examples/providers/yaml_file_reader.py',
            prefix = '*.servers.*'" testdata/regions.yaml
```

### Protocol

- **stdin**: one JSON object, then EOF:
  ```json
  {
    "version": 1,
    "source": "regions.yaml",
    "files": ["regions.yaml"],
    "prefix": "*.servers.*",
    "select": ["key", "cpu"],
    "where": "status = 'up' AND cpu >= 16",
    "order_by": [{"col": "cpu", "desc": true}]
  }
  ```
  Every field after `version` is a hint. `where` is the **raw SQL substring** of the WHERE clause. The script can grep it, ignore it, or reparse it. qql always re-applies the parsed predicate to whatever rows the script returns.

- **stdout**: JSONL. One envelope per line of the form `{"type": "row" | "tree", "value": ...}`:
  - `"type": "row"`: `value` is a flat object whose keys are columns. qql appends it to the result set unchanged.
  - `"type": "tree"`: `value` is a nested document (map/list/scalar). qql unfolds it via the WITH `prefix` glob the same way it would a YAML or JSON file.

  Empty lines and `#`-prefixed comment lines are skipped. Lines that aren't valid envelopes are logged to stderr and dropped.

- **stderr**: passes through to your terminal verbatim, so progress logs and error messages from the script surface naturally.

- **exit code**: non-zero aborts the query. qql discards stdout and surfaces the error.
