package runner

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

	var failedSteps []StepResult
	for _, job := range result.Jobs {
		// Process failed and cancelled jobs — a cancelled job can still
		// contain steps that failed before the cancellation (e.g. fail-fast matrix).
		if job.Conclusion != "failure" && job.Conclusion != "cancelled" {
			continue
		}

		// Fetch the raw log for this job via the REST API.
		// This is ~2x faster than gh run view --log and produces clean
		// output without "UNKNOWN STEP" artifacts.
		jobLog, err := fetchJobLog(job.DatabaseID, runID)
		if err != nil {
			return nil, fmt.Errorf("fetching log for job %d: %w", job.DatabaseID, err)
		}

		for _, step := range job.Steps {
			// Include failed steps and cancelled steps (a cancelled step may
			// have produced test failure output before being interrupted).
			if step.Conclusion != "failure" && step.Conclusion != "cancelled" {
				continue
			}
			if shouldSkipStep(step.Name) {
				continue
			}
			failedSteps = append(failedSteps, StepResult{
				Name: step.Name,
				Log:  jobLog,
			})
		}
	}

	return failedSteps, nil
}

// fetchJobLog retrieves the raw log for a specific job, trying the REST API
// first (faster, cleaner output) then falling back to gh run view --log.
func fetchJobLog(jobID, runID int64) (string, error) {
	// Try REST API: GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs
	// Returns plain text — no tab-prefixed formatting.
	log, err := Executor.Run("api", "repos/{owner}/{repo}/actions/jobs/"+strconv.FormatInt(jobID, 10)+"/logs")
	if err == nil {
		return log, nil
	}

	// Fallback: gh run view --log --job (slower, adds job\tstep\t prefixes).
	idStr := strconv.FormatInt(runID, 10)
	log, err = Executor.Run("run", "view", idStr, "--log", "--job", strconv.FormatInt(jobID, 10))
	if err != nil {
		return "", fmt.Errorf("fetching log: %w", err)
	}

	// Strip the tab-delimited prefixes so parsers get clean content.
	return stripLogPrefixes(log), nil
}

// stripLogPrefixes removes job\tstep\t prefixes from gh run view --log output.
func stripLogPrefixes(log string) string {
	lines := strings.Split(log, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		first := strings.IndexByte(line, '\t')
		if first < 0 {
			out = append(out, line)
			continue
		}
		rest := line[first+1:]
		second := strings.IndexByte(rest, '\t')
		if second < 0 {
			out = append(out, line)
			continue
		}
		out = append(out, rest[second+1:])
	}
	return strings.Join(out, "\n")
}
