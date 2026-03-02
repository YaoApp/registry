# Getting Started

This guide walks you through building, configuring, and running the Yao Registry server, then pushing and pulling your first package.

## Prerequisites

- **Go 1.25+** (build only — the compiled binary has zero runtime dependencies)

## Build

```bash
git clone https://github.com/yaoapp/registry.git
cd registry
go build -o registry .
```

The output is a single static binary.

## Quick Start

### 1. Create a push user

The registry allows anonymous pulls but requires Basic Auth for pushes. Create at least one user before starting the server:

```bash
./registry user add admin
# You will be prompted for a password interactively.
# Or provide it inline (useful in CI):
./registry user add --password s3cret ci-bot
```

> **Note:** Flags (`--password`, `--auth-file`) must appear before the username argument.

Credentials are stored in `./data/.auth` by default (bcrypt-hashed, one `user:hash` per line).

### 2. Start the server

```bash
./registry start
```

Default output:

```
Database: ./data/registry.db
Storage: ./data/storage
Auth: ./data/.auth (1 users)
Listening on 0.0.0.0:8080
```

On first launch the server automatically creates the SQLite database, tables, indexes, enables WAL mode, and creates the storage directory. No manual setup is required.

### 3. Verify

```bash
curl http://localhost:8080/.well-known/yao-registry
```

Expected response:

```json
{
  "registry": { "version": "1", "api": "/v1" },
  "types": ["releases", "robots", "assistants", "mcps"]
}
```

### 4. Push a package

Create a minimal `.yao.zip` archive containing a `package/pkg.yao` manifest (see [Package Authoring Guide](authoring.md) for the full spec):

```bash
mkdir -p my-pkg/package
cat > my-pkg/package/pkg.yao << 'EOF'
{
  "type": "assistant",
  "scope": "@yao",
  "name": "hello",
  "version": "1.0.0",
  "description": "A hello-world assistant"
}
EOF

cd my-pkg && zip -r ../hello-1.0.0.yao.zip package/ && cd ..
```

The archive must contain the `package/` directory prefix — the registry looks for `package/pkg.yao` inside the zip.

Push it:

```bash
curl -u admin:yourpassword \
  -X PUT \
  -H "Content-Type: application/zip" \
  --data-binary @hello-1.0.0.yao.zip \
  http://localhost:8080/v1/assistants/@yao/hello/1.0.0
```

Response (`201 Created`):

```json
{
  "type": "assistants",
  "scope": "@yao",
  "name": "hello",
  "version": "1.0.0",
  "digest": "sha256:a1b2c3..."
}
```

### 5. Pull the package

```bash
curl -o hello.yao.zip \
  http://localhost:8080/v1/assistants/@yao/hello/1.0.0/pull
```

Or pull by dist-tag (the first push automatically sets `latest`):

```bash
curl -o hello.yao.zip \
  http://localhost:8080/v1/assistants/@yao/hello/latest/pull
```

The response includes `X-Digest` and `Content-Length` headers for integrity verification.

---

## Configuration

All settings can be supplied via CLI flags, environment variables, or both (flags take precedence).

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--db-path` | `REGISTRY_DB_PATH` | `./data/registry.db` | SQLite database file |
| `--data-path` | `REGISTRY_DATA_PATH` | `./data/storage` | Package file storage root |
| `--host` | `REGISTRY_HOST` | `0.0.0.0` | Listen IP address |
| `--port` | `REGISTRY_PORT` | `8080` | Listen port |
| `--auth-file` | `REGISTRY_AUTH_FILE` | `./data/.auth` | Credential file path |
| `--max-size` | `REGISTRY_MAX_SIZE` | `512` | Max package size (MB) |

Example with custom settings:

```bash
./registry start --host 127.0.0.1 --port 3000 --db-path /var/lib/registry/registry.db
```

Or via environment:

```bash
export REGISTRY_PORT=3000
export REGISTRY_DATA_PATH=/var/lib/registry/storage
./registry start
```

---

## User Management

```bash
# Add user (interactive password)
./registry user add alice

# Add user (inline password, for scripts)
./registry user add --password hunter2 bob

# Change password
./registry user passwd alice

# Remove user
./registry user remove bob

# List users
./registry user list
```

All `user` subcommands accept `--auth-file <path>` to override the default credential file location.

---

## Commands Reference

| Command | Description |
|---------|-------------|
| `./registry` | Start the server (same as `start`) |
| `./registry start [flags]` | Start the HTTP server |
| `./registry user add <name>` | Add a push user |
| `./registry user remove <name>` | Remove a push user |
| `./registry user passwd <name>` | Change user password |
| `./registry user list` | List all users |
| `./registry version` | Print version |
| `./registry help` | Show help |

---

## Next Steps

- [API Reference](api-reference.md) — full endpoint documentation
- [Package Authoring Guide](authoring.md) — how to build `.yao.zip` packages
