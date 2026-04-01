package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/seanhalberthal/gh-bench/internal/runner"
	"github.com/spf13/cobra"
)

var grepCmd = &cobra.Command{
	Use:   "grep <pattern>",
	Short: "Search CI run logs for a pattern",
	Long:  "Fetch workflow run logs and search for lines matching a pattern (case-insensitive substring by default).",
	Args:  cobra.ExactArgs(1),
	RunE:  runGrep,
}

func init() {
	grepCmd.Flags().StringP("workflow", "w", "", "Workflow filename or name")
	grepCmd.Flags().StringP("runs", "r", "", "Comma-separated list of run IDs")
	grepCmd.Flags().IntP("limit", "l", 10, "Max number of runs to fetch")
	grepCmd.Flags().StringP("branch", "b", "", "Filter runs by branch")
	grepCmd.Flags().IntP("concurrency", "c", 5, "Number of concurrent log fetchers")
	grepCmd.Flags().BoolP("failed-only", "F", false, "Search only failed step logs")
	grepCmd.Flags().BoolP("regex", "E", false, "Treat pattern as a regular expression")
	grepCmd.Flags().IntP("context", "C", 0, "Show N lines of context around each match")
	grepCmd.Flags().IntP("max-matches", "m", 0, "Max matches per run (0 = unlimited)")
	grepCmd.Flags().StringP("step", "s", "", "Filter logs to a specific step name (substring match)")
}

// grepMatch represents a single matching line within a run.
type grepMatch struct {
	Line    int    // 1-based line number within the log
	Content string // The matching line (trimmed)
	Context string // Surrounding context lines (if -C > 0)
}

// grepResult holds all matches for a single run.
type grepResult struct {
	RunID   int64
	Title   string
	Date    string
	Branch  string
	Step    string // Set when --failed-only or --step is used
	Matches []grepMatch
}

func runGrep(cmd *cobra.Command, args []string) error {
	pattern := args[0]

	workflow, _ := cmd.Flags().GetString("workflow")
	runsFlag, _ := cmd.Flags().GetString("runs")
	limit, _ := cmd.Flags().GetInt("limit")
	branch, _ := cmd.Flags().GetString("branch")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	failedOnly, _ := cmd.Flags().GetBool("failed-only")
	useRegex, _ := cmd.Flags().GetBool("regex")
	contextLines, _ := cmd.Flags().GetInt("context")
	maxMatches, _ := cmd.Flags().GetInt("max-matches")
	stepFlag, _ := cmd.Flags().GetString("step")

	// Apply config defaults.
	if workflow == "" && cfg.Workflow != "" {
		workflow = cfg.Workflow
	}

	if workflow == "" && runsFlag == "" {
		return fmt.Errorf("either --workflow or --runs is required")
	}

	// Compile the matcher.
	matcher, err := compileMatcher(pattern, useRegex)
	if err != nil {
		return err
	}

	opts := runner.FetchOpts{
		Workflow:    workflow,
		Branch:      branch,
		Limit:       limit,
		Concurrency: concurrency,
		FailedOnly:  failedOnly,
		Step:        stepFlag,
	}

	if runsFlag != "" {
		ids, err := parseRunIDs(runsFlag)
		if err != nil {
			return err
		}
		opts.RunIDs = ids
	}

	results, err := withSpinner("Fetching run logs…", func() ([]runner.RunResult, error) {
		return runner.FetchLogs(cmd.Context(), opts)
	})
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}

	// Search each run's logs.
	var grepResults []grepResult
	totalMatches := 0

	for _, r := range results {
		if failedOnly && len(r.FailedSteps) > 0 {
			// Search each failed step separately.
			for _, step := range r.FailedSteps {
				matches := searchLog(step.Log, matcher, contextLines, maxMatches)
				if len(matches) > 0 {
					grepResults = append(grepResults, grepResult{
						RunID:   r.RunID,
						Title:   r.Title,
						Date:    r.Date,
						Branch:  r.Branch,
						Step:    step.Name,
						Matches: matches,
					})
					totalMatches += len(matches)
				}
			}
		} else {
			// Search the full log.
			matches := searchLog(r.Log, matcher, contextLines, maxMatches)
			if len(matches) > 0 {
				grepResults = append(grepResults, grepResult{
					RunID:   r.RunID,
					Title:   r.Title,
					Date:    r.Date,
					Branch:  r.Branch,
					Matches: matches,
				})
				totalMatches += len(matches)
			}
		}
	}

	if len(grepResults) == 0 {
		fmt.Fprintf(os.Stderr, "no matches for %q across %d runs\n", pattern, len(results))
		return nil
	}

	switch resolveFormat(cmd) {
	case "json":
		return printGrepJSON(grepResults)
	case "csv":
		return printGrepCSV(grepResults)
	default:
		return printGrepText(grepResults, totalMatches, len(results))
	}
}

