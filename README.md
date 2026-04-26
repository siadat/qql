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
$ qql --sql "SELECT id, cpu, ram WHERE status = 'up' AND cpu >= 16" testdata/servers.yaml
id    cpu  ram
----  ---  ---
web3  16   64
db1   32   128
```
