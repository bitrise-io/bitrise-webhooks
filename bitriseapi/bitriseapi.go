package bitriseapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

// TriggerAPIParamsModel ...
type TriggerAPIParamsModel struct {
	CommitHash    string `json:"commit_hash"`
	CommitMessage string `json:"commit_message"`
	Branch        string `json:"branch"`
	Tag           string `json:"tag"`
	PullRequestID int64  `json:"pull_request_id"`
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
func TriggerBuild(url *url.URL, apiToken string, params TriggerAPIParamsModel, isOnlyLog bool) error {
	jsonStr, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("TriggerBuild: failed to json marshal: %s", err)
	}

	log.Printf("===> Triggering Build: (url:%s)", url)
	log.Printf("====> JSON body: %s", jsonStr)

	if isOnlyLog {
		return nil
	}

	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(jsonStr))
	if err != nil {
		return fmt.Errorf("TriggerBuild: failed to create request: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Token", apiToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("TriggerBuild: failed to send request: %s", err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

	return nil
}
