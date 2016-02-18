package bitriseapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

// BuildParamsModel ...
type BuildParamsModel struct {
	CommitHash    string `json:"commit_hash,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	Branch        string `json:"branch,omitempty"`
	Tag           string `json:"tag,omitempty"`
	PullRequestID *int   `json:"pull_request_id,omitempty"`
}

// TriggerAPIParamsModel ...
type TriggerAPIParamsModel struct {
	BuildParams BuildParamsModel `json:"build_params"`
}

// TriggerAPIResponseModel ...
type TriggerAPIResponseModel struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Service   string `json:"service"`
	AppSlug   string `json:"slug"`
	BuildSlug string `json:"build_slug"`
}

// Validate ...
func (triggerParams TriggerAPIParamsModel) Validate() error {
	if triggerParams.BuildParams.Branch == "" {
		return errors.New("Missing Branch parameter")
	}
	return nil
}

// BuildTriggerURL ...
func BuildTriggerURL(apiRootURL string, appSlug string) (*url.URL, error) {
	baseURL, err := url.Parse(apiRootURL)
	if err != nil {
		return nil, fmt.Errorf("BuildTriggerURL: Failed to parse (%s), error: %s", apiRootURL, err)
	}

	pathURL, err := url.Parse(fmt.Sprintf("/app/%s/build/start.json", appSlug))
	if err != nil {
		return nil, fmt.Errorf("BuildTriggerURL: Failed to parse PATH, error: %s", err)
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
		return TriggerAPIResponseModel{}, false, fmt.Errorf("TriggerBuild: build trigger parameter invalid: %s", err)
	}

	jsonStr, err := json.Marshal(params)
	if err != nil {
		return TriggerAPIResponseModel{}, false, fmt.Errorf("TriggerBuild: failed to json marshal: %s", err)
	}

	log.Printf("===> Triggering Build: (url:%s)", url)
	log.Printf("====> JSON body: %s", jsonStr)

	if isOnlyLog {
		return TriggerAPIResponseModel{
			Status:  "ok",
			Message: "LOG ONLY MODE",
		}, true, nil
	}

	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(jsonStr))
	if err != nil {
		return TriggerAPIResponseModel{}, false, fmt.Errorf("TriggerBuild: failed to create request: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Token", apiToken)
	req.Header.Set("X-Bitrise-Event", "hook")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return TriggerAPIResponseModel{}, false, fmt.Errorf("TriggerBuild: failed to send request: %s", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf(" [!] Exception: TriggerBuild: Failed to close response body, error: %s", err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return TriggerAPIResponseModel{}, false, fmt.Errorf("TriggerBuild: request sent, but failed to read response body (http-code:%d): %s", resp.StatusCode, body)
	}

	var respModel TriggerAPIResponseModel
	if err := json.Unmarshal(body, &respModel); err != nil {
		return TriggerAPIResponseModel{}, false, fmt.Errorf("TriggerBuild: request sent, but failed to parse response (http-code:%d): %s", resp.StatusCode, body)
	}

	if 200 <= resp.StatusCode && resp.StatusCode <= 202 {
		return respModel, true, nil
	}

	return respModel, false, nil
}
