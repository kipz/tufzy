package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

const (
	// LatestTag is the default tag for metadata images
	LatestTag = "latest"
)

// Role represents a TUF role
type Role string

var Roles = []Role{metadata.ROOT, metadata.SNAPSHOT, metadata.TARGETS, metadata.TIMESTAMP}

// RegistryFetcher implements the TUF Fetcher interface for OCI registries.
type RegistryFetcher struct {
	metadataRepo string
	metadataTag  string
	targetsRepo  string
	targetsTag   string
	cache        *ImageCache
	timeout      time.Duration
	metadataURL  string // Original URL for parsing
	targetsURL   string // Original URL for parsing
}

// ImageCache provides in-memory caching for manifests and layers.
type ImageCache struct {
	cache map[string][]byte
}

// NewImageCache creates a new image cache.
func NewImageCache() *ImageCache {
	return &ImageCache{
		cache: make(map[string][]byte),
	}
}

// Get retrieves an image from cache.
func (c *ImageCache) Get(imgRef string) ([]byte, bool) {
	img, found := c.cache[imgRef]
	return img, found
}

// Put adds an image to cache.
func (c *ImageCache) Put(imgRef string, img []byte) {
	c.cache[imgRef] = img
}

// Layer represents an OCI layer with annotations.
type Layer struct {
	Annotations map[string]string `json:"annotations"`
	Digest      string            `json:"digest"`
}

// Layers represents an OCI manifest structure.
type Layers struct {
	Layers    []Layer `json:"layers"`
	Manifests []Layer `json:"manifests"`
	MediaType string  `json:"mediaType"`
}

// NewRegistryFetcher creates a new RegistryFetcher for downloading TUF metadata and targets from OCI registries.
// metadataURL and targetsURL should be in the format: oci://registry/repo:tag
func NewRegistryFetcher(_ context.Context, metadataURL, targetsURL string) (*RegistryFetcher, error) {
	// Strip oci:// prefix for parsing
	metadataURLStripped := strings.TrimPrefix(metadataURL, OCIScheme)
	targetsURLStripped := strings.TrimPrefix(targetsURL, OCIScheme)

	// Parse metadata reference
	metadataRef, err := reference.ParseNormalizedNamed(metadataURLStripped)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata repo: %w", err)
	}
	metadataTag := LatestTag
	if tag, ok := metadataRef.(reference.Tagged); ok {
		metadataTag = tag.Tag()
	}
	metadataRepo := metadataRef.Name()

	// Parse targets reference
	targetsRef, err := reference.ParseNormalizedNamed(targetsURLStripped)
	if err != nil {
		return nil, fmt.Errorf("failed to parse targets repo: %w", err)
	}
	targetsTag := LatestTag
	if tag, ok := targetsRef.(reference.Tagged); ok {
		targetsTag = tag.Tag()
	}
	targetsRepo := targetsRef.Name()

	return &RegistryFetcher{
		metadataRepo: metadataRepo,
		metadataTag:  metadataTag,
		targetsRepo:  targetsRepo,
		targetsTag:   targetsTag,
		cache:        NewImageCache(),
		metadataURL:  metadataURL,
		targetsURL:   targetsURL,
	}, nil
}

// DownloadFile downloads a file from an OCI registry, errors out if it failed,
// its length is larger than maxLength or the timeout is reached.
func (d *RegistryFetcher) DownloadFile(urlPath string, maxLength int64, timeout time.Duration) ([]byte, error) {
	d.timeout = timeout

	imgRef, fileName, err := d.parseImgRef(urlPath)
	if err != nil {
		return nil, err
	}

	// Get manifest for image or index
	mf, err := d.getManifest(imgRef)
	if err != nil {
		return nil, err
	}

	// Search image/index manifest for file
	hash, err := d.findFileInManifest(mf, fileName)
	if err != nil {
		// Return not found error compatible with go-tuf
		return nil, &metadata.ErrDownloadHTTP{StatusCode: http.StatusNotFound}
	}

	// Get file from layer
	parts := strings.Split(imgRef, ":")
	switch len(parts) {
	// default host port
	case 2:
		return d.pullFileLayer(fmt.Sprintf("%s@%s", parts[0], *hash), maxLength)
	// custom host port
	case 3:
		return d.pullFileLayer(fmt.Sprintf("%s:%s@%s", parts[0], parts[1], *hash), maxLength)
	default:
		return nil, fmt.Errorf("invalid image reference: %s", imgRef)
	}
}

// getManifest returns the manifest for an image or index.
func (d *RegistryFetcher) getManifest(ref string) ([]byte, error) {
	// Check cache for manifest
	if mf, found := d.cache.Get(ref); found {
		return mf, nil
	}

	// Pull image manifest
	mf, err := crane.Manifest(ref,
		crane.WithTransport(transportWithTimeout(d.timeout)),
		crane.WithAuth(authn.Anonymous),
		crane.WithAuthFromKeychain(MultiKeychainAll()))
	if err != nil {
		return nil, err
	}

	// Cache the manifest
	d.cache.Put(ref, mf)
	return mf, nil
}

