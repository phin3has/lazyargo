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

By default, lazyArgo uses a mock client.

To connect to a real Argo CD API:

```bash
go run ./cmd/lazyargo --mock=false --server http://localhost:8080 --username admin --password <password>
```

Or using env vars (preferred):

```bash
export ARGOCD_SERVER=http://localhost:8080
export ARGOCD_AUTH_TOKEN=<token>
go run ./cmd/lazyargo --mock=false
```

## Next Steps

- Flesh out application details (resources tree, history, conditions)
- Add actions (refresh, sync, rollback)
- Add filtering/search (like lazygit)
- Add config file parsing (YAML) and default path lookup (`~/.config/lazyargo/`)
