package cli

import (
	"fmt"

	"github.com/kipz/tufzy/internal/client"
	"github.com/kipz/tufzy/internal/display"
	"github.com/spf13/cobra"
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

func runDelegations(cmd *cobra.Command, args []string) error {
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

	// Get delegations
	delegations, err := tufClient.GetDelegations()
	if err != nil {
		return fmt.Errorf("failed to get delegations: %w", err)
	}

	// Display delegation tree
	display.ShowDelegations(delegations)

	return nil
}