// pullFileLayer pulls a layer for an image or index and returns its data.
func (d *RegistryFetcher) pullFileLayer(ref string, maxLength int64) ([]byte, error) {
	// Check cache for layer
	if data, found := d.cache.Get(ref); found {
		return data, nil
	}

	// Pull layer
	layer, err := crane.PullLayer(ref,
		crane.WithTransport(transportWithTimeout(d.timeout)),
		crane.WithAuth(authn.Anonymous),
		crane.WithAuthFromKeychain(MultiKeychainAll()))
	if err != nil {
		return nil, err
	}

	// Extract data from layer
	data, err := getDataFromLayer(layer, maxLength)
	if err != nil {
		return nil, err
	}

	// Cache the layer
	d.cache.Put(ref, data)
	return data, nil
}

// getDataFromLayer returns the data from a layer in an image.
func getDataFromLayer(fileLayer v1.Layer, maxLength int64) ([]byte, error) {
	length, err := fileLayer.Size()
	if err != nil {
		return nil, err
	}
	// Error if the reported size is greater than what is expected.
	if length > maxLength {
		return nil, &metadata.ErrDownloadLengthMismatch{Msg: fmt.Sprintf("download failed, length %d is larger than expected %d", length, maxLength)}
	}
	content, err := fileLayer.Uncompressed()
	if err != nil {
		return nil, err
	}
	defer content.Close()

	data, err := io.ReadAll(io.LimitReader(content, maxLength+1))
	if err != nil {
		return nil, err
	}
	// Error if the actual size is greater than what is expected.
	length = int64(len(data))
	if length > maxLength {
		return nil, &metadata.ErrDownloadLengthMismatch{Msg: fmt.Sprintf("download failed, length %d is larger than expected %d", length, maxLength)}
	}
	return data, nil
}

// parseImgRef parses a URL path to an image reference and file name.
func (d *RegistryFetcher) parseImgRef(urlPath string) (imgRef, fileName string, err error) {
	// Check if path is for targets
	if strings.HasPrefix(urlPath, d.targetsURL) {
		// determine if the target path contains subdirectories and set image name accordingly
		// <repo>/<filename>          -> image = <repo>:<filename>, layer = <filename>
		// <repo>/<subdir>/<filename> -> index = <repo>:<subdir>  , image = <filename> -> layer = <filename>
		target := strings.TrimPrefix(urlPath, d.targetsURL+"/")
		subdir, name, found := strings.Cut(target, "/")
		if found {
			return fmt.Sprintf("%s:%s", d.targetsRepo, subdir), fmt.Sprintf("%s/%s", subdir, name), nil
		}
		return fmt.Sprintf("%s:%s", d.targetsRepo, target), target, nil
	}

	// Check if path is for metadata
	if strings.HasPrefix(urlPath, d.metadataURL) {
		// build the metadata image name
		// determine if role is a delegated role and set the tag accordingly
		fileName = path.Base(urlPath)
		role := roleFromConsistentName(fileName)
		// if the role is a delegated role use the role name as the tag for imgRef
		if isDelegatedRole(role) {
			return fmt.Sprintf("%s:%s", d.metadataRepo, role), fileName, nil
		}
		return fmt.Sprintf("%s:%s", d.metadataRepo, d.metadataTag), fileName, nil
	}

	return "", "", fmt.Errorf("urlPath: %s must be in metadata or targets repo", urlPath)
}

// findFileInManifest searches the image or index manifest for a file with the given name and returns its digest.
func (d *RegistryFetcher) findFileInManifest(mf []byte, name string) (*v1.Hash, error) {
	var index bool

	// unmarshal manifest with annotations
	l := &Layers{}
	err := json.Unmarshal(mf, l)
	if err != nil {
		return nil, err
	}

	// determine image or index manifest
	var layers []Layer
	switch l.MediaType {
	case string(types.OCIImageIndex):
		layers = l.Manifests
		index = true
	case string(types.OCIManifestSchema1):
		layers = l.Layers
		index = false
	default:
		return nil, fmt.Errorf("invalid manifest media type: %s", l.MediaType)
	}

	// find annotation with file name
	var digest string
	for _, layer := range layers {
		if layer.Annotations[TUFFilenameAnnotation] == name {
			digest = layer.Digest
			break
		}
	}
	if digest == "" {
		return nil, fmt.Errorf("file %s not found in image", name)
	}

	// return layer digest as v1.Hash
	hash := new(v1.Hash)
	*hash, err = v1.NewHash(digest)
	if err != nil {
		return nil, err
	}

	// if index manifest pull image to get file layer
	if index {
		mf, err := d.getManifest(fmt.Sprintf("%s@%s", d.targetsRepo, *hash))
		if err != nil {
			return nil, err
		}
		parts := strings.Split(name, "/")
		return d.findFileInManifest(mf, parts[len(parts)-1])
	}
	return hash, nil
}

// transportWithTimeout returns a http.RoundTripper with a specified timeout.
func transportWithTimeout(timeout time.Duration) http.RoundTripper {
	// transport is based on go-containerregistry remote.DefaultTransport
	// with modifications to include a specified timeout
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: timeout,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   50,
	}
}

// isDelegatedRole returns true if the role is a delegated role.
func isDelegatedRole(role string) bool {
	for _, r := range Roles {
		if role == string(r) {
			return false // role is not a delegated role
		}
	}
	return true // role is a delegated role
}

// roleFromConsistentName returns the role name from a consistent snapshot file name.
func roleFromConsistentName(filename string) string {
	name := strings.TrimSuffix(filename, ".json")
	role := strings.Split(name, ".")
	if len(role) > 1 {
		return role[1]
	}
	return role[0]
}
