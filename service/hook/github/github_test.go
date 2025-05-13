package github

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
)

const (
	sampleCodePushData = `{
    "ref": "refs/heads/brencs",
    "before": "61be158044aadc36e08b5a01313e25889360ff38",
    "after": "0036f6352b470de6ede9428ab6e44791e5894aaf",
    "repository": {
      "name": "webhook-test",
      "full_name": "test_user/webhook-test",
      "private": true,
      "html_url": "https://github.com/test_user/webhook-test",
      "description": "test repo for webhooks",
      "fork": false,
      "url": "https://github.com/test_user/webhook-test",
      "ssh_url": "git@github.com:test_user/webhook-test.git",
      "clone_url": "https://github.com/test_user/webhook-test.git"
    },
    "pusher": {
      "name": "test_user",
      "email": "test_user@users.noreply.github.com"
    },
    "sender": {
    },
    "created": false,
    "deleted": false,
    "forced": false,
    "base_ref": null,
    "compare": "https://github.com/test_user/webhook-test/compare/61be158044aa...0036f6352b47",
    "commits": [
      {
        "id": "08832fbc2946056b3f0a0022060ed028d62f3e6f",
        "tree_id": "4c3206e509f20adfc7ede21bf6805fe6ad30f77c",
        "distinct": true,
        "message": "commit1",
        "timestamp": "2024-03-11T14:40:53+01:00",
        "url": "https://github.com/test_user/webhook-test/commit/08832fbc2946056b3f0a0022060ed028d62f3e6f",
        "author": {
          "name": "Test User",
          "email": "test.user@bitrise.io",
          "username": "test_user"
        },
        "committer": {
          "name": "Test User",
          "email": "test.user@bitrise.io",
          "username": "test_user"
        },
        "added": [ "added/file/path1" ],
        "removed": [ "removed/file/path1" ],
        "modified": [ "modified/file/path1" ]
      },
      {
        "id": "bf19af0c71a0fc32ffea55d926c299e55d14fae5",
        "tree_id": "3bd8a21192fe87596bdafbe02d510cf4b31a1ded",
        "distinct": true,
        "message": "commit2",
        "timestamp": "2024-03-11T14:41:02+01:00",
        "url": "https://github.com/test_user/webhook-test/commit/bf19af0c71a0fc32ffea55d926c299e55d14fae5",
        "author": {
          "name": "Test User",
          "email": "test.user@bitrise.io",
          "username": "test_user"
        },
        "committer": {
          "name": "Test User",
          "email": "test.user@bitrise.io",
          "username": "test_user"
        },
        "added": [ "added/file/path2" ],
        "removed": [ "removed/file/path2" ],
        "modified": [ "modified/file/path2" ]
      },
      {
        "id": "0036f6352b470de6ede9428ab6e44791e5894aaf",
        "tree_id": "09a572cb4602e70027db2eadceda73f66eff9bfb",
        "distinct": true,
        "message": "commit3",
        "timestamp": "2024-03-11T14:41:10+01:00",
        "url": "https://github.com/test_user/webhook-test/commit/0036f6352b470de6ede9428ab6e44791e5894aaf",
        "author": {
          "name": "Test User",
          "email": "test.user@bitrise.io",
          "username": "test_user"
        },
        "committer": {
          "name": "Test User",
          "email": "test.user@bitrise.io",
          "username": "test_user"
        },
        "added": [ "added/file/path3" ],
        "removed": [ "removed/file/path3" ],
        "modified": [ "modified/file/path3" ]
      }
    ],
    "head_commit": {
      "id": "0036f6352b470de6ede9428ab6e44791e5894aaf",
      "tree_id": "09a572cb4602e70027db2eadceda73f66eff9bfb",
      "distinct": true,
      "message": "commit3",
      "timestamp": "2024-03-11T14:41:10+01:00",
      "url": "https://github.com/test_user/webhook-test/commit/0036f6352b470de6ede9428ab6e44791e5894aaf",
      "author": {
        "name": "Test User",
        "email": "test.user@bitrise.io",
        "username": "test_user"
      },
      "committer": {
        "name": "Test User",
        "email": "test.user@bitrise.io",
        "username": "test_user"
      },
      "added": [ "added/file/path3" ],
      "removed": [ "removed/file/path3" ],
      "modified": [ "modified/file/path3" ]
    }
  }`

	sampleTagPushData = `{
  "ref": "refs/tags/test-tag",
  "before": "0000000000000000000000000000000000000000",
  "after": "0dbf365304fb3001ff58cdcdf18d74699f6e5375",
  "repository": {
    "name": "webhook-test",
    "full_name": "test_user/webhook-test",
    "private": true,
    "owner": {
    },
    "html_url": "https://github.com/test_user/webhook-test",
    "description": "test repo for webhooks",
    "fork": false,
    "url": "https://github.com/test_user/webhook-test",
    "ssh_url": "git@github.com:test_user/webhook-test.git",
    "clone_url": "https://github.com/test_user/webhook-test.git"
  },
  "pusher": {
    "name": "test_user",
    "email": "test_user@users.noreply.github.com"
  },
  "sender": {
  },
  "created": true,
  "deleted": false,
  "forced": false,
  "base_ref": null,
  "compare": "https://github.com/test_user/webhook-test/compare/test-tag",
  "commits": [

  ],
  "head_commit": {
    "id": "0036f6352b470de6ede9428ab6e44791e5894aaf",
    "tree_id": "09a572cb4602e70027db2eadceda73f66eff9bfb",
    "distinct": true,
    "message": "commit3",
    "timestamp": "2024-03-11T14:41:10+01:00",
    "url": "https://github.com/test_user/webhook-test/commit/0036f6352b470de6ede9428ab6e44791e5894aaf",
    "author": {
      "name": "Test User",
      "email": "test.user@bitrise.io",
      "username": "test_user"
    },
    "committer": {
      "name": "Test User",
      "email": "test.user@bitrise.io",
      "username": "test_user"
    },
    "added": [ "added/file/path" ],
    "removed": [ "removed/file/path" ],
    "modified": [ "modified/file/path" ]
  }
}`

	samplePullRequestData = `{
	"action": "opened",
	"number": 12,
	"pull_request": {
		"draft": false,
		"diff_url": "https://github.com/bitrise-io/bitrise-webhooks/pull/1.diff",
		"head": {
			"ref": "feature/github-pr",
			"sha": "83b86e5f286f546dc5a4a58db66ceef44460c85e",
			"repo": {
				"private": false,
				"ssh_url": "git@github.com:oss-contributor/fork-bitrise-webhooks.git",
				"clone_url": "https://github.com/oss-contributor/fork-bitrise-webhooks.git",
				"owner": {
					"login": "bitrise-team"
				}
			}
		},
		"base": {
			"ref": "master",
			"sha": "3c86b996d8014000a93f3c202fc0963e81e56c4c",
			"repo": {
				"private": false,
				"ssh_url": "git@github.com:bitrise-io/bitrise-webhooks.git",
				"clone_url": "https://github.com/bitrise-io/bitrise-webhooks.git",
				"owner": {
					"login": "bitrise-io"
				}
			}
		},
		"title": "PR test",
		"body": "PR text body",
		"merged": false,
		"mergeable": true,
		"user": {
			"login": "Author Name"
		},
        "labels": [
            {
                "id": 6664654046,
                "node_id": "LA_kwDOLdfcTc8AAAABjT6M3g",
                "url": "https://api.github.com/repos/test_user/webhook-test/labels/enhancement",
                "name": "enhancement",
                "color": "a2eeef",
                "default": true,
                "description": "New feature or request"
            },
		    {
		      "id": 6664654053,
		      "node_id": "LA_kwDOLdfcTc8AAAABjT6M5Q",
		      "url": "https://api.github.com/repos/test_user/webhook-test/labels/good%20first%20issue",
		      "name": "good first issue",
		      "color": "7057ff",
		      "default": true,
		      "description": "Good for newcomers"
		    }
        ]
	},
	"sender": {
        "login": "test_user"
    }
}`

	samplePullRequestEditedData = `{
  "action": "edited",
  "number": 12,
  "pull_request": {
		"draft": false,
    "head": {
      "ref": "feature/github-pr",
      "sha": "83b86e5f286f546dc5a4a58db66ceef44460c85e",
      "repo" : {
        "private": false,
        "ssh_url": "git@github.com:bitrise-io/bitrise-webhooks.git",
        "clone_url": "https://github.com/bitrise-io/bitrise-webhooks.git"
      }
    },
    "base": {
      "ref": "develop",
      "sha": "3c86b996d8014000a93f3c202fc0963e81e56c4c",
      "repo" : {
        "private": false,
        "ssh_url": "git@github.com:bitrise-io/bitrise-webhooks.git",
        "clone_url": "https://github.com/bitrise-io/bitrise-webhooks.git"
      }
    },
    "title": "PR test",
    "body": "PR text body",
    "merged": false,
    "mergeable": true,
	"labels": [
		{
			"id": 6664654046,
			"node_id": "LA_kwDOLdfcTc8AAAABjT6M3g",
			"url": "https://api.github.com/repos/test_user/webhook-test/labels/enhancement",
			"name": "enhancement",
			"color": "a2eeef",
			"default": true,
			"description": "New feature or request"
		},
		{
		  "id": 6664654053,
		  "node_id": "LA_kwDOLdfcTc8AAAABjT6M5Q",
		  "url": "https://api.github.com/repos/test_user/webhook-test/labels/good%20first%20issue",
		  "name": "good first issue",
		  "color": "7057ff",
		  "default": true,
		  "description": "Good for newcomers"
		}
	]
  },
  "changes": {
    "body": {
      "from": "this is the PR body"
    },
    "title": {
      "from": "auto-test - title change - without SKIP CI"
    },
    "base": {
      "ref": {
        "from": "master"
      },
      "sha": {
        "from": "bac0e53691fd6fbc5e8c4f00144bf61069b80087"
      }
    }
  },
  "sender": {
	"login": "test_user"
  }
}`

	sampleDraftPullRequestData = `{
		"action": "opened",
		"number": 12,
		"pull_request": {
			"draft": true,
			"diff_url": "https://github.com/bitrise-io/bitrise-webhooks/pull/1.diff",
			"head": {
				"ref": "feature/github-pr",
				"sha": "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				"repo": {
					"private": false,
					"ssh_url": "git@github.com:oss-contributor/fork-bitrise-webhooks.git",
					"clone_url": "https://github.com/oss-contributor/fork-bitrise-webhooks.git",
					"owner": {
						"login": "bitrise-team"
					}
				}
			},
			"base": {
				"ref": "master",
				"sha": "3c86b996d8014000a93f3c202fc0963e81e56c4c",
				"repo": {
					"private": false,
					"ssh_url": "git@github.com:bitrise-io/bitrise-webhooks.git",
					"clone_url": "https://github.com/bitrise-io/bitrise-webhooks.git",
					"owner": {
						"login": "bitrise-io"
					}
				}
			},
			"title": "PR test",
			"body": "PR text body",
			"merged": false,
			"mergeable": true,
			"labels": [
				{
					"id": 6664654046,
					"node_id": "LA_kwDOLdfcTc8AAAABjT6M3g",
					"url": "https://api.github.com/repos/test_user/webhook-test/labels/enhancement",
					"name": "enhancement",
					"color": "a2eeef",
					"default": true,
					"description": "New feature or request"
				},
				{
				  "id": 6664654053,
				  "node_id": "LA_kwDOLdfcTc8AAAABjT6M5Q",
				  "url": "https://api.github.com/repos/test_user/webhook-test/labels/good%20first%20issue",
				  "name": "good first issue",
				  "color": "7057ff",
				  "default": true,
				  "description": "Good for newcomers"
				}
			],
			"user": {
				"login": "Author Name"
			}
		},
		"sender": {
			"login": "test_user"
		}
	}`

	samplePullRequestLabeledData = `{
    "action": "labeled",
    "number": 1,
    "pull_request": {
        "url": "https://api.github.com/repos/test_user/webhook-test/pulls/1",
        "number": 1,
        "state": "open",
        "locked": false,
        "title": "Brencs",
        "user": {},
        "body": null,
        "created_at": "2024-03-08T12:22:56Z",
        "updated_at": "2024-03-08T12:34:57Z",
        "closed_at": null,
        "merged_at": null,
        "merge_commit_sha": "c96fa0dd145083f4d9c0a525fb525fcfb18489ba",
        "assignee": null,
        "assignees": [],
        "requested_reviewers": [],
        "requested_teams": [],
        "labels": [
            {
                "id": 6664654046,
                "node_id": "LA_kwDOLdfcTc8AAAABjT6M3g",
                "url": "https://api.github.com/repos/test_user/webhook-test/labels/enhancement",
                "name": "enhancement",
                "color": "a2eeef",
                "default": true,
                "description": "New feature or request"
            },
		    {
		      "id": 6664654053,
		      "node_id": "LA_kwDOLdfcTc8AAAABjT6M5Q",
		      "url": "https://api.github.com/repos/test_user/webhook-test/labels/good%20first%20issue",
		      "name": "good first issue",
		      "color": "7057ff",
		      "default": true,
		      "description": "Good for newcomers"
		    }
        ],
        "milestone": null,
        "draft": false,
        "head": {
            "label": "test_user:brencs",
            "ref": "brencs",
            "sha": "61be158044aadc36e08b5a01313e25889360ff38",
            "user": {},
            "repo": {
				"name": "webhook-test",
				"full_name": "test_user/webhook-test",
				"private": true,
				"ssh_url": "git@github.com:test_user/webhook-test.git",
				"clone_url": "https://github.com/test_user/webhook-test.git"
			}
        },
        "base": {
            "label": "test_user:main",
            "ref": "main",
            "sha": "17d68567a0ddb19362e3cef6409180af6a02737d",
            "user": {},
            "repo": {
				"name": "webhook-test",
				"full_name": "test_user/webhook-test",
				"private": true,
				"ssh_url": "git@github.com:test_user/webhook-test.git",
				"clone_url": "https://github.com/test_user/webhook-test.git"
			}
        },
		"merged": false,
		"mergeable": true,
		"rebaseable": true,
		"mergeable_state": "clean",
        "commits": 4,
        "additions": 4,
        "deletions": 3,
        "changed_files": 4
    },
    "label": {
        "id": 6664654046,
        "node_id": "LA_kwDOLdfcTc8AAAABjT6M3g",
        "url": "https://api.github.com/repos/test_user/webhook-test/labels/enhancement",
        "name": "enhancement",
        "color": "a2eeef",
        "default": true,
        "description": "New feature or request"
    },
    "repository": {
        "name": "webhook-test",
        "full_name": "test_user/webhook-test",
        "private": true,
        "ssh_url": "git@github.com:test_user/webhook-test.git",
        "clone_url": "https://github.com/test_user/webhook-test.git"
    },
    "sender": {
        "login": "test_user"
    }
}`

	sampleIssueCommentCreatedData = `{
  "action": "created",
  "issue": {
    "url": "https://api.github.com/repos/test_user/webhook-test/issues/4",
    "repository_url": "https://api.github.com/repos/test_user/webhook-test",
    "labels_url": "https://api.github.com/repos/test_user/webhook-test/issues/4/labels{/name}",
    "comments_url": "https://api.github.com/repos/test_user/webhook-test/issues/4/comments",
    "events_url": "https://api.github.com/repos/test_user/webhook-test/issues/4/events",
    "html_url": "https://github.com/test_user/webhook-test/pull/4",
    "id": 2200602723,
    "node_id": "PR_kwDOLdfcTc5qYww6",
    "number": 4,
    "title": "new PR",
    "user": {
      "login": "test_user",
      "id": 517812
    },
    "labels": [
      {
        "id": 6723067658,
        "node_id": "LA_kwDOLdfcTc8AAAABkLnfCg",
        "url": "https://api.github.com/repos/test_user/webhook-test/labels/trigger-other",
        "name": "trigger-other",
        "color": "56826B",
        "default": false,
        "description": ""
      }
    ],
    "state": "open",
    "locked": false,
    "assignee": null,
    "assignees": [
    ],
    "milestone": null,
    "comments": 1,
    "created_at": "2024-03-21T16:12:48Z",
    "updated_at": "2024-04-04T07:51:10Z",
    "closed_at": null,
    "author_association": "OWNER",
    "active_lock_reason": null,
    "draft": false,
    "pull_request": {
      "url": "https://api.github.com/repos/test_user/webhook-test/pulls/4",
      "html_url": "https://github.com/test_user/webhook-test/pull/4",
      "diff_url": "https://github.com/test_user/webhook-test/pull/4.diff",
      "patch_url": "https://github.com/test_user/webhook-test/pull/4.patch",
      "merged_at": null
    },
    "body": "Very detailed description of a pull request.",
    "reactions": {
    },
    "timeline_url": "https://api.github.com/repos/test_user/webhook-test/issues/4/timeline",
    "performed_via_github_app": null,
    "state_reason": null
  },
  "comment": {
    "url": "https://api.github.com/repos/test_user/webhook-test/issues/comments/2036438149",
    "html_url": "https://github.com/test_user/webhook-test/pull/4#issuecomment-2036438149",
    "issue_url": "https://api.github.com/repos/test_user/webhook-test/issues/4",
    "id": 2036438149,
    "node_id": "IC_kwDOLdfcTc55YZSF",
    "user": {
      "login": "test_user",
      "id": 517812
    },
    "created_at": "2024-04-04T07:51:09Z",
    "updated_at": "2024-04-04T07:51:09Z",
    "author_association": "OWNER",
    "body": "first comment",
    "reactions": {
    },
    "performed_via_github_app": null
  },
  "repository": {
    "id": 769121357,
    "node_id": "R_kgDOLdfcTQ",
    "name": "webhook-test",
    "full_name": "test_user/webhook-test",
    "private": true,
    "owner": {
      "login": "test_user",
      "id": 517812
    },
    "html_url": "https://github.com/test_user/webhook-test",
    "description": "test repo for webhooks",
    "ssh_url": "git@github.com:test_user/webhook-test.git",
    "clone_url": "https://github.com/test_user/webhook-test.git",
    "visibility": "private"
  },
  "sender": {
    "login": "test_user",
    "id": 517812
  }
}`

	sampleIssueCommentEditedData = `{
  "action": "edited",
  "changes": {
    "body": {
      "from": "first comment"
    }
  },
  "issue": {
    "url": "https://api.github.com/repos/test_user/webhook-test/issues/4",
    "repository_url": "https://api.github.com/repos/test_user/webhook-test",
    "labels_url": "https://api.github.com/repos/test_user/webhook-test/issues/4/labels{/name}",
    "comments_url": "https://api.github.com/repos/test_user/webhook-test/issues/4/comments",
    "events_url": "https://api.github.com/repos/test_user/webhook-test/issues/4/events",
    "html_url": "https://github.com/test_user/webhook-test/pull/4",
    "id": 2200602723,
    "node_id": "PR_kwDOLdfcTc5qYww6",
    "number": 4,
    "title": "new PR",
    "user": {
      "login": "test_user",
      "id": 517812
    },
    "labels": [
      {
        "id": 6723067658,
        "node_id": "LA_kwDOLdfcTc8AAAABkLnfCg",
        "url": "https://api.github.com/repos/test_user/webhook-test/labels/trigger-other",
        "name": "trigger-other",
        "color": "56826B",
        "default": false,
        "description": ""
      }
    ],
    "state": "open",
    "locked": false,
    "assignee": null,
    "assignees": [
    ],
    "milestone": null,
    "comments": 1,
    "created_at": "2024-03-21T16:12:48Z",
    "updated_at": "2024-04-04T11:48:35Z",
    "closed_at": null,
    "author_association": "OWNER",
    "active_lock_reason": null,
    "draft": false,
    "pull_request": {
      "url": "https://api.github.com/repos/test_user/webhook-test/pulls/4",
      "html_url": "https://github.com/test_user/webhook-test/pull/4",
      "diff_url": "https://github.com/test_user/webhook-test/pull/4.diff",
      "patch_url": "https://github.com/test_user/webhook-test/pull/4.patch",
      "merged_at": null
    },
    "body": "Very detailed description of a pull request.",
    "reactions": {
    },
    "timeline_url": "https://api.github.com/repos/test_user/webhook-test/issues/4/timeline",
    "performed_via_github_app": null,
    "state_reason": null
  },
  "comment": {
    "url": "https://api.github.com/repos/test_user/webhook-test/issues/comments/2036438149",
    "html_url": "https://github.com/test_user/webhook-test/pull/4#issuecomment-2036438149",
    "issue_url": "https://api.github.com/repos/test_user/webhook-test/issues/4",
    "id": 2036438149,
    "node_id": "IC_kwDOLdfcTc55YZSF",
    "user": {
      "login": "test_user",
      "id": 517812
    },
    "created_at": "2024-04-04T07:51:09Z",
    "updated_at": "2024-04-04T11:48:35Z",
    "author_association": "OWNER",
    "body": "I have a much better idea for a comment now.",
    "reactions": {
    },
    "performed_via_github_app": null
  },
  "repository": {
    "id": 769121357,
    "node_id": "R_kgDOLdfcTQ",
    "name": "webhook-test",
    "full_name": "test_user/webhook-test",
    "private": true,
    "owner": {
      "login": "test_user",
      "id": 517812
    },
    "html_url": "https://github.com/test_user/webhook-test",
    "description": "test repo for webhooks",
    "ssh_url": "git@github.com:test_user/webhook-test.git",
    "clone_url": "https://github.com/test_user/webhook-test.git",
    "visibility": "private"
  },
  "sender": {
    "login": "test_user",
    "id": 517812
  }
}`

	sampleMergeQueuePushData = `{
  "ref": "refs/heads/gh-readonly-queue/main/pr-1-7ed40c455464eaa0c5c4a0aeaefb9ffb16bd2c64",
  "before": "0000000000000000000000000000000000000000",
  "after": "cc76bc3a5ffd4836ca30d0eeb224967b7127ab50",
  "repository": {
    "name": "birmacher-test",
    "full_name": "bitrise-io/birmacher-test",
    "private": true,
    "html_url": "https://github.com/bitrise-io/birmacher-test",
    "description": null,
    "fork": false,
    "url": "https://api.github.com/repos/bitrise-io/birmacher-test",
    "ssh_url": "git@github.com:bitrise-io/birmacher-test.git",
    "clone_url": "https://github.com/bitrise-io/birmacher-test.git"
  },
  "pusher": {
    "name": "github-merge-queue[bot]",
    "email": null
  },
  "sender": {
  },
  "created": true,
  "deleted": false,
  "forced": false,
  "base_ref": "refs/heads/main",
  "compare": "https://github.com/bitrise-io/birmacher-test/compare/gh-readonly-queue/main/pr-1-7ed40c455464eaa0c5c4a0aeaefb9ffb16bd2c64",
  "commits": [
  ],
  "head_commit": {
    "id": "cc76bc3a5ffd4836ca30d0eeb224967b7127ab50",
    "tree_id": "ca78a46cdb752ae92599844f4fe30983eacc27de",
    "distinct": true,
    "message": "Merge pull request #1 from bitrise-io/birmacher-patch-1\n\nUpdate README.md",
    "timestamp": "2025-05-12T16:04:25Z",
    "url": "https://github.com/bitrise-io/birmacher-test/commit/cc76bc3a5ffd4836ca30d0eeb224967b7127ab50",
    "author": {
      "name": "Barnabas Birmacher",
      "email": "birmacher@gmail.com",
      "username": "birmacher"
    },
    "committer": {
      "name": "GitHub",
      "email": "noreply@github.com",
      "username": "web-flow"
    },
    "added": [

    ],
    "removed": [

    ],
    "modified": [
      "README.md"
    ]
  }
}`
)

