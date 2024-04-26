package gitlab

// # Infos / notes:
//
// ## Webhook calls
//
// Official API docs: https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/user/project/integrations/webhooks.md
//
// ### Code Push
//
// A code push webhook is sent with the header: `X-Gitlab-Event: Push Hook`.
// Official docs: https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/user/project/integrations/webhooks.md#push-events
//
// GitLab sends push webhooks for every branch separately. Even if you
// push to two different branches at the same time (git push --all) it'll
// trigger two webhook calls, one for each branch.
//
// Commits are grouped in the webhook - if you push more than one commit
// to a single branch it'll be included in a single webhook call, including
// all of the commits.
//
// The latest commit's hash is included as the "checkout_sha" parameter
// in the webhook. As we don't want to trigger build for every commit
// which is related to a single branch we will only handle the commit
// with the hash / id specified as the "checkout_sha".
//
// ### Tag Push
//
// GitLab sends webhooks for every tag separately. Even if you create 5 tags and push them with `git push --tags`
// GitLab will send out (properly) 5 separate webhooks, one for every tag (other services typically don't send
// these separately, or don't deliver all tags if you push more than ~3 tags in a single `git push --tags`).
//
// ### Merge request
// A merge request is sent with the header: `X-Gitlab-Event: Merge Request Hook`
// Official docs: https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/user/project/integrations/webhooks.md#merge-request-events
//

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/envman/envman"
	"go.uber.org/zap"
)

// --------------------------
// --- Webhook Data Model ---

const (
	tagPushEventID              = "Tag Push Hook"
	codePushEventID             = "Push Hook"
	mergeRequestEventID         = "Merge Request Hook"
	commentEventID              = "Note Hook"
	gitlabPublicVisibilityLevel = 20

	// ProviderID ...
	ProviderID = "gitlab"

	commitMessagesEnvKey      = "BITRISE_WEBHOOK_COMMIT_MESSAGES"
	fallbackEnvBytesLimitInKB = 256
	kbToB                     = 1024
)

// CommitModel ...
type CommitModel struct {
	CommitHash    string   `json:"id"`
	CommitMessage string   `json:"message"`
	AddedFiles    []string `json:"added"`
	ModifiedFiles []string `json:"modified"`
	RemovedFiles  []string `json:"removed"`
}

// CodePushEventModel ...
type CodePushEventModel struct {
	ObjectKind   string          `json:"object_kind"`
	Ref          string          `json:"ref"`
	CheckoutSHA  string          `json:"checkout_sha"`
	Commits      []CommitModel   `json:"commits"`
	Repository   RepositoryModel `json:"respository"`
	UserUsername string          `json:"user_username"`
}

// RepositoryModel ...
type RepositoryModel struct {
	VisibilityLevel int    `json:"visibility_level"`
	GitSSHURL       string `json:"git_ssh_url"`
	GitHTTPURL      string `json:"git_http_url"`
}

// TagPushEventModel ...
type TagPushEventModel struct {
	ObjectKind   string          `json:"object_kind"`
	Ref          string          `json:"ref"`
	CheckoutSHA  string          `json:"checkout_sha"`
	Repository   RepositoryModel `json:"respository"`
	UserUsername string          `json:"user_username"`
}

// BranchInfoModel ...
type BranchInfoModel struct {
	VisibilityLevel int    `json:"visibility_level"`
	GitSSHURL       string `json:"git_ssh_url"`
	GitHTTPURL      string `json:"git_http_url"`
	Namespace       string `json:"namespace"`
}

// LastCommitInfoModel ...
type LastCommitInfoModel struct {
	SHA string `json:"id"`
}

