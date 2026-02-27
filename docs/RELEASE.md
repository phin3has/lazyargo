# Release Process (LazyArgo)

## Versioning

- Use SemVer.
- While the feature set is still stabilizing, publish **alpha** tags: `v0.1.0-alpha.1`, `v0.1.0-alpha.2`, etc.
- Promote to `v0.1.0` once:
  - basic UX is stable
  - config format is stable
  - the "happy path" against a real ArgoCD works end-to-end

## Create a release

1) Ensure `main` is green (CI passing).
2) Update `CHANGELOG.md`.
3) Tag and push:

```bash
git tag v0.1.0-alpha.1
git push origin v0.1.0-alpha.1
```

## What the pipeline does

On tag push (`v*`), GitHub Actions will:

- Build static binaries (CGO disabled) for:
  - linux amd64/arm64
  - darwin amd64/arm64
  - windows amd64
- Package artifacts as `.tar.gz` (unix) / `.zip` (windows)
- Produce `checksums.txt`
- Create a GitHub Release and upload the artifacts

## Smoke test checklist

After the release is published:

- Download the artifact for your OS
- `lazyargo --version` prints the tag
- `lazyargo --mock` launches
- Against a real ArgoCD:
  - list apps loads
  - selecting an app shows details
  - sync action (if supported) behaves as expected
