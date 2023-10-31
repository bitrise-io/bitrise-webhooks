package gitlab

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xanzy/go-gitlab"
)

func TestHookProvider_gatherMetrics_commit_id_before_and_after(t *testing.T) {
	currentTime := time.Date(2023, time.October, 26, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		event   interface{}
		appSlug string
		want    string
	}{
		{
			name:    "Push created webhook - commit id before is null, after isn't",
			event:   testPushWebhook(t),
			appSlug: "slug",
			want:    `{"event":"git_push","action":"created","provider_type":"gitlab","repository":"bitrise-io/project","timestamp":"2023-10-26T08:00:00Z","app_slug":"slug","original_trigger":"push:","user_name":"bitrise-bot","git_ref":"dev-1","commit_id_after":"d6666f44e4a5c82c20a783da58c4274a6e3690c3","commit_id_before":"0000000000000000000000000000000000000000","oldest_commit_timestamp":"2023-10-31T09:39:09Z","latest_commit_timestamp":"2023-10-31T09:39:09Z","master_branch":"master"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hp := HookProvider{}
			got := hp.gatherMetrics(tt.event, tt.appSlug, currentTime)
			gotBytes, err := got.Serialise()
			require.NoError(t, err)
			require.Equal(t, tt.want, string(gotBytes))
		})
	}

}

func Test_parseTime(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want time.Time
	}{
		{
			name: "simple test",
			s:    "2023-10-19 11:50:00 UTC",
			want: time.Date(2023, 10, 19, 11, 50, 00, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTime(tt.s); !reflect.DeepEqual(got, &tt.want) {
				t.Errorf("parseTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testPushWebhook(t *testing.T) interface{} {
	var event gitlab.PushEvent
	err := json.Unmarshal([]byte(pushCreateWebhookPayload), &event)
	require.NoError(t, err)
	return &event
}

const pushCreateWebhookPayload = `{
	"object_kind": "push",
	"event_name": "push",
	"before": "0000000000000000000000000000000000000000",
	"after": "d6666f44e4a5c82c20a783da58c4274a6e3690c3",
	"ref": "refs/heads/dev-1",
	"user_username": "bitrise-bot",
	"project": {
		"path_with_namespace": "bitrise-io/project",
		"default_branch": "master"
	},
	"commits": [
		{
			"timestamp": "2023-10-31T09:39:09+00:00",
			"added": [

			],
			"modified": [
				"README.rdoc"
			],
			"removed": [

			]
		}
	]
}`
