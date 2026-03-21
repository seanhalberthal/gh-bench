package parser

import (
	"regexp"
	"strings"
)

// GoParser extracts failures from go test output.
type GoParser struct{}

var (
	// Matches: "--- FAIL: TestName (0.45s)"
	goFailRe = regexp.MustCompile(`---\s+FAIL:\s+(\S+)\s+\(([^)]+)\)`)
	// Matches: "FAIL\tpackage/path"
	goPackageFailRe = regexp.MustCompile(`^FAIL\t(\S+)`)
	// Matches file:line references in go test output
	goLocationRe = regexp.MustCompile(`(\S+\.go:\d+)`)
)

// Name returns the parser name.
func (g *GoParser) Name() string { return "go test" }

// Detect checks if the log contains go test failure patterns.
func (g *GoParser) Detect(logs string) bool {
	return goFailRe.MatchString(logs) || goPackageFailRe.MatchString(logs)
}

// Extract parses go test failures from logs.
func (g *GoParser) Extract(logs string) []Failure {
	lines := strings.Split(logs, "\n")
	var failures []Failure

	for i := 0; i < len(lines); i++ {
		match := goFailRe.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}

		f := Failure{
			TestName:  match[1],
			Duration:  match[2],
			Framework: "go test",
		}

		// Look backwards for assertion/error lines (go test prints errors before FAIL)
		var msgLines []string
		for j := i - 1; j >= 0 && j > i-30; j-- {
			line := lines[j]
			trimmed := strings.TrimSpace(line)

			// Stop at previous test boundary or non-indented line
			if goFailRe.MatchString(line) {
				break
			}
			if strings.HasPrefix(trimmed, "=== RUN") {
				break
			}

			// Collect indented lines (test output is indented)
			if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
				msgLines = append([]string{trimmed}, msgLines...)

				if locMatch := goLocationRe.FindStringSubmatch(line); locMatch != nil && f.Location == "" {
					f.Location = locMatch[1]
				}
			}
		}

		f.Message = strings.Join(msgLines, "\n")
		failures = append(failures, f)
	}

	return failures
}
