package cmd

import (
	"encoding/json"
	"fmt"
	"os"
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
	failuresCmd.Flags().StringP("workflow", "w", "", "Workflow filename or name")
	failuresCmd.Flags().StringP("runs", "r", "", "Comma-separated list of run IDs")
	failuresCmd.Flags().IntP("limit", "l", 5, "Max failed runs to fetch")
	failuresCmd.Flags().StringP("branch", "b", "", "Filter by branch")
	failuresCmd.Flags().IntP("concurrency", "c", 5, "Number of concurrent log fetchers")
	failuresCmd.Flags().BoolP("group", "g", false, "Group identical failures across runs")
}

func runFailures(cmd *cobra.Command, args []string) error {
	workflow, _ := cmd.Flags().GetString("workflow")
	runsFlag, _ := cmd.Flags().GetString("runs")
	limit, _ := cmd.Flags().GetInt("limit")
	branch, _ := cmd.Flags().GetString("branch")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	groupFlag, _ := cmd.Flags().GetBool("group")

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

	stop := startSpinner("Fetching failed run logs…")
	results, err := runner.FetchLogs(cmd.Context(), opts)
	stop()
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}

	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "warning: no failed runs found")
		if workflow != "" {
			fmt.Fprintf(os.Stderr, " for workflow %q", workflow)
		}
		if branch != "" {
			fmt.Fprintf(os.Stderr, " on branch %q", branch)
		}
		fmt.Fprintln(os.Stderr)
		return nil
	}

	totalFailures := 0
	for _, r := range results {
		for _, step := range r.FailedSteps {
			totalFailures += len(parser.Parse(step.Log))
		}
	}
	if totalFailures == 0 {
		fmt.Fprintf(os.Stderr, "warning: %d failed runs found but no structured failures extracted (framework not detected?)\n", len(results))
	}

	if groupFlag {
		runFailures := make(map[int64][]parser.Failure)
		for _, r := range results {
			for _, step := range r.FailedSteps {
				failures := parser.Parse(step.Log)
				runFailures[r.RunID] = append(runFailures[r.RunID], failures...)
			}
		}
		groups := parser.GroupFailures(runFailures)
		switch resolveFormat(cmd) {
		case "json":
			return printGroupedJSON(groups, len(results))
		default:
			return printGroupedText(groups, len(results))
		}
	}

	switch resolveFormat(cmd) {
	case "json":
		return printFailuresJSON(results)
	case "csv":
		return printFailuresCSV(results)
	default:
		return printFailuresText(results)
	}
}

func printFailuresJSON(results []runner.RunResult) error {
	type failureOutput struct {
		RunID     int64            `json:"run_id"`
		Title     string           `json:"title"`
		Date      string           `json:"date"`
		Step      string           `json:"step"`
		Framework string           `json:"framework"`
		Failures  []parser.Failure `json:"failures"`
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

func printFailuresCSV(results []runner.RunResult) error {
	fmt.Println("run_id,title,date,step,framework,test_name,message,location")
	for _, r := range results {
		for _, step := range r.FailedSteps {
			failures := parser.Parse(step.Log)
			fw := "unknown"
			if len(failures) > 0 {
				fw = failures[0].Framework
			}
			for _, f := range failures {
				fmt.Printf("%d,%q,%q,%q,%q,%q,%q,%q\n",
					r.RunID, r.Title, r.Date, step.Name,
					fw, f.TestName, f.Message, f.Location)
			}
		}
	}
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
					for line := range strings.SplitSeq(f.Message, "\n") {
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

func printGroupedText(groups []parser.FailureGroup, totalRuns int) error {
	if len(groups) == 0 {
		fmt.Println("No failures found.")
		return nil
	}

	fmt.Printf("Failure groups across %d runs (%d unique)\n\n", totalRuns, len(groups))

	for _, g := range groups {
		fmt.Printf("  ✗ %s  [%d/%d runs]\n", g.TestName, g.Count, totalRuns)
		if g.Message != "" {
			for line := range strings.SplitSeq(g.Message, "\n") {
				fmt.Printf("      %s\n", line)
			}
		}
		if g.Location != "" {
			fmt.Printf("      at %s\n", g.Location)
		}
		fmt.Printf("      framework: %s\n", g.Framework)
		fmt.Println()
	}
	return nil
}

func printGroupedJSON(groups []parser.FailureGroup, totalRuns int) error {
	output := struct {
		TotalRuns int                   `json:"total_runs"`
		Groups    []parser.FailureGroup `json:"groups"`
	}{
		TotalRuns: totalRuns,
		Groups:    groups,
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
