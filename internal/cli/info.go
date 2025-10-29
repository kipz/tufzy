package cli

import (
	"fmt"

	"github.com/kipz/tufzy/internal/client"
	"github.com/kipz/tufzy/internal/display"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info [metadata-url]",
	Short: "Show repository information",
	Long: `Display detailed information about the TUF repository including metadata versions,
expiry dates, and role information.

Example:
  tufzy info https://example.github.io/repo/metadata`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func runInfo(cmd *cobra.Command, args []string) error {
	metadataURL := args[0]

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

	// Get repository info
	repoInfo, err := tufClient.GetRepositoryInfo()
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Display repository information
	display.ShowRepositoryInfo(repoInfo)

	return nil
}
