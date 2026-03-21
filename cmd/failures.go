package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seanhalberthal/gh-bench/internal/parser"
	"github.com/seanhalberthal/gh-bench/internal/runner"
	"github.com/spf13/cobra"
)

var failuresCmd = &cobra.Command{
	Use:   "failures",
	Short: "Surface structured errors from failed CI runs",
	Long:  "Fetch failed runs, identify the failing step, and extract structured errors using framework-aware parsers.",
	RunE:  runFailures,
}

func init() {
	failuresCmd.Flags().String("workflow", "", "Workflow filename or name")
	failuresCmd.Flags().String("runs", "", "Comma-separated list of run IDs")
	failuresCmd.Flags().Int("limit", 5, "Max failed runs to fetch")
	failuresCmd.Flags().String("branch", "", "Filter by branch")
	failuresCmd.Flags().Int("concurrency", 5, "Number of concurrent log fetchers")
}

func runFailures(cmd *cobra.Command, args []string) error {
	workflow, _ := cmd.Flags().GetString("workflow")
	runsFlag, _ := cmd.Flags().GetString("runs")
	limit, _ := cmd.Flags().GetInt("limit")
	branch, _ := cmd.Flags().GetString("branch")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	if workflow == "" && runsFlag == "" {
		return fmt.Errorf("either --workflow or --runs is required")
	}

	opts := runner.FetchOpts{
		Workflow:    workflow,
		Branch:      branch,
		Limit:       limit,
		Concurrency: concurrency,
		FailedOnly:  true,
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

	if jsonOutput {
		return printFailuresJSON(results)
	}
	return printFailuresText(results)
}

func printFailuresJSON(results []runner.RunResult) error {
	type failureOutput struct {
		RunID     int64             `json:"run_id"`
		Title     string            `json:"title"`
		Date      string            `json:"date"`
		Step      string            `json:"step"`
		Framework string            `json:"framework"`
		Failures  []parser.Failure  `json:"failures"`
	}

	var output []failureOutput
	for _, r := range results {
		for _, step := range r.FailedSteps {
			failures := parser.Parse(step.Log)
			fw := "unknown"
			if len(failures) > 0 {
				fw = failures[0].Framework
			}
			output = append(output, failureOutput{
				RunID:     r.RunID,
				Title:     r.Title,
				Date:      r.Date,
				Step:      step.Name,
				Framework: fw,
				Failures:  failures,
			})
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func printFailuresText(results []runner.RunResult) error {
	for i, r := range results {
		if i > 0 {
			fmt.Println(strings.Repeat("─", 68))
		}
		for _, step := range r.FailedSteps {
			failures := parser.Parse(step.Log)
			fw := "unknown"
			if len(failures) > 0 {
				fw = failures[0].Framework
			}

			fmt.Printf("● RUN %d — %s (%s)\n", r.RunID, r.Title, r.Date)
			fmt.Printf("  Step: %s\n", step.Name)
			fmt.Printf("  Framework: %s\n", fw)
			fmt.Println()

			if len(failures) == 0 {
				fmt.Println("  No structured failures extracted.")
				fmt.Println()
				continue
			}

			fmt.Printf("  Failed Tests (%d)\n\n", len(failures))
			for _, f := range failures {
				if f.Duration != "" {
					fmt.Printf("  ✗ %s [%s]\n", f.TestName, f.Duration)
				} else {
					fmt.Printf("  ✗ %s\n", f.TestName)
				}
				if f.Message != "" {
					for _, line := range strings.Split(f.Message, "\n") {
						fmt.Printf("      %s\n", line)
					}
				}
				if f.Location != "" {
					fmt.Printf("      at %s\n", f.Location)
				}
				fmt.Println()
			}
		}
	}
	return nil
}
