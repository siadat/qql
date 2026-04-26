# qql

[![test](https://github.com/siadat/qql/actions/workflows/test.yml/badge.svg)](https://github.com/siadat/qql/actions/workflows/test.yml)

qql is a lightweight, extendable command-line data processor for YAML, JSON, git history, and user-supplied scripts. It exposes a SQL-like surface for selecting, filtering, sorting, and counting rows.

## Install

Run `go install github.com/siadat/qql@latest`.

## Invocation

`qql [--sql QUERY] [-o FORMAT] [--no-header] <file> [file ...]`. Output formats are `table` (default), `json`, and `jsonl`. Sources are dispatched by extension — `.json` and `.yaml`/`.yml` use built-in providers — or by source-prefix `git:<repo>` for git log rows.

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

`SELECT *` keeps every column. A comma-separated list of identifiers projects exactly those columns; identifiers may contain dot paths like `address.city` or `tags.0`. `SELECT COUNT(*)` collapses the post-WHERE result to a single row with one `count` column and cannot be combined with regular projections (see "Counting rows").

### FROM

A path to a YAML/JSON file, or `git:<repo>` for git log rows. When absent, positional CLI arguments supply the sources; when present, the FROM source becomes the first positional path and CLI arguments are appended.

### WHERE

A boolean expression over column references and literals. Comparison operators are `=`, `!=`, `<`, `<=`, `>`, `>=`. The pattern operator `MATCHES '<regex>'` runs a Go regular expression against the value (see "Pattern matching"). Logical connectives are `AND`, `OR`, `NOT`, with parentheses for grouping. Type mismatches between operands evaluate to false; only `= null` / `!= null` are useful — relational comparisons against `null` yield false.

### ORDER BY

One or more `<column> [ASC|DESC]` terms separated by commas; `ASC` is the default. The sort is stable, and types compare as `null < bool < number < string` so heterogeneous values still produce a deterministic order.

### LIMIT and OFFSET

`LIMIT N` caps the number of rows after ORDER BY; `OFFSET M` skips the first M; the two combine as `LIMIT N OFFSET M` for top-N pagination. Both N and M are non-negative integers; `LIMIT 0` is valid and returns zero rows (see "Limit and offset").

### WITH

Trailing configuration. Recognized keys today are `prefix = '<glob>'` (extract rows from a nested path — see "Nested rows") and `provider = 'external:<script>'` (replace built-in dispatch with a user-supplied script — see "External providers").

## Counting rows

`SELECT COUNT(*)` collapses the (post-WHERE) result to a single row with one `count` column:

```
$ qql --sql "SELECT COUNT(*) WHERE status = 'up'" testdata/servers.yaml
count
-----
4
```

Only `COUNT(*)` is supported — no `COUNT(col)`, no `COUNT(DISTINCT ...)`, no mixing with other selected columns. It composes with `WHERE`, `WITH prefix = ...`, and external providers.

## Limit and offset

Cap the number of rows with `LIMIT N` and skip the first `M` rows with `OFFSET M` (both non-negative integers). They run after `ORDER BY`, so combining all three gives paginated top-N queries:

```
$ qql --sql "SELECT key, cpu ORDER BY cpu DESC LIMIT 2 OFFSET 1" testdata/servers.yaml
key   cpu
----  ---
web3  16
web1  8
```

`LIMIT 0` returns zero rows. `OFFSET` works without `LIMIT` (skip M, return the rest). Clause order: `ORDER BY ... LIMIT ... OFFSET ... WITH ...`.

## Pattern matching

The `MATCHES` operator runs a Go regular expression against the left-hand value:

```
$ qql --sql "SELECT key, cpu WHERE key MATCHES '^web'" testdata/servers.yaml
key   cpu
----  ---
web1  8
web2  8
web3  16
```

Patterns are not anchored — use `^` and `$` for full matches. Combine with `NOT` to invert, e.g. `WHERE NOT key MATCHES '^web'`. Non-string values are coerced to their textual form, so `size MATCHES '^4\d+$'` works on numbers too. Invalid regexes fail at parse time.

## Nested rows

When the rows of interest live deeper than the top level, point at them with `WITH prefix = '<path>'`. Each `*` segment in the path captures the matched key as a column. The rightmost `*` becomes `key` and shows the **full path from the root** through that row (so `*.servers` against the file below yields `key = "region-a.servers"`); earlier `*`s become `key_capture_1`, `key_capture_2`, … and show just the matched key for filtering convenience.

Given `testdata/regions.yaml` (also included):

```yaml
region-a:
  servers:
    web1: {cpu: 8, ram: 32, status: up}
    db1: {cpu: 32, ram: 128, status: up}
region-b:
  servers:
    web1: {cpu: 4, ram: 16, status: down}
    cache1: {cpu: 4, ram: 16, status: up}
region-c:
  servers:
    web1: {cpu: 16, ram: 64, status: up}
    web2: {cpu: 8, ram: 32, status: down}
```

Run:

```
$ qql --sql "SELECT key, key_capture_1, cpu, ram WHERE status = 'up' ORDER BY cpu DESC, key_capture_1 WITH prefix = '*.servers.*'" testdata/regions.yaml
```
Output:
```
key                      key_capture_1  cpu  ram
-----------------------  -------------  ---  ---
region-a.servers.db1     region-a       32   128
region-c.servers.web1    region-c       16   64
region-a.servers.web1    region-a       8    32
region-b.servers.cache1  region-b       4    16
```

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
