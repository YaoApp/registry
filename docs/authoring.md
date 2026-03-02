# Package Authoring Guide

This document describes how to create `.yao.zip` packages for the Yao Registry.

## Package Format

Every package is a **zip archive** with the `.yao.zip` extension. The archive must contain a manifest file at exactly:

```
package/pkg.yao
```

An optional README at `package/README.md` (case-insensitive) is extracted and displayed in the registry.

The rest of the archive contents are type-specific — the registry stores the entire zip as-is. You can include scripts, binaries, configurations, or any other files your package needs.

## Archive Structure

```
my-package.yao.zip
└── package/
    ├── pkg.yao           # Required — manifest
    ├── README.md         # Optional — displayed in registry
    ├── config.json       # Type-specific content
    ├── scripts/
    │   └── setup.js
    └── ...
```

## pkg.yao Manifest

The manifest file uses **JSONC** format (JSON with `//` and `/* */` comments). The registry strips comments before parsing.

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Package type: `release`, `robot`, `assistant`, or `mcp` |
| `scope` | string | Namespace with `@` prefix (e.g., `@yao`, `@myorg`) |
| `name` | string | Package name (lowercase, alphanumeric, hyphens) |
| `version` | string | Semver version (e.g., `1.0.0`, `2.1.0-beta.1`) |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Short package description |
| `keywords` | string[] | Searchable keywords |
| `icon` | string | URL to package icon |
| `license` | string | SPDX license identifier |
| `author` | object | `{ "name", "email", "url" }` |
| `maintainers` | object[] | Array of `{ "name", "email", "url" }` |
| `homepage` | string | Project homepage URL |
| `repository` | object | `{ "type": "git", "url": "..." }` |
| `bugs` | object | `{ "url": "..." }` |
| `engines` | object | Engine version constraints, e.g., `{ "yao": ">=2.0.0" }` |
| `dependencies` | object[] | Package dependencies (see below) |
| `platform` | object | Platform info for release packages (see below) |
| `metadata` | object | Arbitrary key-value data |

---

## Examples by Type

### Assistant Package

```jsonc
{
  // Yao assistant package manifest
  "type": "assistant",
  "scope": "@yao",
  "name": "keeper",
  "version": "1.0.0",
  "description": "Knowledge management assistant powered by RAG",
  "keywords": ["knowledge", "rag", "assistant"],
  "license": "Apache-2.0",
  "author": {
    "name": "Yao Team",
    "email": "team@yaoapps.com"
  },
  "homepage": "https://yaoapps.com/assistants/keeper",
  "repository": {
    "type": "git",
    "url": "https://github.com/yaoapp/keeper"
  },
  "engines": {
    "yao": ">=2.0.0"
  },
  "dependencies": [
    { "type": "mcp", "scope": "@yao", "name": "rag-tools", "version": "^1.0.0" }
  ]
}
```

### MCP Tool Package

```jsonc
{
  "type": "mcp",
  "scope": "@yao",
  "name": "rag-tools",
  "version": "1.2.0",
  "description": "MCP tools for retrieval-augmented generation",
  "keywords": ["mcp", "rag", "tools"],
  "license": "MIT",
  "engines": {
    "yao": ">=2.0.0"
  },
  "dependencies": [
    { "type": "assistant", "scope": "@yao", "name": "embedder", "version": ">=1.0.0" }
  ]
}
```

### Robot Configuration

```jsonc
{
  "type": "robot",
  "scope": "@yao",
  "name": "customer-support",
  "version": "0.5.0",
  "description": "Customer support robot with multi-channel integration",
  "keywords": ["robot", "support", "customer"],
  "author": { "name": "Yao Team" },
  "dependencies": [
    { "type": "assistant", "scope": "@yao", "name": "keeper", "version": "^1.0.0" },
    { "type": "mcp", "scope": "@yao", "name": "rag-tools", "version": "^1.0.0" }
  ]
}
```

### Release (Platform Binary)

Release packages are special — each platform artifact is uploaded as a separate push with its own `platform` field.

```jsonc
{
  "type": "release",
  "scope": "@yao",
  "name": "yao",
  "version": "1.0.0",
  "description": "Yao Application Engine",
  "license": "Apache-2.0",
  "homepage": "https://yaoapps.com",
  "platform": {
    "os": "darwin",
    "arch": "arm64"
  }
}
```

Push once per platform:

```bash
# macOS ARM
curl -u admin:secret -X PUT \
  -H "Content-Type: application/zip" \
  --data-binary @yao-darwin-arm64.yao.zip \
  http://localhost:8080/v1/releases/@yao/yao/1.0.0

# Linux x86_64
curl -u admin:secret -X PUT \
  -H "Content-Type: application/zip" \
  --data-binary @yao-linux-amd64.yao.zip \
  http://localhost:8080/v1/releases/@yao/yao/1.0.0
```

Each push stores a separate artifact. The version detail endpoint aggregates all platform artifacts.

---

## Dependencies

Dependencies declare cross-package relationships. The `version` field uses **npm-compatible semver constraints**:

| Syntax | Meaning |
|--------|---------|
| `^1.2.0` | `>=1.2.0 <2.0.0` |
| `~1.2.0` | `>=1.2.0 <1.3.0` |
| `>=1.0.0` | Any version `1.0.0` or higher |
| `1.0.x` | Any patch of `1.0` |
| `*` | Any version |
| <code>>=1.0.0 &#124;&#124; <0.5.0</code> | Union of ranges |

Cross-type dependencies are fully supported:

- **Robot** → Assistant, MCP
- **Assistant** → Assistant, MCP
- **MCP** → Assistant, MCP

The registry stores and indexes all dependency edges. Use the [dependency API](api-reference.md#dependencies) to query direct or recursive dependency trees with circular dependency detection.

---

## Engine Constraints

The `engines` field declares which Yao engine versions are compatible:

```jsonc
{
  "engines": {
    "yao": ">=2.0.0 <3.0.0"
  }
}
```

Engine constraints use the same semver syntax as dependencies. They are stored in the registry metadata and returned in both full and abbreviated packument responses. Validation is performed client-side by the Yao CLI.

---

## Building a Package

### Manual

```bash
# Create directory structure
mkdir -p my-pkg/package

# Copy manifest and readme
cp pkg.yao my-pkg/package/
cp README.md my-pkg/package/

# Add your content
cp -r scripts/ my-pkg/package/scripts/
cp config.json my-pkg/package/

# Create the zip (from the parent so "package/" prefix is preserved)
cd my-pkg && zip -r ../my-package-1.0.0.yao.zip package/ && cd ..
```

### Verify the archive

```bash
unzip -l my-package-1.0.0.yao.zip
```

Expected output should show `package/pkg.yao` inside the archive:

```
  Length      Date    Time    Name
---------  ---------- -----   ----
        0  03-02-2026 10:00   package/
      512  03-02-2026 10:00   package/pkg.yao
     1024  03-02-2026 10:00   package/README.md
      ...
```

### Push

```bash
curl -u admin:secret \
  -X PUT \
  -H "Content-Type: application/zip" \
  --data-binary @my-package-1.0.0.yao.zip \
  http://localhost:8080/v1/assistants/@yao/my-package/1.0.0
```

The registry will:

1. Verify the zip is valid
2. Extract and parse `package/pkg.yao` (stripping JSONC comments)
3. Validate that manifest fields match URL parameters
4. Extract `package/README.md` if present
5. Compute SHA-256 digest
6. Store the file on disk
7. Insert package/version/dependency records in the database
8. Set the `latest` dist-tag for non-prerelease versions

---

## Version Management

### Semver

All versions must be valid semver: `MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]`

Examples: `1.0.0`, `2.1.0-beta.1`, `3.0.0-rc.1+build.42`

### Dist-Tags

Tags are mutable pointers to versions (like Docker tags or npm dist-tags):

```bash
# Set a "canary" tag
curl -u admin:secret -X PUT \
  -H "Content-Type: application/json" \
  -d '{"version": "2.0.0-beta.1"}' \
  http://localhost:8080/v1/assistants/@yao/keeper/tags/canary

# Pull by tag
curl -o keeper.yao.zip \
  http://localhost:8080/v1/assistants/@yao/keeper/canary/pull

# Remove a tag (cannot delete "latest")
curl -u admin:secret -X DELETE \
  http://localhost:8080/v1/assistants/@yao/keeper/tags/canary
```

The `latest` tag is automatically managed:
- Set to the current version on every non-prerelease push
- If the first push is a prerelease, `latest` points to it until a stable version is published
- When a version is deleted, `latest` is reassigned to the next most recent non-prerelease version

---

## Size Limits

The registry enforces a maximum package size (default **512 MB**, configurable via `--max-size`). Uploads exceeding this limit receive a `413 Request Entity Too Large` response.
