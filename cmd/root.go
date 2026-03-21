package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "bench",
	Short: "CI benchmarking and failure extraction for GitHub Actions",
	Long:  "gh bench extracts numeric values from CI run logs, aggregates stats, and surfaces structured errors from failed runs.",
	Annotations: map[string]string{
		cobra.CommandDisplayNameAnnotation: "gh bench",
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(failuresCmd)

	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of gh bench",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gh bench %s\n", Version)
		},
	}
}

func init() {
	rootCmd.AddCommand(versionCmd())
}
