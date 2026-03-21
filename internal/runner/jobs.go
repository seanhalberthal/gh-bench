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
// gh run view --log prefixes lines with the job and step name.
func extractStepLog(fullLog, jobName, stepName string) string {
	prefix := jobName + "\t" + stepName + "\t"
	var lines []string
	for _, line := range strings.Split(fullLog, "\n") {
		if strings.HasPrefix(line, prefix) {
			// Strip the prefix to get just the log content
			content := strings.TrimPrefix(line, prefix)
			lines = append(lines, content)
		}
	}
	if len(lines) == 0 {
		// Fallback: return the full log if we can't isolate the step
		return fullLog
	}
	return strings.Join(lines, "\n")
}
