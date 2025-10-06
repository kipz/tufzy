package cli

import (
	"fmt"

	"github.com/kipz/tufzy/internal/client"
	"github.com/kipz/tufzy/internal/display"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [metadata-url]",
	Short: "List all available targets in the repository",
	Long: `List all available target files in the TUF repository.

The metadata-url can be an HTTP(S) URL or a local filesystem path.
Hash prefixes and tuf-on-ci git layout are auto-detected.

Examples:
  tufzy list https://example.github.io/repo/metadata
  tufzy list /path/to/local/repo/metadata
  tufzy list ./metadata`,
	Args: cobra.ExactArgs(1),
	RunE: runList,
}

func runList(cmd *cobra.Command, args []string) error {
	metadataURL := args[0]

	// Create TUF client (auto-detects settings)
	tufClient, err := client.NewClient(metadataURL)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Update metadata
	if err := tufClient.Update(); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	// Get targets
	targets, err := tufClient.GetTargets()
	if err != nil {
		return fmt.Errorf("failed to get targets: %w", err)
	}

	// Get repository info for display
	repoInfo, err := tufClient.GetRepositoryInfo()
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Display results
	display.ShowRepositoryHeader(repoInfo)
	display.ShowTargets(targets)

	return nil
}
