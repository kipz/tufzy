package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"
)

// Client wraps the TUF updater with convenience methods
type Client struct {
	updater            *updater.Updater
	metadataURL        string
	targetsURL         string
	cacheDir           string
	tufOnCiGit         bool
	consistentSnapshot bool
	hashPrefixes       bool
}

// TargetInfo contains information about a target file
type TargetInfo struct {
	Name        string
	Length      int64
	Hashes      map[string]string
	Custom      *json.RawMessage
	DelegatedBy string
}

// RepositoryInfo contains metadata about the repository
type RepositoryInfo struct {
	RootVersion        int64
	RootExpires        time.Time
	TargetsVersion     int64
	TargetsExpires     time.Time
	SnapshotVersion    int64
	SnapshotExpires    time.Time
	TimestampVersion   int64
	TimestampExpires   time.Time
	MetadataURL        string
	TargetsURL         string
	TufOnCiGit         bool
	ConsistentSnapshot bool
	HashPrefixes       bool
}

// Delegation represents a delegated role
type Delegation struct {
	Name      string
	Threshold int
	KeyIDs    []string
	Paths     []string
	Children  []Delegation
}

// NewClient creates a new TUF client with default options
func NewClient(metadataURL string) (*Client, error) {
	return NewClientWithOptions(metadataURL, ClientOptions{})
}

// ClientOptions contains optional configuration for the TUF client
type ClientOptions struct {
	// PrefixTargetsWithHash controls whether target downloads expect hash prefixes
	PrefixTargetsWithHash bool
	// TufOnCiGit enables tuf-on-ci git repository mode (maps versioned to unversioned filenames)
	TufOnCiGit bool
	// TargetsURL specifies the targets repository URL (required for OCI)
	TargetsURL string
}

// NewClientWithOptions creates a new TUF client with custom options
func NewClientWithOptions(metadataURL string, options ClientOptions) (*Client, error) {
	// Determine cache directory (unique per repository URL)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create unique cache directory based on metadata URL hash
	urlHash := sha256.Sum256([]byte(metadataURL))
	cacheID := hex.EncodeToString(urlHash[:8]) // Use first 8 bytes for shorter path
	cacheDir := filepath.Join(homeDir, ".tufzy", "cache", cacheID)

	// Create cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Check if this is an OCI registry URL
	if isOCI, _, _ := detectOCI(metadataURL, options.TargetsURL); isOCI {
		return newOCIClient(metadataURL, options.TargetsURL, cacheDir)
	}

	// Determine if this is a local filesystem path or HTTP URL
	isLocal := false
	var targetsURL string
	var absPath string

	if filepath.IsAbs(metadataURL) || metadataURL == "." || metadataURL == ".." ||
		(len(metadataURL) >= 2 && metadataURL[:2] == "./") ||
		(len(metadataURL) >= 3 && metadataURL[:3] == "../") {
		// Local filesystem path
		isLocal = true
		var err error
		absPath, err = filepath.Abs(metadataURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path: %w", err)
		}
		metadataURL = "file://" + absPath
		// Targets are in ../targets relative to metadata
		targetsURL = "file://" + filepath.Join(filepath.Dir(absPath), "targets")
	} else {
		// HTTP(S) URL
		parsedURL, err := url.Parse(metadataURL)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata URL: %w", err)
		}
		// Assume targets are at ../targets relative to metadata
		parsedURL.Path = filepath.Dir(parsedURL.Path) + "/targets"
		targetsURL = parsedURL.String()
	}

	// Create metadata directory in cache
	metadataDir := filepath.Join(cacheDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Download or copy root.json if not present (TOFU)
	rootPath := filepath.Join(metadataDir, "root.json")
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		if isLocal {
			// For local paths, copy from source (metadata directory itself)
			sourceRootPath := filepath.Join(absPath, "1.root.json")
			if _, err := os.Stat(sourceRootPath); os.IsNotExist(err) {
				// Try root.json if 1.root.json doesn't exist
				sourceRootPath = filepath.Join(absPath, "root.json")
			}
			data, err := os.ReadFile(sourceRootPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read initial root: %w", err)
			}
			if err := os.WriteFile(rootPath, data, 0644); err != nil {
				return nil, fmt.Errorf("failed to write initial root: %w", err)
			}
		} else {
			if err := downloadFile(metadataURL+"/1.root.json", rootPath); err != nil {
				return nil, fmt.Errorf("failed to download initial root: %w", err)
			}
		}
	}

	// Read trusted root
	rootBytes, err := os.ReadFile(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read trusted root: %w", err)
	}

	// Parse root to detect consistent_snapshot setting
	var rootData struct {
		Signed struct {
			ConsistentSnapshot bool `json:"consistent_snapshot"`
		} `json:"signed"`
	}
	if err := json.Unmarshal(rootBytes, &rootData); err != nil {
		return nil, fmt.Errorf("failed to parse root metadata: %w", err)
	}

	// Auto-detect tuf-on-ci git layout for local repositories
	// Check if unversioned metadata files exist (timestamp.json, snapshot.json, targets.json)
	// This indicates tuf-on-ci git layout vs standard TUF (which has N.snapshot.json, etc)
	tufOnCiGit := false
	if isLocal {
		timestampPath := filepath.Join(absPath, "timestamp.json")
		snapshotPath := filepath.Join(absPath, "snapshot.json")
		targetsPath := filepath.Join(absPath, "targets.json")

		// If all three unversioned files exist, it's tuf-on-ci git layout
		_, tsErr := os.Stat(timestampPath)
		_, snapErr := os.Stat(snapshotPath)
		_, tgtErr := os.Stat(targetsPath)

		if tsErr == nil && snapErr == nil && tgtErr == nil {
			tufOnCiGit = true
		}
	}
	// User can still override via options
	if options.TufOnCiGit {
		tufOnCiGit = true
	}

	// Auto-detect hash prefix from consistent_snapshot
	// BUT: tuf-on-ci git repos don't use hash prefixes even when consistent_snapshot=true
	// Only published tuf-on-ci repos use hash prefixes
	prefixTargetsWithHash := rootData.Signed.ConsistentSnapshot
	if tufOnCiGit {
		// tuf-on-ci git layout never uses hash prefixes
		prefixTargetsWithHash = false
	}

	// Create updater configuration
	cfg, err := config.New(metadataURL, rootBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	// Configure optional settings
	cfg.LocalMetadataDir = metadataDir
	cfg.LocalTargetsDir = filepath.Join(cacheDir, "targets")
	cfg.RemoteTargetsURL = targetsURL
	cfg.MaxRootRotations = 32
	cfg.PrefixTargetsWithHash = prefixTargetsWithHash

	// Use custom fetcher that supports file:// URLs and optionally tuf-on-ci git layout
	if tufOnCiGit {
		cfg.Fetcher = NewTufOnCiFetcher(metadataURL)
	} else {
		cfg.Fetcher = NewFilesystemFetcher()
	}

	// Create updater
	tufUpdater, err := updater.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create updater: %w", err)
	}

	return &Client{
		updater:            tufUpdater,
		metadataURL:        metadataURL,
		targetsURL:         targetsURL,
		cacheDir:           cacheDir,
		tufOnCiGit:         tufOnCiGit,
		consistentSnapshot: rootData.Signed.ConsistentSnapshot,
		hashPrefixes:       prefixTargetsWithHash,
	}, nil
}

