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

	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/pkg/errors"
)

// EnvironmentItem ...
type EnvironmentItem struct {
	Name     string `json:"mapped_to"`
	Value    string `json:"value"`
	IsExpand bool   `json:"is_expand"`
}

// BuildParamsModel ...
type BuildParamsModel struct {
	// git commit hash
	CommitHash string `json:"commit_hash,omitempty"`
	// git commit message
	CommitMessage string `json:"commit_message,omitempty"`
	// source branch
	Branch string `json:"branch,omitempty"`
	// destination branch, exposed for pull requests
	BranchDest string `json:"branch_dest,omitempty"`
	// tag
	Tag string `json:"tag,omitempty"`
	// pull request id, exposed for pull requests from the provider's serivce
	PullRequestID *int `json:"pull_request_id,omitempty"`
	// repository url that holds the source for the pull request
	PullRequestRepositoryURL string `json:"pull_request_repository_url,omitempty"`
	// pre-merged branch if the provider supports it, exposed for pull requests
	PullRequestMergeBranch string `json:"pull_request_merge_branch,omitempty"`
	// source branch mapped to the original repository if the provider supports it, exposed for pull requests
	PullRequestHeadBranch string `json:"pull_request_head_branch,omitempty"`
	// workflow id to run
	WorkflowID string `json:"workflow_id,omitempty"`
	// additional environment variables
	Environments []EnvironmentItem `json:"environments,omitempty"`
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
// Returns an error in case it can't send the request, or the response is
//  not a HTTP success response.
// If the response is an HTTP success response then the whole response body
//  will be returned, and error will be nil.
func TriggerBuild(url *url.URL, apiToken string, params TriggerAPIParamsModel, isOnlyLog bool) (TriggerAPIResponseModel, bool, error) {
	if err := params.Validate(); err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrap(err, "TriggerBuild: build trigger parameter invalid")
	}

	params.TriggeredBy = "webhook"

	jsonStr, err := json.Marshal(params)
	if err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrap(err, "TriggerBuild: failed to json marshal")
	}

	if isOnlyLog {
		log.Println(colorstring.Yellowf("===> Triggering Build: (url:%s)", url))
		log.Println(colorstring.Yellowf("====> JSON body: %s", jsonStr))
	}

	if isOnlyLog {
		return TriggerAPIResponseModel{
			Status:  "ok",
			Message: "LOG ONLY MODE",
		}, true, nil
	}

	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(jsonStr))
	if err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrap(err, "TriggerBuild: failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Token", apiToken)
	req.Header.Set("X-Bitrise-Event", "hook")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrap(err, "TriggerBuild: failed to send request")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf(" [!] Exception: TriggerBuild: Failed to close response body: %+v", err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrapf(err, "TriggerBuild: request sent, but failed to read response body (http-code:%d)", resp.StatusCode)
	}

	var respModel TriggerAPIResponseModel
	if err := json.Unmarshal(body, &respModel); err != nil {
		return TriggerAPIResponseModel{}, false, errors.Wrapf(err, "TriggerBuild: request sent, but failed to parse response (http-code:%d)", resp.StatusCode)
	}

	if respModel.Status == "" && respModel.Message == "" {
		respModel.Message = string(body)
	}

	if 200 <= resp.StatusCode && resp.StatusCode <= 202 {
		return respModel, true, nil
	}

	return respModel, false, nil
}
