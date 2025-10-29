package cli

import (
	"github.com/spf13/cobra"
)

var (
	targetsURL string
)

var rootCmd = &cobra.Command{
	Use:   "tufzy",
	Short: "🎯 A friendly TUF client with pretty output",
	Long: `tufzy is a command-line client for The Update Framework (TUF) repositories.
It provides an easy-to-use interface for verifying and downloading files from TUF repositories,
with colorful output and helpful emojis!

Supports HTTP(S), local filesystem, and OCI registry sources.`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&targetsURL, "targets-url", "", "Targets repository URL (required for OCI registries)")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(delegationsCmd)
}
