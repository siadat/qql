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
