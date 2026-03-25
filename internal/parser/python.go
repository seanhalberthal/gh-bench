package parser

import (
	"regexp"
	"strings"

	"github.com/seanhalberthal/gh-bench/internal/logutil"
)

// PythonParser detects and extracts failures from pytest output.
type PythonParser struct{}

var (
	// pytest FAILED markers: "FAILED tests/test_foo.py::test_bar - AssertionError: ..."
	// Not anchored to ^ because lines may have timestamp prefixes.
	pytestFailRe = regexp.MustCompile(`FAILED\s+(\S+::\S+)`)

	// pytest short test summary section header.
	pytestSummaryRe = regexp.MustCompile(`=+ short test summary info =+`)

	// pytest section header for individual failures: "_____ test_name _____"
	pytestSectionRe = regexp.MustCompile(`_{3,}\s+(.+?)\s+_{3,}`)

	// Python assertion/error patterns.
	pythonAssertRe = regexp.MustCompile(`(?i)(?:AssertionError|assert|Error|Exception|Failed)`)
	pythonLocRe    = regexp.MustCompile(`(\S+\.py):(\d+)`)
)

func (p *PythonParser) Name() string { return "pytest" }

func (p *PythonParser) Detect(logs string) bool {
	return pytestFailRe.MatchString(logs) ||
		(pytestSummaryRe.MatchString(logs) && strings.Contains(logs, "FAILED"))
}

func (p *PythonParser) Extract(logs string) []Failure {
	lines := strings.Split(logs, "\n")
	var failures []Failure
	seen := make(map[string]bool)

	// Strategy 1: parse FAILED lines from short test summary.
	matches := pytestFailRe.FindAllStringSubmatch(logs, -1)
	for _, m := range matches {
		testPath := m[1]
		if seen[testPath] {
			continue
		}
		seen[testPath] = true

		// Extract the test name from path::class::method or path::function.
		testName := testPath
		if idx := strings.LastIndex(testPath, "::"); idx >= 0 {
			testName = testPath[idx+2:]
		}

		failures = append(failures, Failure{
			TestName:  testName,
			Message:   "",
			Location:  testPath,
			Framework: "pytest",
		})
	}

	// Strategy 2: parse detailed failure sections for error messages.
	// Look for "_____ test_name _____" section headers and extract context.
	for i, line := range lines {
		line = logutil.StripTimestamp(line)
		m := pytestSectionRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		sectionName := m[1]
		message, location := p.extractSectionContext(lines, i)

		// Try to match this section to an existing failure by test name.
		matched := false
		for j := range failures {
			if strings.Contains(sectionName, failures[j].TestName) ||
				strings.Contains(failures[j].TestName, sectionName) {
				if failures[j].Message == "" {
					failures[j].Message = message
				}
				if failures[j].Location == "" && location != "" {
					failures[j].Location = location
				}
				matched = true
				break
			}
		}

		// If no existing failure matched, create a new one.
		if !matched && message != "" {
			failures = append(failures, Failure{
				TestName:  sectionName,
				Message:   message,
				Location:  location,
				Framework: "pytest",
			})
		}
	}

	return failures
}

func (p *PythonParser) extractSectionContext(lines []string, idx int) (string, string) {
	var msgLines []string
	location := ""
	limit := min(idx+30, len(lines))

	for i := idx + 1; i < limit; i++ {
		line := logutil.StripTimestamp(strings.TrimSpace(lines[i]))
		if line == "" {
			continue
		}

		// Stop at next section boundary.
		if pytestSectionRe.MatchString(line) || strings.HasPrefix(line, "====") {
			break
		}

		if pythonAssertRe.MatchString(line) {
			msgLines = append(msgLines, line)
		}

		if location == "" {
			if m := pythonLocRe.FindStringSubmatch(line); m != nil {
				location = m[1] + ":" + m[2]
			}
		}
	}

	return strings.Join(msgLines, "\n"), location
}
