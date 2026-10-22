package entire

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/stretchr/testify/require"
)

const (
	testCloneURL = "https://aws-us-east-2.entire.io/et/acme/api.git"
	testAfterSHA = "b8c1d4e5f60718293a4b5c6d7e8f9a0b1c2d3e4f"
)

// deliveryJSON builds the envelope the Monolith forwards, with the given
// event type, raw payload JSON and optional enrichment JSON (empty omits it).
func deliveryJSON(eventType, payload, enrichment string) string {
	body := `{
  "delivery_id": "dlv_01J9TESTDELIVERY",
  "plugin_id": "bitrise",
  "installation_id": "inst_01J9TEST",
  "core_url": "https://us.auth.entire.io",
  "api_url": "https://aws-us-east-2.api.entire.io",
  "event": {
    "id": "evt_01J9TESTEVENT",
    "type": "` + eventType + `",
    "time": "2026-09-30T02:14:00Z",
    "payload": ` + payload + `
  }`
	if enrichment != "" {
		body += `,
  "bitrise_enrichment": ` + enrichment
	}
	return body + "\n}"
}

func refPayloadJSON(ref, after, pusher string, forced bool) string {
	forcedStr := "false"
	if forced {
		forcedStr = "true"
	}
	return `{
      "repository": {
        "id": "repo_01J9TEST",
        "path": "/et/acme/api",
        "default_branch": "refs/heads/main",
        "clone_url": "` + testCloneURL + `",
        "entire_clone_url": "entire://aws-us-east-2.entire.io/et/acme/api"
      },
      "ref": "` + ref + `",
      "before": "a1b2c3d4e5f60718293a4b5c6d7e8f9a0b1c2d3e",
      "after": "` + after + `",
      "forced": ` + forcedStr + `,
      "pusher": ` + pusher + `
    }`
}

const (
	danaPusher     = `{ "account": { "id": "acc_01J9DANA", "handle": "dana" } }`
	emptyPusher    = `{ "account": { "id": "acc_01J9DANA", "handle": "" } }`
	testEnrichment = `{ "commit_message": "Add tests", "commit_author": "dana" }`
	zeroSHA        = "0000000000000000000000000000000000000000"
)

func newRequest(headers http.Header, body string) *http.Request {
	request := http.Request{Header: headers}
	if body != "" {
		request.Body = io.NopCloser(strings.NewReader(body))
	}
	return &request
}

func verifiedJSONHeaders() http.Header {
	return http.Header{
		"Content-Type":              {"application/json"},
		"X-Bitrise-Entire-Verified": {"1"},
	}
}

