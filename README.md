# qql

[![test](https://github.com/siadat/qql/actions/workflows/test.yml/badge.svg)](https://github.com/siadat/qql/actions/workflows/test.yml)

qql is a lightweight, extendable command-line data processor for YAML, JSON, git history, and user-supplied scripts. It exposes a SQL-like surface for selecting, filtering, sorting, and counting rows.

## Install

Run `go install github.com/siadat/qql@latest`.

## Invocation

`qql [--sql QUERY] [-o FORMAT] [--no-header] <file> [file ...]`. Output formats are `table` (default), `json`, and `jsonl`. Sources are dispatched by extension — `.json` and `.yaml`/`.yml` use built-in providers — or by source-prefix `git:<repo>` for git log rows.

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
$ qql --sql "SELECT key, cpu, ram WHERE status = 'up' AND cpu >= 16" testdata/servers.yaml
```
Output:
```
key   cpu  ram
----  ---  ---
web3  16   64
db1   32   128
```

## Query syntax

A query is a sequence of clauses written in this fixed order; every clause is optional:

- `SELECT <projection>`
- `FROM <source>`
- `WHERE <predicate>`
- `ORDER BY <terms>`
- `LIMIT <N>`
- `OFFSET <M>`
- `WITH <key> = '<value>' [, ...]`

Keywords are case-insensitive. String literals use single or double quotes (no escapes — bytes are taken verbatim, which keeps regex and path-glob values readable). Numbers may be integer or floating-point. The keywords `true`, `false`, and `null` carry their usual meaning.

### SELECT

`SELECT *` keeps every column. A comma-separated list of identifiers projects exactly those columns; identifiers may contain dot paths like `address.city` or `tags.0`. `SELECT COUNT(*)` collapses the post-WHERE result to a single row with one `count` column and cannot be combined with regular projections; only the bare `COUNT(*)` form is supported (no `COUNT(col)`, no `COUNT(DISTINCT ...)`).

### FROM

A path to a YAML/JSON file, or `git:<repo>` for git log rows. When absent, positional CLI arguments supply the sources; when present, the FROM source becomes the first positional path and CLI arguments are appended.

### WHERE

A boolean expression over column references and literals. Comparison operators are `=`, `!=`, `<`, `<=`, `>`, `>=`. The pattern operator `MATCHES '<regex>'` runs a Go regular expression against the value. Logical connectives are `AND`, `OR`, `NOT`, with parentheses for grouping. Type mismatches between operands evaluate to false; only `= null` / `!= null` are useful — relational comparisons against `null` yield false.

### MATCHES

`<value> MATCHES '<regex>'` is a `WHERE` operator that tests the left-hand value against a Go regular expression. Patterns are not anchored — use `^` and `$` for full matches. Combine with `NOT` to invert (e.g. `WHERE NOT key MATCHES '^web'`). Non-string values are coerced to their textual form, so a numeric column accepts patterns like `MATCHES '^4\d+$'`. Invalid regexes fail at parse time with the offset of the bad pattern.

### ORDER BY

One or more `<column> [ASC|DESC]` terms separated by commas; `ASC` is the default. The sort is stable, and types compare as `null < bool < number < string` so heterogeneous values still produce a deterministic order.

### LIMIT and OFFSET

`LIMIT N` caps the number of rows after ORDER BY; `OFFSET M` skips the first M; the two combine as `LIMIT N OFFSET M` for top-N pagination, and `OFFSET` may appear without `LIMIT` to skip a prefix and return the rest. Both N and M are non-negative integers; `LIMIT 0` is valid and returns zero rows.

### WITH

Trailing configuration as a comma-separated list of `<key> = '<value>'` pairs. Recognized keys:

- `prefix = '<glob>'` — extract rows from a nested dot-path. A `*` segment matches any map key and captures it; literal segments descend without capturing; non-matching branches are silently skipped. The rightmost `*` becomes the `key` column and carries the full path from the root (e.g. `*.servers` yields `key = "region-a.servers"`); earlier `*`s become `key_capture_1`, `key_capture_2`, … and carry just the matched key, useful for `WHERE` filtering.
- `provider = 'external:<script>'` — replace built-in dispatch with a user-supplied script (see "External providers").

## External providers

You can plug in any executable as a row source with `WITH provider = 'external:<path>'`. qql execs the script once per query, hands it a JSON request on stdin, and reads JSONL rows from stdout. WHERE/ORDER BY are re-applied by qql, so the script is free to ignore them — the script doesn't need to understand qql's predicate grammar to participate.

The bundled `examples/providers/fs.py` walks directories and emits one row per file:

```
$ qql --sql "SELECT key, name, size WHERE size > 100 ORDER BY size DESC WITH provider = 'external:./examples/providers/fs.py'" testdata
key                    name          size
---------------------  ------------  ----
testdata/regions.yaml  regions.yaml  311
testdata/servers.yaml  servers.yaml  242
```

### Wire protocol

- **stdin** — one JSON object, then EOF:
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
  Every field after `version` is a hint. `where` is the **raw SQL substring** of the WHERE clause; the script can grep it, ignore it, or reparse it. qql always re-applies the parsed predicate to whatever rows the script returns.

- **stdout** — JSONL. One JSON object per line. Empty lines and `#`-prefixed comment lines are skipped. Malformed lines are logged to stderr and dropped.

- **stderr** — passes through to your terminal verbatim, so progress logs and error messages from the script surface naturally.

- **exit code** — non-zero aborts the query; qql discards stdout and surfaces the error.
