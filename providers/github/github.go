package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/bitrise-io/bitrise-webhooks/providers"
	"github.com/bitrise-io/go-utils/sliceutil"
)

// CommitModel ...
type CommitModel struct {
	Distinct      bool   `json:"distinct"`
	CommitHash    string `json:"id"`
	CommitMessage string `json:"message"`
}

// CodePushEventModel ...
type CodePushEventModel struct {
	Ref        string      `json:"ref"`
	Deleted    bool        `json:"deleted"`
	HeadCommit CommitModel `json:"head_commit"`
}

// HookProvider ...
type HookProvider struct{}

// HookCheck ...
func (hp HookProvider) HookCheck(header http.Header) providers.HookCheckModel {
	contentTypes := header["Content-Type"]
	isContentTypeOK := false
	for _, aContentType := range contentTypes {
		if aContentType == "application/json" || aContentType == "application/x-www-form-urlencoded" {
			isContentTypeOK = true
		}
	}
	if !isContentTypeOK {
		// not a GitHub webhook
		return providers.HookCheckModel{IsSupportedByProvider: false, IsCantTransform: false}
	}

	ghEvents := header["X-Github-Event"]
	if len(ghEvents) < 1 {
		// not a GitHub webhook
		return providers.HookCheckModel{IsSupportedByProvider: false, IsCantTransform: false}
	}
	for _, aGHEvent := range ghEvents {
		if aGHEvent == "push" || aGHEvent == "pull_request" {
			// We'll process this
			return providers.HookCheckModel{IsSupportedByProvider: true, IsCantTransform: false}
		}
	}

	// GitHub webhook, but not supported event type - skip it
	log.Printf(" (debug) Skipping GitHub event: %#v", ghEvents)
	return providers.HookCheckModel{IsSupportedByProvider: true, IsCantTransform: true}
}

func transformCodePushEvent(codePushEvent CodePushEventModel) providers.HookTransformResultModel {
	headCommit := codePushEvent.HeadCommit
	if !headCommit.Distinct {
		return providers.HookTransformResultModel{Error: errors.New("Head Commit is not Distinct"), ShouldSkip: true}
	}

	if !strings.HasPrefix(codePushEvent.Ref, "refs/heads/") {
		return providers.HookTransformResultModel{Error: fmt.Errorf("Ref (%s) is not a head ref", codePushEvent.Ref), ShouldSkip: true}
	}
	branch := strings.TrimPrefix(codePushEvent.Ref, "refs/heads/")

	return providers.HookTransformResultModel{
		TriggerAPIParams: bitriseapi.TriggerAPIParamsModel{
			CommitHash:    headCommit.CommitHash,
			CommitMessage: headCommit.CommitMessage,
			Branch:        branch,
		},
	}
}

// Transform ...
func (hp HookProvider) Transform(r *http.Request) providers.HookTransformResultModel {
	if r.Body == nil {
		return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to read content of request body: no or empty request body")}
	}

	// bodyContentBytes, err := ioutil.ReadAll(r.Body)
	// if err != nil {
	// 	return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to read content of request body: %s", err)}
	// }
	// log.Printf("bodyContentBytes: %s", bodyContentBytes)

	ghEvents := r.Header["X-Github-Event"]
	if sliceutil.IsStringInSlice("push", ghEvents) {
		// code push
		var codePushEvent CodePushEventModel
		contentTypes := r.Header["Content-Type"]
		if sliceutil.IsStringInSlice("application/json", contentTypes) {
			if err := json.NewDecoder(r.Body).Decode(&codePushEvent); err != nil {
				return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
			}
		} else {
			// application/x-www-form-urlencoded
			payloadValue := r.PostFormValue("payload")
			if payloadValue == "" {
				return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to parse request body: empty payload")}
			}
			if err := json.NewDecoder(strings.NewReader(payloadValue)).Decode(&codePushEvent); err != nil {
				return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to parse payload: %s", err)}
			}
		}
		return transformCodePushEvent(codePushEvent)
	}

	return providers.HookTransformResultModel{}
}
