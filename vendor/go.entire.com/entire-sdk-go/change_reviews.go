package entire

import (
	"context"
	"iter"
	"net/http"
)

// This file holds code review on a Change: reviews and their comments.

// ReviewCommentStatus is a review comment's status.
type ReviewCommentStatus string

// Review comment statuses.
const (
	ReviewCommentStatusOpen      ReviewCommentStatus = "open"
	ReviewCommentStatusResolved  ReviewCommentStatus = "resolved"
	ReviewCommentStatusDismissed ReviewCommentStatus = "dismissed"
)

// CommentGranularity is how much code a review comment covers.
type CommentGranularity string

// Comment granularities.
const (
	CommentGranularityLine        CommentGranularity = "line"
	CommentGranularityRange       CommentGranularity = "range"
	CommentGranularityFile        CommentGranularity = "file"
	CommentGranularityWholeChange CommentGranularity = "whole_change" // the whole Change
)

// ReviewSeverity is how serious a review comment is.
type ReviewSeverity string

// Review comment severities.
const (
	ReviewSeverityHigh   ReviewSeverity = "high"
	ReviewSeverityMedium ReviewSeverity = "medium"
	ReviewSeverityLow    ReviewSeverity = "low"
)

// CommentStaleness is whether a review comment applies to the Change's
// current code.
type CommentStaleness string

// Comment stalenesses. CommentStalenessAny only filters.
const (
	CommentStalenessCurrent CommentStaleness = "current"
	CommentStalenessStale   CommentStaleness = "stale"
	CommentStalenessAny     CommentStaleness = "any"
)

// SuggestedChangeType is the form of a suggested change.
type SuggestedChangeType string

// Suggested change types.
const (
	SuggestedChangeTypeUnifiedDiff       SuggestedChangeType = "unified_diff"
	SuggestedChangeTypeFileReplacement   SuggestedChangeType = "file_replacement"
	SuggestedChangeTypeJSONPatch         SuggestedChangeType = "json_patch"
	SuggestedChangeTypeManualInstruction SuggestedChangeType = "manual_instruction"
)

// PublishedCommentStatus is what publishing did with a comment.
type PublishedCommentStatus string

// Published comment statuses.
const (
	PublishedCommentStatusCreated PublishedCommentStatus = "created"
	// PublishedCommentStatusExisting is a comment with the same ClientID
	// that an earlier publish already created.
	PublishedCommentStatusExisting PublishedCommentStatus = "existing"
)

// Review is a code review of a Change.
type Review struct {
	// ID identifies the review; pass it to GetReview and PublishReview.
	ID string `json:"id"`
	// ChangeID is the internal ID of the reviewed Change.
	ChangeID string `json:"trail_id"`
	// ActorID is the account that ran the review, or "agent:" and a runner
	// ID for a review runner. It is empty if no one is recorded.
	ActorID string `json:"actor_id"`
	// Summary is the review's summary, once published, or empty.
	Summary string `json:"summary"`
	// StartedAt is when StartReview began the review.
	StartedAt Timestamp `json:"started_at"`
	// CompletedAt is when the review was published. PublishReview sets it;
	// it is zero in a review that GetReview returns.
	CompletedAt Timestamp `json:"completed_at"`

	// CodeVersion is the code that the review covers. GetReview sets it.
	CodeVersion *CodeVersion `json:"-"`
	// Counts tallies all of the review's comments, whatever the
	// ReviewGetOptions. GetReview sets it.
	Counts *ReviewCounts `json:"-"`
	// Comments holds one page of the review's comments, newest first;
	// Response.Next continues them. GetReview sets them.
	Comments []ReviewComment `json:"-"`
}

// CodeVersion is the code that a review covers.
type CodeVersion struct {
	// ID identifies the code version, which ReviewComment.CodeVersionID
	// refers to.
	ID string `json:"id"`
	// BaseRef is the base branch or other ref, or empty if none was given.
	BaseRef string `json:"base_ref"`
	// HeadRef is the head branch or other ref, or empty if none was given.
	HeadRef string `json:"head_ref"`
	// BaseSHA is the base commit, or empty if unknown.
	BaseSHA string `json:"base_sha"`
	// HeadSHA is the head commit, or empty if unknown.
	HeadSHA string `json:"head_sha"`
	// At is when the code version was captured.
	At Timestamp `json:"captured_at"`
}

