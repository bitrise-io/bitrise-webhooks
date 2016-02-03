package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/bitrise-io/go-utils/sliceutil"
)

// HookRespModel ...
type HookRespModel struct {
	Message string `json:"message"`
}

const (
	hookTypeIDGithub      = "github"
	hookTypeIDBitbucketV1 = "bitbucket-v1"
	hookTypeIDBitbucketV2 = "bitbucket-v2"
)

type hookTypeModel struct {
	typeID        string
	isDontProcess bool
}

func hookTypeCheckGithub(header http.Header) hookTypeModel {
	ghEvents := header["HTTP_X_GITHUB_EVENT"]

	if len(ghEvents) < 1 {
		// not a GitHub webhook
		return hookTypeModel{typeID: "", isDontProcess: false}
	}

	for _, aGHEvent := range ghEvents {
		if aGHEvent == "push" || aGHEvent == "pull_request" {
			// We'll process this
			return hookTypeModel{typeID: hookTypeIDGithub, isDontProcess: false}
		}
	}

	// GitHub webhook, but not supported event type - skip it
	return hookTypeModel{typeID: hookTypeIDGithub, isDontProcess: true}
}

func hookTypeCheckBitbucketV1(header http.Header) hookTypeModel {
	userAgents := header["User-Agent"]
	contentTypes := header["Content-Type"]

	if sliceutil.IsSliceIncludesString("Bitbucket.org", userAgents) &&
		sliceutil.IsSliceIncludesString("application/x-www-form-urlencoded", contentTypes) {
		return hookTypeModel{typeID: hookTypeIDBitbucketV1, isDontProcess: false}
	}

	return hookTypeModel{typeID: "", isDontProcess: false}
}

func hookTypeCheckBitbucketV2(header http.Header) hookTypeModel {
	userAgents := header["HTTP_USER_AGENT"]
	eventKeys := header["X-Event-Key"]

	if len(eventKeys) < 1 {
		// not a Bitbucket webhook
		return hookTypeModel{typeID: "", isDontProcess: false}
	}

	isBitbucketAgent := false
	for _, aUserAgent := range userAgents {
		if strings.HasPrefix(aUserAgent, "Bitbucket-Webhooks/2") {
			isBitbucketAgent = true
		}
	}
	if !isBitbucketAgent {
		// not a Bitbucket webhook
		return hookTypeModel{typeID: "", isDontProcess: false}
	}

	// check event type/key
	for _, aEventKey := range eventKeys {
		if aEventKey == "repo:push" {
			// We'll process this
			return hookTypeModel{typeID: hookTypeIDBitbucketV2, isDontProcess: false}
		}
	}

	// Bitbucket webhook, but not supported event type - skip it
	return hookTypeModel{typeID: hookTypeIDBitbucketV2, isDontProcess: true}
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	hookType := hookTypeModel{typeID: "", isDontProcess: false}

	metrics.Trace("Determine hook type", func() {
		if ht := hookTypeCheckGithub(r.Header); ht.typeID != "" {
			hookType = ht
			return
		}
		if ht := hookTypeCheckBitbucketV2(r.Header); ht.typeID != "" {
			hookType = ht
			return
		}
		if ht := hookTypeCheckBitbucketV1(r.Header); ht.typeID != "" {
			hookType = ht
			return
		}
		// 	type = "bitbucket" if @body["canon_url"].eql?('https://bitbucket.org')
		log.Println("UNSUPPORTED webhook")
	})

	// possible responses:
	// * webhook OK, processed, sent
	// * webhook OK, but no build started - e.g. GitHub's ZEN event
	// * webhook type not supported

	if hookType.typeID == "" {
		respondWithBadRequestError(w, "Unsupported Webhook Type")
		return
	}

	if hookType.isDontProcess {
		resp := HookRespModel{
			Message: "Acknowledged, but skipping - not enough information to start a build, or unsupported event type",
		}
		respondWithSuccess(w, resp)
		return
	}

	resp := HookRespModel{
		Message: fmt.Sprintf("Processing: %#v", hookType),
	}
	respondWithSuccess(w, resp)
}