// LabelInfoModel ...
type LabelInfoModel struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// MergeRequestInfoModel ...
type MergeRequestInfoModel struct {
	ID             int                 `json:"iid"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	State          string              `json:"state"`
	Action         string              `json:"action"`
	MergeStatus    string              `json:"merge_status"`
	MergeCommitSHA string              `json:"merge_commit_sha"`
	MergeError     string              `json:"merge_error"`
	Oldrev         string              `json:"oldrev"`
	Source         BranchInfoModel     `json:"source"`
	SourceBranch   string              `json:"source_branch"`
	Target         BranchInfoModel     `json:"target"`
	TargetBranch   string              `json:"target_branch"`
	LastCommit     LastCommitInfoModel `json:"last_commit"`
	Draft          bool                `json:"draft"`
	Labels         []LabelInfoModel    `json:"labels"`
}

// UserModel ...
type UserModel struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

// BoolChanges ...
type BoolChanges struct {
	Previous bool `json:"previous"`
	Current  bool `json:"current"`
}

// Changes ...
type Changes struct {
	Draft  BoolChanges  `json:"draft"`
	Labels LabelChanges `json:"labels"`
}

// LabelChanges ...
type LabelChanges struct {
	Previous []LabelInfoModel `json:"previous"`
	Current  []LabelInfoModel `json:"current"`
}

// MergeRequestEventModel ...
type MergeRequestEventModel struct {
	ObjectKind       string                `json:"object_kind"`
	ObjectAttributes MergeRequestInfoModel `json:"object_attributes"`
	Labels           []LabelInfoModel      `json:"labels"`
	User             UserModel             `json:"user"`
	Changes          Changes               `json:"changes"`
}

// CommentInfoModel ...
type CommentInfoModel struct {
	ID           int    `json:"id"`
	Note         string `json:"note"`
	NoteableType string `json:"noteable_type"`
}

// MergeRequestCommentEventModel ...
type MergeRequestCommentEventModel struct {
	ObjectKind       string                `json:"object_kind"`
	ObjectAttributes CommentInfoModel      `json:"object_attributes"`
	MergeRequest     MergeRequestInfoModel `json:"merge_request"`
	User             UserModel             `json:"user"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct {
	timeProvider hookCommon.TimeProvider
	logger       *zap.Logger
}

// NewHookProvider ...
func NewHookProvider(timeProvider hookCommon.TimeProvider, logger *zap.Logger) HookProvider {
	return HookProvider{
		timeProvider: timeProvider,
		logger:       logger,
	}
}

// NewDefaultHookProvider ...
func NewDefaultHookProvider(logger *zap.Logger) HookProvider {
	return NewHookProvider(hookCommon.NewDefaultTimeProvider(), logger)
}

func detectContentTypeAndEventID(header http.Header) (string, string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", "", errors.New("No Content-Type Header found")
	}

	eventID := header.Get("X-Gitlab-Event")
	if eventID == "" {
		return "", "", errors.New("No X-Gitlab-Event Header found")
	}

	return contentType, eventID, nil
}

func isAcceptEventType(eventKey string) bool {
	return slices.Contains([]string{tagPushEventID, codePushEventID, mergeRequestEventID, commentEventID}, eventKey)
}

func isAcceptMergeRequestState(prState string) bool {
	return slices.Contains([]string{"opened", "reopened"}, prState)
}

func isAcceptMergeRequestAction(prAction string, prOldrev string, changes Changes) bool {
	if prAction == "open" {
		return true
	}
	if prAction == "update" {
		// an "update" with "oldrev" present is a code change
		if prOldrev != "" {
			return true
		}

		// converted from draft to ready to review
		if changes.Draft.Previous == true && changes.Draft.Current == false {
			return true
		}

		// new labels were added
		newLabels := changes.getNewLabels()
		return len(newLabels) > 0
	}

	return false
}

func (changes Changes) getNewLabels() []string {
	labelMap := make(map[int64]string)
	for _, label := range changes.Labels.Current {
		labelMap[label.ID] = label.Title
	}
	for _, label := range changes.Labels.Previous {
		delete(labelMap, label.ID)
	}

	var newLabels []string
	for _, label := range labelMap {
		newLabels = append(newLabels, label)
	}
	slices.Sort(newLabels)
	return newLabels
}

func (branchInfoModel BranchInfoModel) getRepositoryURL() string {
	if branchInfoModel.VisibilityLevel == gitlabPublicVisibilityLevel {
		return branchInfoModel.GitHTTPURL
	}
	return branchInfoModel.GitSSHURL
}

func (repository RepositoryModel) getRepositoryURL() string {
	if repository.VisibilityLevel == gitlabPublicVisibilityLevel {
		return repository.GitHTTPURL
	}
	return repository.GitSSHURL
}

