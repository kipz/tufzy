package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestOCIRegistryIntegration tests the full OCI registry download flow using testcontainers
func TestOCIRegistryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start a local OCI registry using testcontainers
	req := testcontainers.ContainerRequest{
		Image:        "registry:2",
		ExposedPorts: []string{"5000/tcp"},
		WaitingFor:   wait.ForHTTP("/v2/").WithPort("5000/tcp"),
	}
	registryContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	defer func() {
		_ = registryContainer.Terminate(ctx)
	}()

	// Get the registry host
	host, err := registryContainer.Host(ctx)
	require.NoError(t, err)
	port, err := registryContainer.MappedPort(ctx, "5000")
	require.NoError(t, err)
	registryAddr := fmt.Sprintf("%s:%s", host, port.Port())

	// Create test TUF metadata
	rootMetadata := createTestRootMetadata(t)
	timestampMetadata := createTestTimestampMetadata(t)
	snapshotMetadata := createTestSnapshotMetadata(t)
	targetsMetadata := createTestTargetsMetadata(t)
	delegatedMetadata := createTestDelegatedMetadata(t, "test-role")
	testTarget := []byte("test target content")

	// Push metadata to registry
	metadataRepo := fmt.Sprintf("%s/test/metadata", registryAddr)
	targetsRepo := fmt.Sprintf("%s/test/targets", registryAddr)

	// Create metadata image with all four top-level roles
	err = pushMetadataImage(ctx, metadataRepo, "latest", map[string][]byte{
		"1.root.json":      rootMetadata,
		"timestamp.json":   timestampMetadata,
		"snapshot.json":    snapshotMetadata,
		"targets.json":     targetsMetadata,
	})
	require.NoError(t, err)

	// Create delegated metadata image
	err = pushMetadataImage(ctx, metadataRepo, "test-role", map[string][]byte{
		"test-role.json": delegatedMetadata,
	})
	require.NoError(t, err)

	// Push target file
	err = pushTargetImage(ctx, targetsRepo, "abc123.test.txt", testTarget)
	require.NoError(t, err)

	// Create RegistryFetcher
	metadataURL := fmt.Sprintf("oci://%s", metadataRepo)
	targetsURL := fmt.Sprintf("oci://%s", targetsRepo)

	fetcher, err := NewRegistryFetcher(ctx, metadataURL+":latest", targetsURL+":latest")
	require.NoError(t, err)

	// Test downloading root metadata
	t.Run("download root metadata", func(t *testing.T) {
		data, err := fetcher.DownloadFile(metadataURL+":latest/1.root.json", 512000, 30*time.Second)
		require.NoError(t, err)
		assert.Equal(t, rootMetadata, data)
	})

	// Test downloading timestamp metadata
	t.Run("download timestamp metadata", func(t *testing.T) {
		data, err := fetcher.DownloadFile(metadataURL+":latest/timestamp.json", 512000, 30*time.Second)
		require.NoError(t, err)
		assert.Equal(t, timestampMetadata, data)
	})

	// Test downloading snapshot metadata
	t.Run("download snapshot metadata", func(t *testing.T) {
		data, err := fetcher.DownloadFile(metadataURL+":latest/snapshot.json", 512000, 30*time.Second)
		require.NoError(t, err)
		assert.Equal(t, snapshotMetadata, data)
	})

	// Test downloading targets metadata
	t.Run("download targets metadata", func(t *testing.T) {
		data, err := fetcher.DownloadFile(metadataURL+":latest/targets.json", 512000, 30*time.Second)
		require.NoError(t, err)
		assert.Equal(t, targetsMetadata, data)
	})

	// Test downloading delegated metadata
	t.Run("download delegated metadata", func(t *testing.T) {
		data, err := fetcher.DownloadFile(metadataURL+":latest/test-role.json", 512000, 30*time.Second)
		require.NoError(t, err)
		assert.Equal(t, delegatedMetadata, data)
	})

	// Test downloading target file
	t.Run("download target file", func(t *testing.T) {
		data, err := fetcher.DownloadFile(targetsURL+":latest/abc123.test.txt", 512000, 30*time.Second)
		require.NoError(t, err)
		assert.Equal(t, testTarget, data)
	})

	// Test caching - download same file twice, should use cache
	t.Run("test caching", func(t *testing.T) {
		// First download
		data1, err := fetcher.DownloadFile(metadataURL+":latest/1.root.json", 512000, 30*time.Second)
		require.NoError(t, err)

		// Second download (should hit cache)
		data2, err := fetcher.DownloadFile(metadataURL+":latest/1.root.json", 512000, 30*time.Second)
		require.NoError(t, err)

		assert.Equal(t, data1, data2)
	})
}

// pushMetadataImage creates and pushes an OCI image with metadata layers
func pushMetadataImage(ctx context.Context, repo, tag string, files map[string][]byte) error {
	img := empty.Image

	// Add config layer
	img = mutate.ConfigMediaType(img, types.OCIManifestSchema1)

	// Create layers with annotations
	adds := []mutate.Addendum{}
	for filename, content := range files {
		layer := static.NewLayer(content, TUFMetadataMediaType)
		adds = append(adds, mutate.Addendum{
			Layer: layer,
			Annotations: map[string]string{
				TUFFilenameAnnotation: filename,
			},
		})
	}

	// Add all layers at once with their annotations
	var err error
	img, err = mutate.Append(img, adds...)
	if err != nil {
		return fmt.Errorf("failed to append layers: %w", err)
	}

	// Push image
	ref := fmt.Sprintf("%s:%s", repo, tag)
	return crane.Push(img, ref)
}

