# lazyArgo

A **lazygit-style** terminal UI (TUI) for quickly browsing (and syncing) **Argo CD Applications**.

It’s intentionally minimal: fast list, fast details, a few power keys (filter/sort/drift-only/sync).

## Quickstart

### Install

#### Option A: Download a release binary

- Go to **GitHub Releases** for this repo and download the `lazyargo` binary.
- Make it executable and put it on your `PATH`:

```bash
chmod +x ./lazyargo
sudo mv ./lazyargo /usr/local/bin/lazyargo
```

#### Option B: Build from source

```bash
go build -o lazyargo ./cmd/lazyargo
./lazyargo --mock
```

### Run (mock mode)

Mock mode is great for trying the UI without any Argo CD access:

```bash
lazyargo --mock
```

### Run (real Argo CD)

lazyArgo connects using an Argo CD **server URL** and an **auth token**.

#### Common setup: port-forward

```bash
kubectl -n argocd port-forward svc/argocd-server 8080:443

export ARGOCD_SERVER=https://localhost:8080
export ARGOCD_AUTH_TOKEN=<your-token>

# If your local trust store doesn’t accept Argo CD’s cert:
export ARGOCD_INSECURE=true

lazyargo
```

#### Using flags

```bash
lazyargo \
  --server https://localhost:8080 \
  --token <your-token> \
  --insecure
```

## CLI flags

| Flag | Type | Default | Description |
|---|---:|---|---|
| `--config` | string | *(empty)* | Path to config file (optional). If not set, lazyArgo will try `~/.config/lazyargo/config.yaml` if it exists. |
| `--mock` | bool | `false` | Use the mock Argo CD client (no network calls). |
| `--server` | string | *(from config / env)* | Argo CD server URL (overrides config + `ARGOCD_SERVER`). |
| `--username` | string | *(empty)* | Argo CD username (or `ARGOCD_USERNAME`; optional / future use). |
| `--password` | string | *(empty)* | Argo CD password (or `ARGOCD_PASSWORD`; optional / future use). |
| `--token` | string | *(from config / env)* | Argo CD auth token (overrides config + `ARGOCD_AUTH_TOKEN`). |
| `--insecure` | bool | `false` | Skip TLS verification (or set `ARGOCD_INSECURE=true`). |
| `--log-level` | string | *(from config)* | Log level: `debug`, `info`, `warn`, `error`. |

### Environment variables

| Variable | Purpose |
|---|---|
| `ARGOCD_SERVER` | Server URL (e.g. `https://localhost:8080`) |
| `ARGOCD_AUTH_TOKEN` | Auth token (recommended auth method) |
| `ARGOCD_INSECURE` | Set to `true` / `1` / `yes` to skip TLS verification |
| `ARGOCD_USERNAME` / `ARGOCD_PASSWORD` | Optional / future login flows |
| `LAZYARGO_LOG_LEVEL` | Log level override |

## Keybinds

### Navigation / view

- `j` / `↓` — move down
- `k` / `↑` — move up
- `r` — refresh application list
- `d` — refresh selected application details
- `?` — toggle help
- `q` / `ctrl+c` — quit

### Filtering / sorting

- `/` — filter applications (type to narrow by substring)
- `esc` — clear filter (also exits filter mode)
- `S` — cycle sort: **name** → **health** → **sync**

### Drift + sync

- `D` — toggle **drift-only** (show only non-synced apps)
- `s` — sync all drifted apps (runs a dry-run preview first)

#### Sync modal

- `y` — run the sync (only after the dry-run completes)
- `n` / `esc` — cancel

## Config file

By default, lazyArgo looks for:

- `~/.config/lazyargo/config.yaml` (if it exists)

Example `config.yaml`:

```yaml
argocd:
  server: https://localhost:8080
  token: "${ARGOCD_AUTH_TOKEN}" # (optional; env recommended)
  insecureSkipVerify: false

ui:
  sidebarWidth: 28

logLevel: info
```

Notes:

- CLI flags override environment variables, which override the config file.
- Using `ARGOCD_AUTH_TOKEN` is recommended instead of hard-coding the token in YAML.

## Troubleshooting

### “failed to load apps” / empty list

Common causes:

1. **Server unreachable**
   - If using port-forward, make sure it’s running:
     ```bash
     kubectl -n argocd port-forward svc/argocd-server 8080:443
     ```
2. **Missing token**
   - Set `ARGOCD_AUTH_TOKEN` (or pass `--token`).
3. **TLS errors on localhost**
   - Try `--insecure` (or `ARGOCD_INSECURE=true`).

### The UI says “mock” server

You’re running in mock mode. Start without `--mock`, and ensure `ARGOCD_SERVER` (or `--server`) is set.

### Filtering feels “stuck”

When filter mode is active, keystrokes go to the filter input.

- Press `esc` to exit filter mode and clear the filter.

### Sync does nothing when pressing `y`

The sync flow runs a **dry-run** first. Wait for the dry-run to complete; then press `y` to run the real sync.

## Project layout

- `cmd/lazyargo/` — entrypoint
- `internal/ui/` — Bubble Tea model + styles + key bindings
- `internal/config/` — YAML + env config loader
- `internal/argocd/` — Argo CD client interface + mock implementation
