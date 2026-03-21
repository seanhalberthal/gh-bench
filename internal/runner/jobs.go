package runner

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Job represents a GitHub Actions job.
type Job struct {
	ID         int64  `json:"id"`
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
		if job.Conclusion != "failure" {
			continue
		}

		// Fetch the log for this job
		logOut, err := Executor.Run("run", "view", idStr, "--log", "--job", strconv.FormatInt(job.ID, 10))
		if err != nil {
			// If we can't get per-job logs, try the full run log
			logOut, err = Executor.Run("run", "view", idStr, "--log")
			if err != nil {
				return nil, fmt.Errorf("fetching log for job %d: %w", job.ID, err)
			}
		}

		for _, step := range job.Steps {
			if step.Conclusion != "failure" {
				continue
			}
			if shouldSkipStep(step.Name) {
				continue
			}
			stepLog := extractStepLog(logOut, job.Name, step.Name)
			failedSteps = append(failedSteps, StepResult{
				Name: step.Name,
				Log:  stepLog,
			})
		}
	}

	return failedSteps, nil
}

// extractStepLog extracts the log section for a specific step from the full job log.
// gh run view --log prefixes lines with the job and step name: "jobName\tstepName\tcontent".
// The job name in log output uses the YAML key (e.g. "test-summary"), which may differ
// from the display name returned by the jobs API (e.g. "Test Summary"). We try exact
// match first, then fall back to matching just the step name across all log prefixes.
func extractStepLog(fullLog, jobName, stepName string) string {
	logLines := strings.Split(fullLog, "\n")

	// Try exact prefix match first.
	prefix := jobName + "\t" + stepName + "\t"
	if lines := linesWithPrefix(logLines, prefix); len(lines) > 0 {
		return strings.Join(lines, "\n")
	}

	// The job name from the API may not match the log prefix (YAML key vs display name).
	// Discover all unique job\tstep prefixes and match by step name.
	for _, p := range discoverPrefixes(logLines) {
		parts := strings.SplitN(p, "\t", 2)
		if len(parts) == 2 && parts[1] == stepName {
			if lines := linesWithPrefix(logLines, p+"\t"); len(lines) > 0 {
				return strings.Join(lines, "\n")
			}
		}
	}

	// Case-insensitive step name match as last attempt.
	lower := strings.ToLower(stepName)
	for _, p := range discoverPrefixes(logLines) {
		parts := strings.SplitN(p, "\t", 2)
		if len(parts) == 2 && strings.ToLower(parts[1]) == lower {
			if lines := linesWithPrefix(logLines, p+"\t"); len(lines) > 0 {
				return strings.Join(lines, "\n")
			}
		}
	}

	// Absolute fallback: strip all job\tstep\t prefixes so parsers get clean content.
	return stripAllPrefixes(logLines)
}

// stripAllPrefixes removes the job\tstep\t prefix from every line in the log,
// returning just the content portion. Lines without the expected tab structure
// are kept as-is.
func stripAllPrefixes(logLines []string) string {
	out := make([]string, 0, len(logLines))
	for _, line := range logLines {
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

// linesWithPrefix returns log content lines that match the given prefix, with the prefix stripped.
func linesWithPrefix(logLines []string, prefix string) []string {
	var out []string
	for _, line := range logLines {
		if strings.HasPrefix(line, prefix) {
			out = append(out, strings.TrimPrefix(line, prefix))
		}
	}
	return out
}

// discoverPrefixes scans log output and returns all unique "job\tstep" prefixes found.
func discoverPrefixes(logLines []string) []string {
	seen := make(map[string]struct{})
	var prefixes []string
	for _, line := range logLines {
		// Format: jobName\tstepName\tcontent
		first := strings.IndexByte(line, '\t')
		if first < 0 {
			continue
		}
		second := strings.IndexByte(line[first+1:], '\t')
		if second < 0 {
			continue
		}
		key := line[:first+1+second]
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			prefixes = append(prefixes, key)
		}
	}
	return prefixes
}
