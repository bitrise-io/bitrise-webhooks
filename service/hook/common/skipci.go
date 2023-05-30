package common

import "strings"

// IsSkipBuildByCommitMessage ...
func IsSkipBuildByCommitMessage(commitMsg string) bool {
	if strings.Contains(commitMsg, "[skip ci]") || strings.Contains(commitMsg, "[ci skip]") || strings.Contains(commitMsg, `\\[skip ci\\]`) || strings.Contains(commitMsg, `\\[ci skip\\]`) {
		return true
	}
	return false
}