// ReviewCounts tallies a review's comments by status. Every comment counts
// toward Total and one status; Stale overlaps the status counts, so the
// counts do not add up to Total.
type ReviewCounts struct {
	// Open counts the open comments.
	Open int64 `json:"open"`
	// Resolved counts the resolved comments.
	Resolved int64 `json:"resolved"`
	// Dismissed counts the dismissed comments.
	Dismissed int64 `json:"dismissed"`
	// Stale counts the comments on code that has since changed, whatever
	// their status.
	Stale int64 `json:"stale"`
	// Total counts all of the review's comments, dismissed ones included.
	Total int64 `json:"total"`
}

// ReviewComment is a comment of a review: a finding about the code, with
// where it applies and, optionally, a suggested change.
type ReviewComment struct {
	// ID identifies the comment; pass it to UpdateReviewComment.
	ID string `json:"id"`
	// ReviewID is the review the comment belongs to.
	ReviewID string `json:"review_id"`
	// ChangeID is the internal ID of the reviewed Change.
	ChangeID string `json:"trail_id"`
	// CodeVersionID is the code version the comment was made against.
	CodeVersionID string `json:"code_version_id"`
	// Body is the comment's text, or empty if it has none.
	Body string `json:"body"`
	// Severity is how serious the finding is, or empty when unset.
	Severity ReviewSeverity `json:"severity"`
	// Confidence is the reviewer's confidence in the finding, from 0 to 1,
	// or zero when unset.
	Confidence float64 `json:"confidence"`
	// Status is ReviewCommentStatusOpen, ReviewCommentStatusResolved, or
	// ReviewCommentStatusDismissed.
	Status ReviewCommentStatus `json:"status"`
	// StatusReason explains the status, such as why a comment was
	// dismissed.
	StatusReason string `json:"status_reason"`
	// AuthorID is the account that wrote the comment, or "agent:" and a
	// runner ID for a review runner. It is empty if no one is recorded.
	AuthorID string `json:"actor_id"`
	// AuthorName is the author's login, or empty if it is unknown.
	AuthorName string `json:"actor_display_name"`
	// Author describes the author.
	Author Identity `json:"actor_identity"`
	// Location is where in the code the comment applies.
	Location CommentLocation `json:"location"`
	// DiscussionID is the discussion of the comment, or empty if it has
	// none.
	DiscussionID string `json:"discussion_id"`
	// MessageCount counts the messages of the comment's discussion, which
	// is zero when it has none.
	MessageCount int64 `json:"discussion_message_count"`
	// StaleOutcome reports whether the comment still applies to the Change's
	// current code: CommentStalenessCurrent or CommentStalenessStale.
	StaleOutcome CommentStaleness `json:"stale_outcome"`
	// ClientID is the ID the comment was published with; see
	// NewReviewComment.ClientID.
	ClientID string `json:"client_id"`
	// CreatedAt is when the comment was published.
	CreatedAt Timestamp `json:"created_at"`
	// UpdatedAt is when the comment last changed.
	UpdatedAt Timestamp `json:"updated_at"`
	// SuggestedChanges are the changes the comment suggests. GetReview sets
	// them.
	SuggestedChanges []SuggestedChange `json:"suggested_changes"`
}

// CommentLocation is where in the code a review comment applies. Lines and
// columns start at 1; zero means unset.
type CommentLocation struct {
	// Granularity is how much code the comment covers. It is required, and
	// decides which other fields are: CommentGranularityLine needs FilePath,
	// StartLine, and SelectedText; CommentGranularityRange needs those and
	// EndLine; CommentGranularityFile needs FilePath; and
	// CommentGranularityWholeChange needs none.
	Granularity CommentGranularity `json:"granularity"`
	// FilePath is the path of the file the comment is about.
	FilePath string `json:"file_path,omitempty"`
	// StartLine is the first line the comment covers.
	StartLine int64 `json:"start_line,omitempty"`
	// StartColumn is the column within StartLine where the covered code
	// starts.
	StartColumn int64 `json:"start_column,omitempty"`
	// EndLine is the last line the comment covers. It must not be before
	// StartLine.
	EndLine int64 `json:"end_line,omitempty"`
	// EndColumn is the column within EndLine where the covered code ends.
	EndColumn int64 `json:"end_column,omitempty"`
	// SelectedText is the code the comment is about.
	SelectedText string `json:"selected_text,omitempty"`
	// NearbyText is code around SelectedText.
	NearbyText string `json:"nearby_text,omitempty"`
	// Language is the language of the code.
	Language string `json:"language,omitempty"`
}

