package parser

import (
	"regexp"
	"strings"
	"time"
)

// tsExtractRe captures the ISO 8601 timestamp at the start of a GitHub Actions log line.
var tsExtractRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z)\s`)

// timestampFormat is the human-readable display format for failure timestamps.
const timestampFormat = "02/01/06 15:04:05"

// extractTimestamp returns the formatted timestamp from a raw log line, or "".
func extractTimestamp(line string) string {
	m := tsExtractRe.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	// Truncate fractional seconds to 9 digits max for time.Parse compatibility,
	// then parse with a fixed-length nanosecond reference.
	raw := m[1]
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return ""
	}
	return t.Format(timestampFormat)
}

// AnnotateTimestamps sets the Timestamp field on each failure by matching
// its anchor pattern against the raw (timestamp-preserved) log lines.
func AnnotateTimestamps(failures []Failure, rawLog string) {
	if len(failures) == 0 || rawLog == "" {
		return
	}

	lines := strings.Split(rawLog, "\n")

	for i := range failures {
		anchor := anchorFor(&failures[i])
		if anchor == "" {
			continue
		}
		for _, line := range lines {
			if strings.Contains(line, anchor) {
				if ts := extractTimestamp(line); ts != "" {
					failures[i].Timestamp = ts
					break
				}
			}
		}
	}
}

// anchorFor returns the substring to search for in the raw log to locate
// the line where this failure was reported. Returns "" if no reliable
// anchor exists (e.g. fallback/unknown framework).
func anchorFor(f *Failure) string {
	switch f.Framework {
	case "go test":
		// Anchor: "--- FAIL: TestName"
		return "--- FAIL: " + f.TestName
	case "dotnet":
		// Anchor: "Failed TestName ["
		return "Failed " + f.TestName + " ["
	case "vitest":
		// Anchor: the test name itself (contains " > " or " › " suite delimiter)
		return f.TestName
	case "pytest":
		// Anchor: "FAILED path::test_name" — TestName is the short name
		// after the last "::". The FAILED summary line contains both the
		// full path and the test name, so matching on TestName is sufficient.
		return f.TestName
	default:
		return ""
	}
}
