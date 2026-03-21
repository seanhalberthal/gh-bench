package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

// FetchOpts configures log fetching behaviour.
type FetchOpts struct {
	Workflow    string
	RunIDs     []int64
	Branch     string
	Limit      int
	Concurrency int
	FailedOnly bool
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

	for i, id := range runIDs {
		i, id := i, id
		g.Go(func() error {
			result, err := fetchSingleRun(ctx, id, opts.FailedOnly)
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
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
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

func fetchSingleRun(ctx context.Context, runID int64, failedOnly bool) (RunResult, error) {
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

	if failedOnly {
		// Get failed steps via jobs API
		steps, err := GetFailedSteps(runID)
		if err != nil {
			return result, fmt.Errorf("fetching failed steps: %w", err)
		}
		result.FailedSteps = steps
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
