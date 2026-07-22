# Groku Web UI — Design

A browser remote for Roku devices on the local network. Vim-style keyboard control, minimal chrome, single-binary Go server.

## Goals

- Control a Roku from any browser on the LAN without the official app
- Keyboard-first UX (mouse optional for help / app picker)
- Fast, zero-build frontend (static HTML/CSS/JS)
- Reliable discovery + control via Roku ECP (HTTP :8060)
- Clear feedback when ECP is limited/disabled (Roku OS 14.1+)

## Non-goals

- Cloud / remote-over-internet control
- Official Roku mobile pairing protocol
- Multi-user auth
- Pixel-perfect on-screen Roku UI mirroring

---

## Architecture

```
Browser  ──HTTP──►  groku serve (:8080)
                       │
                       ├─ static UI (embedded)
                       ├─ GET  /api/devices      SSDP discover
                       ├─ GET  /api/device       active device + info
                       ├─ PUT  /api/device       select device by base URL
                       ├─ GET  /api/apps         proxy query/apps
                       ├─ POST /api/key          proxy keypress
                       ├─ POST /api/launch       proxy launch
                       ├─ POST /api/text         proxy Lit_ chars
                       └─ POST /api/search       proxy search/browse
                              │
                              └──HTTP ECP──► Roku :8060
```

Browsers cannot send SSDP multicast or reliably hit arbitrary LAN devices (CORS). The Go process owns discovery and ECP; the UI only talks to the local server.

### Process model

```bash
groku serve          # default :8080
groku serve -addr :9090
groku serve -roku http://192.168.5.106:8060/   # skip initial discover
```

Existing CLI commands (`home`, `apps`, …) remain unchanged.

### Stack

| Layer | Choice |
|-------|--------|
| Server | Go stdlib (`net/http`, `embed`) |
| UI | Single page: `web/index.html`, `web/app.js`, `web/style.css` |
| State | In-memory active device + optional disk cache (reuse `/tmp/groku.json` pattern) |
| Deps | None outside Go stdlib |

---

## Modes

The UI is always in exactly one mode. Mode is shown in a small status bar.

| Mode | Enter | Exit | Behavior |
|------|-------|------|----------|
| **Normal** | default / Esc | — | Global key bindings (nav, transport, overlays) |
| **Text** | `t` | `Esc` | Keystrokes sent to Roku as `Lit_` on **keyup**; Backspace → `Backspace` |
| **Search** | `s` | `Esc` | Type query in browser; Enter → ECP `POST /search/browse?keyword=` |
| **Apps** | `a` | `Esc` | Fuzzy app picker; type to filter; Enter launches |
| **Roku** | `r` | `Esc` | Device picker from live SSDP; type to filter; Enter selects |
| **Help** | `?` | `?` or `Esc` | Bottom sheet listing all bindings |

Overlays (Apps / Roku / Search / Help) capture keys so they do not also fire Normal bindings. Text mode captures almost everything except `Esc`.

### Why Search is not the remote Search key

On modern Roku OS, `keypress/Search` opens **voice search**. A browser has no mic path into that HUD, so it is useless from groku.

Instead, `s` opens a **local query composer**. On Enter, the server calls Roku’s global content search:

```http
POST http://<roku>:8060/search/browse?keyword=<query>
```

This still returns HTTP 200 on Roku OS 15.x (Ultra) in practice, even though older docs marked a related “search” surface as sunset. If it ever 404s, the UI surfaces the error; no silent fall-through to voice search.

---

## Keyboard map (Normal mode)

| Key | Action | ECP |
|-----|--------|-----|
| `h` / `←` | Left | `keypress/Left` |
| `j` / `↓` | Down | `keypress/Down` |
| `k` / `↑` | Up | `keypress/Up` |
| `l` / `→` | Right | `keypress/Right` |
| `Enter` | OK / Select | `keypress/Select` |
| `Backspace` | Back | `keypress/Back` |
| `Home` / `H` (Shift+h) | Home screen | `keypress/Home` |
| `p` | Play / Pause | `keypress/Play` |
| `f` | Fast forward | `keypress/Fwd` |
| `d` | Rewind | `keypress/Rev` |
| `b` | Instant replay (~few seconds back) | `keypress/InstantReplay` |
| `x` | Options / asterisk (*) | `keypress/Info` |
| `t` | Enter **Text** mode | — |
| `s` | Enter **Search** mode | compose → `/api/search` |
| `a` | Enter **Apps** mode | loads `/api/apps` |
| `r` | Enter **Roku** mode | loads `/api/devices` |
| `?` | Toggle **Help** sheet | — |
| `Esc` | Close overlay / leave Text → Normal | — |

### Text mode detail

