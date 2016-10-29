package visualstudioteamservices

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/stretchr/testify/require"
)

const (
	sampleCodeEmptySubscriptionID = `{
		"subscriptionId": "00000000-0000-0000-0000-000000000000",
		"notificationId": 4,
		"id": "daae438c-296b-4512-b08e-571910874e9b",
		"eventType": "git.push",
		"publisherId": "tfs"
	}`

	sampleCodeGitPushBadEventType = `{
		"subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
		"notificationId": 4,
		"id": "daae438c-296b-4512-b08e-571910874e9b",
		"eventType": "message.posted",
		"publisherId": "tfs"
	}`

	sampleCodeGitPushBadPublisherID = `{
		"subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
		"notificationId": 4,
		"id": "daae438c-296b-4512-b08e-571910874e9b",
		"eventType": "git.push",
		"publisherId": "-"
	}`

	sampleCodeGitPushWithNoChanges = `{
	  "subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
	  "notificationId": 10,
	  "id": "03c164c2-8912-4d5e-8009-3707d5f83734",
	  "eventType": "git.push",
	  "publisherId": "tfs",
	  "resource": {
	    "commits": [],
	    "refUpdates": [
	      {
	        "name": "refs/heads/master",
	        "oldObjectId": "aad331d8d3b131fa9ae03cf5e53965b51942618a",
	        "newObjectId": "33b55f7cb7e7e245323987634f960cf4a6e6bc74"
	      }
	    ]
	  }
	}`

	sampleCodeGitPushWithNoBranchInformation = `{
	  "subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
	  "notificationId": 10,
	  "id": "03c164c2-8912-4d5e-8009-3707d5f83734",
	  "eventType": "git.push",
	  "publisherId": "tfs",
	  "resource": {
	    "commits": [
				{
					"commitId": "33b55f7cb7e7e245323987634f960cf4a6e6bc74",
					"author": {
						"name": "Jamal Hartnett",
						"email": "fabrikamfiber4@hotmail.com",
						"date": "2015-02-25T19:01:00Z"
					},
					"committer": {
						"name": "Jamal Hartnett",
						"email": "fabrikamfiber4@hotmail.com",
						"date": "2015-02-25T19:01:00Z"
					},
					"comment": "Fixed bug",
					"url": "https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/commit/33b55f7cb7e7e245323987634f960cf4a6e6bc74"
				}
			],
	    "refUpdates": []
	  }
	}`

	sampleCodeGitPushWithBadlyFormattedBranchInformation = `{
	  "subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
	  "notificationId": 10,
	  "id": "03c164c2-8912-4d5e-8009-3707d5f83734",
	  "eventType": "git.push",
	  "publisherId": "tfs",
	  "resource": {
	    "commits": [
				{
					"commitId": "33b55f7cb7e7e245323987634f960cf4a6e6bc74",
					"author": {
						"name": "Jamal Hartnett",
						"email": "fabrikamfiber4@hotmail.com",
						"date": "2015-02-25T19:01:00Z"
					},
					"committer": {
						"name": "Jamal Hartnett",
						"email": "fabrikamfiber4@hotmail.com",
						"date": "2015-02-25T19:01:00Z"
					},
					"comment": "Fixed bug",
					"url": "https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/commit/33b55f7cb7e7e245323987634f960cf4a6e6bc74"
				}
			],
	    "refUpdates": [
	      {
	        "name": "refs/invalid",
	        "oldObjectId": "aad331d8d3b131fa9ae03cf5e53965b51942618a",
	        "newObjectId": "33b55f7cb7e7e245323987634f960cf4a6e6bc74"
	      }
	    ]
	  }
	}`

	sampleCodeGitPush = `{
	  "subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
	  "notificationId": 10,
	  "id": "03c164c2-8912-4d5e-8009-3707d5f83734",
	  "eventType": "git.push",
	  "publisherId": "tfs",
	  "message": {
	    "text": "Jamal Hartnett pushed updates to branch master of repository Fabrikam-Fiber-Git.",
	    "html": "Jamal Hartnett pushed updates to branch master of repository Fabrikam-Fiber-Git.",
	    "markdown": "Jamal Hartnett pushed updates to branch master of repository Fabrikam-Fiber-Git."
	  },
	  "detailedMessage": {
	    "text": "Jamal Hartnett pushed 1 commit to branch master of repository Fabrikam-Fiber-Git.\n - Fixed bug 33b55f7c",
	    "html": "Jamal Hartnett pushed 1 commit to branch <a href=\"https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/#version=GBmaster\">master</a> of repository <a href=\"https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/\">Fabrikam-Fiber-Git</a>.\n<ul>\n<li>Fixed bug <a href=\"https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/commit/33b55f7cb7e7e245323987634f960cf4a6e6bc74\">33b55f7c</a>\n</ul>",
	    "markdown": "Jamal Hartnett pushed 1 commit to branch [master](https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/#version=GBmaster) of repository [Fabrikam-Fiber-Git](https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/).\n* Fixed bug [33b55f7c](https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/commit/33b55f7cb7e7e245323987634f960cf4a6e6bc74)"
	  },
	  "resource": {
	    "commits": [
	      {
	        "commitId": "33b55f7cb7e7e245323987634f960cf4a6e6bc74",
	        "author": {
	          "name": "Jamal Hartnett",
	          "email": "fabrikamfiber4@hotmail.com",
	          "date": "2015-02-25T19:01:00Z"
	        },
	        "committer": {
	          "name": "Jamal Hartnett",
	          "email": "fabrikamfiber4@hotmail.com",
	          "date": "2015-02-25T19:01:00Z"
	        },
	        "comment": "Fixed bug",
	        "url": "https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/commit/33b55f7cb7e7e245323987634f960cf4a6e6bc74"
	      }
	    ],
	    "refUpdates": [
	      {
	        "name": "refs/heads/master",
	        "oldObjectId": "aad331d8d3b131fa9ae03cf5e53965b51942618a",
	        "newObjectId": "33b55f7cb7e7e245323987634f960cf4a6e6bc74"
	      }
	    ],
	    "repository": {
	      "id": "278d5cd2-584d-4b63-824a-2ba458937249",
	      "name": "Fabrikam-Fiber-Git",
	      "url": "https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_apis/git/repositories/278d5cd2-584d-4b63-824a-2ba458937249",
	      "project": {
	        "id": "6ce954b1-ce1f-45d1-b94d-e6bf2464ba2c",
	        "name": "Fabrikam-Fiber-Git",
	        "url": "https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_apis/projects/6ce954b1-ce1f-45d1-b94d-e6bf2464ba2c",
	        "state": "wellFormed"
	      },
	      "defaultBranch": "refs/heads/master",
	      "remoteUrl": "https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git"
	    },
	    "pushedBy": {
	      "id": "00067FFED5C7AF52@Live.com",
	      "displayName": "Jamal Hartnett",
	      "uniqueName": "Windows Live ID\\fabrikamfiber4@hotmail.com"
	    },
	    "pushId": 14,
	    "date": "2014-05-02T19:17:13.3309587Z",
	    "url": "https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_apis/git/repositories/278d5cd2-584d-4b63-824a-2ba458937249/pushes/14"
	  },
	  "createdDate": "2016-02-17T21:29:54.5474864Z"
	}`

	sampleCodeGitPushWithMultipleCommits = `{
    "subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
    "notificationId": 10,
    "id": "03c164c2-8912-4d5e-8009-3707d5f83734",
    "eventType": "git.push",
    "publisherId": "tfs",
    "resource": {
      "commits": [
        {
          "commitId": "0c23515bcd14e30961356a0a129c732asd9d0wds",
          "author": {
            "name": "Jamal Hartnett",
            "email": "fabrikamfiber4@hotmail.com",
            "date": "2015-02-25T19:02:00Z"
          },
          "committer": {
            "name": "Jamal Hartnett",
            "email": "fabrikamfiber4@hotmail.com",
            "date": "2015-02-25T19:02:00Z"
          },
          "comment": "More changes",
          "url": "https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/commit/33b55f7cb7e7e245323987634f960cf4a6e6bc74"
        },
        {
          "commitId": "33b55f7cb7e7e245323987634f960cf4a6e6bc74",
          "author": {
            "name": "Jamal Hartnett",
            "email": "fabrikamfiber4@hotmail.com",
            "date": "2015-02-25T19:01:00Z"
          },
          "committer": {
            "name": "Jamal Hartnett",
            "email": "fabrikamfiber4@hotmail.com",
            "date": "2015-02-25T19:01:00Z"
          },
          "comment": "Fixed bug",
          "url": "https://fabrikam-fiber-inc.visualstudio.com/DefaultCollection/_git/Fabrikam-Fiber-Git/commit/33b55f7cb7e7e245323987634f960cf4a6e6bc74"
        }
      ],
      "refUpdates": [
        {
          "name": "refs/heads/master",
          "oldObjectId": "aad331d8d3b131fa9ae03cf5e53965b51942618a",
          "newObjectId": "33b55f7cb7e7e245323987634f960cf4a6e6bc74"
        }
      ]
    }
  }`
)

