package assembla

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"io/ioutil"
	"strings"
)

func Test_detectContentType(t *testing.T) {
	t.Log("Push event - should handle")
	{
		header := http.Header{
			"Content-Type":     {"application/json"},
		}
		contentType, err := detectContentType(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
	}
}

func Test_transformPushEvent(t *testing.T) {
	t.Log("Do Transform - code push")
	{
		pushEvent := PushEventModel{
			SpaceEventModel: SpaceEventModel{
				Space: "Space name",
				Action: "committed",
				Object: "Changeset",
			},
			MessageEventModel: MessageEventModel{
				Title: "1 commits [branchname]",
				Body: "ErikPoort pushed 1 commits [branchname]\n",
				Author: "ErikPoort",
			},
			GitEventModel: GitEventModel{
				RepositorySuffix: "origin",
				RepositoryURL: "git@git.assembla.com:username/project.git",
				Branch: "branchname",
				CommitID: "sha1chars11",
			},
		}

		// OK
		{
			hookTransformResult := transformPushEvent(pushEvent)
			err := detectAssemblaData(pushEvent)
			require.Equal(t, err, nil)
			require.NoError(t, hookTransformResult.Error)
			require.False(t, hookTransformResult.ShouldSkip)
			require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						CommitMessage: "ErikPoort pushed 1 commits [branchname]\n",
						Branch:        "branchname",
						CommitHash:    "sha1chars11",
					},
					TriggeredBy: "webhook",
				},
			}, hookTransformResult.TriggerAPIParams)
			require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
		}
	}
}

func Test_incorrectPostOptions(t *testing.T) {
	t.Log("Git Push update")
	{
		pushEvent := PushEventModel{
			SpaceEventModel: SpaceEventModel{
				Space: "Space name",
				Action: "committed",
				Object: "Changeset",
			},
			MessageEventModel: MessageEventModel{
				Title: "1 commits [branchname]",
				Body: "ErikPoort pushed 1 commits [branchname]\n",
				Author: "ErikPoort",
			},
			GitEventModel: GitEventModel{
				RepositorySuffix: "---",
				RepositoryURL: "---",
				Branch: "---",
				CommitID: "---",
			},
		}

		// OK
		{
			err := detectAssemblaData(pushEvent)
			require.EqualError(t, err, "Webhook is not correctly setup, make sure you post updates about 'Code commits' in Assembla")
		}
	}
}

func Test_emptyGitEventOptions(t *testing.T) {
	t.Log("Git Push update")
	{
		pushEvent := PushEventModel{
			SpaceEventModel: SpaceEventModel{
				Space: "Space name",
				Action: "committed",
				Object: "Changeset",
			},
			MessageEventModel: MessageEventModel{
				Title: "1 commits [branchname]",
				Body: "ErikPoort pushed 1 commits [branchname]\n",
				Author: "ErikPoort",
			},
			GitEventModel: GitEventModel{
				RepositorySuffix: "",
				RepositoryURL: "",
				Branch: "",
				CommitID: "",
			},
		}

		// OK
		{
			err := detectAssemblaData(pushEvent)
			require.EqualError(t, err, "Webhook is not correctly setup, make sure you post updates about 'Code commits' in Assembla")
		}
	}
}

const (
	sampleCodePushData = `{
		"assembla": {
	  		"space": "Space name",
	  		"action": "committed",
	  		"object": "Changeset"
	  	},
	  	"message": {
	  		"title": "1 commits [branchname]",
	  		"body": "ErikPoort pushed 1 commits [branchname]\n",
	  		"author": "ErikPoort"
		},
		"git": {
	  		"repository_suffix": "origin",
	  		"repository_url": "git@git.assembla.com:username/project.git",
	  		"branch": "branchname",
	  		"commit_id": "sha1chars11"
		}
	}`
	sampleIncorrectJSONData = `{
		"assembla": {
	  		"space": "Space name",
	  		"action": "committed",
	  		"object": "Changeset",
	  	},
	  	"message": {
	  		"title": "1 commits [branchname]",
	  		"body": "ErikPoort pushed 1 commits [branchname]\n",
	  		"author": "ErikPoort",
		},
		"git": {
	  		"repository_suffix": "origin",
	  		"repository_url": "git@git.assembla.com:username/project.git",
	  		"branch": "branchname",
	  		"commit_id": "sha1chars11",
		}
	}`
)

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Unsupported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":     {"not/supported"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: not/supported")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":     {"application/json"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("Test with Sample Code Push data")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":     {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitMessage: "ErikPoort pushed 1 commits [branchname]\n",
					Branch:        "branchname",
					CommitHash:    "sha1chars11",
				},
				TriggeredBy: "webhook",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_IncorrectJSONData(t *testing.T) {
	provider := HookProvider{}

	t.Log("Test with incorrect JSON data")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":     {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleIncorrectJSONData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.Error(t, hookTransformResult.Error)
	}
}