func (hp HookProvider) transformCodePushEvent(codePushEvent CodePushEventModel) hookCommon.TransformResultModel {
	if !strings.HasPrefix(codePushEvent.Ref, "refs/heads/") {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("Ref (%s) is not a head ref", codePushEvent.Ref),
			ShouldSkip:                 true,
		}
	}
	branch := strings.TrimPrefix(codePushEvent.Ref, "refs/heads/")

	// In case of squashed merge requests, Gitlab sends 3 event hooks: a Push, a Merge Request and another Push.
	// The 2nd Push event does not contain commits and its checkout_sha is set to null.
	//
	// Related issue: https://bitrise.atlassian.net/browse/SSW-127
	if codePushEvent.CheckoutSHA == "" {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("The 'checkout_sha' field is not set - potential squashed merge request"),
			ShouldSkip:                 true,
		}
	}

	lastCommit := CommitModel{}
	isLastCommitFound := false
	for _, aCommit := range codePushEvent.Commits {
		if aCommit.CommitHash == codePushEvent.CheckoutSHA {
			isLastCommitFound = true
			lastCommit = aCommit
			break
		}
	}

	if !isLastCommitFound {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      errors.New("The commit specified by 'checkout_sha' was not included in the 'commits' array - no match found"),
		}
	}

	var commitPaths []bitriseapi.CommitPaths
	var commitMessages []string
	for _, aCommit := range codePushEvent.Commits {
		commitPaths = append(commitPaths, bitriseapi.CommitPaths{
			Added:    aCommit.AddedFiles,
			Removed:  aCommit.RemovedFiles,
			Modified: aCommit.ModifiedFiles,
		})
		commitMessages = append(commitMessages, aCommit.CommitMessage)
	}
	maxSize := envVarSizeLimitInByte()
	commitMessagesStr, err := hp.commitMessagesToString(commitMessages, maxSize)
	if err != nil {
		hp.logger.Warn("gitlab.HookProvider.transformCodePushEvent: failed to convert commit messages", zap.Error(err))
	}

	var environments []bitriseapi.EnvironmentItem
	if len(commitMessagesStr) > 0 {
		environments = []bitriseapi.EnvironmentItem{
			{Name: commitMessagesEnvKey, Value: commitMessagesStr, IsExpand: false},
		}
	}
	return hookCommon.TransformResultModel{
		DontWaitForTriggerResponse: true,
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:        lastCommit.CommitHash,
					CommitMessage:     lastCommit.CommitMessage,
					CommitMessages:    commitMessages,
					PushCommitPaths:   commitPaths,
					Branch:            branch,
					BaseRepositoryURL: codePushEvent.Repository.getRepositoryURL(),
					Environments:      environments,
				},
				TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, codePushEvent.UserUsername),
			},
		},
	}
}

func envVarSizeLimitInByte() int {
	config, err := envman.GetConfigs()
	if err == nil {
		return config.EnvBytesLimitInKB * kbToB
	}
	return fallbackEnvBytesLimitInKB * kbToB
}

func decreaseMaxMessageSizeByControlCharsSize(commitMessages []string, maxSize int) int {
	controlCharsPerMessageSize := len([]byte("- \n"))
	controlCharsSize := len(commitMessages) * controlCharsPerMessageSize
	return maxSize - controlCharsSize
}

func (hp HookProvider) ensureCommitMessagesSize(commitMessages []string, maxSize int) ([]string, error) {
	commitMessagesCount := len(commitMessages)
	if commitMessagesCount > 20 {
		// The count of push events commits, shouldn't be more than 20:
		// https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html#push-events
		// With this limit, 256KB max env var size, and 20 commits, every commit message has ~12KB (~12.000 chars) limitation.
		// A higher number of commit messages might require a more sophisticated size limitation mechanism.
		hp.logger.Warn(fmt.Sprintf("Expected 20 commits in the push event, got: %d, limiting commit messages count to 20", commitMessagesCount))
		commitMessages = commitMessages[:20]
	}

	maxSize = decreaseMaxMessageSizeByControlCharsSize(commitMessages, maxSize)
	if maxSize <= 0 {
		return nil, fmt.Errorf("max messages size should be greater than 0, got: %d", maxSize)
	}

	maxMessageSize := int(math.Floor(float64(maxSize) / float64(len(commitMessages))))
	trimmedMessageSuffix := []byte("...")
	trimmedMessageSuffixSize := len(trimmedMessageSuffix)
	if maxMessageSize-trimmedMessageSuffixSize <= 0 {
		return nil, fmt.Errorf("max message size should be greater than %d, got: %d", trimmedMessageSuffixSize, maxMessageSize)
	}

	for idx, message := range commitMessages {
		messageBytes := []byte(message)
		messageSize := len(messageBytes)
		if messageSize > maxMessageSize {
			trimmedMessageBytes := messageBytes[:maxMessageSize-trimmedMessageSuffixSize]
			commitMessages[idx] = string(append(trimmedMessageBytes, trimmedMessageSuffix...))
		}
	}

	return commitMessages, nil
}

