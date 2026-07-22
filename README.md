# groku

CLI + browser remote for your [Roku](https://www.roku.com/)

## Web UI

```bash
go run . serve
# open http://127.0.0.1:8080
```

```bash
go run . serve -addr :9090
go run . serve -roku http://192.168.1.50:8060/
```

Keyboard (vim-style): `hjkl`/arrows · `Enter` OK · `p` play · `f`/`d` ff/rew · `b` replay · `x` options · `Space` text · `a` apps · `r` device · `?` help

## Roku setting (required on OS 14.1+)

**Settings → System → Advanced system settings → Control by mobile apps → Network access → Enabled**

If this is “Limited”, discovery works but keypress returns 403.

## CLI

```bash
go build -o groku .
./groku discover
./groku home
./groku play
./groku text "Breaking Bad"
./groku apps
./groku app "Netflix"
```

## Install

```bash
go install github.com/zankich/groku@latest
```

Or download a release binary.