// matchFunc reports whether a line matches the search pattern.
type matchFunc func(line string) bool

// compileMatcher builds a case-insensitive matcher from the user's pattern.
func compileMatcher(pattern string, useRegex bool) (matchFunc, error) {
	if useRegex {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
		return re.MatchString, nil
	}
	lower := strings.ToLower(pattern)
	return func(line string) bool {
		return strings.Contains(strings.ToLower(line), lower)
	}, nil
}

// searchLog finds all matching lines in a log, with optional context.
func searchLog(log string, match matchFunc, contextSize, maxMatches int) []grepMatch {
	lines := strings.Split(log, "\n")
	var matches []grepMatch

	for i, line := range lines {
		if match(line) {
			m := grepMatch{
				Line:    i + 1,
				Content: line,
			}
			if contextSize > 0 {
				start := max(0, i-contextSize)
				end := min(len(lines), i+contextSize+1)
				m.Context = strings.Join(lines[start:end], "\n")
			}
			matches = append(matches, m)
			if maxMatches > 0 && len(matches) >= maxMatches {
				break
			}
		}
	}

	return matches
}

func printGrepText(results []grepResult, totalMatches, totalRuns int) error {
	var b strings.Builder

	fmt.Fprintf(&b, "%d matches across %d/%d runs\n\n", totalMatches, len(results), totalRuns)

	for i, r := range results {
		if i > 0 {
			b.WriteString(strings.Repeat("─", 68))
			b.WriteByte('\n')
		}
		if r.Branch != "" {
			fmt.Fprintf(&b, "● RUN %d — %s (%s) [%s]\n", r.RunID, r.Title, r.Date, r.Branch)
		} else {
			fmt.Fprintf(&b, "● RUN %d — %s (%s)\n", r.RunID, r.Title, r.Date)
		}
		if r.Step != "" {
			fmt.Fprintf(&b, "  Step: %s\n", r.Step)
		}
		b.WriteByte('\n')

		for _, m := range r.Matches {
			if m.Context != "" {
				for _, cl := range strings.Split(m.Context, "\n") {
					fmt.Fprintf(&b, "  %s\n", cl)
				}
				b.WriteByte('\n')
			} else {
				fmt.Fprintf(&b, "  %d: %s\n", m.Line, m.Content)
			}
		}
	}

	_, err := os.Stdout.WriteString(b.String())
	return err
}

func printGrepJSON(results []grepResult) error {
	type matchOutput struct {
		Line    int    `json:"line"`
		Content string `json:"content"`
		Context string `json:"context,omitempty"`
	}
	type runOutput struct {
		RunID   int64         `json:"run_id"`
		Title   string        `json:"title"`
		Date    string        `json:"date"`
		Branch  string        `json:"branch"`
		Step    string        `json:"step,omitempty"`
		Matches []matchOutput `json:"matches"`
	}

	output := make([]runOutput, len(results))
	for i, r := range results {
		matches := make([]matchOutput, len(r.Matches))
		for j, m := range r.Matches {
			matches[j] = matchOutput(m)
		}
		output[i] = runOutput{
			RunID:   r.RunID,
			Title:   r.Title,
			Date:    r.Date,
			Branch:  r.Branch,
			Step:    r.Step,
			Matches: matches,
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func printGrepCSV(results []grepResult) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	if err := w.Write([]string{"run_id", "title", "date", "branch", "step", "line", "content"}); err != nil {
		return err
	}
	for _, r := range results {
		for _, m := range r.Matches {
			if err := w.Write([]string{
				strconv.FormatInt(r.RunID, 10), r.Title, r.Date, r.Branch, r.Step,
				strconv.Itoa(m.Line), m.Content,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
