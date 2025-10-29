package bitbucketserver

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
)

const (
	sampleCodePushData = `{
  "eventKey":"repo:refs_changed",
  "date":"2017-09-19T09:45:32+1000",
  "actor":{
    "name":"admin",
    "emailAddress":"admin@example.com",
    "id":1,
    "displayName":"Administrator",
    "active":true,
    "slug":"admin",
    "type":"NORMAL"
  },
  "repository":{
    "slug":"repository",
    "id":84,
    "name":"repository",
    "scmId":"git",
    "state":"AVAILABLE",
    "statusMessage":"Available",
    "forkable":true,
    "project":{
      "key":"PROJ",
      "id":84,
      "name":"project",
      "public":false,
      "type":"NORMAL"
    },
    "public":false,
	"links": {
		"clone": [
			{
				"name": "ssh",
				"href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
			},
			{
				"name": "http",
				"href": "https://bitbucket.example.com/scm/test/repo.git"
			}
		],
		"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
	}
  },
  "changes":[
    {
      "ref":{
        "id":"refs/heads/master",
        "displayId":"master",
        "type":"BRANCH"
      },
      "refId":"refs/heads/master",
      "fromHash":"from-hash-1",
      "toHash":"to-hash-1",
      "type":"UPDATE"
    },
    {
      "ref":{
        "id":"refs/heads/master",
        "displayId":"a-branch",
        "type":"BRANCH"
      },
      "refId":"refs/heads/master",
      "fromHash":"from-hash-2",
      "toHash":"to-hash-2",
      "type":"UPDATE"
    }
  ],
  "commits": [
    {
      "id": "a00945762949b7b787ecabc388c0e20b1b85f0b4",
      "displayId": "a0094576294",
      "author": {
        "name": "Administrator",
        "emailAddress": "admin@example.com"
      },
      "authorTimestamp": 1673403328000,
      "committer": {
        "name": "Administrator",
        "emailAddress": "admin@example.com"
      },
      "committerTimestamp": 1673403328000,
      "message": "My commit message",
      "parents": [
          {
            "id": "197a3e0d2f9a2b3ed1c4fe5923d5dd701bee9fdd",
            "displayId": "197a3e0d2f9"
          }
      ]
    }
  ]
}`

	sampleTagPushData = `{
  "eventKey": "repo:refs_changed",
  "date": "2017-12-08T12:19:44+0100",
  "actor": {
    "name": "user",
    "displayName": "User",
    "slug": "user-slug"
  },
  "repository": {
    "slug": "android",
    "id": 2,
    "name": "Android",
    "scmId": "git",
    "state": "AVAILABLE",
    "statusMessage": "Available",
    "forkable": true,
    "project": {
      "key": "APP",
      "id": 2,
      "name": "App",
      "public": false,
      "type": "NORMAL"
    },
    "public": false,
	"links": {
		"clone": [
			{
				"name": "ssh",
				"href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
			},
			{
				"name": "http",
				"href": "https://bitbucket.example.com/scm/test/repo.git"
			}
		],
		"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
	}
  },
  "changes": [
    {
      "ref": {
        "id": "refs/tags/3.0.4",
        "displayId": "3.0.4",
        "type": "TAG"
      },
      "refId": "refs/tags/3.0.4",
      "fromHash": "0000000000000000000000000000000000000000",
      "toHash": "2943d981c36ca9a241326a8c9520bec15edef8c5",
      "type": "ADD"
    }
  ]
}`

	samplePullRequestData = `{
  "eventKey":"pr:opened",
  "date":"2017-09-19T09:58:11+1000",
  "actor":{
    "name":"admin",
    "emailAddress":"admin@example.com",
    "id":1,
    "displayName":"Administrator",
    "active":true,
    "slug":"admin",
    "type":"NORMAL"
  },
  "pullRequest":{
    "id":1,
    "version":0,
    "title":"a new file added",
    "state":"OPEN",
    "open":true,
    "closed":false,
    "createdDate":1505779091796,
    "updatedDate":1505779091796,
    "fromRef":{
      "id":"refs/heads/a-branch",
      "displayId":"a-branch",
      "latestCommit":"ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
      "repository":{
        "slug":"repository",
        "id":84,
        "name":"repository",
        "scmId":"git",
        "state":"AVAILABLE",
        "statusMessage":"Available",
        "forkable":true,
        "project":{
          "key":"PROJ",
          "id":84,
          "name":"project",
          "public":false,
          "type":"NORMAL"
        },
        "public":false,
        "links": {
            "clone": [
                {
                    "name": "ssh",
                    "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                },
                {
                    "name": "http",
                    "href": "https://bitbucket.example.com/scm/test/repo.git"
                }
            ],
            "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
        }
      }
    },
    "toRef":{
      "id":"refs/heads/master",
      "displayId":"master",
      "latestCommit":"178864a7d521b6f5e720b386b2c2b0ef8563e0dc",
      "repository":{
        "slug":"repository",
        "id":84,
        "name":"repository",
        "scmId":"git",
        "state":"AVAILABLE",
        "statusMessage":"Available",
        "forkable":true,
        "project":{
          "key":"PROJ",
          "id":84,
          "name":"project",
          "public":false,
          "type":"NORMAL"
        },
        "public":false,
        "links": {
            "clone": [
                {
                    "name": "ssh",
                    "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                },
                {
                    "name": "http",
                    "href": "https://bitbucket.example.com/scm/test/repo.git"
                }
            ],
            "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
        }
      }
    },
    "locked":false,
    "author":{
      "user":{
        "name":"admin",
        "emailAddress":"admin@example.com",
        "id":1,
        "displayName":"Administrator",
        "active":true,
        "slug":"admin",
        "type":"NORMAL"
      },
      "role":"AUTHOR",
      "approved":false,
      "status":"UNAPPROVED"
    },
    "reviewers":[

    ],
    "participants":[

    ],
    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/pull-requests/1"}]}
  }
}`

	samplePullRequestModifiedData = `{
"eventKey":"pr:modified",
"date":"2017-09-19T09:58:11+1000",
"actor":{
    "name":"admin",
    "emailAddress":"admin@example.com",
    "id":1,
    "displayName":"Administrator",
    "active":true,
    "slug":"admin",
    "type":"NORMAL"
},
"pullRequest":{
    "id":1,
    "version":0,
    "title":"a new file added",
    "state":"OPEN",
    "open":true,
    "closed":false,
    "createdDate":1505779091796,
    "updatedDate":1505779091796,
    "fromRef":{
        "id":"refs/heads/a-branch",
        "displayId":"a-branch",
        "latestCommit":"ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
        "repository":{
            "slug":"repository",
            "id":84,
            "name":"repository",
            "scmId":"git",
            "state":"AVAILABLE",
            "statusMessage":"Available",
            "forkable":true,
            "project":{
                "key":"PROJ",
                "id":84,
                "name":"project",
                "public":false,
                "type":"NORMAL"
            },
            "public":false,
            "links": {
                "clone": [
                    {
                        "name": "ssh",
                        "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                    },
                    {
                        "name": "http",
                        "href": "https://bitbucket.example.com/scm/test/repo.git"
                    }
                ],
                "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
            }
        }
    },
    "toRef":{
        "id":"refs/heads/master",
        "displayId":"master",
        "latestCommit":"178864a7d521b6f5e720b386b2c2b0ef8563e0dc",
        "repository":{
            "slug":"repository",
            "id":84,
            "name":"repository",
            "scmId":"git",
            "state":"AVAILABLE",
            "statusMessage":"Available",
            "forkable":true,
            "project":{
                "key":"PROJ",
                "id":84,
                "name":"project",
                "public":false,
                "type":"NORMAL"
            },
            "public":false,
            "links": {
                "clone": [
                    {
                        "name": "ssh",
                        "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                    },
                    {
                        "name": "http",
                        "href": "https://bitbucket.example.com/scm/test/repo.git"
                    }
                ],
                "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
            }
        }
    },
    "locked":false,
    "author":{
        "user":{
            "name":"admin",
            "emailAddress":"admin@example.com",
            "id":1,
            "displayName":"Administrator",
            "active":true,
            "slug":"admin",
            "type":"NORMAL"
        },
        "role":"AUTHOR",
        "approved":false,
        "status":"UNAPPROVED"
    },
    "reviewers":[

    ],
    "participants":[

    ],
    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/pull-requests/1"}]}
}
}`

	samplePullRequestFromRefUpdatedData = `{
    "eventKey":"pr:from_ref_updated",
    "date":"2017-09-19T09:58:11+1000",
    "actor":{
    "name":"admin",
    "emailAddress":"admin@example.com",
    "id":1,
    "displayName":"Administrator",
    "active":true,
    "slug":"admin",
    "type":"NORMAL"
    },
    "pullRequest":{
    "id":1,
    "version":0,
    "title":"a new file added",
    "state":"OPEN",
    "open":true,
    "closed":false,
    "createdDate":1505779091796,
    "updatedDate":1505779091796,
    "fromRef":{
        "id":"refs/heads/a-branch",
        "displayId":"a-branch",
        "latestCommit":"ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
        "repository":{
            "slug":"repository",
            "id":84,
            "name":"repository",
            "scmId":"git",
            "state":"AVAILABLE",
            "statusMessage":"Available",
            "forkable":true,
            "project":{
                "key":"PROJ",
                "id":84,
                "name":"project",
                "public":false,
                "type":"NORMAL"
            },
            "public":false,
            "links": {
                "clone": [
                    {
                        "name": "ssh",
                        "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                    },
                    {
                        "name": "http",
                        "href": "https://bitbucket.example.com/scm/test/repo.git"
                    }
                ],
                "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
            }
        }
    },
    "toRef":{
        "id":"refs/heads/master",
        "displayId":"master",
        "latestCommit":"178864a7d521b6f5e720b386b2c2b0ef8563e0dc",
        "repository":{
            "slug":"repository",
            "id":84,
            "name":"repository",
            "scmId":"git",
            "state":"AVAILABLE",
            "statusMessage":"Available",
            "forkable":true,
            "project":{
                "key":"PROJ",
                "id":84,
                "name":"project",
                "public":false,
                "type":"NORMAL"
            },
            "public":false,
            "links": {
                "clone": [
                    {
                        "name": "ssh",
                        "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                    },
                    {
                        "name": "http",
                        "href": "https://bitbucket.example.com/scm/test/repo.git"
                    }
                ],
                "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
            }
        }
    },
    "locked":false,
    "author":{
        "user":{
            "name":"admin",
            "emailAddress":"admin@example.com",
            "id":1,
            "displayName":"Administrator",
            "active":true,
            "slug":"admin",
            "type":"NORMAL"
        },
        "role":"AUTHOR",
        "approved":false,
        "status":"UNAPPROVED"
    },
    "reviewers":[
    
    ],
    "participants":[
    
    ],
    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/pull-requests/1"}]}
    }
}`

	samplePullRequestMergedData = `{
    "eventKey": "pr:merged",
    "date": "2017-09-19T10:39:36+1000",
    "actor": {
        "name": "user",
        "emailAddress": "user@example.com",
        "id": 2,
        "displayName": "User",
        "active": true,
        "slug": "user",
        "type": "NORMAL"
    },
    "pullRequest": {
        "id": 9,
        "version": 2,
        "title": "Awesome feature",
        "state": "MERGED",
        "open": false,
        "closed": true,
        "createdDate": 1505781560908,
        "updatedDate": 1505781576361,
        "closedDate": 1505781576361,
        "fromRef": {
            "id": "refs/heads/admin/file-1505781548644",
            "displayId": "admin/file-1505781548644",
            "latestCommit": "45f9690c928915a5e1c4366d5ee1985eea03f05d",
            "repository": {
                "slug": "repository",
                "id": 84,
                "name": "repository",
                "scmId": "git",
                "state": "AVAILABLE",
                "statusMessage": "Available",
                "forkable": true,
                "project": {
                    "key": "PROJ",
                    "id": 84,
                    "name": "project",
                    "public": false,
                    "type": "NORMAL"
                },
                "public": false,
                "links": {
                    "clone": [
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        },
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                }
            }
        },
        "toRef": {
            "id": "refs/heads/master",
            "displayId": "master",
            "latestCommit": "8d2ad38c918fa6943859fca2176c89ea98b92a21",
            "repository": {
                "slug": "repository",
                "id": 84,
                "name": "repository",
                "scmId": "git",
                "state": "AVAILABLE",
                "statusMessage": "Available",
                "forkable": true,
                "project": {
                    "key": "PROJ",
                    "id": 84,
                    "name": "project",
                    "public": false,
                    "type": "NORMAL"
                },
                "public": false,
                "links": {
                    "clone": [
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        },
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                }
            }
        },
        "locked": false,
        "author": {
            "user": {
                "name": "admin",
                "emailAddress": "admin@example.com",
                "id": 1,
                "displayName": "Administrator",
                "active": true,
                "slug": "admin",
                "type": "NORMAL"
            },
            "role": "AUTHOR",
            "approved": false,
            "status": "UNAPPROVED"
        },
        "reviewers": [

        ],
        "participants": [{
            "user": {
                "name": "user",
                "emailAddress": "user@example.com",
                "id": 2,
                "displayName": "User",
                "active": true,
                "slug": "user",
                "type": "NORMAL"
            },
            "role": "PARTICIPANT",
            "approved": false,
            "status": "UNAPPROVED"
        }],
        "properties": {
            "mergeCommit": {
                "displayId": "7e48f426f0a",
                "id": "7e48f426f0a6e47c5b5e862c31be6ca965f82c9c"
            }
        },
        "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/pull-requests/1"}]}
    }
}`

	samplePRCommentAddedData = `{
    "date": "2025-10-27T15:54:08+0000",
    "actor": {
        "emailAddress": "test_user@example.com",
        "displayName": "admin",
        "name": "admin",
        "active": true,
        "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
        "id": 3,
        "type": "NORMAL",
        "slug": "admin"
    },
    "eventKey": "pr:comment:added",
    "comment": {
        "severity": "NORMAL",
        "createdDate": 1761580448905,
        "comments": [],
        "threadResolved": false,
        "author": {
            "emailAddress": "test_user@example.com",
            "displayName": "admin",
            "name": "admin",
            "active": true,
            "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
            "id": 3,
            "type": "NORMAL",
            "slug": "admin"
        },
        "id": 7,
        "text": "This is a test comment.",
        "updatedDate": 1761580448905,
        "state": "OPEN",
        "version": 0,
        "properties": {"repositoryId": 2}
    },
    "pullRequest": {
        "author": {
            "approved": false,
            "role": "AUTHOR",
            "user": {
                "emailAddress": "test_user@example.com",
                "displayName": "admin",
                "name": "admin",
                "active": true,
                "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
                "id": 3,
                "type": "NORMAL",
                "slug": "admin"
            },
            "status": "UNAPPROVED"
        },
        "description": "Test PR with comments",
        "updatedDate": 1761138018299,
        "title": "Test PR",
        "version": 6,
        "reviewers": [],
        "toRef": {
            "latestCommit": "70b0d7be6f073634f7910a2cb5bbed9ec1306dff",
            "id": "refs/heads/master",
            "displayId": "master",
            "type": "BRANCH",
            "repository": {
                "archived": false,
                "public": false,
                "hierarchyId": "4e8506d8cbb6287a8dcd",
                "name": "repo",
                "forkable": true,
                "project": {
                    "public": false,
                    "name": "test",
                    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST"}]},
                    "id": 2,
                    "type": "NORMAL",
                    "key": "TEST"
                },
                "links": {
                    "clone": [
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        },
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                },
                "id": 2,
                "scmId": "git",
                "state": "AVAILABLE",
                "slug": "repo",
                "statusMessage": "Available"
            }
        },
        "createdDate": 1761134047464,
        "draft": false,
        "closed": false,
        "fromRef": {
            "latestCommit": "535dd99fabbecd4594c3dc844f387413fe6b97d4",
            "id": "refs/heads/test-branch",
            "displayId": "test-branch",
            "type": "BRANCH",
            "repository": {
                "archived": false,
                "public": false,
                "hierarchyId": "4e8506d8cbb6287a8dcd",
                "name": "repo",
                "forkable": true,
                "project": {
                    "public": false,
                    "name": "test",
                    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST"}]},
                    "id": 2,
                    "type": "NORMAL",
                    "key": "TEST"
                },
                "links": {
                    "clone": [
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        },
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                },
                "id": 2,
                "scmId": "git",
                "state": "AVAILABLE",
                "slug": "repo",
                "statusMessage": "Available"
            }
        },
        "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/pull-requests/1"}]},
        "id": 1,
        "state": "OPEN",
        "locked": false,
        "open": true,
        "participants": []
    }
}`

	samplePRCommentEditedData = `{
    "date": "2025-10-27T16:10:28+0000",
    "actor": {
        "emailAddress": "test-user@example.com",
        "displayName": "admin",
        "name": "admin",
        "active": true,
        "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
        "id": 3,
        "type": "NORMAL",
        "slug": "admin"
    },
    "eventKey": "pr:comment:edited",
    "comment": {
        "severity": "NORMAL",
        "createdDate": 1761580448905,
        "comments": [],
        "threadResolved": false,
        "author": {
            "emailAddress": "test-user@example.com",
            "displayName": "admin",
            "name": "admin",
            "active": true,
            "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
            "id": 3,
            "type": "NORMAL",
            "slug": "admin"
        },
        "id": 7,
        "text": "This is an updated test comment.",
        "updatedDate": 1761581427399,
        "state": "OPEN",
        "version": 1,
        "properties": {"repositoryId": 2}
    },
    "pullRequest": {
        "author": {
            "approved": false,
            "role": "AUTHOR",
            "user": {
                "emailAddress": "test-user@example.com",
                "displayName": "admin",
                "name": "admin",
                "active": true,
                "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
                "id": 3,
                "type": "NORMAL",
                "slug": "admin"
            },
            "status": "UNAPPROVED"
        },
        "description": "Test PR with comments",
        "updatedDate": 1761138018299,
        "title": "Test PR",
        "version": 6,
        "reviewers": [],
        "toRef": {
            "latestCommit": "70b0d7be6f073634f7910a2cb5bbed9ec1306dff",
            "id": "refs/heads/master",
            "displayId": "master",
            "type": "BRANCH",
            "repository": {
                "archived": false,
                "public": false,
                "hierarchyId": "4e8506d8cbb6287a8dcd",
                "name": "repo",
                "forkable": true,
                "project": {
                    "public": false,
                    "name": "test",
                    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST"}]},
                    "id": 2,
                    "type": "NORMAL",
                    "key": "TEST"
                },
                "links": {
                    "clone": [
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        },
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                },
                "id": 2,
                "scmId": "git",
                "state": "AVAILABLE",
                "slug": "repo",
                "statusMessage": "Available"
            }
        },
        "createdDate": 1761134047464,
        "draft": false,
        "closed": false,
        "fromRef": {
            "latestCommit": "535dd99fabbecd4594c3dc844f387413fe6b97d4",
            "id": "refs/heads/test-branch",
            "displayId": "test-branch",
            "type": "BRANCH",
            "repository": {
                "archived": false,
                "public": false,
                "hierarchyId": "4e8506d8cbb6287a8dcd",
                "name": "repo",
                "forkable": true,
                "project": {
                    "public": false,
                    "name": "test",
                    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST"}]},
                    "id": 2,
                    "type": "NORMAL",
                    "key": "TEST"
                },
                "links": {
                    "clone": [
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        },
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                },
                "id": 2,
                "scmId": "git",
                "state": "AVAILABLE",
                "slug": "repo",
                "statusMessage": "Available"
            }
        },
        "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/pull-requests/1"}]},
        "id": 1,
        "state": "OPEN",
        "locked": false,
        "open": true,
        "participants": []
    },
    "previousComment": "This is a test comment."
}`

	samplePRCommentAddedData = `{
    "date": "2025-10-27T15:54:08+0000",
    "actor": {
        "emailAddress": "test_user@example.com",
        "displayName": "admin",
        "name": "admin",
        "active": true,
        "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
        "id": 3,
        "type": "NORMAL",
        "slug": "admin"
    },
    "eventKey": "pr:comment:added",
    "comment": {
        "severity": "NORMAL",
        "createdDate": 1761580448905,
        "comments": [],
        "threadResolved": false,
        "author": {
            "emailAddress": "test_user@example.com",
            "displayName": "admin",
            "name": "admin",
            "active": true,
            "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
            "id": 3,
            "type": "NORMAL",
            "slug": "admin"
        },
        "id": 7,
        "text": "This is a test comment.",
        "updatedDate": 1761580448905,
        "state": "OPEN",
        "version": 0,
        "properties": {"repositoryId": 2}
    },
    "pullRequest": {
        "author": {
            "approved": false,
            "role": "AUTHOR",
            "user": {
                "emailAddress": "test_user@example.com",
                "displayName": "admin",
                "name": "admin",
                "active": true,
                "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
                "id": 3,
                "type": "NORMAL",
                "slug": "admin"
            },
            "status": "UNAPPROVED"
        },
        "description": "Test PR with comments",
        "updatedDate": 1761138018299,
        "title": "Test PR",
        "version": 6,
        "reviewers": [],
        "toRef": {
            "latestCommit": "70b0d7be6f073634f7910a2cb5bbed9ec1306dff",
            "id": "refs/heads/master",
            "displayId": "master",
            "type": "BRANCH",
            "repository": {
                "archived": false,
                "public": false,
                "hierarchyId": "4e8506d8cbb6287a8dcd",
                "name": "repo",
                "forkable": true,
                "project": {
                    "public": false,
                    "name": "test",
                    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST"}]},
                    "id": 2,
                    "type": "NORMAL",
                    "key": "TEST"
                },
                "links": {
                    "clone": [
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        },
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                },
                "id": 2,
                "scmId": "git",
                "state": "AVAILABLE",
                "slug": "repo",
                "statusMessage": "Available"
            }
        },
        "createdDate": 1761134047464,
        "draft": false,
        "closed": false,
        "fromRef": {
            "latestCommit": "535dd99fabbecd4594c3dc844f387413fe6b97d4",
            "id": "refs/heads/test-branch",
            "displayId": "test-branch",
            "type": "BRANCH",
            "repository": {
                "archived": false,
                "public": false,
                "hierarchyId": "4e8506d8cbb6287a8dcd",
                "name": "repo",
                "forkable": true,
                "project": {
                    "public": false,
                    "name": "test",
                    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST"}]},
                    "id": 2,
                    "type": "NORMAL",
                    "key": "TEST"
                },
                "links": {
                    "clone": [
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        },
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                },
                "id": 2,
                "scmId": "git",
                "state": "AVAILABLE",
                "slug": "repo",
                "statusMessage": "Available"
            }
        },
        "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/pull-requests/2"}]},
        "id": 1,
        "state": "OPEN",
        "locked": false,
        "open": true,
        "participants": []
    }
}`

	samplePRCommentEditedData = `{
    "date": "2025-10-27T16:10:28+0000",
    "actor": {
        "emailAddress": "test-user@example.com",
        "displayName": "admin",
        "name": "admin",
        "active": true,
        "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
        "id": 3,
        "type": "NORMAL",
        "slug": "admin"
    },
    "eventKey": "pr:comment:edited",
    "comment": {
        "severity": "NORMAL",
        "createdDate": 1761580448905,
        "comments": [],
        "threadResolved": false,
        "author": {
            "emailAddress": "test-user@example.com",
            "displayName": "admin",
            "name": "admin",
            "active": true,
            "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
            "id": 3,
            "type": "NORMAL",
            "slug": "admin"
        },
        "id": 7,
        "text": "This is an updated test comment.",
        "updatedDate": 1761581427399,
        "state": "OPEN",
        "version": 1,
        "properties": {"repositoryId": 2}
    },
    "pullRequest": {
        "author": {
            "approved": false,
            "role": "AUTHOR",
            "user": {
                "emailAddress": "test-user@example.com",
                "displayName": "admin",
                "name": "admin",
                "active": true,
                "links": {"self": [{"href": "https://bitbucket.example.com/users/admin"}]},
                "id": 3,
                "type": "NORMAL",
                "slug": "admin"
            },
            "status": "UNAPPROVED"
        },
        "description": "Test PR with comments",
        "updatedDate": 1761138018299,
        "title": "Test PR",
        "version": 6,
        "reviewers": [],
        "toRef": {
            "latestCommit": "70b0d7be6f073634f7910a2cb5bbed9ec1306dff",
            "id": "refs/heads/master",
            "displayId": "master",
            "type": "BRANCH",
            "repository": {
                "archived": false,
                "public": false,
                "hierarchyId": "4e8506d8cbb6287a8dcd",
                "name": "repo",
                "forkable": true,
                "project": {
                    "public": false,
                    "name": "test",
                    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST"}]},
                    "id": 2,
                    "type": "NORMAL",
                    "key": "TEST"
                },
                "links": {
                    "clone": [
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        },
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                },
                "id": 2,
                "scmId": "git",
                "state": "AVAILABLE",
                "slug": "repo",
                "statusMessage": "Available"
            }
        },
        "createdDate": 1761134047464,
        "draft": false,
        "closed": false,
        "fromRef": {
            "latestCommit": "535dd99fabbecd4594c3dc844f387413fe6b97d4",
            "id": "refs/heads/test-branch",
            "displayId": "test-branch",
            "type": "BRANCH",
            "repository": {
                "archived": false,
                "public": false,
                "hierarchyId": "4e8506d8cbb6287a8dcd",
                "name": "repo",
                "forkable": true,
                "project": {
                    "public": false,
                    "name": "test",
                    "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST"}]},
                    "id": 2,
                    "type": "NORMAL",
                    "key": "TEST"
                },
                "links": {
                    "clone": [
                        {
                            "name": "http",
                            "href": "https://bitbucket.example.com/scm/test/repo.git"
                        },
                        {
                            "name": "ssh",
                            "href": "ssh://git@bitbucket.example.com:7999/test/repo.git"
                        }
                    ],
                    "self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/browse"}]
                },
                "id": 2,
                "scmId": "git",
                "state": "AVAILABLE",
                "slug": "repo",
                "statusMessage": "Available"
            }
        },
        "links": {"self": [{"href": "https://bitbucket.example.com/projects/TEST/repos/repo/pull-requests/2"}]},
        "id": 1,
        "state": "OPEN",
        "locked": false,
        "open": true,
        "participants": []
    },
    "previousComment": "This is a test comment."
}`

	samplePingData = `{
    "test": true
}`
)

