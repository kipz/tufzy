package client

import (
	"net/url"
	"path/filepath"
	"regexp"
	"time"
)

// TufOnCiFetcher wraps FilesystemFetcher and maps TUF versioned filenames
// to tuf-on-ci git repository layout (unversioned files)
type TufOnCiFetcher struct {
	*FilesystemFetcher
	metadataBaseURL string
}

// NewTufOnCiFetcher creates a fetcher for tuf-on-ci git repositories
func NewTufOnCiFetcher(metadataBaseURL string) *TufOnCiFetcher {
	return &TufOnCiFetcher{
		FilesystemFetcher: NewFilesystemFetcher(),
		metadataBaseURL:   metadataBaseURL,
	}
}

// DownloadFile maps TUF versioned filenames to tuf-on-ci git layout
func (f *TufOnCiFetcher) DownloadFile(urlPath string, maxLength int64, timeout time.Duration) ([]byte, error) {
	// Map the TUF request to tuf-on-ci layout
	mappedURL := f.mapTufOnCiURL(urlPath)

	// Use parent fetcher with mapped URL
	return f.FilesystemFetcher.DownloadFile(mappedURL, maxLength, timeout)
}

// mapTufOnCiURL converts TUF versioned URLs to tuf-on-ci git layout
//
// TUF expects:
//   - N.root.json (where N > 1) → metadata/root_history/N.root.json
//   - 1.root.json → metadata/1.root.json (initial root)
//   - N.snapshot.json → metadata/snapshot.json (always use current)
//   - N.timestamp.json → metadata/timestamp.json (always use current)
//   - N.targets.json → metadata/targets.json (always use current)
//   - delegated.json → metadata/delegated.json (no version prefix)
func (f *TufOnCiFetcher) mapTufOnCiURL(tufURL string) string {
	parsedURL, err := url.Parse(tufURL)
	if err != nil {
		return tufURL
	}

	// Extract filename from path
	filename := filepath.Base(parsedURL.Path)
	dirPath := filepath.Dir(parsedURL.Path)

	// Pattern: N.metadata.json where N is version number
	versionedPattern := regexp.MustCompile(`^(\d+)\.(root|snapshot|timestamp|targets)\.json$`)
	matches := versionedPattern.FindStringSubmatch(filename)

	if matches != nil {
		version := matches[1]
		role := matches[2]

		var newFilename string
		switch role {
		case "root":
			if version == "1" {
				// 1.root.json stays as is (initial root)
				newFilename = filename
			} else {
				// N.root.json → root_history/N.root.json
				dirPath = filepath.Join(dirPath, "root_history")
				newFilename = filename
			}
		case "snapshot", "timestamp", "targets":
			// N.snapshot.json → snapshot.json (use current, ignore version)
			newFilename = role + ".json"
		default:
			newFilename = filename
		}

		parsedURL.Path = filepath.Join(dirPath, newFilename)
		return parsedURL.String()
	}

	// Non-versioned files (delegated roles) pass through as-is
	return tufURL
}
