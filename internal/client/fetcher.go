package client

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// FilesystemFetcher implements fetcher.Fetcher for both HTTP and file:// URLs
type FilesystemFetcher struct {
	httpClient *http.Client
}

// NewFilesystemFetcher creates a new fetcher that supports file:// and http(s)://
func NewFilesystemFetcher() *FilesystemFetcher {
	return &FilesystemFetcher{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DownloadFile downloads a file from the provided URL, supporting both HTTP and file:// schemes
func (f *FilesystemFetcher) DownloadFile(urlPath string, maxLength int64, _ time.Duration) ([]byte, error) {
	parsedURL, err := url.Parse(urlPath)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme == "file" {
		// Local filesystem access
		filePath := parsedURL.Path
		data, err := os.ReadFile(filePath)
		if err != nil {
			// Return 404-like error for file not found (TUF client expects this)
			if errors.Is(err, os.ErrNotExist) {
				return nil, &metadata.ErrDownloadHTTP{StatusCode: 404, URL: urlPath}
			}
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		if maxLength > 0 && int64(len(data)) > maxLength {
			return nil, fmt.Errorf("file size %d exceeds max length %d", len(data), maxLength)
		}

		return data, nil
	}

	// HTTP(S) access
	req, err := http.NewRequest("GET", urlPath, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "tufzy/1.0")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &metadata.ErrDownloadHTTP{StatusCode: resp.StatusCode, URL: urlPath}
	}

	var reader io.Reader = resp.Body
	if maxLength > 0 {
		reader = io.LimitReader(resp.Body, maxLength+1) // +1 to detect if it exceeds
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	if maxLength > 0 && int64(len(data)) > maxLength {
		return nil, fmt.Errorf("response size exceeds max length %d", maxLength)
	}

	return data, nil
}
