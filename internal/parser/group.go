package parser

import (
	"fmt"
	"hash/fnv"
	"sort"
)

// FailureGroup represents a unique failure seen across multiple runs.
type FailureGroup struct {
	Signature string  `json:"signature"`
	TestName  string  `json:"test_name"`
	Message   string  `json:"message"`
	Location  string  `json:"location,omitempty"`
	Framework string  `json:"framework"`
	RunIDs    []int64 `json:"run_ids"`
	Count     int     `json:"count"`
}

// GroupKey returns a stable hash for deduplication.
// Uses test name + first line of message + framework to group.
func GroupKey(f Failure) string {
	firstLine := f.Message
	for i, c := range f.Message {
		if c == '\n' {
			firstLine = f.Message[:i]
			break
		}
	}
	h := fnv.New64a()
	h.Write([]byte(f.Framework))
	h.Write([]byte{0})
	h.Write([]byte(f.TestName))
	h.Write([]byte{0})
	h.Write([]byte(firstLine))
	return fmt.Sprintf("%016x", h.Sum64())
}

// GroupFailures deduplicates failures across runs, returning groups sorted
// by frequency (most common first).
func GroupFailures(runFailures map[int64][]Failure) []FailureGroup {
	groups := make(map[string]*FailureGroup)

	for runID, failures := range runFailures {
		for _, f := range failures {
			key := GroupKey(f)
			g, ok := groups[key]
			if !ok {
				g = &FailureGroup{
					Signature: key,
					TestName:  f.TestName,
					Message:   f.Message,
					Location:  f.Location,
					Framework: f.Framework,
				}
				groups[key] = g
			}
			g.RunIDs = append(g.RunIDs, runID)
			g.Count++
		}
	}

	result := make([]FailureGroup, 0, len(groups))
	for _, g := range groups {
		sort.Slice(g.RunIDs, func(i, j int) bool { return g.RunIDs[i] < g.RunIDs[j] })
		result = append(result, *g)
	}

	// Sort by count descending, then by test name for stability.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].TestName < result[j].TestName
	})

	return result
}
