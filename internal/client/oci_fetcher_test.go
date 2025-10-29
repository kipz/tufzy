package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistryFetcher(t *testing.T) {
	tests := []struct {
		name         string
		metadataURL  string
		targetsURL   string
		wantErr      bool
		wantMetaRepo string
		wantMetaTag  string
		wantTgtRepo  string
		wantTgtTag   string
	}{
		{
			name:         "basic URLs with explicit tags",
			metadataURL:  "oci://registry.example.com/repo/metadata:v1",
			targetsURL:   "oci://registry.example.com/repo/targets:v1",
			wantErr:      false,
			wantMetaRepo: "registry.example.com/repo/metadata",
			wantMetaTag:  "v1",
			wantTgtRepo:  "registry.example.com/repo/targets",
			wantTgtTag:   "v1",
		},
		{
			name:         "URLs with default latest tag",
			metadataURL:  "oci://registry.example.com/repo/metadata",
			targetsURL:   "oci://registry.example.com/repo/targets",
			wantErr:      false,
			wantMetaRepo: "registry.example.com/repo/metadata",
			wantMetaTag:  LatestTag,
			wantTgtRepo:  "registry.example.com/repo/targets",
			wantTgtTag:   LatestTag,
		},
		{
			name:        "invalid metadata URL",
			metadataURL: "oci://not valid",
			targetsURL:  "oci://registry.example.com/repo/targets",
			wantErr:     true,
		},
		{
			name:        "invalid targets URL",
			metadataURL: "oci://registry.example.com/repo/metadata",
			targetsURL:  "oci://not valid",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			fetcher, err := NewRegistryFetcher(ctx, tt.metadataURL, tt.targetsURL)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, fetcher)
			assert.Equal(t, tt.wantMetaRepo, fetcher.metadataRepo)
			assert.Equal(t, tt.wantMetaTag, fetcher.metadataTag)
			assert.Equal(t, tt.wantTgtRepo, fetcher.targetsRepo)
			assert.Equal(t, tt.wantTgtTag, fetcher.targetsTag)
			assert.NotNil(t, fetcher.cache)
		})
	}
}

func TestParseImgRef(t *testing.T) {
	ctx := context.Background()
	fetcher, err := NewRegistryFetcher(ctx,
		"oci://registry.example.com/repo/metadata:latest",
		"oci://registry.example.com/repo/targets:latest")
	require.NoError(t, err)

	tests := []struct {
		name         string
		urlPath      string
		wantImgRef   string
		wantFileName string
		wantErr      bool
	}{
		{
			name:         "top-level metadata - timestamp",
			urlPath:      "oci://registry.example.com/repo/metadata:latest/timestamp.json",
			wantImgRef:   "registry.example.com/repo/metadata:latest",
			wantFileName: "timestamp.json",
			wantErr:      false,
		},
		{
			name:         "top-level metadata - versioned root",
			urlPath:      "oci://registry.example.com/repo/metadata:latest/1.root.json",
			wantImgRef:   "registry.example.com/repo/metadata:latest",
			wantFileName: "1.root.json",
			wantErr:      false,
		},
		{
			name:         "delegated metadata",
			urlPath:      "oci://registry.example.com/repo/metadata:latest/role.json",
			wantImgRef:   "registry.example.com/repo/metadata:role",
			wantFileName: "role.json",
			wantErr:      false,
		},
		{
			name:         "top-level target",
			urlPath:      "oci://registry.example.com/repo/targets:latest/abc123.file.txt",
			wantImgRef:   "registry.example.com/repo/targets:abc123.file.txt",
			wantFileName: "abc123.file.txt",
			wantErr:      false,
		},
		{
			name:         "delegated target with subdir",
			urlPath:      "oci://registry.example.com/repo/targets:latest/subdir/file.txt",
			wantImgRef:   "registry.example.com/repo/targets:subdir",
			wantFileName: "subdir/file.txt",
			wantErr:      false,
		},
		{
			name:    "invalid URL - not in metadata or targets",
			urlPath: "oci://registry.example.com/other/file.txt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imgRef, fileName, err := fetcher.parseImgRef(tt.urlPath)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantImgRef, imgRef)
			assert.Equal(t, tt.wantFileName, fileName)
		})
	}
}

