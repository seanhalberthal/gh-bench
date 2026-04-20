package parser

import (
	"regexp"
	"strings"
)

// VitestParser extracts failures from Vitest/Jest output.
type VitestParser struct{}

var (
	// Matches: "FAIL src/components/Foo.test.tsx" or "FAIL src/components/Foo.test.ts"
	vitestFileFailRe = regexp.MustCompile(`FAIL\s+(\S+\.(?:test|spec)\.\w+)`)
	// Matches: "✗ Suite > Test Name" or "● Suite › Test Name" or "× Suite > Test Name"
	// Requires ">" or "›" to distinguish test-level failures from file-level lines
	vitestTestFailRe = regexp.MustCompile(`[✗●×]\s+(.+[>›].+)`)
	// Matches: "AssertionError:" or "Error:" lines
	vitestErrorRe = regexp.MustCompile(`^\s+((?:AssertionError|AssertError|Error|TypeError|ReferenceError):.+)$`)
	// Matches: "at src/components/Foo.test.tsx:38:18"
	vitestLocationRe = regexp.MustCompile(`at\s+(\S+\.\w+:\d+:\d+)`)
	// Matches: "expected 'X' to equal 'Y'"
	vitestExpectedRe = regexp.MustCompile(`^\s+(expected\s+.+)$`)
	// Matches vitest's --typecheck banner or the disable hint it prints on failure.
	vitestTypecheckRe = regexp.MustCompile("TypeScript typecheck via|--typecheck=disable")
	// Matches a tsc diagnostic line: "path/to/file.ts:7:29 - error TS2339: message"
	vitestTscErrorRe = regexp.MustCompile(`^(\S+\.\w+):(\d+):(\d+)\s+-\s+error\s+(TS\d+):\s+(.+)$`)
)

// Name returns the parser name.
func (v *VitestParser) Name() string { return "Vitest" }

// Detect checks if the log contains Vitest/Jest failure patterns.
func (v *VitestParser) Detect(logs string) bool {
	return vitestFileFailRe.MatchString(logs) ||
		vitestTestFailRe.MatchString(logs) ||
		vitestTypecheckRe.MatchString(logs)
}

// Extract parses Vitest/Jest failures from logs.
func (v *VitestParser) Extract(logs string) []Failure {
	lines := strings.Split(logs, "\n")
	var failures []Failure

	for i := range lines {
		match := vitestTestFailRe.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}

		// Skip if this looks like a passing test indicator
		testName := strings.TrimSpace(match[1])
		if testName == "" {
			continue
		}

		f := Failure{
			TestName:  testName,
			Framework: "Vitest",
		}

		// Collect error details from subsequent lines
		var msgLines []string
		for j := i + 1; j < len(lines) && j < i+20; j++ {
			line := lines[j]

			// Stop at next test marker
			if vitestTestFailRe.MatchString(line) {
				break
			}

			if errMatch := vitestErrorRe.FindStringSubmatch(line); errMatch != nil {
				msgLines = append(msgLines, strings.TrimSpace(errMatch[1]))
			} else if expMatch := vitestExpectedRe.FindStringSubmatch(line); expMatch != nil {
				msgLines = append(msgLines, strings.TrimSpace(expMatch[1]))
			}

			if locMatch := vitestLocationRe.FindStringSubmatch(line); locMatch != nil && f.Location == "" {
				f.Location = locMatch[1]
			}
		}

		f.Message = strings.Join(msgLines, "\n")
		failures = append(failures, f)
	}

	// If the normal runtime-failure extraction came up empty but the log
	// looks like vitest's --typecheck mode, fall back to extracting tsc
	// diagnostics as individual failures.
	if len(failures) == 0 && vitestTypecheckRe.MatchString(logs) {
		failures = extractVitestTypecheckFailures(lines)
	}

	return failures
}

// extractVitestTypecheckFailures parses tsc diagnostic lines emitted by
// vitest when running in --typecheck mode.
func extractVitestTypecheckFailures(lines []string) []Failure {
	var failures []Failure
	for _, line := range lines {
		m := vitestTscErrorRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		loc := m[1] + ":" + m[2] + ":" + m[3]
		failures = append(failures, Failure{
			TestName:  loc,
			Location:  loc,
			Message:   m[4] + ": " + strings.TrimSpace(m[5]),
			Framework: "Vitest",
		})
	}
	return failures
}
