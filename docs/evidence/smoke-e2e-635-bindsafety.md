# smoke:e2e — #635 empty `server.host` bind-safety hole

Change type: `bug-fix` (security) · Branch: `fix/635-empty-host-bind-safety` · Date: 2026-08-26

## Scope note on what was NOT run

**No server was started with an empty or non-loopback host.** Doing so binds every interface on the developer's machine, which is outward-facing. The independent reviewer made the same call. The pre-fix behavior therefore rests on measured component semantics plus code reading, not on an actual wildcard bind:

- `fmt.Sprintf("%s:%d", "", 3199)` → `":3199"`, and `net.Listen` on `":port"` binds all interfaces — standard, documented Go behavior.
- `net.SplitHostPort("[]:3199")` → host `""`, **no error** (measured, below).
- Pre-fix `isLoopback("")` returned `true` (the old source had `if h == "" || h == "localhost" || ...`).

The post-fix behavior **is** measured end to end.

## 1. End-to-end — a bare `host:` now binds loopback only

Config used (`host:` deliberately left with no value — what commenting out the line produces):

```yaml
server:
  host:
  port: 3199
database:
  url: postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test?sslmode=disable
```

Server started on **:3199** against **`nanobrain_test`**. Never `nanobrain_dev`, never `:3100`. Stopped by exact PID afterwards; `nanobrain-pg` untouched.

```
grep -o '"addr":"[^"]*"' server.log
"addr":"localhost:3199"

lsof -nP -iTCP:3199 -sTCP:LISTEN
   127.0.0.1:3199

curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3199/api/status
200
```

And the point of the fix — the same daemon is **not** reachable on this
machine's LAN address, because no listener exists there. Measured against the
real interface address from `ipconfig getifaddr en0` (redacted below as
`<LAN-IP>`; a private 192.168.0.0/16 address):

```
curl -s -o /dev/null -w 'code=%{http_code}\n' --max-time 3 http://127.0.0.1:3199/api/status
code=200

curl -s -o /dev/null -w 'code=%{http_code}\n' --max-time 3 http://<LAN-IP>:3199/api/status
code=000
curl: (28) Connection timed out after 3004 milliseconds
```

Pre-fix, this config produced the listen address `":3199"` — a wildcard bind
— so that second probe would have been served.

Bound to `127.0.0.1` only. Pre-fix this config produced the listen address `":3199"`.

## 2. `isLoopback` / `checkBindSafety` matrix

Measured via a temporary test in `cmd/nano-brain/` (deleted afterwards; `git status` clean). `checkBindSafety` evaluated with `authEnabled=false, unsafeNoAuth=false`.

| host | `isLoopback` | blocked? | note |
|---|---|---|---|
| `""` | false | yes | wildcard; normalized away by `Load` before this is reached |
| `"   "` | false | yes | same |
| `"[]"` | false | yes | **the variant `Load` does not normalize** — this check is its only defense |
| `"[::]"` | false | yes | IPv6 wildcard, bracketed |
| `"::"` | false | yes | IPv6 wildcard |
| `"0.0.0.0"` | false | yes | IPv4 wildcard |
| `"0000:0000:...:0000"` | false | yes | expanded IPv6 zero |
| `"::ffff:127.0.0.1"` | true | no | genuinely loopback |
| `"LOCALHOST"` | true | no | case-insensitive |
| `"127.0.0.5"` | true | no | 127/8 |
| `"localhost."` | false | yes | trailing-dot FQDN; fails closed |

No value was found where `isLoopback` reports true but the resulting bind is not loopback.

## 3. The bracket-pair case, measured

```
host="[]"    Trim->""     addr="[]:3199"    SplitHostPort host=""    err=<nil>
host="[::]"  Trim->"::"   addr="[::]:3199"  SplitHostPort host="::"  err=<nil>
```

`net.SplitHostPort("[]:3199")` returning an empty host with **no error** is what made this a live wildcard bind pre-fix.

## 4. Escapes still work

`checkBindSafety` must not trap an operator who genuinely wants a wildcard bind:

```
checkBindSafety("",  authEnabled=true)  -> nil
checkBindSafety("[]", authEnabled=true) -> nil
```

`--unsafe-no-auth` likewise (covered by `TestCheckBindSafety_UnsafeFlagBypasses`).

## 5. Rejection messages

```
host=""        -> server.host="" names no host, so it binds every network interface. ...
host="[]"      -> server.host="[]" names no host, so it binds every network interface. ...
host="0.0.0.0" -> server.host="0.0.0.0" binds to a non-loopback address without authentication. ...
```

## 6. Test suite

```
go build ./...                clean
go vet ./...                  clean
go test -race -short ./...    32 packages ok, 0 fail
go test -race -tags=integration ./internal/config/... ./cmd/...
                              both ok
```

The pre-existing tests asserted the vulnerable behavior — `{"", true}` in `TestIsLoopback` and `""` in `TestCheckBindSafety_AllowsLoopback`'s allow-list. Both corrected; the test had been encoding the bug as the specification, which is why the hole survived.