func TestIsDelegatedRole(t *testing.T) {
	tests := []struct {
		name string
		role string
		want bool
	}{
		{"root is not delegated", "root", false},
		{"timestamp is not delegated", "timestamp", false},
		{"snapshot is not delegated", "snapshot", false},
		{"targets is not delegated", "targets", false},
		{"custom role is delegated", "custom", true},
		{"opkl is delegated", "opkl", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDelegatedRole(tt.role)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRoleFromConsistentName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"unversioned root", "root.json", "root"},
		{"versioned root", "1.root.json", "root"},
		{"versioned root v2", "2.root.json", "root"},
		{"unversioned targets", "targets.json", "targets"},
		{"versioned targets", "1.targets.json", "targets"},
		{"delegated role", "opkl.json", "opkl"},
		{"versioned delegated", "1.opkl.json", "opkl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := roleFromConsistentName(tt.filename)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectOCI(t *testing.T) {
	tests := []struct {
		name        string
		metadataURL string
		targetsURL  string
		wantIsOCI   bool
		wantMeta    string
		wantTargets string
	}{
		{
			name:        "valid OCI URLs",
			metadataURL: "oci://registry.example.com/metadata:latest",
			targetsURL:  "oci://registry.example.com/targets:latest",
			wantIsOCI:   true,
			wantMeta:    "oci://registry.example.com/metadata:latest",
			wantTargets: "oci://registry.example.com/targets:latest",
		},
		{
			name:        "not OCI",
			metadataURL: "https://example.com/metadata",
			targetsURL:  "https://example.com/targets",
			wantIsOCI:   false,
			wantMeta:    "",
			wantTargets: "",
		},
		{
			name:        "OCI metadata but no targets",
			metadataURL: "oci://registry.example.com/metadata:latest",
			targetsURL:  "",
			wantIsOCI:   true,
			wantMeta:    "",
			wantTargets: "",
		},
		{
			name:        "OCI metadata but non-OCI targets",
			metadataURL: "oci://registry.example.com/metadata:latest",
			targetsURL:  "https://example.com/targets",
			wantIsOCI:   true,
			wantMeta:    "",
			wantTargets: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIsOCI, gotMeta, gotTargets := detectOCI(tt.metadataURL, tt.targetsURL)
			assert.Equal(t, tt.wantIsOCI, gotIsOCI)
			assert.Equal(t, tt.wantMeta, gotMeta)
			assert.Equal(t, tt.wantTargets, gotTargets)
		})
	}
}

func TestHasOCIScheme(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"valid OCI URL", "oci://registry.example.com/repo:tag", true},
		{"HTTP URL", "https://example.com", false},
		{"file URL", "file:///path/to/file", false},
		{"empty string", "", false},
		{"just oci", "oci", false},
		{"oci with slash", "oci:/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasOCIScheme(tt.url)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImageCache(t *testing.T) {
	cache := NewImageCache()
	require.NotNil(t, cache)

	// Test cache miss
	_, found := cache.Get("test-ref")
	assert.False(t, found)

	// Test cache put and get
	testData := []byte("test data")
	cache.Put("test-ref", testData)

	data, found := cache.Get("test-ref")
	assert.True(t, found)
	assert.Equal(t, testData, data)

	// Test multiple entries
	cache.Put("ref1", []byte("data1"))
	cache.Put("ref2", []byte("data2"))

	data1, found1 := cache.Get("ref1")
	assert.True(t, found1)
	assert.Equal(t, []byte("data1"), data1)

	data2, found2 := cache.Get("ref2")
	assert.True(t, found2)
	assert.Equal(t, []byte("data2"), data2)
}
