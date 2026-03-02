# API Reference

Base URL: `http://<host>:<port>` (default `http://localhost:8080`)

All responses are JSON. Errors use the format `{"error": "message"}`.

## Authentication

- **Read** endpoints are public (no auth required).
- **Write** endpoints (push, delete, tag management) require HTTP Basic Authentication.

```
Authorization: Basic base64(username:password)
```

---

## Discovery

### GET `/.well-known/yao-registry`

Registry discovery endpoint for clients to auto-detect capabilities.

**Response** `200 OK`

```json
{
  "registry": { "version": "1", "api": "/v1" },
  "types": ["releases", "robots", "assistants", "mcps"]
}
```

### GET `/v1/`

Registry info.

**Response** `200 OK`

```json
{
  "name": "yao-registry",
  "version": "0.1.0"
}
```

---

## Package Types

All endpoints use a plural type name in the URL path:

| URL Type | Singular (DB) | Description |
|----------|---------------|-------------|
| `releases` | `release` | Yao engine binaries (platform-specific) |
| `robots` | `robot` | Robot configurations |
| `assistants` | `assistant` | AI assistant packages |
| `mcps` | `mcp` | MCP tool packages |

Scopes must start with `@` (e.g., `@yao`, `@community`).

---

## List Packages

### GET `/v1/:type`

List packages of a given type with optional filtering and pagination.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `scope` | string | Filter by scope (e.g., `@yao`) |
| `q` | string | Search within name/description/keywords |
| `page` | int | Page number (default `1`) |
| `pagesize` | int | Results per page (default `20`, max `100`) |

**Example**

```bash
curl "http://localhost:8080/v1/assistants?scope=@yao&page=1&pagesize=10"
```

**Response** `200 OK`

```json
{
  "total": 42,
  "page": 1,
  "pagesize": 10,
  "packages": [
    {
      "ID": 1,
      "Type": "assistant",
      "Scope": "@yao",
      "Name": "keeper",
      "Description": "Knowledge keeper assistant",
      ...
    }
  ]
}
```

---

## Search

### GET `/v1/search`

Cross-type search across all packages.

**Query Parameters**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `q` | string | yes | Search term |
| `type` | string | no | Filter by type (plural form: `assistants`, `mcps`, etc.) |
| `page` | int | no | Page number |
| `pagesize` | int | no | Results per page |

**Example**

```bash
curl "http://localhost:8080/v1/search?q=knowledge&type=assistants"
```

**Response** `200 OK`

```json
{
  "total": 3,
  "page": 1,
  "pagesize": 20,
  "packages": [...]
}
```

---

## Packument (Package Metadata)

### GET `/v1/:type/:scope/:name`

Full package metadata including all versions, similar to npm's packument.

**Headers**

| Header | Value | Effect |
|--------|-------|--------|
| `Accept` | `application/vnd.yao.abbreviated+json` | Return abbreviated metadata (versions list digest, deps, and engines only — no readme/license/author) |

**Example**

```bash
curl http://localhost:8080/v1/assistants/@yao/keeper
```

**Response** `200 OK`

```json
{
  "type": "assistants",
  "scope": "@yao",
  "name": "keeper",
  "description": "Knowledge keeper assistant",
  "keywords": ["knowledge", "keeper"],
  "license": "Apache-2.0",
  "author": { "name": "Yao Team" },
  "maintainers": [{ "name": "admin" }],
  "homepage": "",
  "repository": {},
  "bugs": {},
  "readme": "# Keeper\n...",
  "dist_tags": { "latest": "1.2.0", "canary": "1.3.0-beta" },
  "versions": {
    "1.0.0": {
      "version": "1.0.0",
      "digest": "sha256:abc...",
      "size": 102400,
      "engines": { "yao": ">=2.0.0" },
      "dependencies": [
        { "type": "mcp", "scope": "@yao", "name": "tools", "version": "^1.0.0" }
      ],
      "metadata": { ... },
      "created_at": "2026-03-01T10:00:00Z"
    },
    "1.2.0": { ... }
  },
  "created_at": "2026-03-01T10:00:00Z",
  "updated_at": "2026-03-02T14:00:00Z"
}
```

---

## Version Detail

### GET `/v1/:type/:scope/:name/:version`

Metadata for a single version.

For **release** type packages, this returns all platform artifacts:

```bash
curl "http://localhost:8080/v1/releases/@yao/yao/1.0.0"
```

```json
{
  "type": "releases",
  "scope": "@yao",
  "name": "yao",
  "version": "1.0.0",
  "artifacts": [
    { "os": "linux", "arch": "amd64", "variant": "", "digest": "sha256:...", "size": 52428800 },
    { "os": "darwin", "arch": "arm64", "variant": "", "digest": "sha256:...", "size": 48234567 }
  ]
}
```

Filter release artifacts with query parameters:

```bash
curl "http://localhost:8080/v1/releases/@yao/yao/1.0.0?os=linux&arch=amd64"
```

For **non-release** types, returns version metadata directly:

```json
{
  "type": "assistants",
  "scope": "@yao",
  "name": "keeper",
  "version": "1.0.0",
  "digest": "sha256:abc...",
  "size": 102400,
  "dependencies": [...],
  "metadata": { ... },
  "created_at": "2026-03-01T10:00:00Z"
}
```

---

## Pull (Download)

### GET `/v1/:type/:scope/:name/:version/pull`

Download the `.yao.zip` file. The `:version` parameter accepts either a semver version or a dist-tag name (e.g., `latest`).

**Query Parameters** (for releases)

