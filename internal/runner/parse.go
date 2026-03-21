package runner

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

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
	for _, line := range strings.Split(log, "\n") {
		match := re.FindStringSubmatch(line)
		if match != nil && groupIdx < len(match) {
			return match[groupIdx]
		}
	}
	return ""
}