func (hp HookProvider) commitMessagesToString(commitMessages []string, maxSize int) (string, error) {
	var err error
	commitMessages, err = hp.ensureCommitMessagesSize(commitMessages, maxSize)
	if err != nil {
		return "", err
	}

	commitMessagesStr := ""
	for _, commitMessage := range commitMessages {
		commitMessagesStr += fmt.Sprintf("- %s\n", commitMessage)
	}
	return commitMessagesStr, nil
}

func transformTagPushEvent(tagPushEvent TagPushEventModel) hookCommon.TransformResultModel {
	if tagPushEvent.ObjectKind != "tag_push" {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("Not a Tag Push object: %s", tagPushEvent.ObjectKind),
		}
	}

	if !strings.HasPrefix(tagPushEvent.Ref, "refs/tags/") {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("Ref (%s) is not a tags ref", tagPushEvent.Ref),
		}
	}
	tag := strings.TrimPrefix(tagPushEvent.Ref, "refs/tags/")

	if len(tagPushEvent.CheckoutSHA) < 1 {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      errors.New("This is a Tag Deleted event, no build is required"),
			ShouldSkip:                 true,
		}
	}

	return hookCommon.TransformResultModel{
		DontWaitForTriggerResponse: true,
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:               tag,
					CommitHash:        tagPushEvent.CheckoutSHA,
					BaseRepositoryURL: tagPushEvent.Repository.getRepositoryURL(),
				},
				TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, tagPushEvent.UserUsername),
			},
		},
	}
}

func transformMergeRequestEvent(event MergeRequestEventModel) hookCommon.TransformResultModel {
	if event.ObjectKind != "merge_request" {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      errors.New("Not a Merge Request object"),
			ShouldSkip:                 true,
		}
	}

	mergeRequest := event.ObjectAttributes
	if !isAcceptMergeRequestAction(mergeRequest.Action, mergeRequest.Oldrev, event.Changes) {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("Merge Request action doesn't require a build: %s", mergeRequest.Action),
			ShouldSkip:                 true,
		}
	}

	newLabels := event.Changes.getNewLabels()
	readyState := mergeRequestReadyState(event)
	user := event.User

	return transformMergeRequest(mergeRequest, user, readyState, newLabels, "")
}

func transformMergeRequestCommentEvent(event MergeRequestCommentEventModel) hookCommon.TransformResultModel {
	if event.ObjectKind != "note" {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      errors.New("Not a Note object"),
			ShouldSkip:                 true,
		}
	}

	if event.ObjectAttributes.NoteableType != "MergeRequest" {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      errors.New("Not a Merge Request note"),
			ShouldSkip:                 true,
		}
	}

	comment := event.ObjectAttributes
	mergeRequest := event.MergeRequest
	user := event.User
	var newLabels []string

	var readyState bitriseapi.PullRequestReadyState
	if mergeRequest.Draft {
		readyState = bitriseapi.PullRequestReadyStateDraft
	} else {
		readyState = bitriseapi.PullRequestReadyStateReadyForReview
	}

	return transformMergeRequest(mergeRequest, user, readyState, newLabels, comment.Note)
}

