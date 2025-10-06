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

### Using with tuf-on-ci repositories

#### Published tuf-on-ci repositories (GitHub Pages)

For repositories using hash-prefixed target paths:

```bash
tufzy list https://example.github.io/repo/metadata --hash-prefix
tufzy get https://example.github.io/repo/metadata file.txt --hash-prefix
```

#### Local tuf-on-ci git repositories

For local git checkouts of tuf-on-ci repositories (which use unversioned metadata files):

```bash
tufzy list /path/to/repo/metadata --hash-prefix --tuf-on-ci-git
tufzy info /path/to/repo/metadata --hash-prefix --tuf-on-ci-git
tufzy delegations /path/to/repo/metadata --hash-prefix --tuf-on-ci-git
```

The `--tuf-on-ci-git` flag maps TUF's versioned metadata requests to tuf-on-ci's git layout:
- `N.root.json` (N > 1) → `root_history/N.root.json`
- `N.snapshot.json` → `snapshot.json` (current version)
- `N.timestamp.json` → `timestamp.json` (current version)
- `N.targets.json` → `targets.json` (current version)

## Example Output

### List Command
```
$ tufzy list https://jku.github.io/tuf-demo/metadata

✅ TUF Repository
📍 Metadata: https://jku.github.io/tuf-demo/metadata
📦 Targets:  https://jku.github.io/tuf-demo/targets

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

⏰ Metadata Expiry:
  ✅ Root:      v4   expires 2026-02-03 (expires in 3 months)
  ✅ Targets:   v6   expires 2026-03-21 (expires in 5 months)
  ✅ Snapshot:  v14  expires 2026-04-08 (expires in 6 months)
  ⚠️ Timestamp: v642 expires 2025-10-08 (expires in 1 days)
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
- Remote: https://jku.github.io/tuf-demo/metadata
- Local tuf-on-ci git: Use `--tuf-on-ci-git` flag

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
