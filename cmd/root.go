package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/seanhalberthal/gh-bench/internal/config"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

// cfg holds the project-level config loaded from .gh-bench.yml.
var cfg config.Config

var rootCmd = &cobra.Command{
	Use:   "bench",
	Short: "CI benchmarking and failure extraction for GitHub Actions",
	Long:  "gh bench extracts numeric values from CI run logs, aggregates stats, and surfaces structured errors from failed runs.",
	Annotations: map[string]string{
		cobra.CommandDisplayNameAnnotation: "gh bench",
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		return err
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

// withSpinner runs fn while displaying a terminal spinner with the given title.
// The spinner renders to stderr so it doesn't interfere with command output.
func withSpinner[T any](title string, fn func() (T, error)) (T, error) {
	var result T
	var fnErr error
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	err := spinner.New().
		Title(title).
		Type(spinner.Dots).
		Style(style).
		Action(func() { result, fnErr = fn() }).
		Run()
	if fnErr != nil {
		return result, fnErr
	}
	return result, err
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