func Test_detectContentType(t *testing.T) {
	t.Log("Proper Content-Type")
	{
		header := http.Header{
			"Content-Type": {"application/json; charset=utf-8"},
		}
		contentType, err := detectContentType(header)
		require.NoError(t, err)
		require.Equal(t, "application/json; charset=utf-8", contentType)
	}
	t.Log("Missing Content-Type")
	{
		header := http.Header{}
		contentType, err := detectContentType(header)
		require.EqualError(t, err, "Issue with Content-Type Header: No value found in HEADER for the key: Content-Type")
		require.Equal(t, "", contentType)
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Unsupported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/x-www-form-urlencoded"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: application/x-www-form-urlencoded")
	}

	t.Log("Missing Content-Type")
	{
		request := http.Request{
			Header: http.Header{},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Issue with Content-Type Header: No value found in HEADER for the key: Content-Type")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("Initial Subscription ID")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodeEmptySubscriptionID)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "Initial (test) event detected, skipping.")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Bad publisher id")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodeGitPushBadPublisherID)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "Not a Team Foundation Server notification, can't start a build.")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Bad event type")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodeGitPushBadEventType)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "Not a push event, can't start a build.")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Empty commit list")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodeGitPushWithNoChanges)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "No 'commits' included in the webhook, can't start a build.")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Empty branch information")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodeGitPushWithNoBranchInformation)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "Can't detect branch information (resource.refUpdates is empty), can't start a build.")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Badly formatted branch information")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodeGitPushWithBadlyFormattedBranchInformation)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "Unsupported refs/, can't start a build: refs/invalid")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Git.push with one commit")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodeGitPush)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "33b55f7cb7e7e245323987634f960cf4a6e6bc74",
					CommitMessage: "Fixed bug",
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Git.push with multiple commits - only the first one (latest commit) should be picked")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodeGitPushWithMultipleCommits)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "0c23515bcd14e30961356a0a129c732asd9d0wds",
					CommitMessage: "More changes",
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Git.push - Tag (create)")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(`{
  "subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
  "eventType": "git.push",
  "publisherId": "tfs",
  "resource": {
    "refUpdates": [
      {
        "name": "refs/tags/v0.0.2",
        "oldObjectId": "0000000000000000000000000000000000000000",
        "newObjectId": "7c0d90dc542b86747e42ac8f03f794a96ecfc68a"
      }
    ]
  }
}`)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:        "v0.0.2",
					CommitHash: "7c0d90dc542b86747e42ac8f03f794a96ecfc68a",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Git.push - Tag Delete")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(`{
  "subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
  "eventType": "git.push",
  "publisherId": "tfs",
  "resource": {
    "refUpdates": [
      {
        "name": "refs/tags/v0.0.2",
        "oldObjectId": "7c0d90dc542b86747e42ac8f03f794a96ecfc68a",
        "newObjectId": "0000000000000000000000000000000000000000"
      }
    ]
  }
}`)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "Tag delete event - does not require a build")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Git.push - Branch Delete")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(`{
  "subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
  "eventType": "git.push",
  "publisherId": "tfs",
  "resource": {
    "refUpdates": [
      {
        "name": "refs/heads/test-branch",
        "oldObjectId": "7c0d90dc542b86747e42ac8f03f794a96ecfc68a",
        "newObjectId": "0000000000000000000000000000000000000000"
      }
    ]
  }
}`)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "Branch delete event - does not require a build")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Git.push - Branch Created")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(`{
  "subscriptionId": "f0c23515-bcd1-4e30-9613-56a0a129c732",
  "eventType": "git.push",
  "publisherId": "tfs",
  "resource": {
    "refUpdates": [
      {
        "name": "refs/heads/test-branch",
        "oldObjectId": "0000000000000000000000000000000000000000",
        "newObjectId": "7c0d90dc542b86747e42ac8f03f794a96ecfc68a"
      }
    ]
  }
}`)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:        "test-branch",
					CommitHash:    "7c0d90dc542b86747e42ac8f03f794a96ecfc68a",
					CommitMessage: "Branch created",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}
}