var intOne = 1

func Test_detectContentTypeSecretAndEventKey(t *testing.T) {
	t.Log("All required headers - should handle")
	{
		header := http.Header{
			"X-Event-Key":  {"repo:refs_changed"},
			"Content-Type": {"application/json"},
		}
		contentType, eventKey, err := detectContentTypeAndEventKey(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "repo:refs_changed", eventKey)
	}

	t.Log("No signature header - should handle")
	{
		header := http.Header{
			"X-Event-Key":  {"repo:refs_changed"},
			"Content-Type": {"application/json"},
		}
		contentType, eventKey, err := detectContentTypeAndEventKey(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "repo:refs_changed", eventKey)
	}

	t.Log("Missing X-Event-Key header")
	{
		header := http.Header{
			"Content-Type": {"application/json"},
		}
		contentType, eventKey, err := detectContentTypeAndEventKey(header)
		require.EqualError(t, err, "No X-Event-Key Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", eventKey)
	}

	t.Log("Missing Content-Type header")
	{
		header := http.Header{
			"X-Event-Key": {"repo:refs_changed"},
		}
		contentType, eventKey, err := detectContentTypeAndEventKey(header)
		require.EqualError(t, err, "No Content-Type Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", eventKey)
	}

	t.Log("Bitbucket Server UTF8 charset Content-Type header")
	{
		header := http.Header{
			"Content-Type": {"application/json; charset=utf-8"},
			"X-Event-Key":  {"repo:refs_changed"},
		}
		contentType, eventKey, err := detectContentTypeAndEventKey(header)
		require.NoError(t, err)
		require.Equal(t, "application/json; charset=utf-8", contentType)
		require.Equal(t, "repo:refs_changed", eventKey)
	}
}

