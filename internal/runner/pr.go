package runner

import "strings"

// ListOpenPRBranches returns the set of branch names that have open pull requests
// in the current repository.
func ListOpenPRBranches() (map[string]bool, error) {
	out, err := Executor.Run("pr", "list", "--state", "open", "--json", "headRefName", "--jq", ".[].headRefName")
	if err != nil {
		return nil, err
	}

	branches := make(map[string]bool)
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches[line] = true
		}
	}
	return branches, nil
}