// SuggestedChange is a change that a review comment suggests.
type SuggestedChange struct {
	// Type is the form of the change, which decides how Patch is read.
	Type SuggestedChangeType `json:"change_type"`
	// Patch is the change, in the form Type names: a unified diff of a
	// single file, the file's replacement, or a JSON Patch array. Every
	// Type but SuggestedChangeTypeManualInstruction requires it.
	Patch string `json:"patch,omitempty"`
	// Instruction describes a manual change, and is required when Type is
	// SuggestedChangeTypeManualInstruction.
	Instruction string `json:"instruction,omitempty"`
	// ExpectedFilePath is the path of the file the change applies to.
	// It and the other Expected fields describe the code the change
	// applies to, so it is not applied to code that has changed; every
	// Type but SuggestedChangeTypeManualInstruction requires them.
	ExpectedFilePath string `json:"expected_file_path,omitempty"`
	// ExpectedFileHash is a hash of the file the change applies to.
	ExpectedFileHash string `json:"expected_file_hash,omitempty"`
	// ExpectedStartLine is the first line the change replaces, from 1.
	ExpectedStartLine int64 `json:"expected_start_line,omitempty"`
	// ExpectedEndLine is the last line the change replaces. It must not be
	// before ExpectedStartLine.
	ExpectedEndLine int64 `json:"expected_end_line,omitempty"`
	// ExpectedLines is the text of the lines from ExpectedStartLine to
	// ExpectedEndLine.
	ExpectedLines string `json:"expected_lines,omitempty"`
}

// ReviewStartOptions selects the code that StartReview covers. The zero
// value reviews the Change's branch against its base.
type ReviewStartOptions struct {
	// BaseRef is the base branch or other ref, of at most 255 characters.
	BaseRef string `json:"base_ref,omitempty"`
	// HeadRef is the head branch or other ref, of at most 255 characters.
	HeadRef string `json:"head_ref,omitempty"`
	// BaseSHA is the base commit, as a 40-character hexadecimal SHA.
	BaseSHA string `json:"base_sha,omitempty"`
	// HeadSHA is the head commit, as a 40-character hexadecimal SHA.
	HeadSHA string `json:"head_sha,omitempty"`
}

// StartedReview is a review that StartReview began.
type StartedReview struct {
	// ReviewID is the review to publish to with PublishReview.
	ReviewID string
	// CodeVersionID is the code version under review, which the review's
	// comments are made against.
	CodeVersionID string
	// BaseSHA is the base commit under review, or empty if unknown.
	BaseSHA string
	// HeadSHA is the head commit under review, or empty if unknown.
	HeadSHA string
	// MaxComments caps the comments one PublishReview call may carry.
	MaxComments int64
}

// ReviewPublishRequest is the body of ChangesService.PublishReview.
type ReviewPublishRequest struct {
	// Summary is the review's summary. Empty leaves the review's summary
	// unchanged.
	Summary string `json:"summary,omitempty"`
	// Comments are the review's comments, at most StartedReview.MaxComments
	// of them. Each needs a distinct ClientID.
	Comments []NewReviewComment `json:"comments"`
}

// NewReviewComment is a review comment to publish.
type NewReviewComment struct {
	// ClientID identifies the comment within its review, so that publishing
	// it again does not post it twice. It is at most 255 characters.
	// PublishReview generates one when it is empty, which covers only that
	// call's own retries; set it to repeat a publish yourself. Reusing a
	// ClientID for a different comment fails the publish.
	ClientID string `json:"client_id"`
	// Location is where in the code the comment applies; Granularity is
	// required.
	Location CommentLocation `json:"location"`
	// Body is the comment's text, of at most 65,536 characters.
	Body string `json:"body,omitempty"`
	// Severity is how serious the finding is. Empty leaves it unset.
	Severity ReviewSeverity `json:"severity,omitempty"`
	// Confidence is the reviewer's confidence in the finding, from 0 to 1.
	// Zero leaves it unset.
	Confidence float64 `json:"confidence,omitempty"`
	// Status is ReviewCommentStatusOpen unless set.
	Status ReviewCommentStatus `json:"status,omitempty"`
	// StatusReason explains the status, in at most 4,096 characters.
	StatusReason string `json:"status_reason,omitempty"`
	// SuggestedChange is a change the comment suggests, or nil for none.
	SuggestedChange *SuggestedChange `json:"suggested_change,omitempty"`
}

