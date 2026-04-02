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

// Step represents a GitHub Actions job step.
type Step struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Number     int    `json:"number"`
}

// jobInfo pairs a Job with the run attempt it came from.
type jobInfo struct {
	Job
	Attempt int
}

// apiJob is the REST API representation of a job, including run_attempt.
type apiJob struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	RunAttempt int    `json:"run_attempt"`
	Steps      []Step `json:"steps"`
}

// jobSteps pairs a jobInfo with its failed steps.
type jobSteps struct {
	info  jobInfo
	steps []Step
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

// GetFailedSteps retrieves the failed steps for a workflow run and fetches their logs.
func GetFailedSteps(ctx context.Context, runID int64) ([]StepResult, error) {
	jobs, err := getJobsForRun(strconv.FormatInt(runID, 10))
	if err != nil {
		return nil, fmt.Errorf("fetching jobs: %w", err)
	}

	toFetch := collectFailedJobSteps(jobs)
	if len(toFetch) == 0 {
		return nil, nil
	}

	logs, err := fetchJobLogsParallel(ctx, toFetch, runID)
	if err != nil {
		return nil, err
	}

	return assembleStepResults(toFetch, logs), nil
}

// collectFailedJobSteps filters the job list down to jobs with failed steps,
// returning each job paired with its relevant failed steps.
func collectFailedJobSteps(jobs []jobInfo) []jobSteps {
	var out []jobSteps
	for _, info := range jobs {
		if info.Conclusion != "failure" && info.Conclusion != "cancelled" {
			continue
		}
		if failed := failedStepsFor(info.Steps); len(failed) > 0 {
			out = append(out, jobSteps{info: info, steps: failed})
		}
	}
	return out
}

// failedStepsFor returns the non-infrastructure steps with a failure conclusion.
func failedStepsFor(steps []Step) []Step {
	var out []Step
	for _, s := range steps {
		if s.Conclusion == "failure" && !shouldSkipStep(s.Name) {
			out = append(out, s)
		}
	}
	return out
}

// fetchJobLogsParallel fetches logs for each job in toFetch concurrently.
func fetchJobLogsParallel(ctx context.Context, toFetch []jobSteps, runID int64) ([]logPair, error) {
	logs := make([]logPair, len(toFetch))
	g, ctx := errgroup.WithContext(ctx)
	for i, js := range toFetch {
		g.Go(func() error {
			lp, err := fetchJobLog(ctx, js.info.DatabaseID, runID)
			if err != nil {
				return fmt.Errorf("fetching log for job %d: %w", js.info.DatabaseID, err)
			}
			logs[i] = lp
			return nil
		})
	}
	return logs, g.Wait()
}

// assembleStepResults builds StepResult values from fetched logs,
// segmenting by step name when group markers are present.
func assembleStepResults(toFetch []jobSteps, logs []logPair) []StepResult {
	var out []StepResult
	for i, js := range toFetch {
		for _, step := range js.steps {
			out = append(out, StepResult{
				Name:    step.Name,
				Log:     segmentByStep(logs[i].clean, step.Name),
				RawLog:  segmentByStep(logs[i].raw, step.Name),
				Attempt: js.info.Attempt,
			})
		}
	}
	return out
}

// getJobsForRun fetches jobs for a run using the REST API with filter=all so
// that jobs from all re-run attempts are visible. It deduplicates by job name,
// keeping only the result from the latest attempt for each job. Falls back to
// gh run view when the REST API is unavailable.
func getJobsForRun(runID string) ([]jobInfo, error) {
	if jobs, err := fetchJobsFromAPI(runID); err == nil {
		return deduplicateByLatestAttempt(jobs), nil
	}
	return fetchJobsFromRunView(runID)
}

// fetchJobsFromAPI calls the REST API for all jobs across all attempts.
func fetchJobsFromAPI(runID string) ([]apiJob, error) {
	out, err := Executor.Run("api",
		"repos/{owner}/{repo}/actions/runs/"+runID+"/jobs?filter=all&per_page=100")
	if err != nil {
		return nil, err
	}
	var result struct {
		Jobs []apiJob `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, err
	}
	return result.Jobs, nil
}

// deduplicateByLatestAttempt keeps only the highest-attempt result for each job name.
func deduplicateByLatestAttempt(jobs []apiJob) []jobInfo {
	latest := make(map[string]apiJob, len(jobs))
	for _, rj := range jobs {
		if existing, ok := latest[rj.Name]; !ok || rj.RunAttempt > existing.RunAttempt {
			latest[rj.Name] = rj
		}
	}
	out := make([]jobInfo, 0, len(latest))
	for _, rj := range latest {
		out = append(out, jobInfo{
			Job: Job{
				DatabaseID: rj.ID,
				Name:       rj.Name,
				Status:     rj.Status,
				Conclusion: rj.Conclusion,
				Steps:      rj.Steps,
			},
			Attempt: rj.RunAttempt,
		})
	}
	return out
}

// fetchJobsFromRunView falls back to gh run view, which returns only the latest
// attempt's jobs and carries no attempt metadata.
func fetchJobsFromRunView(runID string) ([]jobInfo, error) {
	out, err := Executor.Run("run", "view", runID, "--json", "jobs")
	if err != nil {
		return nil, err
	}
	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parsing jobs JSON: %w", err)
	}
	infos := make([]jobInfo, len(result.Jobs))
	for i, j := range result.Jobs {
		infos[i] = jobInfo{Job: j}
	}
	return infos, nil
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

		_, after, ok := strings.Cut(line, "\t")
		if !ok {
			b.WriteString(logutil.StripTimestamp(line))
			continue
		}
		rest := after
		_, after, ok = strings.Cut(rest, "\t")
		if !ok {
			b.WriteString(logutil.StripTimestamp(line))
			continue
		}
		b.WriteString(logutil.StripTimestamp(after))
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

		_, after, ok := strings.Cut(line, "\t")
		if !ok {
			b.WriteString(line)
			continue
		}
		rest := after
		_, after, ok = strings.Cut(rest, "\t")
		if !ok {
			b.WriteString(line)
			continue
		}
		b.WriteString(after)
	}

	return b.String()
}