func transformMergeRequest(
	mergeRequest MergeRequestInfoModel,
	user UserModel,
	readyState bitriseapi.PullRequestReadyState,
	newLabels []string,
	comment string,
) hookCommon.TransformResultModel {
	if mergeRequest.State == "" {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      errors.New("No Merge Request state specified"),
			ShouldSkip:                 true,
		}
	}

	if mergeRequest.MergeCommitSHA != "" {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      errors.New("Merge Request already merged"),
			ShouldSkip:                 true,
		}
	}

	if !isAcceptMergeRequestState(mergeRequest.State) {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("Merge Request state doesn't require a build: %s", mergeRequest.State),
			ShouldSkip:                 true,
		}
	}

	if mergeRequest.MergeStatus == "cannot_be_merged" || mergeRequest.MergeError != "" {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      errors.New("Merge Request is not mergeable"),
			ShouldSkip:                 true,
		}
	}

	commitMsg := mergeRequest.Title
	if mergeRequest.Description != "" {
		commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, mergeRequest.Description)
	}

	var mergeRef string
	mergeStatus := mergeRequest.MergeStatus
	if mergeStatus != "preparing" && mergeStatus != "unchecked" {
		mergeRef = fmt.Sprintf("merge-requests/%d/merge", mergeRequest.ID)
	}

	var labels []string
	for _, label := range mergeRequest.Labels {
		labels = append(labels, label.Title)
	}

	return hookCommon.TransformResultModel{
		DontWaitForTriggerResponse: true,
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitMessage:            commitMsg,
					CommitHash:               mergeRequest.LastCommit.SHA,
					Branch:                   mergeRequest.SourceBranch,
					BranchRepoOwner:          mergeRequest.Source.Namespace,
					BranchDest:               mergeRequest.TargetBranch,
					BranchDestRepoOwner:      mergeRequest.Target.Namespace,
					PullRequestID:            &mergeRequest.ID,
					BaseRepositoryURL:        mergeRequest.Target.getRepositoryURL(),
					HeadRepositoryURL:        mergeRequest.Source.getRepositoryURL(),
					PullRequestRepositoryURL: mergeRequest.Source.getRepositoryURL(),
					PullRequestAuthor:        user.Name,
					PullRequestMergeBranch:   mergeRef,
					PullRequestHeadBranch:    fmt.Sprintf("merge-requests/%d/head", mergeRequest.ID),
					PullRequestReadyState:    readyState,
					PullRequestLabelsAdded:   newLabels,
					PullRequestLabels:        labels,
					PullRequestComment:       comment,
				},
				TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, user.Username),
			},
		},
		SkippedByPrDescription: !hookCommon.IsSkipBuildByCommitMessage(mergeRequest.Title) &&
			hookCommon.IsSkipBuildByCommitMessage(mergeRequest.Description),
	}
}

func mergeRequestReadyState(mergeRequest MergeRequestEventModel) bitriseapi.PullRequestReadyState {
	// converted from draft to ready to review
	if mergeRequest.Changes.Draft.Previous == true && mergeRequest.Changes.Draft.Current == false {
		return bitriseapi.PullRequestReadyStateConvertedToReadyForReview
	}

	if mergeRequest.ObjectAttributes.Draft {
		return bitriseapi.PullRequestReadyStateDraft
	}

	return bitriseapi.PullRequestReadyStateReadyForReview
}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	contentType, eventID, err := detectContentTypeAndEventID(r.Header)
	if err != nil {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("Issue with Headers: %s", err),
		}
	}

	if contentType != "application/json" {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	if !isAcceptEventType(eventID) {
		// Unsupported Event
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("Unsupported Webhook event: %s", eventID),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			DontWaitForTriggerResponse: true,
			Error:                      fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	if eventID == codePushEventID {
		// code push
		var codePushEvent CodePushEventModel
		if contentType == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&codePushEvent); err != nil {
				return hookCommon.TransformResultModel{
					DontWaitForTriggerResponse: true,
					Error:                      fmt.Errorf("Failed to parse request body: %s", err),
				}
			}
		}
		return hp.transformCodePushEvent(codePushEvent)
	} else if eventID == tagPushEventID {
		// tag push
		var tagPushEvent TagPushEventModel
		if contentType == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&tagPushEvent); err != nil {
				return hookCommon.TransformResultModel{
					DontWaitForTriggerResponse: true,
					Error:                      fmt.Errorf("Failed to parse request body: %s", err),
				}
			}
		}
		return transformTagPushEvent(tagPushEvent)
	} else if eventID == mergeRequestEventID {
		var mergeRequestEvent MergeRequestEventModel
		if err := json.NewDecoder(r.Body).Decode(&mergeRequestEvent); err != nil {
			return hookCommon.TransformResultModel{
				DontWaitForTriggerResponse: true,
				Error:                      fmt.Errorf("Failed to parse request body as JSON: %s", err),
			}
		}

		return transformMergeRequestEvent(mergeRequestEvent)
	} else if eventID == commentEventID {
		var commentEvent MergeRequestCommentEventModel
		if err := json.NewDecoder(r.Body).Decode(&commentEvent); err != nil {
			return hookCommon.TransformResultModel{
				DontWaitForTriggerResponse: true,
				Error:                      fmt.Errorf("Failed to parse request body as JSON: %s", err),
			}
		}

		return transformMergeRequestCommentEvent(commentEvent)
	}

	return hookCommon.TransformResultModel{
		DontWaitForTriggerResponse: true,
		Error:                      fmt.Errorf("Unsupported GitLab event type: %s", eventID),
	}
}
