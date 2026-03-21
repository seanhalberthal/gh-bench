package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/seanhalberthal/gh-bench/internal/runner"
	"github.com/seanhalberthal/gh-bench/internal/stats"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Extract numeric values from CI run logs and aggregate",
	Long:  "Extract a numeric value from logs across multiple workflow runs and compute aggregations (median, mean, p95, min, max).",
	RunE:  runStats,
}

func init() {
	statsCmd.Flags().String("workflow", "", "Workflow filename or name")
	statsCmd.Flags().String("runs", "", "Comma-separated list of run IDs")
	statsCmd.Flags().String("pattern", "", "Regex with a named capture group")
	statsCmd.Flags().String("preset", "", "Use a built-in pattern preset (see --list-presets)")
	statsCmd.Flags().Bool("list-presets", false, "List available pattern presets and exit")
	statsCmd.Flags().Int("limit", 10, "Max number of runs to fetch")
	statsCmd.Flags().String("branch", "", "Filter runs by branch")
	statsCmd.Flags().String("agg", "median", "Aggregations: median, mean, p95, min, max (comma-separated)")
	statsCmd.Flags().Int("concurrency", 5, "Number of concurrent log fetchers")
}

func runStats(cmd *cobra.Command, args []string) error {
	listPresets, _ := cmd.Flags().GetBool("list-presets")
	if listPresets {
		return printPresets()
	}

	workflow, _ := cmd.Flags().GetString("workflow")
	runsFlag, _ := cmd.Flags().GetString("runs")
	patternFlag, _ := cmd.Flags().GetString("pattern")
	presetFlag, _ := cmd.Flags().GetString("preset")
	limit, _ := cmd.Flags().GetInt("limit")
	branch, _ := cmd.Flags().GetString("branch")
	aggFlag, _ := cmd.Flags().GetString("agg")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	pattern, err := resolvePatternFlag(patternFlag, presetFlag)
	if err != nil {
		return err
	}

	if workflow == "" && runsFlag == "" {
		return fmt.Errorf("either --workflow or --runs is required")
	}

	opts := runner.FetchOpts{
		Workflow:    workflow,
		Branch:      branch,
		Limit:       limit,
		Concurrency: concurrency,
	}

	if runsFlag != "" {
		ids, err := parseRunIDs(runsFlag)
		if err != nil {
			return err
		}
		opts.RunIDs = ids
	}

	results, err := runner.FetchLogs(cmd.Context(), opts)
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}

	values, err := runner.ExtractValues(results, pattern)
	if err != nil {
		return fmt.Errorf("extracting values: %w", err)
	}

	if len(values) == 0 {
		fmt.Fprintf(os.Stderr, "warning: no values matched pattern %q across %d runs\n", pattern, len(results))
		fmt.Fprintf(os.Stderr, "hint: check your regex has a named capture group (?P<name>...) and matches your log output\n")
		return nil
	}

	aggs := strings.Split(aggFlag, ",")
	aggResults := stats.Compute(values.Numbers(), aggs)

	if jsonOutput {
		return printStatsJSON(values, aggResults)
	}
	return printStatsTable(values, aggResults)
}

func parseRunIDs(s string) ([]int64, error) {
	parts := strings.Split(s, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid run ID %q: %w", p, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func printStatsJSON(values runner.ExtractedValues, aggs map[string]float64) error {
	fmt.Println("{")
	fmt.Println("  \"runs\": [")
	for i, v := range values {
		comma := ","
		if i == len(values)-1 {
			comma = ""
		}
		fmt.Printf("    {\"run_id\": %d, \"title\": %q, \"value\": %s}%s\n", v.RunID, v.Title, v.Raw, comma)
	}
	fmt.Println("  ],")
	fmt.Println("  \"aggregations\": {")
	keys := make([]string, 0, len(aggs))
	for k := range aggs {
		keys = append(keys, k)
	}
	for i, k := range keys {
		comma := ","
		if i == len(keys)-1 {
			comma = ""
		}
		fmt.Printf("    %q: %.2f%s\n", k, aggs[k], comma)
	}
	fmt.Println("  }")
	fmt.Println("}")
	return nil
}

func printStatsTable(values runner.ExtractedValues, aggs map[string]float64) error {
	fmt.Printf("%-15s %-35s %s\n", "RUN ID", "TITLE", "VALUE")
	for _, v := range values {
		fmt.Printf("%-15d %-35s %s\n", v.RunID, truncate(v.Title, 35), v.Raw)
	}
	fmt.Println(strings.Repeat("─", 60))

	parts := make([]string, 0, len(aggs))
	for k, v := range aggs {
		parts = append(parts, fmt.Sprintf("%s: %.1f", k, v))
	}
	fmt.Println(strings.Join(parts, "  "))
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func resolvePatternFlag(pattern, preset string) (string, error) {
	if pattern != "" && preset != "" {
		return "", fmt.Errorf("--pattern and --preset are mutually exclusive")
	}
	if pattern == "" && preset == "" {
		return "", fmt.Errorf("either --pattern or --preset is required")
	}
	if preset != "" {
		return runner.ResolvePattern(preset)
	}
	return pattern, nil
}

func printPresets() error {
	fmt.Println("Available pattern presets:")
	fmt.Println()
	for _, name := range runner.PresetNames() {
		p := runner.Presets[name]
		fmt.Printf("  %-14s %s\n", name, p.Description)
		fmt.Printf("  %-14s pattern: %s\n", "", p.Pattern)
		fmt.Printf("  %-14s e.g.    %s\n", "", p.Example)
		fmt.Println()
	}
	return nil
}
