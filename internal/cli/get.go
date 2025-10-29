package cli

import (
	"fmt"
	"path/filepath"

	"github.com/kipz/tufzy/internal/client"
	"github.com/kipz/tufzy/internal/display"
	"github.com/spf13/cobra"
)

var outputPath string

var getCmd = &cobra.Command{
	Use:   "get [metadata-url] [target-file]",
	Short: "Download and verify a target file",
	Long: `Download a specific target file from the TUF repository and verify its integrity.

Example:
  tufzy get https://example.github.io/repo/metadata myfile.txt
  tufzy get https://example.github.io/repo/metadata myfile.txt -o /path/to/output`,
	Args: cobra.ExactArgs(2),
	RunE: runGet,
}

func init() {
	getCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (default: current directory)")
}

func runGet(cmd *cobra.Command, args []string) error {
	metadataURL := args[0]
	targetName := args[1]

	// Determine output path
	destPath := outputPath
	if destPath == "" {
		destPath = filepath.Base(targetName)
	}

	// Create TUF client with options
	options := client.ClientOptions{
		TargetsURL: targetsURL,
	}
	tufClient, err := client.NewClientWithOptions(metadataURL, options)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Update metadata
	if err := tufClient.Update(); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	// Download target
	display.ShowDownloadStart(targetName, destPath)

	targetInfo, err := tufClient.DownloadTarget(targetName, destPath)
	if err != nil {
		display.ShowDownloadError(targetName, err)
		return err
	}

	display.ShowDownloadSuccess(targetName, destPath, targetInfo)

	return nil
}
