# Integration tests (LazyArgo)

LazyArgo's integration tests are optional and run only when you pass the `integration` build tag.

These tests are intended to exercise LazyArgo against a real Argo CD instance running in a local Kubernetes cluster.

## Prerequisites

- Go (see project README for supported versions)
- `kind` (Kubernetes in Docker)
- `kubectl`
- `helm` (optional; can also install Argo CD with kubectl manifests)
- Docker

## Quickstart (kind + Argo CD)

1. Create a kind cluster:

```bash
kind create cluster --name lazyargo
kubectl cluster-info --context kind-lazyargo
```

2. Install Argo CD into the cluster.

Option A (recommended): upstream manifests

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

Wait for pods:

```bash
kubectl -n argocd rollout status deploy/argocd-server
kubectl -n argocd get pods
```

3. Port-forward the API server:

```bash
kubectl -n argocd port-forward svc/argocd-server 8080:443
```

4. Obtain initial admin password:

```bash
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath='{.data.password}' | base64 -d
```

5. Run the integration tests:

```bash
go test -tags=integration ./... -run Integration
```

## Notes

- Integration tests are *not* run in CI by default.
- Tests should be written defensively and clean up any resources they create.
- Prefer uniquely-named namespaces/resources (e.g. include a random suffix).