// PublishedReview is the outcome of PublishReview: the completed review,
// and each of its comments.
type PublishedReview struct {
	// Review is the review, now completed.
	Review Review `json:"review"`
	// Results report what publishing did with each comment, in the order of
	// ReviewPublishRequest.Comments.
	Results []PublishedComment `json:"results"`
}

// PublishedComment is one comment of a publish.
type PublishedComment struct {
	// ClientID is the comment's ID within the publish.
	ClientID string `json:"client_id"`
	// Status is PublishedCommentStatusCreated for a new comment, or
	// PublishedCommentStatusExisting for one an earlier publish created.
	Status PublishedCommentStatus `json:"status"`
	// Comment is the published comment.
	Comment *ReviewComment `json:"comment"`
	// SuggestedChange is the comment's suggested change, as stored, or nil
	// when the comment suggests none.
	SuggestedChange *SuggestedChange `json:"suggested_change"`
}

// ReviewCommentUpdate is the body of ChangesService.UpdateReviewComment.
// Unset fields are left unchanged, and Null clears a field.
type ReviewCommentUpdate struct {
	// Body replaces the comment's text, of at most 65,536 characters.
	Body Nullable[string] `json:"body,omitzero"`
	// Severity replaces how serious the finding is.
	Severity Nullable[ReviewSeverity] `json:"severity,omitzero"`
	// Confidence replaces the reviewer's confidence, from 0 to 1.
	Confidence Nullable[float64] `json:"confidence,omitzero"`
	// Status changes the comment's status. Empty leaves it unchanged; it
	// cannot be cleared. Resolving or dismissing a comment requires a
	// StatusReason, set here or already on the comment.
	Status ReviewCommentStatus `json:"status,omitempty"`
	// StatusReason replaces the explanation of the status. A resolved or
	// dismissed comment's reason cannot be cleared.
	StatusReason Nullable[string] `json:"status_reason,omitzero"`
}

// ReviewCommentListOptions filters ListReviewComments.
type ReviewCommentListOptions struct {
	// Status selects comments with any of these statuses. Selecting any
	// status overrides the default of leaving dismissed comments out.
	Status []ReviewCommentStatus `url:"status"`
	// Severity selects comments with any of these severities. Comments
	// without a severity are left out.
	Severity []ReviewSeverity `url:"severity"`
	// Stale selects comments by whether they apply to the Change's current
	// code; empty selects CommentStalenessCurrent, and CommentStalenessAny
	// selects all.
	Stale CommentStaleness `url:"stale"`
	// IncludeDismissed includes dismissed comments, which are left out
	// unless Status selects them.
	IncludeDismissed bool `url:"include_dismissed"`
	ListOptions
}

// ReviewGetOptions selects the page of comments that GetReview returns.
type ReviewGetOptions struct {
	// Stale selects comments by whether they apply to the Change's current
	// code; empty selects CommentStalenessCurrent, and CommentStalenessAny
	// selects all.
	Stale CommentStaleness `url:"stale"`
	// IncludeDismissed includes dismissed comments, which are otherwise
	// left out.
	IncludeDismissed bool `url:"include_dismissed"`
	ListOptions
}

// StartReview begins a review of a Change's code, for publishing comments to
// with PublishReview.
func (s *ChangesService) StartReview(ctx context.Context, repoID string, number int64, opts *ReviewStartOptions, reqOpts ...RequestOption) (*Response[StartedReview], error) {
	if opts == nil {
		opts = &ReviewStartOptions{}
	}
	req := repoRequest(http.MethodPost, repoID, opts, "/repos/%s/changes/%s/reviews", repoID, number)
	req.Header.Set("Idempotency-Key", NewIdempotencyKey())
	type started struct {
		ReviewID      string `json:"review_id"`
		CodeVersionID string `json:"code_version_id"`
		BaseSHA       string `json:"base_sha"`
		HeadSHA       string `json:"head_sha"`
		Limits        struct {
			MaxComments int64 `json:"max_comments_per_batch"`
		} `json:"limits"`
	}
	return unwrap(ctx, s.client, req, reqOpts, func(r *started) StartedReview {
		return StartedReview{ReviewID: r.ReviewID, CodeVersionID: r.CodeVersionID, BaseSHA: r.BaseSHA, HeadSHA: r.HeadSHA, MaxComments: r.Limits.MaxComments}
	})
}