// Update refreshes the metadata from the remote repository
func (c *Client) Update() error {
	return c.updater.Refresh()
}

// GetTargets returns all available targets
func (c *Client) GetTargets() ([]TargetInfo, error) {
	targetFiles := c.updater.GetTopLevelTargets()

	var targets []TargetInfo
	for name, targetFile := range targetFiles {
		hashes := make(map[string]string)
		for alg, hash := range targetFile.Hashes {
			hashes[alg] = fmt.Sprintf("%x", hash)
		}

		targets = append(targets, TargetInfo{
			Name:   name,
			Length: targetFile.Length,
			Hashes: hashes,
			Custom: targetFile.Custom,
		})
	}

	return targets, nil
}

// GetRepositoryInfo returns metadata about the repository
func (c *Client) GetRepositoryInfo() (*RepositoryInfo, error) {
	// Get trusted metadata
	trusted := c.updater.GetTrustedMetadataSet()

	info := &RepositoryInfo{
		MetadataURL:        c.metadataURL,
		TargetsURL:         c.targetsURL,
		TufOnCiGit:         c.tufOnCiGit,
		ConsistentSnapshot: c.consistentSnapshot,
		HashPrefixes:       c.hashPrefixes,
	}

	// Root info
	if root := trusted.Root; root != nil {
		info.RootVersion = root.Signed.Version
		info.RootExpires = root.Signed.Expires
	}

	// Targets info
	if targets := trusted.Targets["targets"]; targets != nil {
		info.TargetsVersion = targets.Signed.Version
		info.TargetsExpires = targets.Signed.Expires
	}

	// Snapshot info
	if snapshot := trusted.Snapshot; snapshot != nil {
		info.SnapshotVersion = snapshot.Signed.Version
		info.SnapshotExpires = snapshot.Signed.Expires
	}

	// Timestamp info
	if timestamp := trusted.Timestamp; timestamp != nil {
		info.TimestampVersion = timestamp.Signed.Version
		info.TimestampExpires = timestamp.Signed.Expires
	}

	return info, nil
}

