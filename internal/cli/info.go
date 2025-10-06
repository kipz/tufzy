package cli

import (
	"fmt"

	"github.com/kipz/tufzy/internal/client"
	"github.com/kipz/tufzy/internal/display"
	"github.com/spf13/cobra"
)

var (
	infoHashPrefix  bool
	infoTufOnCiGit bool
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

func init() {
	infoCmd.Flags().BoolVar(&infoHashPrefix, "hash-prefix", false, "Target paths include hash prefixes")
	infoCmd.Flags().BoolVar(&infoTufOnCiGit, "tuf-on-ci-git", false, "Use tuf-on-ci git repository layout")
}

func runInfo(cmd *cobra.Command, args []string) error {
	metadataURL := args[0]

	// Create TUF client with options
	opts := client.ClientOptions{
		PrefixTargetsWithHash: infoHashPrefix,
		TufOnCiGit:            infoTufOnCiGit,
	}
	tufClient, err := client.NewClientWithOptions(metadataURL, opts)
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
