package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

// FetchOpts configures log fetching behaviour.
type FetchOpts struct {
	Workflow    string
	RunIDs      []int64
	Branch      string
	Limit       int
	Concurrency int
	FailedOnly  bool
	Step        string // Filter logs to a specific step name
}

// fetchRunOpts are per-run options passed to fetchSingleRun.
type fetchRunOpts struct {
	FailedOnly bool
	Step       string
}

// RunResult holds the output for a single workflow run.
type RunResult struct {
	RunID       int64
	Title       string
	Date        string
	Log         string
	FailedSteps []StepResult
}

// StepResult holds the output for a single failing step.
type StepResult struct {
	Name string
	Log  string
}

// GHExecutor abstracts gh CLI execution for testing.
type GHExecutor interface {
	Run(args ...string) (string, error)
}

// DefaultExecutor calls the real gh CLI.
type DefaultExecutor struct{}

// Run executes a gh command and returns its stdout.
func (d DefaultExecutor) Run(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh %s: %s: %w", strings.Join(args, " "), stderr.String(), err)
	}
	return stdout.String(), nil
}

// Executor is the package-level executor used for gh commands.
// Override in tests to stub external calls.
var Executor GHExecutor = DefaultExecutor{}

// FetchLogs retrieves logs for workflow runs concurrently.
func FetchLogs(ctx context.Context, opts FetchOpts) ([]RunResult, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 5
	}

	runIDs := opts.RunIDs
	if len(runIDs) == 0 {
		var err error
		runIDs, err = listRunIDs(opts)
		if err != nil {
			return nil, fmt.Errorf("listing runs: %w", err)
		}
	}

	if len(runIDs) == 0 {
		return nil, nil
	}

	results := make([]RunResult, len(runIDs))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Concurrency)

	runOpts := fetchRunOpts{FailedOnly: opts.FailedOnly, Step: opts.Step}
	for i, id := range runIDs {
		g.Go(func() error {
			result, err := fetchSingleRun(ctx, id, runOpts)
			if err != nil {
				return fmt.Errorf("run %d: %w", id, err)
			}
			results[i] = result
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

func listRunIDs(opts FetchOpts) ([]int64, error) {
	args := []string{"run", "list", "--limit", strconv.Itoa(opts.Limit)}

	if opts.Workflow != "" {
		args = append(args, "--workflow", opts.Workflow)
	}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	if opts.FailedOnly {
		args = append(args, "--status", "failure")
	}
	args = append(args, "--json", "databaseId", "--jq", ".[].databaseId")

	out, err := Executor.Run(args...)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		id, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing run ID %q: %w", line, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func fetchSingleRun(_ context.Context, runID int64, opts fetchRunOpts) (RunResult, error) {
	idStr := strconv.FormatInt(runID, 10)

	// Get run metadata
	metaOut, err := Executor.Run("run", "view", idStr, "--json", "displayTitle,createdAt", "--jq", ".displayTitle + \"\\t\" + .createdAt")
	if err != nil {
		return RunResult{}, fmt.Errorf("fetching metadata: %w", err)
	}

	parts := strings.SplitN(strings.TrimSpace(metaOut), "\t", 2)
	title := parts[0]
	date := ""
	if len(parts) > 1 {
		date = parts[1]
	}

	result := RunResult{
		RunID: runID,
		Title: title,
		Date:  date,
	}

	if opts.FailedOnly {
		// Get failed steps via jobs API
		steps, err := GetFailedSteps(runID)
		if err != nil {
			return result, fmt.Errorf("fetching failed steps: %w", err)
		}
		result.FailedSteps = steps
	} else if opts.Step != "" {
		log, err := getStepLog(runID, opts.Step)
		if err != nil {
			return result, fmt.Errorf("fetching step log: %w", err)
		}
		result.Log = log
	} else {
		// Get full log
		logOut, err := Executor.Run("run", "view", idStr, "--log")
		if err != nil {
			return result, fmt.Errorf("fetching log: %w", err)
		}
		result.Log = logOut
	}

	return result, nil
}

// getStepLog fetches the log for a specific step name within a run.
// It finds the first step matching the name (case-insensitive contains)
// across all jobs in the run.
func getStepLog(runID int64, stepName string) (string, error) {
	idStr := strconv.FormatInt(runID, 10)

	out, err := Executor.Run("run", "view", idStr, "--json", "jobs")
	if err != nil {
		return "", fmt.Errorf("fetching jobs: %w", err)
	}

	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return "", fmt.Errorf("parsing jobs JSON: %w", err)
	}

	lowerStep := strings.ToLower(stepName)
	for _, job := range result.Jobs {
		for _, step := range job.Steps {
			if strings.Contains(strings.ToLower(step.Name), lowerStep) {
				log, err := fetchJobLog(job.DatabaseID, runID)
				if err != nil {
					return "", err
				}
				return log, nil
			}
		}
	}

	return "", fmt.Errorf("step matching %q not found in run %d", stepName, runID)
}
