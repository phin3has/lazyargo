# Minimal runtime image for lazyargo (CLI/TUI)
# Note: interactive TUI in containers requires a TTY; intended for devpod-style environments.
FROM gcr.io/distroless/static:nonroot

ARG TARGETOS
ARG TARGETARCH

# goreleaser will place the compiled binary at /usr/local/bin/lazyargo
COPY lazyargo /usr/local/bin/lazyargo

USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/lazyargo"]
