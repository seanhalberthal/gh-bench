package logutil

import (
	"regexp"
	"strings"
)

// TimestampRe matches the GitHub Actions log timestamp prefix.
// Format: 2026-03-16T13:34:37.3465175Z (ISO 8601 with fractional seconds).
// The trailing space is optional to handle lines where the timestamp is
// immediately followed by content without a separator.
var TimestampRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z ?`)

// StripTimestamp removes a GitHub Actions timestamp prefix from a single line.
func StripTimestamp(line string) string {
	if loc := TimestampRe.FindStringIndex(line); loc != nil {
		return line[loc[1]:]
	}
	return line
}

// StripTimestamps removes GitHub Actions timestamp prefixes from all lines.
func StripTimestamps(log string) string {
	var b strings.Builder
	b.Grow(len(log))

	for line := range strings.SplitSeq(log, "\n") {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(StripTimestamp(line))
	}

	return b.String()
}
