# 🎯 tufzy

A friendly [TUF (The Update Framework)](https://theupdateframework.io/) client with pretty output and emojis!

## Features

- ✅ **Trust On First Use (TOFU)**: Automatically bootstraps trust with the repository's root.json
- 📦 **List Targets**: View all available files in the repository
- ⬇️  **Download & Verify**: Securely download and verify target files
- 🌳 **Show Delegations**: Visualize the delegation tree
- 📊 **Repository Info**: Display metadata about the repository (versions, expiry dates, etc.)
- 🎨 **Pretty Output**: Colorful, emoji-rich output with formatted tables
- 📁 **Local & Remote**: Works with both HTTP(S) URLs and local filesystem paths

## Installation

```bash
go install github.com/kipz/tufzy/cmd/tufzy@latest
```

Or build from source:

```bash
git clone https://github.com/kipz/tufzy.git
cd tufzy
go build -o tufzy ./cmd/tufzy
```

## Usage

### List all targets

```bash
# Remote repository
tufzy list https://jku.github.io/tuf-demo/metadata

# Local repository
tufzy list /path/to/repo/metadata
tufzy list ./metadata
```

### Download a specific file

```bash
tufzy get https://jku.github.io/tuf-demo/metadata rdimitrov/artifact-example.md

# Download to specific location
tufzy get https://jku.github.io/tuf-demo/metadata file1.txt -o /tmp/downloaded.txt
```

### Show repository information

```bash
tufzy info https://jku.github.io/tuf-demo/metadata
```

### Show delegation tree

```bash
tufzy delegations https://jku.github.io/tuf-demo/metadata
```

### Auto-Detection

tufzy automatically detects repository configuration with **zero manual flags**:

#### What gets auto-detected:
- **Repository type**: Local filesystem vs remote HTTP(S)
- **tuf-on-ci git layout**: Checks for unversioned metadata files (`timestamp.json`, `snapshot.json`, `targets.json`)
- **Hash-prefixed targets**: Based on `consistent_snapshot` field in root metadata
- **Hash prefix override**: Disabled for tuf-on-ci git repos (source files don't have prefixes)

#### How it works:

**For standard TUF repositories**:
- Reads `consistent_snapshot` from root metadata
- If `true`: expects hash-prefixed targets (`{sha256}.filename`)
- If `false`: expects plain filenames

**For tuf-on-ci git repositories** (local checkouts):
- Detects unversioned metadata files in directory
- Automatically maps versioned requests to unversioned files:
  - `N.root.json` (N > 1) → `root_history/N.root.json`
  - `N.snapshot.json` → `snapshot.json`
  - `N.timestamp.json` → `timestamp.json`
  - `N.targets.json` → `targets.json`
- Forces hash prefixes OFF (git source files don't have them)

**For tuf-on-ci published repositories** (GitHub Pages):
- Works like standard TUF (has versioned files and hash prefixes)

### Using with any TUF repository

Just point tufzy at the metadata URL or path - no flags needed:

```bash
# Remote standard TUF
tufzy list https://jku.github.io/tuf-demo/metadata

# Remote tuf-on-ci (published)
tufzy list https://example.github.io/repo/metadata

# Local tuf-on-ci git checkout
tufzy list /path/to/tuf-on-ci-repo/metadata
tufzy list ./metadata

# All commands work the same way
tufzy info <url-or-path>
tufzy delegations <url-or-path>
tufzy get <url-or-path> <target-file>
```

## Example Output

### List Command
```
$ tufzy list https://jku.github.io/tuf-demo/metadata

✅ TUF Repository
📍 Metadata: https://jku.github.io/tuf-demo/metadata
📦 Targets:  https://jku.github.io/tuf-demo/targets
🔍 Detected: consistent_snapshot (hash prefixes enabled)

🎯 Targets (1)

  Name                  Size        SHA256
  ────────────────────────────────────────────────────
  file1.txt             5 B         6663346235666436...
```

### Info Command
```
$ tufzy info https://jku.github.io/tuf-demo/metadata

📊 Repository Information

URLs:
  Metadata: https://jku.github.io/tuf-demo/metadata
  Targets:  https://jku.github.io/tuf-demo/targets

🔍 Auto-detected:
  Layout: standard TUF
  Hash prefixes: enabled (consistent_snapshot=true)

⏰ Metadata Expiry:
  ✅ Root:      v4   expires 2026-02-03 (expires in 3 months)
  ✅ Targets:   v6   expires 2026-03-21 (expires in 5 months)
  ✅ Snapshot:  v14  expires 2026-04-08 (expires in 6 months)
  ⚠️ Timestamp: v642 expires 2025-10-08 (expires in 1 days)
```

### Local tuf-on-ci Git Repository
```
$ tufzy list ./metadata

✅ TUF Repository 📝 tuf-on-ci git
📍 Metadata: file:///path/to/repo/metadata
📦 Targets:  file:///path/to/repo/targets
🔍 Detected: tuf-on-ci git layout

🎯 Targets (1)

  Name                  Size        SHA256
  ────────────────────────────────────────────────────
  policies.json         41.9 KB     3366323162663364...
```

### Delegations Command
```
$ tufzy delegations https://jku.github.io/tuf-demo/metadata

🌳 Delegation Tree

  targets
  ├── 📄 jku (threshold: 1/1)
  │   └── patterns: jku/*
  ├── 📄 rdimitrov (threshold: 1/2)
  │   └── patterns: rdimitrov/*
  └── 📄 kommendorkapten (threshold: 1/1)
      └── patterns: kommendorkapten/*
```

### Get Command
```
$ tufzy get https://jku.github.io/tuf-demo/metadata rdimitrov/artifact-example.md

⬇️ Downloading rdimitrov/artifact-example.md to artifact-example.md...
✅ Downloaded and verified rdimitrov/artifact-example.md (23 B)
   Saved to: artifact-example.md
```

## How It Works

tufzy uses the [go-tuf v2](https://github.com/theupdateframework/go-tuf) library to interact with TUF repositories. On first run, it downloads and caches the root.json file (TOFU), then uses it to verify all subsequent metadata and target files according to the TUF specification.

Each repository gets its own isolated cache directory (based on URL hash) in `~/.tufzy/cache/`, preventing conflicts when working with multiple repositories.

## Known Limitations

Due to go-tuf v2 implementation details, tufzy requires:

- **ECDSA/RSA keys in PEM format** - Repositories with hex-encoded raw EC keys are not supported
- **Hex-encoded signatures** - Repositories using base64-encoded signatures (e.g., Sigstore keyless signing) are not supported
- **Standard TUF metadata format** - Extra fields like Sigstore bundles may cause issues

### Compatible Repositories

✅ Works with:
- Repositories using Cloud KMS (GCP, Azure, AWS) for signing
- Repositories using Yubikey/hardware tokens for signing
- Standard TUF repositories with PEM keys and hex signatures
- tuf-on-ci repositories (both published and git checkouts)

❌ Does NOT work with:
- tuf-on-ci repositories using Sigstore keyless signing (base64 signatures)
- Repositories with hex-encoded raw EC public keys (not PEM)

**Example working repositories**:
- Remote standard TUF: https://jku.github.io/tuf-demo/metadata
- Local tuf-on-ci git: Any local checkout with `metadata/` directory

## Development

```bash
# Run tests
go test ./...

# Build
go build -o tufzy ./cmd/tufzy

# Test with demo repository
./tufzy list https://jku.github.io/tuf-demo/metadata
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please open an issue or PR.