| Param | Type | Description |
|-------|------|-------------|
| `os` | string | Operating system (e.g., `linux`, `darwin`) |
| `arch` | string | Architecture (e.g., `amd64`, `arm64`) |
| `variant` | string | Build variant (e.g., `prod`) |

**Response Headers**

| Header | Description |
|--------|-------------|
| `Content-Type` | `application/zip` |
| `Content-Disposition` | `attachment; filename="<name>-<version>.yao.zip"` |
| `X-Digest` | `sha256:<hex>` |
| `Content-Length` | File size in bytes |

**Examples**

```bash
# Pull by version
curl -o keeper.yao.zip \
  http://localhost:8080/v1/assistants/@yao/keeper/1.0.0/pull

# Pull by dist-tag
curl -o keeper.yao.zip \
  http://localhost:8080/v1/assistants/@yao/keeper/latest/pull

# Pull a release for a specific platform
curl -o yao \
  "http://localhost:8080/v1/releases/@yao/yao/1.0.0/pull?os=darwin&arch=arm64"
```

---

## Push (Upload)

### PUT `/v1/:type/:scope/:name/:version` 🔒

Upload a `.yao.zip` package. Requires Basic Auth.

The request body must be a valid zip archive containing `package/pkg.yao`. The manifest fields (`type`, `scope`, `name`, `version`) must match the URL parameters exactly.

**Request Headers**

| Header | Value |
|--------|-------|
| `Authorization` | `Basic base64(user:pass)` |
| `Content-Type` | `application/zip` |

**Example**

```bash
curl -u admin:secret \
  -X PUT \
  -H "Content-Type: application/zip" \
  --data-binary @keeper-1.0.0.yao.zip \
  http://localhost:8080/v1/assistants/@yao/keeper/1.0.0
```

**Response** `201 Created`

```json
{
  "type": "assistants",
  "scope": "@yao",
  "name": "keeper",
  "version": "1.0.0",
  "digest": "sha256:a1b2c3..."
}
```

**Error Codes**

| Code | Meaning |
|------|---------|
| `400` | Invalid zip, missing `pkg.yao`, or field mismatch |
| `401` | Missing or invalid credentials |
| `409` | Version already exists |
| `413` | Package exceeds max size limit |

---

## Delete Version

### DELETE `/v1/:type/:scope/:name/:version` 🔒

Remove a published version. Requires Basic Auth.

If the deleted version was tagged as `latest`, the tag is automatically reassigned to the most recent non-prerelease version. Other dist-tags pointing to the deleted version are removed.

```bash
curl -u admin:secret \
  -X DELETE \
  http://localhost:8080/v1/assistants/@yao/keeper/1.0.0
```

**Response** `200 OK`

```json
{
  "deleted": "1.0.0",
  "type": "assistants",
  "scope": "@yao",
  "name": "keeper"
}
```

---

## Dist-Tag Management

### PUT `/v1/:type/:scope/:name/tags/:tag` 🔒

Set a dist-tag to point to a specific version. Requires Basic Auth.

```bash
curl -u admin:secret \
  -X PUT \
  -H "Content-Type: application/json" \
  -d '{"version": "1.2.0"}' \
  http://localhost:8080/v1/assistants/@yao/keeper/tags/canary
```

**Response** `200 OK`

```json
{ "tag": "canary", "version": "1.2.0" }
```

### DELETE `/v1/:type/:scope/:name/tags/:tag` 🔒

Remove a dist-tag. The `latest` tag cannot be deleted.

```bash
curl -u admin:secret \
  -X DELETE \
  http://localhost:8080/v1/assistants/@yao/keeper/tags/canary
```

**Response** `200 OK`

```json
{ "deleted": "canary" }
```

---

## Dependencies

### GET `/v1/:type/:scope/:name/:version/dependencies`

List direct dependencies declared in this version's `pkg.yao`.

Add `?recursive=true` for a full resolved dependency tree with cycle detection.

**Example**

```bash
# Direct dependencies
curl http://localhost:8080/v1/assistants/@yao/keeper/1.0.0/dependencies

# Full tree
curl "http://localhost:8080/v1/assistants/@yao/keeper/1.0.0/dependencies?recursive=true"
```

**Direct response:**

```json
{
  "dependencies": [
    { "type": "mcp", "scope": "@yao", "name": "tools", "version": "^1.0.0" }
  ]
}
```

**Recursive response:**

```json
{
  "dependencies": [
    {
      "type": "mcp",
      "scope": "@yao",
      "name": "tools",
      "version_constraint": "^1.0.0",
      "resolved": "1.2.0",
      "required_by": ["assistant:@yao/keeper@1.0.0"]
    }
  ]
}
```

Circular dependencies are flagged with `"circular": true` to prevent infinite loops.

---

## Dependents (Reverse Lookup)

### GET `/v1/:type/:scope/:name/dependents`

Find all packages that depend on the given package.

```bash
curl http://localhost:8080/v1/mcps/@yao/tools/dependents
```

**Response** `200 OK`

```json
{
  "dependents": [
    { "type": "assistants", "scope": "@yao", "name": "keeper", "version": "1.0.0" }
  ]
}
```

---

## HTTP Status Codes

| Code | Meaning |
|------|---------|
| `200` | Success |
| `201` | Created (push) |
| `400` | Bad request (invalid type, scope, missing fields) |
| `401` | Unauthorized (missing or wrong credentials) |
| `404` | Package or version not found |
| `409` | Conflict (duplicate version) |
| `413` | Payload too large |
| `500` | Internal server error |
