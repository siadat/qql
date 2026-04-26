# qql

## Install

```
go install github.com/siadat/qql@latest
```

## Example

Given `servers.json`:

```json
{
  "web1":   {"cpu":  8, "ram":  32, "status": "up",   "role": "web"},
  "web2":   {"cpu":  8, "ram":  32, "status": "down", "role": "web"},
  "web3":   {"cpu": 16, "ram":  64, "status": "up",   "role": "web"},
  "db1":    {"cpu": 32, "ram": 128, "status": "up",   "role": "db"},
  "cache1": {"cpu":  4, "ram":  16, "status": "up",   "role": "cache"}
}
```

Run:

```
$ qql --sql "SELECT id, cpu, ram WHERE status = 'up' AND cpu >= 16" servers.json
id    cpu  ram
----  ---  ---
db1   32   128
web3  16   64
```
