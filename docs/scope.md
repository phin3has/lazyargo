# LazyArgo — Scope & MVP Success Criteria

## What LazyArgo is
LazyArgo is a **lazygit-style terminal UI** for interacting with **Argo CD Applications**.

Primary goals:
- **Fast browsing** of apps and their current state (health/sync, repo/path/revision, destination)
- **Safe operations** (plan/dry-run mindset, explicit confirmations, guardrails)
- A tool you can point at a **sandbox Argo CD** for development, and later at real environments with strong safety defaults

## What LazyArgo is not
- Not a full replacement for the Argo CD UI.
- Not a general Kubernetes dashboard.
- Not an auto-remediator that “just syncs everything” by default.

## Primary users
- Platform/SRE engineers
- DevOps engineers
- Security-minded operators (who care about RBAC + auditability)

## MVP (Phase 1) capabilities
### Read-only browsing
- List applications
- View application details
- View key resources (group/kind/name/namespace/status/health)

### Safe action model (initial)
- "Refresh" / reload data
- No destructive actions by default

## MVP Success Criteria
MVP is “done” when:
- You can run lazyArgo against a sandbox Argo CD and:
  - load the application list quickly
  - select an app and see details/resources
  - refresh without crashing
- Authentication works using either:
  - `ARGOCD_AUTH_TOKEN`, or
  - username/password session login
- The app remains usable entirely from keyboard (lazygit-style)

## Post-MVP (Phase 2+) ideas
- Search/filter apps
- Per-app actions: sync, hard refresh, diff view
- Guardrails: allowlists, "protected" apps, confirmation flows
- Config file parsing + default config path
- Better error surfaces (auth errors, network errors) in the UI