func expectedBranchTrigger(branch, commitMessage, triggeredBy string) []bitriseapi.TriggerAPIParamsModel {
	return []bitriseapi.TriggerAPIParamsModel{
		{
			BuildParams: bitriseapi.BuildParamsModel{
				Branch:            branch,
				CommitHash:        testAfterSHA,
				CommitMessage:     commitMessage,
				BaseRepositoryURL: testCloneURL,
				HeadRepositoryURL: testCloneURL,
			},
			TriggeredBy: triggeredBy,
		},
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	tests := []struct {
		name          string
		headers       http.Header
		body          string
		wantParams    []bitriseapi.TriggerAPIParamsModel
		wantSkip      bool
		wantErr       string
		wantErrPrefix string
	}{
		{
			name:       "branch push",
			headers:    verifiedJSONHeaders(),
			body:       deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", testAfterSHA, danaPusher, false), testEnrichment),
			wantParams: expectedBranchTrigger("main", "Add tests", "webhook-entire/dana"),
		},
		{
			name:       "branch created",
			headers:    verifiedJSONHeaders(),
			body:       deliveryJSON("ref.created", refPayloadJSON("refs/heads/feature/new-thing", testAfterSHA, danaPusher, false), testEnrichment),
			wantParams: expectedBranchTrigger("feature/new-thing", "Add tests", "webhook-entire/dana"),
		},
		{
			name:    "tag push",
			headers: verifiedJSONHeaders(),
			body:    deliveryJSON("ref.created", refPayloadJSON("refs/tags/v1.2.3", testAfterSHA, danaPusher, false), testEnrichment),
			wantParams: []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						Tag:               "v1.2.3",
						CommitHash:        testAfterSHA,
						CommitMessage:     "Add tests",
						BaseRepositoryURL: testCloneURL,
						HeadRepositoryURL: testCloneURL,
					},
					TriggeredBy: "webhook-entire/dana",
				},
			},
		},
		{
			name:       "forced push is a normal update",
			headers:    verifiedJSONHeaders(),
			body:       deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", testAfterSHA, danaPusher, true), testEnrichment),
			wantParams: expectedBranchTrigger("main", "Add tests", "webhook-entire/dana"),
		},
		{
			name:     "ref deleted is skipped",
			headers:  verifiedJSONHeaders(),
			body:     deliveryJSON("ref.deleted", refPayloadJSON("refs/heads/old", zeroSHA, danaPusher, false), ""),
			wantSkip: true,
			wantErr:  "Ref (refs/heads/old) was deleted - ignoring",
		},
		{
			name:     "zero SHA after is treated as a delete",
			headers:  verifiedJSONHeaders(),
			body:     deliveryJSON("ref.updated", refPayloadJSON("refs/heads/old", zeroSHA, danaPusher, false), ""),
			wantSkip: true,
			wantErr:  "Ref (refs/heads/old) was deleted ('after' is the zero SHA) - ignoring",
		},
		{
			name:     "installation event is skipped",
			headers:  verifiedJSONHeaders(),
			body:     deliveryJSON("installation.created", `{ "installation": { "id": "inst_01J9TEST", "status": "active" } }`, ""),
			wantSkip: true,
			wantErr:  "Installation event (installation.created) - nothing to build",
		},
		{
			name:    "unknown event type",
			headers: verifiedJSONHeaders(),
			body:    deliveryJSON("pull_request.opened", `{}`, ""),
			wantErr: "Unsupported Webhook event: pull_request.opened",
		},
		{
			name: "missing verified header",
			headers: http.Header{
				"Content-Type": {"application/json"},
			},
			body:          deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", testAfterSHA, danaPusher, false), testEnrichment),
			wantErrPrefix: "Issue with Headers: X-Bitrise-Entire-Verified header is missing or not '1'",
		},
		{
			name: "verified header with wrong value",
			headers: http.Header{
				"Content-Type":              {"application/json"},
				"X-Bitrise-Entire-Verified": {"true"},
			},
			body:          deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", testAfterSHA, danaPusher, false), testEnrichment),
			wantErrPrefix: "Issue with Headers: X-Bitrise-Entire-Verified header is missing or not '1'",
		},
		{
			name: "missing content type",
			headers: http.Header{
				"X-Bitrise-Entire-Verified": {"1"},
			},
			body:    deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", testAfterSHA, danaPusher, false), testEnrichment),
			wantErr: "Issue with Headers: No Content-Type Header found",
		},
		{
			name: "unsupported content type",
			headers: http.Header{
				"Content-Type":              {"not/supported"},
				"X-Bitrise-Entire-Verified": {"1"},
			},
			body:    deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", testAfterSHA, danaPusher, false), testEnrichment),
			wantErr: "Content-Type is not supported: not/supported",
		},
		{
			name:    "no request body",
			headers: verifiedJSONHeaders(),
			wantErr: "Failed to read content of request body: no or empty request body",
		},
		{
			name:          "malformed JSON",
			headers:       verifiedJSONHeaders(),
			body:          `{"delivery_id": "dlv_01J9", "event": {`,
			wantErrPrefix: "Failed to parse request body: ",
		},
		{
			name:    "missing after",
			headers: verifiedJSONHeaders(),
			body:    deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", "", danaPusher, false), testEnrichment),
			wantErr: "Missing commit hash ('after')",
		},
		{
			name:    "missing ref",
			headers: verifiedJSONHeaders(),
			body:    deliveryJSON("ref.updated", refPayloadJSON("", testAfterSHA, danaPusher, false), testEnrichment),
			wantErr: "Missing ref",
		},
		{
			name:    "ref outside heads and tags",
			headers: verifiedJSONHeaders(),
			body:    deliveryJSON("ref.updated", refPayloadJSON("refs/notes/commits", testAfterSHA, danaPusher, false), testEnrichment),
			wantErr: "Ref (refs/notes/commits) is neither a branch nor a tag ref",
		},
		{
			name:       "null pusher falls back to the default triggered_by",
			headers:    verifiedJSONHeaders(),
			body:       deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", testAfterSHA, "null", false), testEnrichment),
			wantParams: expectedBranchTrigger("main", "Add tests", hookCommon.DefaultTriggeredBy),
		},
		{
			name:       "empty handle falls back to the default triggered_by",
			headers:    verifiedJSONHeaders(),
			body:       deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", testAfterSHA, emptyPusher, false), testEnrichment),
			wantParams: expectedBranchTrigger("main", "Add tests", hookCommon.DefaultTriggeredBy),
		},
		{
			name:       "missing enrichment leaves the commit message empty",
			headers:    verifiedJSONHeaders(),
			body:       deliveryJSON("ref.updated", refPayloadJSON("refs/heads/main", testAfterSHA, danaPusher, false), ""),
			wantParams: expectedBranchTrigger("main", "", "webhook-entire/dana"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.TransformRequest(newRequest(tt.headers, tt.body))

			require.Equal(t, tt.wantSkip, result.ShouldSkip)
			require.False(t, result.DontWaitForTriggerResponse)

			switch {
			case tt.wantErr != "":
				require.EqualError(t, result.Error, tt.wantErr)
				require.Nil(t, result.TriggerAPIParams)
			case tt.wantErrPrefix != "":
				require.Error(t, result.Error)
				require.True(t, strings.HasPrefix(result.Error.Error(), tt.wantErrPrefix), "error %q should start with %q", result.Error, tt.wantErrPrefix)
				require.Nil(t, result.TriggerAPIParams)
			default:
				require.NoError(t, result.Error)
				require.Equal(t, tt.wantParams, result.TriggerAPIParams)
			}
		})
	}
}

func Test_isZeroSHA(t *testing.T) {
	require.True(t, isZeroSHA(zeroSHA))
	require.False(t, isZeroSHA(""))
	require.False(t, isZeroSHA(testAfterSHA))
	require.False(t, isZeroSHA("0000000000000000000000000000000000000001"))
}