func Test_transformPushEvent(t *testing.T) {
	t.Log("Do Transform - single change - code push")
	{
		pushEvent := PushEventModel{
			EventKey: "repo:refs_changed",
			Date:     "2017-09-19T09:58:11+1000",
			Actor: UserInfoModel{
				Name:        "user",
				DisplayName: "Username",
			},
			RepositoryInfo: RepositoryInfoModel{
				Slug:   "android",
				ID:     1,
				Name:   "Android",
				Public: false,
				Scm:    "git",
				Project: ProjectInfoModel{
					Key:    "APP",
					ID:     2,
					Name:   "App Repo",
					Public: false,
					Type:   "normal",
				},
			},
			Changes: []ChangeItemModel{
				{
					Type:     "UPDATE",
					FromHash: "FROM-966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					ToHash:   "TO-966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					RefID:    "refs/heads/master",
					Ref: RefModel{
						ID:        "refs/heads/master",
						DisplayID: "master",
						Type:      "BRANCH",
					},
				},
			},
			Commits: []CommitModel{
				{
					ID:      "abc123",
					Message: "first commit",
				},
				{
					ID:      "TO-966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					Message: "second commit",
				},
			},
		}

		// OK
		{
			hookTransformResult := transformPushEvent(pushEvent)
			require.NoError(t, hookTransformResult.Error)
			require.False(t, hookTransformResult.ShouldSkip)
			require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						CommitHash:     "TO-966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
						CommitMessage:  "second commit",
						CommitMessages: []string{"first commit", "second commit"},
						Branch:         "master",
					},
					TriggeredBy: "webhook-bitbucket-server/user",
				},
			}, hookTransformResult.TriggerAPIParams)
			require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
		}

		// no Scm info
		pushEvent.RepositoryInfo.Scm = "invalid-scm-or-empty"
		{
			hookTransformResult := transformPushEvent(pushEvent)
			require.EqualError(t, hookTransformResult.Error, "Unsupported repository / source control type (SCM): invalid-scm-or-empty")
			require.False(t, hookTransformResult.ShouldSkip)
			require.Nil(t, hookTransformResult.TriggerAPIParams)
			require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
		}
	}

	t.Log("Do Transform - single change - push new branch")
	{
		pushEvent := PushEventModel{
			EventKey: "repo:refs_changed",
			Date:     "2017-09-19T09:58:11+1000",
			Actor: UserInfoModel{
				Name:        "user",
				DisplayName: "Username",
			},
			RepositoryInfo: RepositoryInfoModel{
				Slug:   "android",
				ID:     1,
				Name:   "Android",
				Public: false,
				Scm:    "git",
				Project: ProjectInfoModel{
					Key:    "APP",
					ID:     2,
					Name:   "App Repo",
					Public: false,
					Type:   "normal",
				},
			},
			Changes: []ChangeItemModel{
				{
					Type:     "ADD",
					FromHash: "0000000000000000000000000000000000000000",
					ToHash:   "TO-966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					RefID:    "refs/heads/newbranch",
					Ref: RefModel{
						ID:        "refs/heads/newbranch",
						DisplayID: "newbranch",
						Type:      "BRANCH",
					},
				},
			},
		}

		// OK
		{
			hookTransformResult := transformPushEvent(pushEvent)
			require.NoError(t, hookTransformResult.Error)
			require.False(t, hookTransformResult.ShouldSkip)
			require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						CommitHash: "TO-966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
						Branch:     "newbranch",
					},
					TriggeredBy: "webhook-bitbucket-server/user",
				},
			}, hookTransformResult.TriggerAPIParams)
			require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
		}
	}

	t.Log("Do Transform - single change - tag")
	{
		tagPushEvent := PushEventModel{
			EventKey: "repo:refs_changed",
			Date:     "2017-09-19T09:58:11+1000",
			Actor: UserInfoModel{
				Name:        "user",
				DisplayName: "Username",
			},
			RepositoryInfo: RepositoryInfoModel{
				Slug:   "android",
				ID:     1,
				Name:   "Android",
				Public: false,
				Scm:    "git",
				Project: ProjectInfoModel{
					Key:    "APP",
					ID:     2,
					Name:   "App Repo",
					Public: false,
					Type:   "normal",
				},
			},
			Changes: []ChangeItemModel{
				{
					Type:     "ADD",
					FromHash: "0000000000000000000000000000000000000000",
					ToHash:   "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					RefID:    "refs/tags/3.0.4",
					Ref: RefModel{
						ID:        "refs/tags/3.0.4",
						DisplayID: "3.0.4",
						Type:      "TAG",
					},
				},
			},
		}
		hookTransformResult := transformPushEvent(tagPushEvent)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:        "3.0.4",
					CommitHash: "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
				},
				TriggeredBy: "webhook-bitbucket-server/user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Do Transform - multiple changes - code push")
	{
		pushEvent := PushEventModel{
			EventKey: "repo:refs_changed",
			Date:     "2017-09-19T09:58:11+1000",
			Actor: UserInfoModel{
				Name:        "user",
				DisplayName: "Username",
			},
			RepositoryInfo: RepositoryInfoModel{
				Slug:   "android",
				ID:     1,
				Name:   "Android",
				Public: false,
				Scm:    "git",
				Project: ProjectInfoModel{
					Key:    "APP",
					ID:     2,
					Name:   "App Repo",
					Public: false,
					Type:   "normal",
				},
			},
			Changes: []ChangeItemModel{
				{
					Type:     "UPDATE",
					FromHash: "from-hash-1",
					ToHash:   "to-hash-1",
					RefID:    "refs/heads/master",
					Ref: RefModel{
						ID:        "refs/heads/master",
						DisplayID: "master",
						Type:      "BRANCH",
					},
				},
				{
					Type:     "UPDATE",
					FromHash: "from-hash-2",
					ToHash:   "to-hash-2",
					RefID:    "refs/heads/test",
					Ref: RefModel{
						ID:        "refs/heads/test",
						DisplayID: "test",
						Type:      "BRANCH",
					},
				},
			},
			// It is possible that a push webhook has details about multiple branches in a single payload for cascading merge (https://confluence.atlassian.com/bitbucketserver/cascading-merge-776639993.html)
			// There is no official source for this, just an open source issue(https://github.com/gocd/gocd/issues/10071).
			// There is no example that includes commit data in this case, so we don’t know how we could correlate commits with changesets.
			// As such, when detecting multiple sets of changes, commit messages and changed files will be left empty.
			Commits: []CommitModel{
				{
					ID:      "abc123",
					Message: "this commit message should not be included",
				},
			},
		}

		hookTransformResult := transformPushEvent(pushEvent)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash: "to-hash-1",
					Branch:     "master",
				},
				TriggeredBy: "webhook-bitbucket-server/user",
			},
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash: "to-hash-2",
					Branch:     "test",
				},
				TriggeredBy: "webhook-bitbucket-server/user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Do Transform - multiple changes - tag push")
	{
		pushEvent := PushEventModel{
			EventKey: "repo:refs_changed",
			Date:     "2017-09-19T09:58:11+1000",
			Actor: UserInfoModel{
				Name:        "user",
				DisplayName: "Username",
			},
			RepositoryInfo: RepositoryInfoModel{
				Slug:   "android",
				ID:     1,
				Name:   "Android",
				Public: false,
				Scm:    "git",
				Project: ProjectInfoModel{
					Key:    "APP",
					ID:     2,
					Name:   "App Repo",
					Public: false,
					Type:   "normal",
				},
			},
			Changes: []ChangeItemModel{
				{
					Type:     "ADD",
					FromHash: "0000000000000000000000000000000000000000",
					ToHash:   "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					RefID:    "refs/tags/3.0.4",
					Ref: RefModel{
						ID:        "refs/tags/3.0.4",
						DisplayID: "3.0.4",
						Type:      "TAG",
					},
				},
				{
					Type:     "ADD",
					FromHash: "0000000000000000000000000000000000000000",
					ToHash:   "966d0bfe79b80f97268c2f6bb45e65e79ef09b32",
					RefID:    "refs/tags/3.0.5",
					Ref: RefModel{
						ID:        "refs/tags/3.0.5",
						DisplayID: "3.0.5",
						Type:      "TAG",
					},
				},
			},
			// It is possible that a push webhook has details about multiple branches in a single payload for cascading merge (https://confluence.atlassian.com/bitbucketserver/cascading-merge-776639993.html)
			// There is no official source for this, just an open source issue(https://github.com/gocd/gocd/issues/10071).
			// There is no example that includes commit data in this case, so we don’t know how we could correlate commits with changesets.
			// As such, when detecting multiple sets of changes, commit messages and changed files will be left empty.
			Commits: []CommitModel{
				{
					ID:      "abc123",
					Message: "this commit message should not be included",
				},
			},
		}

		hookTransformResult := transformPushEvent(pushEvent)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:        "3.0.4",
					CommitHash: "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
				},
				TriggeredBy: "webhook-bitbucket-server/user",
			},
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:        "3.0.5",
					CommitHash: "966d0bfe79b80f97268c2f6bb45e65e79ef09b32",
				},
				TriggeredBy: "webhook-bitbucket-server/user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Multiple changes, one of the changes is a not supported (type) change")
	{
		pushEvent := PushEventModel{
			EventKey: "repo:refs_changed",
			Date:     "2017-09-19T09:58:11+1000",
			Actor: UserInfoModel{
				Name:        "user",
				DisplayName: "Username",
			},
			RepositoryInfo: RepositoryInfoModel{
				Slug:   "android",
				ID:     1,
				Name:   "Android",
				Public: false,
				Scm:    "git",
				Project: ProjectInfoModel{
					Key:    "APP",
					ID:     2,
					Name:   "App Repo",
					Public: false,
					Type:   "normal",
				},
			},
			Changes: []ChangeItemModel{
				{
					Type:     "ADD",
					FromHash: "0000000000000000000000000000000000000000",
					ToHash:   "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					RefID:    "refs/tags/3.0.4",
					Ref: RefModel{
						ID:        "refs/tags/3.0.4",
						DisplayID: "3.0.4",
						Type:      "TAG",
					},
				},
				{
					Type:     "INVALID",
					FromHash: "0000000000000000000000000000000000000000",
					ToHash:   "966d0bfe79b80f97268c2f6bb45e65e79ef09b32",
					RefID:    "refs/tags/3.0.5",
					Ref: RefModel{
						ID:        "refs/tags/3.0.5",
						DisplayID: "3.0.5",
						Type:      "TAG",
					},
				},
			},
		}
		hookTransformResult := transformPushEvent(pushEvent)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:        "3.0.4",
					CommitHash: "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
				},
				TriggeredBy: "webhook-bitbucket-server/user",
			},
		}, hookTransformResult.TriggerAPIParams)

		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a UPDATE nor ADD change type")
	{
		pushEvent := PushEventModel{
			EventKey: "repo:refs_changed",
			Date:     "2017-09-19T09:58:11+1000",
			Actor: UserInfoModel{
				DisplayName: "Username",
			},
			RepositoryInfo: RepositoryInfoModel{
				Slug:   "android",
				ID:     1,
				Name:   "Android",
				Public: false,
				Scm:    "git",
				Project: ProjectInfoModel{
					Key:    "APP",
					ID:     2,
					Name:   "App Repo",
					Public: false,
					Type:   "normal",
				},
			},
			Changes: []ChangeItemModel{
				{
					Type:     "INVALID",
					FromHash: "0000000000000000000000000000000000000000",
					ToHash:   "966d0bfe79b80f97268c2f6bb45e65e79ef09b32",
					RefID:    "refs/tags/3.0.5",
					Ref: RefModel{
						ID:        "refs/tags/3.0.5",
						DisplayID: "3.0.5",
						Type:      "TAG",
					},
				},
			},
		}
		hookTransformResult := transformPushEvent(pushEvent)
		require.EqualError(t, hookTransformResult.Error, "'changes' specified in the webhook, but none can be transformed into a build. Collected errors: [Not a type=ADD change. Change.Type was: INVALID]")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a BRANCH nor TAG ref")
	{
		pushEvent := PushEventModel{
			EventKey: "repo:refs_changed",
			Date:     "2017-09-19T09:58:11+1000",
			Actor: UserInfoModel{
				DisplayName: "Username",
			},
			RepositoryInfo: RepositoryInfoModel{
				Slug:   "android",
				ID:     1,
				Name:   "Android",
				Public: false,
				Scm:    "git",
				Project: ProjectInfoModel{
					Key:    "APP",
					ID:     2,
					Name:   "App Repo",
					Public: false,
					Type:   "normal",
				},
			},
			Changes: []ChangeItemModel{
				{
					Type:     "UPDATE",
					FromHash: "from-hash-1",
					ToHash:   "to-hash-1",
					RefID:    "refs/heads/master",
					Ref: RefModel{
						ID:        "refs/heads/master",
						DisplayID: "master",
						Type:      "NOT-BRANCH",
					},
				},
				{
					Type:     "ADD",
					FromHash: "from-hash-2",
					ToHash:   "to-hash-2",
					RefID:    "refs/tags/3.0.5",
					Ref: RefModel{
						ID:        "refs/tags/3.0.5",
						DisplayID: "3.0.5",
						Type:      "NOT-TAG",
					},
				},
			},
		}
		hookTransformResult := transformPushEvent(pushEvent)
		require.EqualError(t, hookTransformResult.Error, "'changes' specified in the webhook, but none can be transformed into a build. Collected errors: [Ref was not a type=BRANCH nor type=TAG change. Type was: NOT-BRANCH Ref was not a type=BRANCH nor type=TAG change. Type was: NOT-TAG]")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

}

func Test_transformPullRequestEvent(t *testing.T) {

	t.Log("Already Merged")
	{
		pullRequest := PullRequestEventModel{
			PullRequest: PullRequestInfoModel{
				State: "MERGED",
			},
		}

		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request state doesn't require a build: MERGED")
	}

	t.Log("Already Declined")
	{
		pullRequest := PullRequestEventModel{
			PullRequest: PullRequestInfoModel{
				State: "DECLINED",
			},
		}

		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request state doesn't require a build: DECLINED")
	}

	t.Log("Open")
	{
		pullRequest := PullRequestEventModel{
			Actor: UserInfoModel{
				Name:        "user",
				DisplayName: "UserName",
			},
			PullRequest: PullRequestInfoModel{
				ID:     1,
				Title:  "Title of pull request",
				State:  "OPEN",
				Closed: false,
				Open:   true,
				FromRef: PullRequestRefModel{
					ID:           "refs/heads/a-branch",
					DisplayID:    "a-branch",
					LatestCommit: "ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
				},
				ToRef: PullRequestRefModel{
					ID:           "refs/heads/master",
					DisplayID:    "master",
					LatestCommit: "178864a7d521b6f5e720b386b2c2b0ef8563e0dc",
				},
			},
		}

		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
					CommitMessage: "Title of pull request",
					Branch:        "a-branch",
					BranchDest:    "master",
					PullRequestID: &intOne,
				},
				TriggeredBy: "webhook-bitbucket-server/user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

}

func Test_isAcceptEventType(t *testing.T) {
	t.Log("Accept")
	{
		for _, anAction := range []string{
			"repo:refs_changed",
			"pr:opened",
			"pr:comment:added",
			"pr:comment:edited",
		} {
			t.Log(" * " + anAction)
			require.Equal(t, true, isAcceptEventType(anAction))
		}
	}

	t.Log("Don't accept")
	{
		for _, anAction := range []string{"",
			"a", "not-an-action",
			"repo:forked", "repo:modified", "repo:comment:added", "repo:comment:edited", "repo:comment:deleted", "pr:reviewer:approved",
			"pr:reviewer:unapproved", "pr:reviewer:needs_work", "pr:declined", "pr:deleted",
			"pr:comment:deleted",
		} {
			t.Log(" * " + anAction)
			require.Equal(t, false, isAcceptEventType(anAction))
		}
	}
}

func Test_transformPingEvent(t *testing.T) {
	provider := HookProvider{}

	t.Log("Bitbucket Server Ping")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"diagnostics:ping"},
				"Content-Type": {"application/json; charset=utf-8"},
				"X-Request-Id": {"009af3f7-21ef-4806-8649-e6916498ab0f"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePingData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Bitbucket event type: diagnostics:ping is successful")
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Unsupported Event Type")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"not:supported"},
				"Content-Type":     {"application/json; charset=utf-8"},
				"X-Attempt-Number": {"1"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "X-Event-Key is not supported: not:supported")
	}

	t.Log("Unsupported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"repo:refs_changed"},
				"Content-Type": {"not/supported"},
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
				"X-Event-Key":  {"repo:refs_changed"},
				"Content-Type": {"application/json; charset=utf-8"},
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
				"X-Event-Key":  {"repo:refs_changed"},
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:        "to-hash-1",
					Branch:            "master",
					BaseRepositoryURL: "ssh://git@bitbucket.example.com:7999/test/repo.git",
				},
				TriggeredBy: "webhook-bitbucket-server/admin",
			},
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:        "to-hash-2",
					Branch:            "a-branch",
					BaseRepositoryURL: "ssh://git@bitbucket.example.com:7999/test/repo.git",
				},
				TriggeredBy: "webhook-bitbucket-server/admin",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Test with Sample Tag Push data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"repo:refs_changed"},
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleTagPushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:               "3.0.4",
					CommitHash:        "2943d981c36ca9a241326a8c9520bec15edef8c5",
					BaseRepositoryURL: "ssh://git@bitbucket.example.com:7999/test/repo.git",
				},
				TriggeredBy: "webhook-bitbucket-server/user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Test with Sample Pull Request data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"pr:opened"},
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePullRequestData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
					CommitMessage:            "a new file added",
					Branch:                   "a-branch",
					BranchRepoOwner:          "PROJ",
					BranchDest:               "master",
					BranchDestRepoOwner:      "PROJ",
					PullRequestID:            &intOne,
					BaseRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					HeadRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestRepositoryURL: "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestAuthor:        "admin",
				},
				TriggeredBy: "webhook-bitbucket-server/admin",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Test with Sample Pull Request modification data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"pr:modified"},
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePullRequestModifiedData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
					CommitMessage:            "a new file added",
					Branch:                   "a-branch",
					BranchRepoOwner:          "PROJ",
					BranchDest:               "master",
					BranchDestRepoOwner:      "PROJ",
					PullRequestID:            &intOne,
					BaseRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					HeadRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestRepositoryURL: "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestAuthor:        "admin",
				},
				TriggeredBy: "webhook-bitbucket-server/admin",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Test with Sample Pull Request From Ref Updated Data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"pr:from_ref_updated"},
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePullRequestFromRefUpdatedData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
					CommitMessage:            "a new file added",
					Branch:                   "a-branch",
					BranchRepoOwner:          "PROJ",
					BranchDest:               "master",
					BranchDestRepoOwner:      "PROJ",
					PullRequestID:            &intOne,
					BaseRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					HeadRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestRepositoryURL: "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestAuthor:        "admin",
				},
				TriggeredBy: "webhook-bitbucket-server/admin",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Test with Sample Pull Request merged data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"pr:merged"},
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePullRequestMergedData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request state doesn't require a build: MERGED")
	}

	t.Log("Test with Sample Pull Request comment added data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"pr:comment:added"},
				"Content-Type": {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePRCommentAddedData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "535dd99fabbecd4594c3dc844f387413fe6b97d4",
					CommitMessage:            "Test PR",
					Branch:                   "test-branch",
					BranchRepoOwner:          "TEST",
					BranchDest:               "master",
					BranchDestRepoOwner:      "TEST",
					PullRequestID:            &intOne,
					BaseRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					HeadRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestRepositoryURL: "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestAuthor:        "admin",
					PullRequestComment:       "This is a test comment.",
					PullRequestCommentID:     "7",
				},
				TriggeredBy: "webhook-bitbucket-server/admin",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Test with Sample Pull Request comment edited data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"pr:comment:edited"},
				"Content-Type": {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePRCommentEditedData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "535dd99fabbecd4594c3dc844f387413fe6b97d4",
					CommitMessage:            "Test PR",
					Branch:                   "test-branch",
					BranchRepoOwner:          "TEST",
					BranchDest:               "master",
					BranchDestRepoOwner:      "TEST",
					PullRequestID:            &intOne,
					BaseRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					HeadRepositoryURL:        "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestRepositoryURL: "ssh://git@bitbucket.example.com:7999/test/repo.git",
					PullRequestAuthor:        "admin",
					PullRequestComment:       "This is an updated test comment.",
					PullRequestCommentID:     "7",
				},
				TriggeredBy: "webhook-bitbucket-server/admin",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}
