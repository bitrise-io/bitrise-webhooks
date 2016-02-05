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
)

// TriggerAPIParamsModel ...
type TriggerAPIParamsModel struct {
	CommitHash    string `json:"commit_hash"`
	CommitMessage string `json:"commit_message"`
	Branch        string `json:"branch"`
	Tag           string `json:"tag,omitempty"`
	PullRequestID *int   `json:"pull_request_id,omitempty"`
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
func TriggerBuild(url *url.URL, apiToken string, params TriggerAPIParamsModel, isOnlyLog bool) ([]byte, error) {
	jsonStr, err := json.Marshal(params)
	if err != nil {
		return []byte{}, fmt.Errorf("TriggerBuild: failed to json marshal: %s", err)
	}

	log.Printf("===> Triggering Build: (url:%s)", url)
	log.Printf("====> JSON body: %s", jsonStr)

	if isOnlyLog {
		return []byte("LOG-ONLY-MODE"), nil
	}

	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(jsonStr))
	if err != nil {
		return []byte{}, fmt.Errorf("TriggerBuild: failed to create request: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Token", apiToken)
	req.Header.Set("X-Bitrise-Event", "hook")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("TriggerBuild: failed to send request: %s", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf(" [!] Exception: TriggerBuild: Failed to close response body, error: %s", err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("TriggerBuild: request sent, but failed to read response body (http-code:%d): %s", resp.StatusCode, body)
	}

	if resp.StatusCode != 200 {
		return []byte{}, fmt.Errorf("TriggerBuild: request sent, but received a non success response (http-code:%d): %s", resp.StatusCode, body)
	}

	return body, nil
}
