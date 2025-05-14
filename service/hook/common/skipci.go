package common

import "strings"

// IsSkipBuildByCommitMessage ...
func IsSkipBuildByCommitMessage(commitMsg string) bool {
	if checkSkipPatternPair(commitMsg, "ci", "skip") {
		return true
	}
	if checkSkipPatternPair(commitMsg, "bitrise", "skip") {
		return true
	}
	return false
}

func checkSkipPatternPair(commitMsg, a, b string) bool {
	if strings.Contains(commitMsg, "["+a+" "+b+"]") ||
		strings.Contains(commitMsg, "["+b+" "+a+"]") ||
		strings.Contains(commitMsg, `\[`+a+` `+b+`\]`) ||
		strings.Contains(commitMsg, `\[`+b+` `+a+`\]`) ||
		strings.Contains(commitMsg, `\\[`+a+` `+b+`\\]`) ||
		strings.Contains(commitMsg, `\\[`+b+` `+a+`\\]`) {
		return true
	}
	return false
}
