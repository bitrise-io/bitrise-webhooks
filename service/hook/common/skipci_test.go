package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSkipBuildByCommitMessage(t *testing.T) {
	t.Log("Should skip")
	{
		for _, aCommitMsg := range []string{
			"[ci skip]",
			"[skip ci]",
			`multi line
with [ci skip] in
the middle`,
			"this should be [ci skip]ped",
			"[skip ci] will skip",
			`[skip ci] this
multi
line too`,
			`this has \\[skip ci\\] because of markdown`
		} {
			t.Log(" * Commit message:", aCommitMsg)
			require.Equal(t, true, IsSkipBuildByCommitMessage(aCommitMsg))
		}
	}

	t.Log("Should NOT skip")
	{
		for _, aCommitMsg := range []string{
			"",
			"[CI SKIP]",
			"[SKIP CI]",
			"this will not be [CI SKIP]ped",
			"this won't be skipped either: [ ci skip ]",
		} {
			t.Log(" * Commit message:", aCommitMsg)
			require.Equal(t, false, IsSkipBuildByCommitMessage(aCommitMsg))
		}
	}
}