// GetReview returns a review with one page of its comments, which
// Response.Next continues.
func (s *ChangesService) GetReview(ctx context.Context, repoID string, number int64, reviewID string, opts *ReviewGetOptions, reqOpts ...RequestOption) (*Response[Review], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s/reviews/%s", repoID, number, reviewID)
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "per_page", "cursor")
	}
	type snapshot struct {
		Review      Review          `json:"review"`
		CodeVersion CodeVersion     `json:"code_version"`
		Counts      ReviewCounts    `json:"counts"`
		Comments    []ReviewComment `json:"comments"`
		NextCursor  string          `json:"next_cursor"`
	}
	resp, err := Call[snapshot](ctx, s.client, req, reqOpts...)
	if resp == nil {
		return nil, err
	}
	out := &Response[Review]{StatusCode: resp.StatusCode, Header: resp.Header, HTTPResponse: resp.HTTPResponse, Next: resp.Data.NextCursor}
	if err == nil {
		out.Data = resp.Data.Review
		out.Data.CodeVersion, out.Data.Counts, out.Data.Comments = &resp.Data.CodeVersion, &resp.Data.Counts, resp.Data.Comments
	}
	return out, err
}

// PublishReview publishes a review's summary and comments together, and
// completes the review. If any comment is invalid, nothing is published and
// the call fails with 409 Conflict. A comment without a ClientID gets a
// generated one.
func (s *ChangesService) PublishReview(ctx context.Context, repoID string, number int64, reviewID string, body ReviewPublishRequest, reqOpts ...RequestOption) (*Response[PublishedReview], error) {
	publish := body
	publish.Comments = append([]NewReviewComment{}, body.Comments...)
	for i := range publish.Comments {
		if publish.Comments[i].ClientID == "" {
			publish.Comments[i].ClientID = NewIdempotencyKey()
		}
	}
	req := repoRequest(http.MethodPost, repoID, &publish, "/repos/%s/changes/%s/reviews/%s/comments", repoID, number, reviewID)
	req.Header.Set("Idempotency-Key", NewIdempotencyKey())
	return Call[PublishedReview](ctx, s.client, req, reqOpts...)
}

// ListReviewComments returns one page of the comments of all of a Change's
// reviews.
func (s *ChangesService) ListReviewComments(ctx context.Context, repoID string, number int64, opts *ReviewCommentListOptions, reqOpts ...RequestOption) (*Response[[]ReviewComment], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s/reviews/comments", repoID, number)
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "per_page", "cursor")
	}
	type envelope struct {
		Comments   []ReviewComment `json:"comments"`
		NextCursor string          `json:"next_cursor"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[ReviewComment] {
		return page[ReviewComment]{items: e.Comments, next: e.NextCursor}
	})
}

// ListReviewCommentsAll iterates over every comment that ListReviewComments
// returns, page by page.
func (s *ChangesService) ListReviewCommentsAll(ctx context.Context, repoID string, number int64, opts *ReviewCommentListOptions, reqOpts ...RequestOption) iter.Seq2[ReviewComment, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]ReviewComment], error) {
		p := o
		p.PageToken = token
		return s.ListReviewComments(ctx, repoID, number, &p, reqOpts...)
	})
}

// UpdateReviewComment changes a review comment, such as to resolve or
// dismiss it.
func (s *ChangesService) UpdateReviewComment(ctx context.Context, repoID string, number int64, reviewID, commentID string, body ReviewCommentUpdate, reqOpts ...RequestOption) (*Response[ReviewComment], error) {
	req := repoRequest(http.MethodPatch, repoID, body, "/repos/%s/changes/%s/reviews/%s/comments/%s", repoID, number, reviewID, commentID)
	return Call[ReviewComment](ctx, s.client, req, reqOpts...)
}