// GetDelegations returns the delegation tree
func (c *Client) GetDelegations() ([]Delegation, error) {
	trusted := c.updater.GetTrustedMetadataSet()

	var delegations []Delegation

	// Get targets metadata
	if targetsMetadata := trusted.Targets["targets"]; targetsMetadata != nil {
		if targetsMetadata.Signed.Delegations != nil {
			for _, role := range targetsMetadata.Signed.Delegations.Roles {
				delegations = append(delegations, Delegation{
					Name:      role.Name,
					Threshold: role.Threshold,
					KeyIDs:    role.KeyIDs,
					Paths:     role.Paths,
				})
			}
		}
	}

	return delegations, nil
}

// DownloadTarget downloads and verifies a specific target file
func (c *Client) DownloadTarget(name string, destPath string) (*TargetInfo, error) {
	// Get target info
	targetFile, err := c.updater.GetTargetInfo(name)
	if err != nil {
		return nil, fmt.Errorf("target not found: %w", err)
	}

	// Download and verify
	path, _, err := c.updater.DownloadTarget(targetFile, destPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to download target: %w", err)
	}

	hashes := make(map[string]string)
	for alg, hash := range targetFile.Hashes {
		hashes[alg] = fmt.Sprintf("%x", hash)
	}

	return &TargetInfo{
		Name:   path,
		Length: targetFile.Length,
		Hashes: hashes,
		Custom: targetFile.Custom,
	}, nil
}

// downloadFile downloads a file from a URL
func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)
	return err
}

// detectOCI checks if the metadata URL uses the OCI scheme and validates targets URL
func detectOCI(metadataURL, targetsURL string) (isOCI bool, metadata string, targets string) {
	if !hasOCIScheme(metadataURL) {
		return false, "", ""
	}

	if targetsURL == "" {
		return true, "", "" // OCI but no targets URL provided (error case)
	}

	if !hasOCIScheme(targetsURL) {
		return true, "", "" // OCI but targets not OCI (error case)
	}

	return true, metadataURL, targetsURL
}

// hasOCIScheme checks if a URL has the OCI scheme prefix
func hasOCIScheme(url string) bool {
	return len(url) >= len(OCIScheme) && url[:len(OCIScheme)] == OCIScheme
}

// newOCIClient creates a TUF client for OCI registries
func newOCIClient(metadataURL, targetsURL, cacheDir string) (*Client, error) {
	if targetsURL == "" {
		return nil, fmt.Errorf("targets URL is required for OCI repositories")
	}

	// Create metadata directory in cache
	metadataDir := filepath.Join(cacheDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Download initial root.json if not present (TOFU)
	rootPath := filepath.Join(metadataDir, "root.json")
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		// Create a temporary RegistryFetcher just to download the initial root
		ctx := contextWithTimeout(30 * time.Second)
		fetcher, err := NewRegistryFetcher(ctx, metadataURL, targetsURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create registry fetcher: %w", err)
		}

		// Try to download 1.root.json
		rootData, err := fetcher.DownloadFile(metadataURL+"/1.root.json", 512000, 30*time.Second)
		if err != nil {
			return nil, fmt.Errorf("failed to download initial root: %w", err)
		}

		if err := os.WriteFile(rootPath, rootData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write initial root: %w", err)
		}
	}

	// Read trusted root
	rootBytes, err := os.ReadFile(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read trusted root: %w", err)
	}

	// Parse root to detect consistent_snapshot setting
	var rootData struct {
		Signed struct {
			ConsistentSnapshot bool `json:"consistent_snapshot"`
		} `json:"signed"`
	}
	if err := json.Unmarshal(rootBytes, &rootData); err != nil {
		return nil, fmt.Errorf("failed to parse root metadata: %w", err)
	}

	// Create updater configuration
	cfg, err := config.New(metadataURL, rootBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	// Configure settings
	cfg.LocalMetadataDir = metadataDir
	cfg.LocalTargetsDir = filepath.Join(cacheDir, "targets")
	cfg.RemoteTargetsURL = targetsURL
	cfg.MaxRootRotations = 32
	cfg.PrefixTargetsWithHash = rootData.Signed.ConsistentSnapshot

	// Create OCI registry fetcher
	ctx := contextWithTimeout(30 * time.Second)
	fetcher, err := NewRegistryFetcher(ctx, metadataURL, targetsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry fetcher: %w", err)
	}
	cfg.Fetcher = fetcher

	// Create updater
	tufUpdater, err := updater.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create updater: %w", err)
	}

	return &Client{
		updater:            tufUpdater,
		metadataURL:        metadataURL,
		targetsURL:         targetsURL,
		cacheDir:           cacheDir,
		tufOnCiGit:         false,
		consistentSnapshot: rootData.Signed.ConsistentSnapshot,
		hashPrefixes:       rootData.Signed.ConsistentSnapshot,
	}, nil
}

// contextWithTimeout creates a context with timeout
func contextWithTimeout(timeout time.Duration) context.Context {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	return ctx
}
