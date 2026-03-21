package runner

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Preset defines a reusable extraction pattern with metadata.
type Preset struct {
	Pattern     string
	Description string
	Example     string
}

// Presets maps preset names to their definitions.
var Presets = map[string]Preset{
	"duration": {
		Pattern:     `(?i)(?:took|duration|time|elapsed|finished in|completed in)\s*[:=]?\s*(?P<duration>[0-9.]+)\s*(?:ms|s)\b`,
		Description: "Common duration/timing output",
		Example:     "Took 12.5s, duration: 45ms, elapsed: 3.2s",
	},
	"coverage": {
		Pattern:     `(?i)coverage\s*[:=]?\s*(?P<coverage>[0-9.]+)\s*%`,
		Description: "Test coverage percentages",
		Example:     "Coverage: 85.2%, coverage=91%",
	},
	"go-test": {
		Pattern:     `^ok\s+\S+\s+(?P<duration>[0-9.]+)s`,
		Description: "Go test package duration",
		Example:     "ok  github.com/foo/bar  1.234s",
	},
	"jest": {
		Pattern:     `(?i)time\s*:\s*(?P<duration>[0-9.]+)\s*(?:ms|s)`,
		Description: "Jest/Vitest test suite time",
		Example:     "Time:        4.589 s",
	},
	"pytest": {
		Pattern:     `(?i)passed in\s+(?P<duration>[0-9.]+)s`,
		Description: "Pytest suite duration",
		Example:     "42 passed in 3.45s",
	},
	"bundle-size": {
		Pattern:     `(?i)(?:bundle[- ]?size|size)\s*[:=]?\s*(?P<size>[0-9.]+)\s*(?:kb|mb|gb|bytes|b)\b`,
		Description: "Bundle or file size output",
		Example:     "Bundle size: 245.3 kB, size: 1.2 MB",
	},
}

// PresetNames returns sorted preset names.
func PresetNames() []string {
	names := make([]string, 0, len(Presets))
	for name := range Presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolvePattern returns the regex for a preset name, or an error if not found.
func ResolvePattern(name string) (string, error) {
	p, ok := Presets[name]
	if !ok {
		return "", fmt.Errorf("unknown preset %q (available: %s)", name, strings.Join(PresetNames(), ", "))
	}
	return p.Pattern, nil
}

// ExtractedValue holds a single extracted value from a run's log.
type ExtractedValue struct {
	RunID int64
	Title string
	Raw   string
	Value float64
}

// ExtractedValues is a slice of ExtractedValue.
type ExtractedValues []ExtractedValue

// Numbers returns just the numeric values.
func (ev ExtractedValues) Numbers() []float64 {
	nums := make([]float64, len(ev))
	for i, v := range ev {
		nums[i] = v.Value
	}
	return nums
}

// ExtractValues applies a named capture group regex to each run's log and extracts numeric values.
func ExtractValues(results []RunResult, pattern string) (ExtractedValues, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compiling pattern %q: %w", pattern, err)
	}

	// Find the first named capture group
	groupName := ""
	for _, name := range re.SubexpNames() {
		if name != "" {
			groupName = name
			break
		}
	}
	if groupName == "" {
		return nil, fmt.Errorf("pattern %q must contain at least one named capture group (?P<name>...)", pattern)
	}

	groupIdx := re.SubexpIndex(groupName)

	var values ExtractedValues
	for _, r := range results {
		match := findFirstMatch(re, r.Log, groupIdx)
		if match == "" {
			continue
		}

		num, err := strconv.ParseFloat(match, 64)
		if err != nil {
			return nil, fmt.Errorf("value %q from run %d is not numeric: %w", match, r.RunID, err)
		}

		values = append(values, ExtractedValue{
			RunID: r.RunID,
			Title: r.Title,
			Raw:   match,
			Value: num,
		})
	}

	return values, nil
}

// findFirstMatch finds the first match of the regex in the log and returns the named group value.
func findFirstMatch(re *regexp.Regexp, log string, groupIdx int) string {
	for line := range strings.SplitSeq(log, "\n") {
		match := re.FindStringSubmatch(line)
		if match != nil && groupIdx < len(match) {
			return match[groupIdx]
		}
	}
	return ""
}
