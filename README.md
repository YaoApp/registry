# Yao Registry

A lightweight package registry for the [Yao](https://github.com/yaoapp/yao) ecosystem. Built with Go, Gin, and SQLite — zero external infrastructure required.

## What It Stores

| Type         | URL prefix    | Description                          |
| ------------ | ------------- | ------------------------------------ |
| `releases`   | `/v1/releases`   | Yao engine binaries (multi-platform) |
| `assistants` | `/v1/assistants` | AI assistant packages                |
| `robots`     | `/v1/robots`     | Robot configuration packages         |
| `mcps`       | `/v1/mcps`       | MCP tool packages                    |

All packages use the unified `.yao.zip` format with a `package/pkg.yao` JSON manifest inside.

## Quick Start

```bash
# Build
go build -o registry .

# Create a user
registry user add --password s3cret admin

# Start (auto-creates SQLite tables)
registry start --host 0.0.0.0 --port 8080
```

### Configuration Flags

| Flag          | Default            | Description             |
| ------------- | ------------------ | ----------------------- |
| `--host`      | `0.0.0.0`         | Listen address          |
| `--port`      | `8080`             | Listen port             |
| `--db-path`   | `./data/registry.db` | SQLite database path  |
| `--data-path` | `./data/storage`   | Package file storage    |
| `--auth-file` | `./data/.auth`     | Bcrypt credentials file |

## API Overview

**Discovery**

```
GET /.well-known/yao-registry
GET /v1/
```

**Read (public)**

```
GET /v1/:type                                    # List packages
GET /v1/:type/:scope/:name                       # Package metadata (packument)
GET /v1/:type/:scope/:name/:version              # Version detail
GET /v1/:type/:scope/:name/:version/pull         # Download .yao.zip
GET /v1/:type/:scope/:name/:version/dependencies # Dependency list (?recursive=true)
GET /v1/:type/:scope/:name/dependents            # Reverse dependencies
GET /v1/search?q=keyword                         # Cross-type search
```

**Write (Basic Auth)**

```
PUT    /v1/:type/:scope/:name/:version        # Push package
DELETE /v1/:type/:scope/:name/:version        # Delete version
PUT    /v1/:type/:scope/:name/tags/:tag       # Set dist-tag
DELETE /v1/:type/:scope/:name/tags/:tag       # Delete dist-tag
```

### Example

```bash
# Push
curl -u admin:s3cret -X PUT \
  -H "Content-Type: application/zip" \
  --data-binary @hello-1.0.0.yao.zip \
  http://localhost:8080/v1/assistants/@yao/hello/1.0.0

# Pull
curl -O http://localhost:8080/v1/assistants/@yao/hello/1.0.0/pull
```

## Package Format

Every `.yao.zip` must contain `package/pkg.yao`:

```jsonc
{
  "type": "assistant",          // release | robot | assistant | mcp
  "scope": "@yao",
  "name": "hello",
  "version": "1.0.0",
  "description": "A hello-world assistant",
  "dependencies": [
    { "type": "mcp", "scope": "@yao", "name": "tools", "version": "^1.0.0" }
  ],
  "engines": { "yao": ">=0.10.0" }
}
```

## Go Client SDK

A client SDK lives in the [yao](https://github.com/yaoapp/yao) repository at `registry/`:

```go
import "github.com/yaoapp/yao/registry"

c := registry.New("https://registry.yaoagents.com",
    registry.WithAuth("admin", "s3cret"),
)

// Push
result, _ := c.Push("assistants", "@yao", "hello", "1.0.0", zipBytes)

// Pull
data, digest, _ := c.Pull("assistants", "@yao", "hello", "latest")

// Search
list, _ := c.Search("hello", "assistants", 1, 20)
```

## Docker

```bash
docker run -d -p 8080:8080 -v registry-data:/data yaoapp/registry:latest
```

## Development

```bash
make fmt          # Format
make vet          # Static analysis
make unit-test    # Unit tests
make e2e-test     # E2E tests
make build        # Build binary
make build-all    # Cross-compile (linux/darwin × amd64/arm64)
```

## Docs

- [Getting Started](docs/getting-started.md)
- [API Reference](docs/api-reference.md)
- [Authoring Packages](docs/authoring.md)
- [Design Document](design.md)

## License

Apache-2.0
