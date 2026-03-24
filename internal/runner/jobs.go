package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

// Job represents a GitHub Actions job.
type Job struct {
	DatabaseID int64  `json:"databaseId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Steps      []Step `json:"steps"`
}

// skipStepNames are GitHub Actions infrastructure steps that never contain test output.
var skipStepNames = []string{
	"Set up job",
	"Complete job",
	"Post ",
	"Initialize containers",
	"Stop containers",
}

// shouldSkipStep returns true for GitHub Actions infrastructure steps
// that never contain test output.
func shouldSkipStep(name string) bool {
	for _, s := range skipStepNames {
		if name == s || strings.HasPrefix(name, s) {
			return true
		}
	}
	return false
}

// Step represents a GitHub Actions job step.
type Step struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Number     int    `json:"number"`
}

// GetFailedSteps retrieves the failed steps for a workflow run and fetches their logs.
func GetFailedSteps(runID int64) ([]StepResult, error) {
	idStr := strconv.FormatInt(runID, 10)

	out, err := Executor.Run("run", "view", idStr, "--json", "jobs")
	if err != nil {
		return nil, fmt.Errorf("fetching jobs: %w", err)
	}

	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parsing jobs JSON: %w", err)
	}

	// Collect failed/cancelled jobs that need log fetches.
	type jobSteps struct {
		job   Job
		steps []Step
	}
	var toFetch []jobSteps
	for _, job := range result.Jobs {
		if job.Conclusion != "failure" && job.Conclusion != "cancelled" {
			continue
		}
		var failed []Step
		for _, step := range job.Steps {
			if step.Conclusion != "failure" {
				continue
			}
			if shouldSkipStep(step.Name) {
				continue
			}
			failed = append(failed, step)
		}
		if len(failed) > 0 {
			toFetch = append(toFetch, jobSteps{job: job, steps: failed})
		}
	}

	if len(toFetch) == 0 {
		return nil, nil
	}

	// Fetch job logs in parallel.
	logs := make([]string, len(toFetch))
	g, _ := errgroup.WithContext(context.Background())
	for i, js := range toFetch {
		g.Go(func() error {
			log, err := fetchJobLog(js.job.DatabaseID, runID)
			if err != nil {
				return fmt.Errorf("fetching log for job %d: %w", js.job.DatabaseID, err)
			}
			logs[i] = log
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Assemble step results.
	var failedSteps []StepResult
	for i, js := range toFetch {
		for _, step := range js.steps {
			failedSteps = append(failedSteps, StepResult{
				Name: step.Name,
				Log:  logs[i],
			})
		}
	}

	return failedSteps, nil
}

// fetchJobLog retrieves the raw log for a specific job, trying the REST API
// first (faster, cleaner output) then falling back to gh run view --log.
func fetchJobLog(jobID, runID int64) (string, error) {
	// Try REST API: GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs
	// Returns plain text — no tab-prefixed formatting, but still has timestamps.
	log, err := Executor.Run("api", "repos/{owner}/{repo}/actions/jobs/"+strconv.FormatInt(jobID, 10)+"/logs")
	if err == nil {
		return stripTimestamps(log), nil
	}

	// Fallback: gh run view --log --job (slower, adds job\tstep\t prefixes).
	idStr := strconv.FormatInt(runID, 10)
	log, err = Executor.Run("run", "view", idStr, "--log", "--job", strconv.FormatInt(jobID, 10))
	if err != nil {
		return "", fmt.Errorf("fetching log: %w", err)
	}

	// Strip tab-delimited prefixes and timestamps so parsers get clean content.
	return stripLogPrefixes(log), nil
}

// timestampRe matches the GitHub Actions log timestamp prefix.
// Format: 2026-03-16T13:34:37.3465175Z (ISO 8601 with fractional seconds).
var timestampRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z `)

// stripTimestamp removes a GitHub Actions timestamp prefix from a single line.
func stripTimestamp(line string) string {
	if loc := timestampRe.FindStringIndex(line); loc != nil {
		return line[loc[1]:]
	}
	return line
}

// stripTimestamps removes GitHub Actions timestamp prefixes from all lines.
func stripTimestamps(log string) string {
	var b strings.Builder
	b.Grow(len(log))

	for line := range strings.SplitSeq(log, "\n") {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(stripTimestamp(line))
	}

	return b.String()
}

// stripLogPrefixes removes job\tstep\t prefixes and GitHub Actions timestamps
// from gh run view --log output.
func stripLogPrefixes(log string) string {
	var b strings.Builder
	b.Grow(len(log))

	for line := range strings.SplitSeq(log, "\n") {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}

		first := strings.IndexByte(line, '\t')
		if first < 0 {
			b.WriteString(stripTimestamp(line))
			continue
		}
		rest := line[first+1:]
		second := strings.IndexByte(rest, '\t')
		if second < 0 {
			b.WriteString(stripTimestamp(line))
			continue
		}
		b.WriteString(stripTimestamp(rest[second+1:]))
	}

	return b.String()
}
