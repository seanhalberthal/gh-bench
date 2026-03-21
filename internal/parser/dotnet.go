package parser

import (
	"regexp"
	"strings"
)

// DotnetParser extracts failures from xUnit/NUnit/MSBuild output.
type DotnetParser struct{}

var (
	// Matches: "Failed TestNamespace.TestClass.TestMethod [45ms]"
	dotnetFailedTestRe = regexp.MustCompile(`Failed\s+(\S+)\s+\[(\d+\s*m?s)\]`)
	// Matches exception lines: "System.NullReferenceException: ..."
	dotnetExceptionRe = regexp.MustCompile(`^\s+(System\.\S+Exception:.+)$`)
	// Matches assert failures: "Assert.Equal() Failure"
	dotnetAssertRe = regexp.MustCompile(`^\s+(Assert\.\w+\(\)\s+Failure.*)$`)
	// Matches "at Namespace.Class.Method() in File.cs:line 42"
	dotnetLocationRe = regexp.MustCompile(`at\s+\S+\s+in\s+(\S+:line\s+\d+)`)
	// Matches MSBuild errors: "error CS1234: ..."
	dotnetMSBuildRe = regexp.MustCompile(`error\s+CS\d+:`)
	// Matches "Expected: X" / "Actual: Y"
	dotnetExpectedActualRe = regexp.MustCompile(`^\s+(Expected:|Actual:)\s+(.+)$`)
)

// Name returns the parser name.
func (d *DotnetParser) Name() string { return "xUnit" }

// Detect checks if the log contains .NET test failure patterns.
func (d *DotnetParser) Detect(logs string) bool {
	return dotnetFailedTestRe.MatchString(logs) || dotnetMSBuildRe.MatchString(logs)
}

// Extract parses .NET test failures from logs.
func (d *DotnetParser) Extract(logs string) []Failure {
	lines := strings.Split(logs, "\n")
	var failures []Failure

	for i := 0; i < len(lines); i++ {
		match := dotnetFailedTestRe.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}

		f := Failure{
			TestName:  match[1],
			Duration:  match[2],
			Framework: "xUnit",
		}

		// Collect error details from subsequent lines
		var msgLines []string
		for j := i + 1; j < len(lines) && j < i+20; j++ {
			line := lines[j]

			// Stop at the next test or blank section
			if dotnetFailedTestRe.MatchString(line) {
				break
			}

			if exMatch := dotnetExceptionRe.FindStringSubmatch(line); exMatch != nil {
				msgLines = append(msgLines, strings.TrimSpace(exMatch[1]))
			} else if assertMatch := dotnetAssertRe.FindStringSubmatch(line); assertMatch != nil {
				msgLines = append(msgLines, strings.TrimSpace(assertMatch[1]))
			} else if eaMatch := dotnetExpectedActualRe.FindStringSubmatch(line); eaMatch != nil {
				msgLines = append(msgLines, strings.TrimSpace(eaMatch[1])+" "+strings.TrimSpace(eaMatch[2]))
			}

			if locMatch := dotnetLocationRe.FindStringSubmatch(line); locMatch != nil {
				f.Location = locMatch[1]
			}
		}

		f.Message = strings.Join(msgLines, "\n")
		failures = append(failures, f)
	}

	return failures
}
