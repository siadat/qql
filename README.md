# qql

[![test](https://github.com/siadat/qql/actions/workflows/test.yml/badge.svg)](https://github.com/siadat/qql/actions/workflows/test.yml)

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
$ qql --sql "SELECT key, cpu, ram WHERE status = 'up' AND cpu >= 16" testdata/servers.yaml
```
Output:
```
key   cpu  ram
----  ---  ---
web3  16   64
db1   32   128
```

## Limit

Cap the number of rows with `LIMIT N` (non-negative integer). It runs after `ORDER BY`, so combining the two gives top-N queries:

```
$ qql --sql "SELECT key, cpu ORDER BY cpu DESC LIMIT 2" testdata/servers.yaml
key   cpu
----  ---
db1   32
web3  16
```

`LIMIT 0` is valid and returns zero rows. Place `LIMIT` after `ORDER BY` and before `WITH`.

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
