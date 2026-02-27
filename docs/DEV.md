# Development

## Go toolchain

This repo targets Go **1.22.x**.

If you don't have Go installed system-wide, you can install a local toolchain:

```bash
GO_VERSION=1.22.10
cd /tmp
curl -fsSLO "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
mkdir -p "$HOME/.local"
rm -rf "$HOME/.local/go" "$HOME/.local/go${GO_VERSION}"
tar -C "$HOME/.local" -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
mv "$HOME/.local/go" "$HOME/.local/go${GO_VERSION}"
ln -sfn "$HOME/.local/go${GO_VERSION}" "$HOME/.local/go"

export PATH="$HOME/.local/go/bin:$PATH"
go version
```

## Common commands

```bash
go test ./...
go build -o lazyargo ./cmd/lazyargo
./lazyargo --mock
```
