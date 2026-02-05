# lazyArgo

A **lazygit-style** TUI for browsing Argo CD Applications (scaffold).

This repo is an initial Bubble Tea + Lip Gloss layout with:
- Sidebar (applications list)
- Main pane (selected application details)
- Keybind help bar (via `bubbles/help`)
- Config loading placeholder
- Internal Argo CD client interface + mock implementation

## Project Layout

- `cmd/lazyargo/` – entrypoint
- `internal/ui/` – Bubble Tea model + styles + key bindings
- `internal/config/` – config loader placeholder
- `internal/argocd/` – Argo CD client interface + mock

## Requirements

- Go 1.22+

## Build

```bash
go build ./cmd/lazyargo
```

## Run

Run with the mock Argo CD client (default):

```bash
go run ./cmd/lazyargo --mock
```

Optionally provide a config path (currently only checked for existence):

```bash
go run ./cmd/lazyargo --config ./config.yaml
```

### Keybinds

- `j` / `↓` – move down
- `k` / `↑` – move up
- `r` – refresh
- `?` – toggle help
- `q` / `ctrl+c` – quit

## Connect to a real Argo CD server

lazyArgo can connect to Argo CD using a **token** (recommended).

### Common “out of the box” setup (port-forward)

```bash
kubectl -n argocd port-forward svc/argocd-server 8080:443
export ARGOCD_SERVER=https://localhost:8080
export ARGOCD_AUTH_TOKEN=<token>
# If your local trust store doesn’t accept Argo CD’s cert:
export ARGOCD_INSECURE=true

go run ./cmd/lazyargo
```

### Explicit flags

```bash
go run ./cmd/lazyargo --server https://localhost:8080 --token <token> --insecure
```

### Mock mode

If you just want to run the UI without an Argo CD server:

```bash
go run ./cmd/lazyargo --mock
```

## Next Steps

- Flesh out application details (resources tree, history, conditions)
- Add actions (refresh, sync, rollback)
- Add filtering/search (like lazygit)
- Add config file parsing (YAML) and default path lookup (`~/.config/lazyargo/`)