var boolFalse = false
var boolTrue = true
var intOne = 1
var intFour = 4
var intTwelve = 12

func Test_detectContentTypeAndEventID(t *testing.T) {
	t.Log("Push event - should handle")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "push", ghEvent)
	}

	t.Log("Pull Request event - should handle")
	{
		header := http.Header{
			"X-Github-Event": {"pull_request"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "pull_request", ghEvent)
	}

	t.Log("Issue comment event - should handle")
	{
		header := http.Header{
			"X-Github-Event": {"issue_comment"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "issue_comment", ghEvent)
	}

	t.Log("Ping event")
	{
		header := http.Header{
			"X-Github-Event": {"ping"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "ping", ghEvent)
	}

	t.Log("Unsupported GH event - will be handled in Transform")
	{
		header := http.Header{
			"X-Github-Event": {"label"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "label", ghEvent)
	}

	t.Log("Missing X-Github-Event header")
	{
		header := http.Header{
			"Content-Type": {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "No X-Github-Event Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", ghEvent)
	}

	t.Log("Missing Content-Type")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "No Content-Type Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", ghEvent)
	}
}

func Test_transformPushEvent(t *testing.T) {
	t.Log("Do Transform - Code Push")
	{
		codePush := PushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
			Pusher: PusherModel{
				Name: "test_user",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:      "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:   "re-structuring Hook Providers, with added tests",
					CommitMessages:  []string{"re-structuring Hook Providers, with added tests"},
					PushCommitPaths: []bitriseapi.CommitPaths{{}},
					Branch:          "master",
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Do Transform - Tag Push")
	{
		tagPush := PushEventModel{
			Ref: "refs/tags/v0.0.2",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "2e197ebd2330183ae11338151cf3a75db0c23c92",
				CommitMessage: "generalize Push Event (previously Code Push)",
			},
			Pusher: PusherModel{
				Name: "test_user",
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:             "v0.0.2",
					CommitHash:      "2e197ebd2330183ae11338151cf3a75db0c23c92",
					CommitMessage:   "generalize Push Event (previously Code Push)",
					CommitMessages:  []string{"generalize Push Event (previously Code Push)"},
					PushCommitPaths: []bitriseapi.CommitPaths{{}},
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not Distinct Head Commit - should still trigger a build (e.g. this can happen if you rebase-merge a PR, without creating a merge commit)")
	{
		codePush := PushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      false,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
			Pusher: PusherModel{
				Name: "test_user",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:      "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:   "re-structuring Hook Providers, with added tests",
					CommitMessages:  []string{"re-structuring Hook Providers, with added tests"},
					PushCommitPaths: []bitriseapi.CommitPaths{{}},
					Branch:          "master",
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Tag - Not Distinct Head Commit - should still trigger a build")
	{
		tagPush := PushEventModel{
			Ref: "refs/tags/v0.0.2",
			HeadCommit: CommitModel{
				Distinct:      false,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
			Pusher: PusherModel{
				Name: "test_user",
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:             "v0.0.2",
					CommitHash:      "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:   "re-structuring Hook Providers, with added tests",
					CommitMessages:  []string{"re-structuring Hook Providers, with added tests"},
					PushCommitPaths: []bitriseapi.CommitPaths{{}},
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Missing Commit Hash")
	{
		codePush := PushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
			Pusher: PusherModel{
				Name: "test_user",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "missing commit hash")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Missing Commit Hash - Tag")
	{
		tagPush := PushEventModel{
			Ref: "refs/tags/v0.0.2",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
			Pusher: PusherModel{
				Name: "test_user",
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.EqualError(t, hookTransformResult.Error, "missing commit hash")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("This is a 'deleted' event")
	{
		codePush := PushEventModel{
			Deleted: true,
			Ref:     "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
			Pusher: PusherModel{
				Name: "test_user",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "this is a 'Deleted' event, no build can be started")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("This is a 'deleted' event - Tag")
	{
		tagPush := PushEventModel{
			Deleted: true,
			Ref:     "refs/tags/v0.0.2",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
			Pusher: PusherModel{
				Name: "test_user",
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "this is a 'Deleted' event, no build can be started")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a head nor a tag ref")
	{
		codePush := PushEventModel{
			Ref: "refs/not/head",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
			Pusher: PusherModel{
				Name: "test_user",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "ref (refs/not/head) is not a head nor a tag ref")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_transformPullRequestEvent(t *testing.T) {
	t.Log("Unsupported Pull Request action")
	{
		pullRequest := PullRequestEventModel{
			Action: "milestoned",
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "pull Request action doesn't require a build: milestoned")
	}

	t.Log("Empty Pull Request action")
	{
		pullRequest := PullRequestEventModel{}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "no Pull Request action specified")
	}

	t.Log("Already Merged")
	{
		pullRequest := PullRequestEventModel{
			Action: "opened",
			PullRequestInfo: PullRequestInfoModel{
				Merged: true,
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "pull Request already merged")
	}

	t.Log("Not Mergeable")
	{
		pullRequest := PullRequestEventModel{
			Action: "reopened",
			PullRequestInfo: PullRequestInfoModel{
				Merged:    false,
				Mergeable: &boolFalse,
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "pull Request is not mergeable")
	}

	t.Log("Mergeable: not yet decided")
	{
		pullRequest := PullRequestEventModel{
			Action:        "synchronize",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Merged:    false,
				Mergeable: nil,
				Draft:     false,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.False(t, hookTransformResult.ShouldSkip)
		require.NoError(t, hookTransformResult.Error)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test",
					Branch:                           "feature/github-pr",
					BranchDest:                       "master",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:           "",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Mergeable: true")
	{
		pullRequest := PullRequestEventModel{
			Action:        "synchronize",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Merged:    false,
				Mergeable: &boolTrue,
				Draft:     false,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				Labels: []LabelInfoModel{
					{
						ID:   1,
						Name: "first label",
					},
					{
						ID:   2,
						Name: "second label",
					},
				},
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test",
					Branch:                           "feature/github-pr",
					BranchDest:                       "master",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:           "pull/12/merge",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
					PullRequestLabels:                []string{"first label", "second label"},
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - Title & Body")
	{
		pullRequest := PullRequestEventModel{
			Action:        "opened",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: &boolTrue,
				Draft:     false,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test\n\nPR text body",
					Branch:                           "feature/github-pr",
					BranchDest:                       "master",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:           "pull/12/merge",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Draft Pull Request - Title & Body")
	{
		pullRequest := PullRequestEventModel{
			Action:        "synchronize",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: nil,
				Draft:     true,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test\n\nPR text body",
					Branch:                           "feature/github-pr",
					BranchDest:                       "master",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:           "",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments: []bitriseapi.EnvironmentItem{
						{
							Name:     "GITHUB_PR_IS_DRAFT",
							Value:    "true",
							IsExpand: false,
						},
					},
					PullRequestReadyState: bitriseapi.PullRequestReadyStateDraft,
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - edited - only title change - no skip ci change - no build")
	{
		pullRequest := PullRequestEventModel{
			Action:        "edited",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: &boolTrue,
				Draft:     false,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Changes: PullRequestChangesInfoModel{
				Title: PullRequestChangeFromItemModel{
					From: "previous title",
				},
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.EqualError(t, hookTransformResult.Error, "pull Request edit doesn't require a build: only title and/or description was changed, and previous one was not skipped")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel(nil), hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - edited - only title changed - BUT the previous title included a skip CI pattern - should build")
	{
		pullRequest := PullRequestEventModel{
			Action:        "edited",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: &boolTrue,
				Draft:     false,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Changes: PullRequestChangesInfoModel{
				Title: PullRequestChangeFromItemModel{
					From: "previous title with [skip ci] in it",
				},
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test\n\nPR text body",
					Branch:                           "feature/github-pr",
					BranchDest:                       "develop",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:           "pull/12/merge",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - edited - only body/description change - no skip ci in previous - no build")
	{
		pullRequest := PullRequestEventModel{
			Action:        "edited",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: &boolTrue,
				Draft:     false,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Changes: PullRequestChangesInfoModel{
				Body: PullRequestChangeFromItemModel{
					From: "previous body",
				},
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.EqualError(t, hookTransformResult.Error, "pull Request edit doesn't require a build: only title and/or description was changed, and previous one was not skipped")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel(nil), hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - edited - only body/description change - BUT the previous title included a skip CI pattern - should build")
	{
		pullRequest := PullRequestEventModel{
			Action:        "edited",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: &boolTrue,
				Draft:     false,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Changes: PullRequestChangesInfoModel{
				Body: PullRequestChangeFromItemModel{
					From: "previous body with [skip ci] in it",
				},
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test\n\nPR text body",
					Branch:                           "feature/github-pr",
					BranchDest:                       "develop",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:           "pull/12/merge",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - labeled - not open yet - no build")
	{
		pullRequest := PullRequestEventModel{
			Action:        "labeled",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: nil,
				Draft:     false,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				Labels: []LabelInfoModel{
					{
						ID:   1,
						Name: "first label",
					},
					{
						ID:   2,
						Name: "second label",
					},
				},
			},
			Label: &LabelInfoModel{
				ID:   3,
				Name: "third label",
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.EqualError(t, hookTransformResult.Error, "pull Request label added to PR that is not open yet")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel(nil), hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - labeled - mergeable")
	{
		pullRequest := PullRequestEventModel{
			Action:        "labeled",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: &boolTrue,
				Draft:     false,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				Labels: []LabelInfoModel{
					{
						ID:   1,
						Name: "first label",
					},
					{
						ID:   2,
						Name: "second label",
					},
				},
			},
			Label: &LabelInfoModel{
				ID:   3,
				Name: "third label",
			},
			Sender: UserModel{
				Login: "test_user",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test\n\nPR text body",
					Branch:                           "feature/github-pr",
					BranchDest:                       "develop",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:           "pull/12/merge",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
					PullRequestLabels:                []string{"first label", "second label"},
					PullRequestLabelsAdded:           []string{"third label"},
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_transformIssueCommentEvent(t *testing.T) {
	t.Log("Empty issue comment action")
	{
		event := IssueCommentEventModel{}
		hookTransformResult := transformIssueCommentEvent(event)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "no issue comment action specified")
	}

	t.Log("Unsupported issue comment action")
	{
		event := IssueCommentEventModel{
			Action: "deleted",
		}
		hookTransformResult := transformIssueCommentEvent(event)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "issue comment action doesn't require a build: deleted")
	}

	t.Log("Unsupported issue comment type")
	{
		event := IssueCommentEventModel{
			Action: "created",
			Issue:  IssueInfoModel{},
		}
		hookTransformResult := transformIssueCommentEvent(event)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "issue comment is not for a pull request")
	}

	t.Log("Empty PR state")
	{
		event := IssueCommentEventModel{
			Action: "created",
			Issue: IssueInfoModel{
				PullRequest: &IssuePullRequestInfoModel{},
			},
		}
		hookTransformResult := transformIssueCommentEvent(event)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "issue comment is for a pull request that has an unknown state")
	}

	t.Log("PR not open")
	{
		event := IssueCommentEventModel{
			Action: "created",
			Issue: IssueInfoModel{
				PullRequest: &IssuePullRequestInfoModel{},
				State:       "closed",
			},
		}
		hookTransformResult := transformIssueCommentEvent(event)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "issue comment is for a pull request that is not open: closed")
	}

	t.Log("PR already merged")
	{
		event := IssueCommentEventModel{
			Action: "created",
			Issue: IssueInfoModel{
				PullRequest: &IssuePullRequestInfoModel{
					MergedAt: "2024-04-01T01:23:45Z",
				},
				State: "open",
			},
		}
		hookTransformResult := transformIssueCommentEvent(event)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "issue comment is for a pull request that is already merged")
	}
}

func Test_isAcceptPullRequestAction(t *testing.T) {
	t.Log("Accept")
	{
		for _, anAction := range []string{"opened", "reopened", "synchronize", "edited", "ready_for_review", "labeled"} {
			t.Log(" * " + anAction)
			require.Equal(t, true, isAcceptPullRequestAction(anAction))
		}
	}

	t.Log("Don't accept")
	{
		for _, anAction := range []string{"",
			"a", "not-an-action",
			"assigned", "unassigned", "unlabeled", "closed"} {
			t.Log(" * " + anAction)
			require.Equal(t, false, isAcceptPullRequestAction(anAction))
		}
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Ping - should be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"ping"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "ping event received")
	}

	t.Log("Unsupported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"not/supported"},
				"X-Github-Event": {"ping"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: not/supported")
	}

	t.Log("Unsupported event type - should error")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"label"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "unsupported GitHub Webhook event: label")
	}

	t.Log("Push Event - should not be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"push"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "failed to read content of request body: no or empty request body")
	}

	t.Log("Pull Request Event - should not be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"pull_request"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "failed to read content of request body: no or empty request body")
	}

	t.Log("Pull Request Comment Event - should not be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"issue_comment"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "failed to read content of request body: no or empty request body")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"push"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "failed to read content of request body: no or empty request body")
	}

	t.Log("Code Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"push"},
				"Content-Type":   {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:     "0036f6352b470de6ede9428ab6e44791e5894aaf",
					CommitMessage:  "commit3",
					CommitMessages: []string{"commit1", "commit2", "commit3"},
					Branch:         "brencs",
					PushCommitPaths: []bitriseapi.CommitPaths{
						{
							Added:    []string{"added/file/path1"},
							Removed:  []string{"removed/file/path1"},
							Modified: []string{"modified/file/path1"},
						},
						{
							Added:    []string{"added/file/path2"},
							Removed:  []string{"removed/file/path2"},
							Modified: []string{"modified/file/path2"},
						},
						{
							Added:    []string{"added/file/path3"},
							Removed:  []string{"removed/file/path3"},
							Modified: []string{"modified/file/path3"},
						},
					},
					BaseRepositoryURL: "git@github.com:test_user/webhook-test.git",
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Merge Queue Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"push"},
				"Content-Type":   {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(sampleMergeQueuePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:     "cc76bc3a5ffd4836ca30d0eeb224967b7127ab50",
					CommitMessage:  "Merge pull request #1 from bitrise-io/birmacher-patch-1\n\nUpdate README.md",
					CommitMessages: []string{"Merge pull request #1 from bitrise-io/birmacher-patch-1\n\nUpdate README.md"},
					Branch:         "gh-readonly-queue/main/pr-1-7ed40c455464eaa0c5c4a0aeaefb9ffb16bd2c64",
					PushCommitPaths: []bitriseapi.CommitPaths{
						{
							Added:    []string{},
							Removed:  []string{},
							Modified: []string{"README.md"},
						},
					},
					BaseRepositoryURL: "git@github.com:bitrise-io/birmacher-test.git",
					MergeQueue: bitriseapi.MergeQueueModel{
						QueueProvider: "github",
						PullRequestID: 1,
						BaseBranch:    "main",
						BaseSHA:       "7ed40c455464eaa0c5c4a0aeaefb9ffb16bd2c64",
						SyntheticSHA:  "cc76bc3a5ffd4836ca30d0eeb224967b7127ab50",
					},
				},
				TriggeredBy: "webhook-github/github-merge-queue[bot]",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Tag Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"push"},
				"Content-Type":   {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(sampleTagPushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:            "test-tag",
					CommitHash:     "0036f6352b470de6ede9428ab6e44791e5894aaf",
					CommitMessage:  "commit3",
					CommitMessages: []string{"commit3"},
					PushCommitPaths: []bitriseapi.CommitPaths{
						{
							Added:    []string{"added/file/path"},
							Removed:  []string{"removed/file/path"},
							Modified: []string{"modified/file/path"},
						},
					},
					BaseRepositoryURL: "git@github.com:test_user/webhook-test.git",
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"pull_request"},
				"Content-Type":   {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(samplePullRequestData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					DiffURL:                          "https://github.com/bitrise-io/bitrise-webhooks/pull/1.diff",
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test\n\nPR text body",
					Branch:                           "feature/github-pr",
					BranchRepoOwner:                  "bitrise-team",
					BranchDest:                       "master",
					BranchDestRepoOwner:              "bitrise-io",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/oss-contributor/fork-bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/oss-contributor/fork-bitrise-webhooks.git",
					PullRequestAuthor:                "Author Name",
					PullRequestMergeBranch:           "pull/12/merge",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
					PullRequestLabels:                []string{"enhancement", "good first issue"},
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Draft Pull Request - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"pull_request"},
				"Content-Type":   {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(sampleDraftPullRequestData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					DiffURL:                          "https://github.com/bitrise-io/bitrise-webhooks/pull/1.diff",
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test\n\nPR text body",
					Branch:                           "feature/github-pr",
					BranchRepoOwner:                  "bitrise-team",
					BranchDest:                       "master",
					BranchDestRepoOwner:              "bitrise-io",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/oss-contributor/fork-bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/oss-contributor/fork-bitrise-webhooks.git",
					PullRequestAuthor:                "Author Name",
					PullRequestMergeBranch:           "pull/12/merge",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments: []bitriseapi.EnvironmentItem{
						{
							Name:     "GITHUB_PR_IS_DRAFT",
							Value:    "true",
							IsExpand: false,
						},
					},
					PullRequestReadyState: bitriseapi.PullRequestReadyStateDraft,
					PullRequestLabels:     []string{"enhancement", "good first issue"},
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request :: edited - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"pull_request"},
				"Content-Type":   {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(samplePullRequestEditedData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:                       "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:                    "PR test\n\nPR text body",
					Branch:                           "feature/github-pr",
					BranchDest:                       "develop",
					PullRequestID:                    &intTwelve,
					PullRequestRepositoryURL:         "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:                "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:           "pull/12/merge",
					PullRequestUnverifiedMergeBranch: "pull/12/merge",
					PullRequestHeadBranch:            "pull/12/head",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
					PullRequestLabels:                []string{"enhancement", "good first issue"},
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request :: labeled - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"pull_request"},
				"Content-Type":   {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(samplePullRequestLabeledData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:                       "61be158044aadc36e08b5a01313e25889360ff38",
					CommitMessage:                    "Brencs",
					CommitMessages:                   nil,
					Branch:                           "brencs",
					BranchDest:                       "main",
					PullRequestID:                    &intOne,
					PullRequestRepositoryURL:         "git@github.com:test_user/webhook-test.git",
					BaseRepositoryURL:                "git@github.com:test_user/webhook-test.git",
					HeadRepositoryURL:                "git@github.com:test_user/webhook-test.git",
					PullRequestMergeBranch:           "pull/1/merge",
					PullRequestUnverifiedMergeBranch: "pull/1/merge",
					PullRequestHeadBranch:            "pull/1/head",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
					PullRequestLabels:                []string{"enhancement", "good first issue"},
					PullRequestLabelsAdded:           []string{"enhancement"},
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request :: comment created - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"issue_comment"},
				"Content-Type":   {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(sampleIssueCommentCreatedData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitMessage:                    "new PR\n\nVery detailed description of a pull request.",
					BranchDestRepoOwner:              "test_user",
					PullRequestID:                    &intFour,
					PullRequestRepositoryURL:         "git@github.com:test_user/webhook-test.git",
					HeadRepositoryURL:                "git@github.com:test_user/webhook-test.git",
					PullRequestAuthor:                "test_user",
					PullRequestUnverifiedMergeBranch: "pull/4/merge",
					PullRequestHeadBranch:            "pull/4/head",
					DiffURL:                          "https://github.com/test_user/webhook-test/pull/4.diff",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
					PullRequestLabels:                []string{"trigger-other"},
					PullRequestComment:               "first comment",
					PullRequestCommentID:             "2036438149",
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request :: comment updated - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"issue_comment"},
				"Content-Type":   {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(sampleIssueCommentEditedData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitMessage:                    "new PR\n\nVery detailed description of a pull request.",
					BranchDestRepoOwner:              "test_user",
					PullRequestID:                    &intFour,
					PullRequestRepositoryURL:         "git@github.com:test_user/webhook-test.git",
					HeadRepositoryURL:                "git@github.com:test_user/webhook-test.git",
					PullRequestAuthor:                "test_user",
					PullRequestUnverifiedMergeBranch: "pull/4/merge",
					PullRequestHeadBranch:            "pull/4/head",
					DiffURL:                          "https://github.com/test_user/webhook-test/pull/4.diff",
					Environments:                     make([]bitriseapi.EnvironmentItem, 0),
					PullRequestReadyState:            bitriseapi.PullRequestReadyStateReadyForReview,
					PullRequestLabels:                []string{"trigger-other"},
					PullRequestComment:               "I have a much better idea for a comment now.",
					PullRequestCommentID:             "2036438149",
				},
				TriggeredBy: "webhook-github/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_transformPullRequestEvent_readyState(t *testing.T) {
	tests := []struct {
		name           string
		pullRequest    PullRequestEventModel
		wantReadyState bitriseapi.PullRequestReadyState
	}{
		{
			name: "Draft PR opened",
			pullRequest: PullRequestEventModel{
				Action: "opened",
				PullRequestInfo: PullRequestInfoModel{
					Draft: true,
				},
			},
			wantReadyState: bitriseapi.PullRequestReadyStateDraft,
		},
		{
			name: "Draft PR updated with code push",
			pullRequest: PullRequestEventModel{
				Action: "synchronize",
				PullRequestInfo: PullRequestInfoModel{
					Draft: true,
				},
			},
			wantReadyState: bitriseapi.PullRequestReadyStateDraft,
		},
		{
			name: "Draft PR converted to ready to review PR",
			pullRequest: PullRequestEventModel{
				Action: "ready_for_review",
				PullRequestInfo: PullRequestInfoModel{
					Draft: false,
				},
			},
			wantReadyState: bitriseapi.PullRequestReadyStateConvertedToReadyForReview,
		},
		{
			name: "Ready to review PR updated with code push",
			pullRequest: PullRequestEventModel{
				Action: "synchronize",
				PullRequestInfo: PullRequestInfoModel{
					Draft: false,
				},
			},
			wantReadyState: bitriseapi.PullRequestReadyStateReadyForReview,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformPullRequestEvent(tt.pullRequest)
			require.Equal(t, 1, len(got.TriggerAPIParams))
			require.Equal(t, tt.wantReadyState, got.TriggerAPIParams[0].BuildParams.PullRequestReadyState)
		})
	}
}