- Entered with `t`.
- On each **keyup** of a printable character, POST one `keypress/Lit_<urlencoded UTF-8>`.
- `Backspace` → `keypress/Backspace`.
- `Enter` → `keypress/Enter` (keyboard field submit, not Select).
- `Esc` exits Text mode; nothing sent for Esc.
- Modifier-only and browser shortcuts (Ctrl/Meta combos) are ignored.
- Status bar shows `TEXT` and a live echo of characters sent this session.

### Search mode detail

- Entered with `s` (does **not** send `keypress/Search`).
- Focus a single input; type the full query in the browser (paste OK).
- `Enter` → `POST /api/search` `{ "keyword": "…" }` → Roku `search/browse`.
- Roku jumps to global search results for that keyword.
- `Esc` cancels without searching.

### Apps mode detail

1. Fetch app list once on open (refresh if stale > 30s or on re-open after device change).
2. Input line at top; list below filtered by fuzzy match on app name.
3. Fuzzy: subsequence match, case-insensitive, rank by consecutive runs + early matches (simple scoring; no external lib).
4. `j`/`k` or arrows move highlight; `Enter` launches highlighted app; click also works.
5. `Esc` closes without launch.

### Roku (discover) mode detail

Same UX shell as Apps:

1. On open, `GET /api/devices` runs SSDP (`ST: roku:ecp`, ~3s).
2. List shows friendly name (from `query/device-info` when reachable), IP, model.
3. Filter by typing; `j`/`k` navigate; `Enter` sets active device via `PUT /api/device`.
4. Active device is marked; selecting one exits to Normal.
5. If discovery returns nothing, show hint: same LAN, ECP, firewall/multicast.

### Help sheet

- Bottom panel (~40% viewport), dimmed backdrop optional.
- Static table of the bindings above + short mode notes.
- Toggle with `?`; also a visible `?` button in the corner for discoverability.

---

## Visual design

Dark, low-distraction remote. No framework.

```
┌──────────────────────────────────────────────┐
│  groku          Roku Ultra · 192.168.5.106  ?│  header
├──────────────────────────────────────────────┤
│                                              │
│              [ optional D-pad               │
│                for touch/mouse ]             │  main
│                                              │
│   status: NORMAL · last: Select · ok         │
├──────────────────────────────────────────────┤
│  TEXT | apps filter | device filter | help   │  mode chrome
└──────────────────────────────────────────────┘
```

- **Header:** app name, active device label, `?` control.
- **Main:** large soft D-pad + Play/Back row for mouse/touch; keyboard remains primary.
- **Status line:** mode · last command · HTTP result (ok / 403 / error).
- **Overlays:** slide-up panels for Apps, Roku, Help, and Text indicator.
- Typography: system UI sans; monospace for keycaps in help.
- Colors: near-black bg, muted fg, one accent (e.g. violet) for selection/focus.
- Responsive: usable at phone width; D-pad scales down.

No animations beyond short overlay transitions (~150ms).

---

## API (server)

All JSON unless noted. Errors: `{ "error": "message" }` with appropriate status.

### `GET /api/devices`

SSDP M-SEARCH, collect responses up to timeout (default 3s). For each LOCATION, optionally GET `query/device-info` (short timeout) to fill name/model.

```json
{
  "devices": [
    {
      "location": "http://192.168.5.106:8060/",
      "name": "Roku Ultra",
      "model": "Ultra",
      "serial": "…",
      "ecpMode": "enabled"
    }
  ]
}
```

Parse SSDP by scanning headers for `LOCATION:` (case-insensitive) — not fixed line index.

### `GET /api/device`

```json
{
  "location": "http://192.168.5.106:8060/",
  "name": "Roku Ultra",
  "model": "Ultra",
  "ecpMode": "enabled",
  "activeApp": "YouTube"
}
```

### `PUT /api/device`

```json
{ "location": "http://192.168.5.106:8060/" }
```

Validates with `GET …/query/device-info`. Persists to cache file. 400 if unreachable; 403 body if `ecp-setting-mode` is `limited` (still allow select, but UI warns).

### `GET /api/apps`

Proxies `GET {location}query/apps` → JSON array `{ id, name }[]`.

### `POST /api/key/{key}`

Proxies `POST {location}keypress/{key}`. Allowlist keys:  
`Home, Rev, Fwd, Play, Select, Left, Right, Down, Up, Back, InstantReplay, Info, Backspace, Search, Enter, VolumeUp, VolumeDown, VolumeMute, PowerOff` plus `Lit_*` pattern for text.

Returns upstream status. On 403, body explains Limited mode and Settings path.

### `POST /api/launch/{id}`

Proxies `POST {location}launch/{id}`.

### `POST /api/text`

```json
{ "text": "hello" }
```