// pushTargetImage creates and pushes an OCI image for a target file
func pushTargetImage(ctx context.Context, repo, filename string, content []byte) error {
	img := empty.Image
	img = mutate.ConfigMediaType(img, types.OCIManifestSchema1)

	layer := static.NewLayer(content, TUFTargetMediaType)

	// Add layer with annotation
	var err error
	img, err = mutate.Append(img, mutate.Addendum{
		Layer: layer,
		Annotations: map[string]string{
			TUFFilenameAnnotation: filename,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to append layer: %w", err)
	}

	ref := fmt.Sprintf("%s:%s", repo, filename)
	return crane.Push(img, ref)
}

// createTestRootMetadata creates a minimal TUF root metadata for testing
func createTestRootMetadata(t *testing.T) []byte {
	root := map[string]interface{}{
		"signed": map[string]interface{}{
			"_type":               "root",
			"spec_version":        "1.0.0",
			"version":             1,
			"expires":             time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339),
			"consistent_snapshot": true,
			"keys":                map[string]interface{}{},
			"roles": map[string]interface{}{
				"root":      map[string]interface{}{"keyids": []string{}, "threshold": 1},
				"targets":   map[string]interface{}{"keyids": []string{}, "threshold": 1},
				"snapshot":  map[string]interface{}{"keyids": []string{}, "threshold": 1},
				"timestamp": map[string]interface{}{"keyids": []string{}, "threshold": 1},
			},
		},
		"signatures": []interface{}{},
	}

	data, err := json.MarshalIndent(root, "", "  ")
	require.NoError(t, err)
	return data
}

// createTestTimestampMetadata creates a minimal TUF timestamp metadata for testing
func createTestTimestampMetadata(t *testing.T) []byte {
	timestamp := map[string]interface{}{
		"signed": map[string]interface{}{
			"_type":        "timestamp",
			"spec_version": "1.0.0",
			"version":      1,
			"expires":      time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
			"meta": map[string]interface{}{
				"snapshot.json": map[string]interface{}{
					"version": 1,
				},
			},
		},
		"signatures": []interface{}{},
	}

	data, err := json.MarshalIndent(timestamp, "", "  ")
	require.NoError(t, err)
	return data
}

// createTestSnapshotMetadata creates a minimal TUF snapshot metadata for testing
func createTestSnapshotMetadata(t *testing.T) []byte {
	snapshot := map[string]interface{}{
		"signed": map[string]interface{}{
			"_type":        "snapshot",
			"spec_version": "1.0.0",
			"version":      1,
			"expires":      time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339),
			"meta": map[string]interface{}{
				"targets.json": map[string]interface{}{
					"version": 1,
				},
			},
		},
		"signatures": []interface{}{},
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	require.NoError(t, err)
	return data
}

// createTestTargetsMetadata creates a minimal TUF targets metadata for testing
func createTestTargetsMetadata(t *testing.T) []byte {
	targets := map[string]interface{}{
		"signed": map[string]interface{}{
			"_type":        "targets",
			"spec_version": "1.0.0",
			"version":      1,
			"expires":      time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339),
			"targets": map[string]interface{}{
				"test.txt": map[string]interface{}{
					"length": 19,
					"hashes": map[string]string{
						"sha256": "abc123",
					},
				},
			},
		},
		"signatures": []interface{}{},
	}

	data, err := json.MarshalIndent(targets, "", "  ")
	require.NoError(t, err)
	return data
}

// createTestDelegatedMetadata creates a minimal TUF delegated targets metadata for testing
func createTestDelegatedMetadata(t *testing.T, roleName string) []byte {
	delegated := map[string]interface{}{
		"signed": map[string]interface{}{
			"_type":        "targets",
			"spec_version": "1.0.0",
			"version":      1,
			"expires":      time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339),
			"targets":      map[string]interface{}{},
		},
		"signatures": []interface{}{},
	}

	data, err := json.MarshalIndent(delegated, "", "  ")
	require.NoError(t, err)
	return data
}

// TestNewOCIClient tests creating a TUF client with OCI URLs
func TestNewOCIClient(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp cache dir
	tmpDir, err := os.MkdirTemp("", "tufzy-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test root metadata file
	rootMetadata := createTestRootMetadata(t)
	metadataDir := filepath.Join(tmpDir, "metadata")
	err = os.MkdirAll(metadataDir, 0755)
	require.NoError(t, err)

	rootPath := filepath.Join(metadataDir, "root.json")
	err = os.WriteFile(rootPath, rootMetadata, 0644)
	require.NoError(t, err)

	// Test with invalid targets URL (should error)
	t.Run("missing targets URL", func(t *testing.T) {
		_, err := newOCIClient("oci://registry.example.com/metadata:latest", "", tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "targets URL is required")
	})
}
