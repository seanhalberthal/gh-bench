package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/mattn/go-isatty"
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

	rootCmd.PersistentFlags().StringP("format", "f", "table", "Output format: table, json, csv")
	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON (shorthand for --format json)")
	_ = rootCmd.PersistentFlags().MarkHidden("json")
}

// resolveFormat returns the output format from --format / --json flags.
func resolveFormat(cmd *cobra.Command) string {
	format, _ := cmd.Flags().GetString("format")
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		return "json"
	}
	return format
}

// startSpinner creates and starts a stderr spinner if stderr is a terminal.
// Returns a stop function that is always safe to call.
func startSpinner(suffix string) func() {
	if !isatty.IsTerminal(os.Stderr.Fd()) && !isatty.IsCygwinTerminal(os.Stderr.Fd()) {
		return func() {}
	}
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
	s.Suffix = " " + suffix
	s.Start()
	return func() { s.Stop() }
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
