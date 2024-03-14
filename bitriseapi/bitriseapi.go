package bitriseapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/pkg/errors"

	"github.com/bitrise-io/api-utils/logging"
)

// EnvironmentItem ...
type EnvironmentItem struct {
	Name     string `json:"mapped_to"`
	Value    string `json:"value"`
	IsExpand bool   `json:"is_expand"`
}

// CommitPaths ...
type CommitPaths struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

// PullRequestReadyState ...
type PullRequestReadyState string

// PullRequestReadyState ...
const (
	PullRequestReadyStateDraft                     PullRequestReadyState = "draft"
	PullRequestReadyStateReadyForReview            PullRequestReadyState = "ready_for_review"
	PullRequestReadyStateConvertedToReadyForReview PullRequestReadyState = "converted_to_ready_for_review"
)

// BuildParamsModel ...
type BuildParamsModel struct {
	// git commit hash
	CommitHash string `json:"commit_hash,omitempty"`
	// git commit message of head commit
	CommitMessage string `json:"commit_message,omitempty"`
	// git commit messages of all commits
	CommitMessages []string `json:"commit_messages,omitempty"`
	// source branch
	Branch string `json:"branch,omitempty"`
	// source branch repo owner
	BranchRepoOwner string `json:"branch_repo_owner,omitempty"`
	// destination branch, exposed for pull requests
	BranchDest string `json:"branch_dest,omitempty"`
	// destination branch repo owner, exposed for pull requests
	BranchDestRepoOwner string `json:"branch_dest_repo_owner,omitempty"`
	// tag
	Tag string `json:"tag,omitempty"`
	// pull request id, exposed for pull requests from the provider's serivce
	PullRequestID *int `json:"pull_request_id,omitempty"`
	// Deprecated: Use HeadRepositoryURL instead
	PullRequestRepositoryURL string `json:"pull_request_repository_url,omitempty"`
	// URL of the base repository
	BaseRepositoryURL string `json:"base_repository_url,omitempty"`
	// URL of the head repository
	HeadRepositoryURL string `json:"head_repository_url,omitempty"`
	// Pre-merged PR state, created by the git provider (if supported).
	// IMPORTANT: This should only be defined if the state is already up-to-date with the latest PR head state
	// Otherwise, use PullRequestUnverifiedMergeBranch.
	PullRequestMergeBranch string `json:"pull_request_merge_branch,omitempty"`
	// Similar to PullRequestMergeBranch, but this field contains a potentially stale state. One example is when a
	// PR branch gets a new commit and the merge ref is not updated yet.
	// A system using this field should check the freshness of the merge ref by other means before using it for checkouts.
	PullRequestUnverifiedMergeBranch string `json:"pull_request_unverified_merge_branch,omitempty"`
	// source branch mapped to the original repository if the provider supports it, exposed for pull requests
	PullRequestHeadBranch string `json:"pull_request_head_branch,omitempty"`
	// The creator of the pull request
	PullRequestAuthor string `json:"pull_request_author,omitempty"`
	// workflow id to run
	WorkflowID string `json:"workflow_id,omitempty"`
	// additional environment variables
	Environments []EnvironmentItem `json:"environments,omitempty"`
	// URL of the diff
	DiffURL string `json:"diff_url"`
	// paths of changes of all commits
	PushCommitPaths []CommitPaths `json:"commit_paths"`
	// pull request ready state
	PullRequestReadyState PullRequestReadyState `json:"pull_request_ready_state,omitempty"`
	// newly added pull request label
	NewPullRequestLabel string `json:"pull_request_label,omitempty"`
	// pull request labels
	PullRequestLabels []string `json:"pull_request_labels,omitempty"`
}

// TriggerAPIParamsModel ...
type TriggerAPIParamsModel struct {
	BuildParams BuildParamsModel `json:"build_params"`
	TriggeredBy string           `json:"triggered_by"`
}

// TriggerAPIResponseModel ...
type TriggerAPIResponseModel struct {
	Status            string `json:"status"`
	Message           string `json:"message"`
	Service           string `json:"service"`
	AppSlug           string `json:"slug"`
	BuildSlug         string `json:"build_slug"`
	BuildNumber       int    `json:"build_number"`
	BuildURL          string `json:"build_url"`
	TriggeredWorkflow string `json:"triggered_workflow"`
}

// Validate ...
func (triggerParams TriggerAPIParamsModel) Validate() error {
	if triggerParams.BuildParams.Branch == "" && triggerParams.BuildParams.WorkflowID == "" && triggerParams.BuildParams.Tag == "" {
		return errors.New("Missing Branch, Tag and WorkflowID parameters - at least one of these is required")
	}
	if triggerParams.TriggeredBy == "" {
		return errors.New("Missing TriggeredBy parameter")
	}
	return nil
}

// BuildTriggerURL ...
func BuildTriggerURL(apiRootURL string, appSlug string) (*url.URL, error) {
	baseURL, err := url.Parse(apiRootURL)
	if err != nil {
		return nil, errors.Wrapf(err, "BuildTriggerURL: Failed to parse (%s)", apiRootURL)
	}

	pathURL, err := url.Parse(fmt.Sprintf("/app/%s/build/start.json", appSlug))
	if err != nil {
		return nil, errors.Wrap(err, "BuildTriggerURL: Failed to parse PATH")
	}
	return baseURL.ResolveReference(pathURL), nil
}

// TriggerBuild ...
// Returns an error in case it can't send the request, or the response is not a HTTP success response.
//
// If the response is an HTTP success response then the whole response body will be returned, and error will be nil.
func TriggerBuild(url *url.URL, apiToken string, params TriggerAPIParamsModel, isOnlyLog bool) (TriggerAPIResponseModel, bool, error) {
	logger := logging.WithContext(nil)

	if err := params.Validate(); err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrapf(err, "TriggerBuild (url:%s): build trigger parameter invalid", url.String())
	}

	jsonStr, err := json.Marshal(params)
	if err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrapf(err, "TriggerBuild (url:%s): failed to json marshal", url.String())
	}

	if isOnlyLog {
		log.Printf("\\x1b[33;1m===> Triggering Build: (url:%s)\\x1b[0m\n", url)
		log.Printf("\\x1b[33;1m====> JSON body: %s\\x1b[0m\n", jsonStr)
	}

	if isOnlyLog {
		return TriggerAPIResponseModel{
			Status:  "ok",
			Message: "LOG ONLY MODE",
		}, true, nil
	}

	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(jsonStr))
	if err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrapf(err, "TriggerBuild (url:%s): failed to create request", url.String())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Token", apiToken)
	req.Header.Set("X-Bitrise-Event", "hook")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrapf(err, "TriggerBuild (url:%s): failed to send request", url.String())
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error(" [!] Exception: TriggerBuild (url:%s): Failed to close response body", zap.String("url", url.String()), zap.Error(err))
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrapf(err, "TriggerBuild (url:%s): request sent, but failed to read response body (http-code:%d)", url.String(), resp.StatusCode)
	}
	bodyString := string(body)

	var respModel TriggerAPIResponseModel
	if err := json.Unmarshal(body, &respModel); err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrapf(err, "TriggerBuild (url:%s): request sent, but failed to parse response (http-code:%d, response body:%s)", url.String(), resp.StatusCode, bodyString)
	}

	if respModel.Status == "" && respModel.Message == "" {
		respModel.Message = bodyString
	}

	if 200 <= resp.StatusCode && resp.StatusCode <= 202 {
		return respModel, true, nil
	}

	return respModel, false, nil
}
