# Packaging

LazyArgo ships official binaries via **GitHub Releases**.

This repo also includes helper scripts to generate packaging metadata for:
- Homebrew (tap)
- Scoop

## Install script

- `scripts/install.sh`
- Downloads a tagged release asset for your OS/arch
- Verifies SHA256 using `checksums.txt`
- Installs into a chosen directory

## Homebrew

Recommended approach:
- Create a separate tap repo, e.g. `phin3has/homebrew-tap`
- Add `Formula/lazyargo.rb`

Generate a formula snippet for a given version:

```bash
./scripts/gen-packaging.sh v0.1.0-alpha.2 > /tmp/packaging.txt
```

Then copy the Homebrew section into your tap.

## Scoop

Recommended approach:
- Create a Scoop bucket repo, e.g. `phin3has/scoop-bucket`
- Add `bucket/lazyargo.json`

Use the same `gen-packaging.sh` output to populate the manifest.

## Why not ship these inside this repo?

Homebrew/Scoop generally work best as separate repos (taps/buckets).
Keeping them separate also avoids mixing CLI source history with packaging logistics.