Sends each rune as `Lit_` sequentially (CLI parity). UI primarily uses per-key `POST /api/key` with `Lit_…` for snappier feedback; bulk endpoint kept for paste.

### `POST /api/search`

```json
{ "keyword": "the bear" }
```

Proxies `POST {location}search/browse?keyword=…`. Opens Roku global search results (not voice search).

### Static

`GET /` and `/assets/*` from `embed.FS`.

---

## ECP / OS constraints (product copy)

Surface these in the UI when relevant:

1. **Limited mode (OS 14.1+):**  
   Settings → System → Advanced system settings → Control by mobile apps → Network access → **Enabled**.
2. Discovery needs multicast UDP `239.255.255.250:1900` on the same L2/L3 network.
3. Control is plain HTTP to port **8060** (no TLS).
4. Official mobile app can work while ECP is limited — different channel.

Status bar turns amber on 403 with a one-line fix hint.

---

## Server internals (reuse & fix)

Refactor current `groku.go` ideas into small packages or files:

| File | Role |
|------|------|
| `main.go` | CLI dispatch + `serve` |
| `ecp.go` | HTTP client, keypress, apps, device-info, allowlist |
| `ssdp.go` | Robust discovery (header parse, multi-device, timeout) |
| `device.go` | Active device + JSON cache |
| `server.go` | HTTP routes, middleware, embed |
| `web/*` | Frontend assets |

Fixes vs legacy CLI while touching this code:

- Parse `LOCATION` by header name, not `ret[6]`
- Surface HTTP errors (CLI + API)
- Optional `--roku` / cache for manual IP
- Close UDP sockets; HTTP client timeouts (e.g. 5s)

CLI behavior stays backward compatible where practical.

---

## Frontend structure

```
web/
  index.html   # shell, D-pad markup, overlay roots
  style.css
  app.js       # mode machine, keys, fetch wrappers, fuzzy
```

### State (in JS)

```js
{
  mode: 'normal' | 'text' | 'apps' | 'roku' | 'help',
  device: { location, name, model, ecpMode } | null,
  apps: [{ id, name }],
  devices: [...],
  filter: '',
  selectedIndex: 0,
  lastResult: { cmd, ok, status, message },
  textBuffer: ''  // echo only
}
```

### Key handling

- `window` `keydown` / `keyup` with `event.preventDefault()` for bound keys when focused on body (not when a real `<input>` is focused — Apps/Roku use a controlled input).
- Text mode: listen `keyup` for chars; prevent browser search-on-type etc.
- Debounce nothing on keys (Roku expects discrete presses); optional small queue if requests overlap.

### Fuzzy match

```
score(query, name):
  lower both
  find all subsequence match positions
  score += 1 per char; bonus for consecutive; bonus for match at start / word boundary
  reject if not full subsequence
sort by score desc, then name length asc
```

---

## Security

- Bind default `127.0.0.1:8080` **or** `0.0.0.0` with LAN assumption documented — prefer **`0.0.0.0:8080`** for phone-on-WiFi use, document that anyone on LAN can control the TV.
- No auth v1 (home LAN tool).
- Key allowlist prevents open proxy abuse to non-ECP paths.
- Do not proxy arbitrary URLs; device location must look like `http://ip[:port]/`.

---

## Testing plan

1. `go run . serve` → open `/`
2. `r` discover → select Ultra
3. `hjkl` + arrows move focus on Roku home
4. `Enter` opens tile; `Backspace` back
5. `a` → type `net` → Netflix → Enter launches
6. `t` → type into an on-screen field → `Esc`
6b. `s` → type “the bear” → Enter opens global search results
7. `p`/`f`/`d`/`b` during video
8. `x` opens options star panel
9. `?` toggles help
10. With ECP Limited: keypress shows amber 403 hint (manual toggle on device)

---

## Implementation order

1. Refactor discovery/ECP + `groku serve` API skeleton  
2. Static page shell + Normal mode keys + status  
3. Text mode  
4. Apps overlay + fuzzy  
5. Roku overlay + device select  
6. Help sheet + D-pad click targets  
7. Polish errors (limited mode), README blurb  

---

## Open choices (defaults)

| Topic | Default |
|-------|---------|
| Listen addr | `:8080` (all interfaces) |
| SSDP wait | 3s |
| Device cache TTL | 5m for address; re-validate on serve start |
| `x` mapping | `Info` (Options \*) |
| Shift+h vs `Home` key | both send Home |
| Paste in Text mode | yes, via bulk `/api/text` on paste event |

---

## Success criteria

- From a cold start on the LAN, user can discover the Ultra, navigate, launch an app, type search text, and control playback using only the keyboard map above.
- `?` documents every binding without leaving the page.
- Limited ECP fails loudly with the exact Settings path, not silently.
