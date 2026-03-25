package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/seanhalberthal/gh-bench/internal/logutil"
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
func GetFailedSteps(ctx context.Context, runID int64) ([]StepResult, error) {
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
	logs := make([]logPair, len(toFetch))
	g, ctx := errgroup.WithContext(ctx)
	for i, js := range toFetch {
		g.Go(func() error {
			lp, err := fetchJobLog(ctx, js.job.DatabaseID, runID)
			if err != nil {
				return fmt.Errorf("fetching log for job %d: %w", js.job.DatabaseID, err)
			}
			logs[i] = lp
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Assemble step results, segmenting logs by step when markers are present.
	var failedSteps []StepResult
	for i, js := range toFetch {
		for _, step := range js.steps {
			clean := segmentByStep(logs[i].clean, step.Name)
			raw := segmentByStep(logs[i].raw, step.Name)
			failedSteps = append(failedSteps, StepResult{
				Name:   step.Name,
				Log:    clean,
				RawLog: raw,
			})
		}
	}

	return failedSteps, nil
}

// segmentByStep extracts the log section for a specific step using
// GitHub Actions ##[group] / ##[endgroup] markers. Falls back to
// the full log when no matching markers are found.
func segmentByStep(log, stepName string) string {
	groupPrefix := "##[group]"
	endGroup := "##[endgroup]"

	lower := strings.ToLower(stepName)
	var capturing bool
	var b strings.Builder

	for line := range strings.SplitSeq(log, "\n") {
		if strings.HasPrefix(line, groupPrefix) {
			label := line[len(groupPrefix):]
			if strings.ToLower(strings.TrimSpace(label)) == lower {
				capturing = true
				continue
			} else if capturing {
				// Entered a different step's group — stop capturing.
				break
			}
			continue
		}
		if line == endGroup {
			if capturing {
				break
			}
			continue
		}
		if capturing {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(line)
		}
	}

	if b.Len() == 0 {
		return log // no markers found — fall back to full log
	}
	return b.String()
}

// logPair holds both the cleaned log (for parsers) and raw log (timestamps preserved).
type logPair struct {
	clean string // timestamps stripped
	raw   string // timestamps preserved, tab-prefixes stripped
}

// fetchJobLog retrieves the raw log for a specific job, trying the REST API
// first (faster, cleaner output) then falling back to gh run view --log.
func fetchJobLog(_ context.Context, jobID, runID int64) (logPair, error) {
	// Try REST API: GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs
	// Returns plain text — no tab-prefixed formatting, but still has timestamps.
	log, err := Executor.Run("api", "repos/{owner}/{repo}/actions/jobs/"+strconv.FormatInt(jobID, 10)+"/logs")
	if err == nil {
		return logPair{clean: logutil.StripTimestamps(log), raw: log}, nil
	}

	// Fallback: gh run view --log --job (slower, adds job\tstep\t prefixes).
	idStr := strconv.FormatInt(runID, 10)
	log, err = Executor.Run("run", "view", idStr, "--log", "--job", strconv.FormatInt(jobID, 10))
	if err != nil {
		return logPair{}, fmt.Errorf("fetching log: %w", err)
	}

	// Strip tab-delimited prefixes; preserve timestamps in raw version.
	return logPair{clean: stripLogPrefixes(log), raw: stripTabPrefixesOnly(log)}, nil
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
			b.WriteString(logutil.StripTimestamp(line))
			continue
		}
		rest := line[first+1:]
		second := strings.IndexByte(rest, '\t')
		if second < 0 {
			b.WriteString(logutil.StripTimestamp(line))
			continue
		}
		b.WriteString(logutil.StripTimestamp(rest[second+1:]))
	}

	return b.String()
}

// stripTabPrefixesOnly removes job\tstep\t prefixes from gh run view --log
// output but preserves timestamps.
func stripTabPrefixesOnly(log string) string {
	var b strings.Builder
	b.Grow(len(log))

	for line := range strings.SplitSeq(log, "\n") {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}

		first := strings.IndexByte(line, '\t')
		if first < 0 {
			b.WriteString(line)
			continue
		}
		rest := line[first+1:]
		second := strings.IndexByte(rest, '\t')
		if second < 0 {
			b.WriteString(line)
			continue
		}
		b.WriteString(rest[second+1:])
	}

	return b.String()
}
