# LazyArgo — Architecture (Initial)

## Goals
- **TUI-first** (lazygit-style), keyboard-driven.
- Small, testable core (planner/decision logic separated from UI).
- Pluggable Argo CD backend (mock vs HTTP).

## High-level components
- `cmd/lazyargo/`
  - CLI flags, env wiring, program bootstrap.
- `internal/ui/`
  - Bubble Tea model + rendering.
  - Does not know about HTTP details; depends on an `argocd.Client` interface.
- `internal/argocd/`
  - `Client` interface.
  - `HTTPClient` implementation (Argo CD REST API).
  - `MockClient` for offline dev/testing.
- `internal/config/`
  - Configuration loading (currently minimal; intended to grow into YAML + defaults).

## Data flow
1. UI requests app list/details via `argocd.Client`.
2. Client fetches from either:
   - mock data, or
   - Argo CD REST API (`/api/v1/applications`, `/api/v1/applications/{name}`)
3. UI renders:
   - sidebar list (apps)
   - main pane (selected app details + resources)

## Authentication
Supported patterns:
- Token auth: `ARGOCD_AUTH_TOKEN` → `Authorization: Bearer <token>`
- Session login: username/password → `POST /api/v1/session` → bearer token cached in-memory

## Repo layout conventions
- Keep public API surface small (interfaces in `internal/argocd/client.go`).
- Prefer pure functions for any planning/decision logic to enable unit tests.
- Avoid baking “sync/apply” mutations into the UI until safety guardrails are defined.

## Next architectural steps
- Introduce an `internal/planner/` package for any "plan/apply" operations.
- Introduce a config schema for guardrails:
  - app allowlist / denylist
  - namespace allowlist
  - protected apps requiring extra confirmation
- Add structured logging + error surfaces in UI.
