package cli

import (
	"fmt"

	"github.com/kipz/tufzy/internal/client"
	"github.com/kipz/tufzy/internal/display"
	"github.com/spf13/cobra"
)

var (
	delegationsHashPrefix  bool
	delegationsTufOnCiGit bool
)

var delegationsCmd = &cobra.Command{
	Use:   "delegations [metadata-url]",
	Short: "Show the delegation tree",
	Long: `Display the delegation tree showing how trust is delegated across different roles.

Example:
  tufzy delegations https://example.github.io/repo/metadata`,
	Args: cobra.ExactArgs(1),
	RunE: runDelegations,
}

func init() {
	delegationsCmd.Flags().BoolVar(&delegationsHashPrefix, "hash-prefix", false, "Target paths include hash prefixes")
	delegationsCmd.Flags().BoolVar(&delegationsTufOnCiGit, "tuf-on-ci-git", false, "Use tuf-on-ci git repository layout")
}

func runDelegations(cmd *cobra.Command, args []string) error {
	metadataURL := args[0]

	// Create TUF client with options
	opts := client.ClientOptions{
		PrefixTargetsWithHash: delegationsHashPrefix,
		TufOnCiGit:            delegationsTufOnCiGit,
	}
	tufClient, err := client.NewClientWithOptions(metadataURL, opts)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Update metadata
	if err := tufClient.Update(); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	// Get delegations
	delegations, err := tufClient.GetDelegations()
	if err != nil {
		return fmt.Errorf("failed to get delegations: %w", err)
	}

	// Display delegation tree
	display.ShowDelegations(delegations)

	return nil
}
