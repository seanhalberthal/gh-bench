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

// enrichedStep holds a step result with pre-parsed failure data.
type enrichedStep struct {
	runner.StepResult
	Failures  []parser.Failure
	Framework string
}

// enrichedRun holds a run result with pre-parsed steps.
type enrichedRun struct {
	runner.RunResult
	Steps []enrichedStep
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

	results, err := withSpinner("Fetching failed run logs…", func() ([]runner.RunResult, error) {
		return runner.FetchLogs(cmd.Context(), opts)
	})
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

	// Parse each step log exactly once and build enriched results.
	enriched := make([]enrichedRun, len(results))
	totalFailures := 0
	for ri, r := range results {
		er := enrichedRun{RunResult: r, Steps: make([]enrichedStep, len(r.FailedSteps))}
		for si, step := range r.FailedSteps {
			failures := parser.Parse(step.Log)
			fw := "unknown"
			if len(failures) > 0 {
				fw = failures[0].Framework
			}
			er.Steps[si] = enrichedStep{
				StepResult: step,
				Failures:   failures,
				Framework:  fw,
			}
			totalFailures += len(failures)
		}
		enriched[ri] = er
	}

	if totalFailures == 0 {
		fmt.Fprintf(os.Stderr, "warning: %d failed runs found but no structured failures extracted (framework not detected?)\n", len(results))
	}

	if groupFlag {
		runFailures := make(map[int64][]parser.Failure)
		for _, er := range enriched {
			for _, step := range er.Steps {
				runFailures[er.RunID] = append(runFailures[er.RunID], step.Failures...)
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
		return printFailuresJSON(enriched)
	case "csv":
		return printFailuresCSV(enriched)
	default:
		return printFailuresText(enriched)
	}
}

func printFailuresJSON(enriched []enrichedRun) error {
	type failureOutput struct {
		RunID     int64            `json:"run_id"`
		Title     string           `json:"title"`
		Date      string           `json:"date"`
		Step      string           `json:"step"`
		Framework string           `json:"framework"`
		Failures  []parser.Failure `json:"failures"`
	}

	var output []failureOutput
	for _, r := range enriched {
		for _, step := range r.Steps {
			output = append(output, failureOutput{
				RunID:     r.RunID,
				Title:     r.Title,
				Date:      r.Date,
				Step:      step.Name,
				Framework: step.Framework,
				Failures:  step.Failures,
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

func printFailuresCSV(enriched []enrichedRun) error {
	fmt.Println("run_id,title,date,step,framework,test_name,message,location")
	for _, r := range enriched {
		for _, step := range r.Steps {
			for _, f := range step.Failures {
				fmt.Printf("%d,%q,%q,%q,%q,%q,%q,%q\n",
					r.RunID, r.Title, r.Date, step.Name,
					step.Framework, f.TestName, f.Message, f.Location)
			}
		}
	}
	return nil
}

func printFailuresText(enriched []enrichedRun) error {
	var b strings.Builder

	for i, r := range enriched {
		if i > 0 {
			b.WriteString(strings.Repeat("─", 68))
			b.WriteByte('\n')
		}
		for _, step := range r.Steps {
			fmt.Fprintf(&b, "● RUN %d — %s (%s)\n", r.RunID, r.Title, r.Date)
			fmt.Fprintf(&b, "  Step: %s\n", step.Name)
			fmt.Fprintf(&b, "  Framework: %s\n\n", step.Framework)

			if len(step.Failures) == 0 {
				b.WriteString("  No structured failures extracted.\n\n")
				continue
			}

			fmt.Fprintf(&b, "  Failed Tests (%d)\n\n", len(step.Failures))
			for _, f := range step.Failures {
				if f.Duration != "" {
					fmt.Fprintf(&b, "  ✗ %s [%s]\n", f.TestName, f.Duration)
				} else {
					fmt.Fprintf(&b, "  ✗ %s\n", f.TestName)
				}
				if f.Message != "" {
					for line := range strings.SplitSeq(f.Message, "\n") {
						fmt.Fprintf(&b, "      %s\n", line)
					}
				}
				if f.Location != "" {
					fmt.Fprintf(&b, "      at %s\n", f.Location)
				}
				b.WriteByte('\n')
			}
		}
	}

	_, err := os.Stdout.WriteString(b.String())
	return err
}

func printGroupedText(groups []parser.FailureGroup, totalRuns int) error {
	if len(groups) == 0 {
		fmt.Println("No failures found.")
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Failure groups across %d runs (%d unique)\n\n", totalRuns, len(groups))

	for _, g := range groups {
		fmt.Fprintf(&b, "  ✗ %s  [%d/%d runs]\n", g.TestName, g.Count, totalRuns)
		if g.Message != "" {
			for line := range strings.SplitSeq(g.Message, "\n") {
				fmt.Fprintf(&b, "      %s\n", line)
			}
		}
		if g.Location != "" {
			fmt.Fprintf(&b, "      at %s\n", g.Location)
		}
		fmt.Fprintf(&b, "      framework: %s\n\n", g.Framework)
	}

	_, err := os.Stdout.WriteString(b.String())
	return err
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
