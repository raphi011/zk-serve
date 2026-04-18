# zk-serve

A web viewer for [zk](https://github.com/zk-org/zk) notebooks. Browse notes in a folder tree, search full-text, filter by tag, and read rendered Markdown with wiki links, syntax highlighting, and Mermaid diagrams.

## Requirements

- `zk` on `$PATH`
- A zk notebook (directory containing `.zk/config.toml`)

## Usage

```bash
# Build
make build

# Run
zk-serve --notebook ~/notes --addr :8080 --open
```

The `--notebook` flag defaults to `$ZK_NOTEBOOK_DIR` if set.

## Docker

```bash
# Build
docker build -t zk-serve .

# Run (mount your notebook at /notebook)
docker run -v ~/notes:/notebook -p 8080:8080 zk-serve

# Multi-arch for a cluster
docker buildx build --platform linux/amd64,linux/arm64 \
  -t your-registry/zk-serve:latest --push .
```

## Development

```bash
make assets   # download htmx + mermaid into static/
make build    # build binary → bin/zk-serve
make test     # go test ./...
```
