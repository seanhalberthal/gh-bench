package parser

import (
	"regexp"
	"strings"
)

// DotnetParser extracts failures from xUnit/NUnit/MSBuild output.
type DotnetParser struct{}

var (
	// Matches: "Failed TestNamespace.TestClass.TestMethod [45ms]"
	// The test name may contain spaces when parameterised (e.g. "(data: Tuple ("foo", "bar"))").
	// We capture everything between "Failed " and the final " [duration]".
	dotnetFailedTestRe = regexp.MustCompile(`Failed\s+(.+?)\s+\[(<?\s*\d+[\d.]*\s*m?s)\]`)
	// Matches exception lines: "System.NullReferenceException: ..."
	dotnetExceptionRe = regexp.MustCompile(`^\s+(System\.\S+Exception:.+)$`)
	// Matches assert failures: "Assert.Equal() Failure"
	dotnetAssertRe = regexp.MustCompile(`^\s+(Assert\.\w+\(\)\s+Failure.*)$`)
	// Matches "at Namespace.Class.Method() in File.cs:line 42"
	// Uses .+ (greedy) for the method name to handle parameterised tests with spaces.
	dotnetLocationRe = regexp.MustCompile(`at\s+.+\s+in\s+(\S+:line\s+\d+)`)
	// Matches MSBuild errors: "error CS1234: ..."
	dotnetMSBuildRe = regexp.MustCompile(`error\s+CS\d+:`)
	// Matches "Expected: X" / "Actual: Y"
	dotnetExpectedActualRe = regexp.MustCompile(`^\s+(Expected:|Actual:)\s+(.+)$`)

	// Broader detection patterns — these identify .NET even when individual
	// test failure lines aren't present in the log.
	dotnetSummaryRe    = regexp.MustCompile(`Failed!\s+-\s+Failed:\s+(\d+)`)
	dotnetNUnitSummary = regexp.MustCompile(`Overall result:\s+Failed`)
	dotnetMSTestRe     = regexp.MustCompile(`Test Run Failed\.`)
	dotnetTotalTestsRe = regexp.MustCompile(`Total tests:\s+\d+`)

	// Matches "Error Message:" header in dotnet test verbose output.
	dotnetErrorMsgRe = regexp.MustCompile(`\bError Message:\s*$`)
	// Matches "Stack Trace:" header.
	dotnetStackTraceRe = regexp.MustCompile(`\bStack Trace:\s*$`)

)

// Name returns the parser name.
func (d *DotnetParser) Name() string { return "dotnet" }

// Detect checks if the log contains .NET test failure patterns.
func (d *DotnetParser) Detect(logs string) bool {
	return dotnetFailedTestRe.MatchString(logs) ||
		dotnetMSBuildRe.MatchString(logs) ||
		dotnetSummaryRe.MatchString(logs) ||
		dotnetNUnitSummary.MatchString(logs) ||
		dotnetMSTestRe.MatchString(logs)
}

// Extract parses .NET test failures from logs.
func (d *DotnetParser) Extract(logs string) []Failure {
	lines := strings.Split(logs, "\n")
	var failures []Failure

	for i := range lines {
		match := dotnetFailedTestRe.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}

		f := Failure{
			TestName:  match[1],
			Duration:  match[2],
			Framework: "dotnet",
		}

		// Collect error details from subsequent lines.
		var msgLines []string
		inErrorMsg := false
		inStackTrace := false
		for j := i + 1; j < len(lines) && j < i+30; j++ {
			line := lines[j]

			// Stop at the next test or blank section.
			if dotnetFailedTestRe.MatchString(line) {
				break
			}

			// Strip GitHub Actions timestamp prefix for detail matching.
			cleaned := stripTimestamp(line)

			// Track "Error Message:" / "Stack Trace:" sections.
			if dotnetErrorMsgRe.MatchString(cleaned) {
				inErrorMsg = true
				inStackTrace = false
				continue
			}
			if dotnetStackTraceRe.MatchString(cleaned) {
				inErrorMsg = false
				inStackTrace = true
				continue
			}

			if exMatch := dotnetExceptionRe.FindStringSubmatch(cleaned); exMatch != nil {
				msgLines = append(msgLines, strings.TrimSpace(exMatch[1]))
			} else if assertMatch := dotnetAssertRe.FindStringSubmatch(cleaned); assertMatch != nil {
				msgLines = append(msgLines, strings.TrimSpace(assertMatch[1]))
			} else if eaMatch := dotnetExpectedActualRe.FindStringSubmatch(cleaned); eaMatch != nil {
				msgLines = append(msgLines, strings.TrimSpace(eaMatch[1])+" "+strings.TrimSpace(eaMatch[2]))
			} else if inErrorMsg {
				trimmed := strings.TrimSpace(cleaned)
				if trimmed != "" {
					msgLines = append(msgLines, trimmed)
				}
			} else if inStackTrace {
				// Only extract the first location from the stack trace.
				if locMatch := dotnetLocationRe.FindStringSubmatch(cleaned); locMatch != nil && f.Location == "" {
					f.Location = locMatch[1]
				}
			}

			// Legacy location extraction for output without Error Message/Stack Trace headers.
			if !inStackTrace {
				if locMatch := dotnetLocationRe.FindStringSubmatch(cleaned); locMatch != nil && f.Location == "" {
					f.Location = locMatch[1]
				}
			}
		}

		f.Message = strings.Join(msgLines, "\n")
		failures = append(failures, f)
	}

	// If we detected .NET but couldn't extract individual test failures,
	// produce a summary failure from the summary line.
	if len(failures) == 0 {
		failures = d.extractFromSummary(logs)
	}

	return failures
}

// extractFromSummary creates failure entries from dotnet test summary lines
// when individual "Failed TestName [duration]" lines aren't available.
// It scans for error lines, MSBuild errors, and summary counts to produce
// the most useful output possible.
func (d *DotnetParser) extractFromSummary(logs string) []Failure {
	lines := strings.Split(logs, "\n")

	// Collect all summary/result lines and MSBuild errors.
	var summaryLines []string
	var msbuildErrors []string
	var errorLines []string

	for _, raw := range lines {
		line := strings.TrimSpace(stripTimestamp(raw))
		if line == "" {
			continue
		}

		if dotnetMSBuildRe.MatchString(line) {
			msbuildErrors = append(msbuildErrors, line)
		}

		// Collect dotnet test result summary lines.
		if dotnetSummaryRe.MatchString(line) ||
			dotnetNUnitSummary.MatchString(line) ||
			dotnetMSTestRe.MatchString(line) ||
			dotnetTotalTestsRe.MatchString(line) {
			summaryLines = append(summaryLines, line)
		}

		// Collect ##[error] lines — often contain the actual failure reason.
		if strings.HasPrefix(line, "##[error]") {
			msg := strings.TrimPrefix(line, "##[error]")
			if msg != "Process completed with exit code 1." {
				errorLines = append(errorLines, msg)
			}
		}
	}

	// Build the message from the most specific information available.
	var parts []string

	if len(msbuildErrors) > 0 {
		parts = append(parts, msbuildErrors...)
	}
	if len(errorLines) > 0 {
		parts = append(parts, errorLines...)
	}
	if len(summaryLines) > 0 {
		parts = append(parts, summaryLines...)
	}

	if len(parts) == 0 {
		return nil
	}

	return []Failure{
		{
			TestName:  "(test run failed)",
			Message:   strings.Join(parts, "\n"),
			Framework: "dotnet",
		},
	}
}
