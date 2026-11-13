package entire

import (
	"context"
	"iter"
	"net/http"
)

// This file holds conversation on a Change: discussions and their messages.

// Discussion is a conversation on a Change: a general discussion, or the
// thread of a review comment.
type Discussion struct {
	// ID identifies the discussion; pass it to GetDiscussion and the other
	// discussion methods.
	ID string `json:"id"`
	// TrailID is the internal ID of what the discussion is on: a Change,
	// or a project Trail for the TrailsService discussion methods.
	TrailID string `json:"trail_id"`
	// Title is the discussion's title. A discussion created without one
	// takes the start of its first message, up to 120 characters, or
	// "Conversation" if it has no messages.
	Title string `json:"title"`
	// Kind is "discussion", or "code_review" for the thread of a review
	// comment.
	Kind string `json:"kind"`
	// Resolved reports a resolved discussion.
	Resolved bool `json:"resolved"`
	// ResolvedBy is the account that resolved the discussion, or empty if
	// it is not resolved.
	ResolvedBy string `json:"resolved_by"`
	// ResolvedAt is when the discussion was resolved, or zero if it is not.
	ResolvedAt Timestamp `json:"resolved_at"`
	// CreatedBy is the account that started the discussion, or empty if
	// none is recorded or a plugin started it.
	CreatedBy string `json:"created_by"`
	// CreatedByIdentity describes who started the discussion. For a
	// plugin, its Automation names the plugin and installation.
	CreatedByIdentity Identity `json:"created_by_identity"`
	// CreatedAt is when the discussion was started.
	CreatedAt Timestamp `json:"created_at"`
	// UpdatedAt is when the discussion last changed.
	UpdatedAt Timestamp `json:"updated_at"`
	// MessageCount counts the discussion's messages, replies included.
	MessageCount int64 `json:"message_count"`
	// LastMessageAuthor is the login of the latest message's author, or
	// empty if there are no messages.
	LastMessageAuthor string `json:"last_message_author"`
	// LastMessageAt is when the latest message was written, or zero if
	// there are no messages.
	LastMessageAt Timestamp `json:"last_message_at"`
	// Participants are the people who wrote in the discussion, ordered by
	// login.
	Participants []Participant `json:"participants"`
	// ReviewComment is the review comment that the discussion is the thread
	// of, or nil for a Kind of "discussion".
	ReviewComment *ReviewComment `json:"review_comment"`
	// ETag identifies this version of the discussion. Pass it to IfMatch to
	// make UpdateDiscussion change only this version.
	ETag string `json:"etag"`
	// Messages are the discussion's messages, oldest first. GetDiscussion
	// sets them, and CreateDiscussion sets the first.
	Messages []Message `json:"-"`
}

// Participant is a person who wrote in a discussion.
type Participant struct {
	// Login is the person's login.
	Login string `json:"login"`
}

// Message is a message in a discussion.
type Message struct {
	// ID identifies the message; pass it to UpdateMessage and
	// DeleteMessage.
	ID string `json:"id"`
	// Author is the login of the message's author, or empty for a plugin.
	Author string `json:"author"`
	// Identity describes the message's author. For a plugin, its
	// Automation names the plugin and installation.
	Identity Identity `json:"identity"`
	// Body is the message's text.
	Body string `json:"body"`
	// CreatedAt is when the message was written.
	CreatedAt Timestamp `json:"created_at"`
	// Replies are the replies to the message. A reply has no Replies or
	// ETag of its own.
	Replies []Message `json:"replies"`
	// ETag identifies this version of the message. Pass it to IfMatch to
	// make UpdateMessage or DeleteMessage act only on this version.
	ETag string `json:"etag"`
}

// DiscussionCreateRequest is the body of ChangesService.CreateDiscussion.
type DiscussionCreateRequest struct {
	// Title is the discussion's title, of at most 512 characters. Empty
	// takes a title from Body.
	Title string `json:"title,omitempty"`
	// Body is the first message, which is required, of at most 65,536
	// characters.
	Body string `json:"body"`
}

// DiscussionUpdateRequest is the body of ChangesService.UpdateDiscussion. Nil
// fields are left unchanged.
type DiscussionUpdateRequest struct {
	// Title retitles the discussion. It must not be blank, and is at most
	// 512 characters.
	Title *string `json:"title,omitempty"`
	// Resolved resolves the discussion when true, and reopens it when
	// false.
	Resolved *bool `json:"resolved,omitempty"`
}

// ListDiscussions returns one page of a Change's discussions.
func (s *ChangesService) ListDiscussions(ctx context.Context, repoID string, number int64, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]Discussion], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s/discussions", repoID, number)
	Deref(opts).add(req.Query, "per_page", "cursor")
	type envelope struct {
		Items      []Discussion `json:"items"`
		NextCursor string       `json:"next_cursor"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[Discussion] {
		return page[Discussion]{items: e.Items, next: e.NextCursor}
	})
}

// ListDiscussionsAll iterates over every discussion that ListDiscussions
// returns, page by page.
func (s *ChangesService) ListDiscussionsAll(ctx context.Context, repoID string, number int64, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[Discussion, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Discussion], error) {
		return s.ListDiscussions(ctx, repoID, number, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// GetDiscussion returns a discussion with its messages.
func (s *ChangesService) GetDiscussion(ctx context.Context, repoID string, number int64, discussionID string, reqOpts ...RequestOption) (*Response[Discussion], error) {
	type detail struct {
		Discussion Discussion `json:"discussion"`
		Messages   []Message  `json:"messages"`
	}
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s/discussions/%s", repoID, number, discussionID)
	return unwrap(ctx, s.client, req, reqOpts, func(d *detail) Discussion {
		d.Discussion.Messages = d.Messages
		return d.Discussion
	})
}

// CreateDiscussion starts a discussion on a Change.
func (s *ChangesService) CreateDiscussion(ctx context.Context, repoID string, number int64, body DiscussionCreateRequest, reqOpts ...RequestOption) (*Response[Discussion], error) {
	type created struct {
		Discussion Discussion `json:"discussion"`
		Message    Message    `json:"message"`
	}
	req := repoRequest(http.MethodPost, repoID, body, "/repos/%s/changes/%s/discussions", repoID, number)
	return unwrap(ctx, s.client, req, reqOpts, func(c *created) Discussion {
		c.Discussion.Messages = []Message{c.Message}
		return c.Discussion
	})
}

// UpdateDiscussion retitles, resolves, or reopens a discussion.
func (s *ChangesService) UpdateDiscussion(ctx context.Context, repoID string, number int64, discussionID string, body DiscussionUpdateRequest, reqOpts ...RequestOption) (*Response[Discussion], error) {
	type updated struct {
		Discussion Discussion `json:"discussion"`
	}
	req := repoRequest(http.MethodPatch, repoID, body, "/repos/%s/changes/%s/discussions/%s", repoID, number, discussionID)
	return unwrap(ctx, s.client, req, reqOpts, func(u *updated) Discussion { return u.Discussion })
}

// MessageRequest is the body of ChangesService.CreateMessage and
// ChangesService.UpdateMessage.
type MessageRequest struct {
	// Body is the message text, which is required, of at most 65,536
	// characters. Leading and trailing white space is removed.
	Body string `json:"body"`
}

// CreateMessage adds a message to a discussion.
func (s *ChangesService) CreateMessage(ctx context.Context, repoID string, number int64, discussionID string, body MessageRequest, reqOpts ...RequestOption) (*Response[Message], error) {
	req := repoRequest(http.MethodPost, repoID, body, "/repos/%s/changes/%s/discussions/%s/messages", repoID, number, discussionID)
	return unwrap(ctx, s.client, req, reqOpts, func(m *messageEnvelope) Message { return m.Message })
}

// UpdateMessage replaces the text of a message.
func (s *ChangesService) UpdateMessage(ctx context.Context, repoID string, number int64, discussionID, messageID string, body MessageRequest, reqOpts ...RequestOption) (*Response[Message], error) {
	req := repoRequest(http.MethodPatch, repoID, body, "/repos/%s/changes/%s/discussions/%s/messages/%s", repoID, number, discussionID, messageID)
	return unwrap(ctx, s.client, req, reqOpts, func(m *messageEnvelope) Message { return m.Message })
}

// messageEnvelope wraps a written message.
type messageEnvelope struct {
	Message Message `json:"message"`
}

// DeleteMessage deletes a message.
func (s *ChangesService) DeleteMessage(ctx context.Context, repoID string, number int64, discussionID, messageID string, reqOpts ...RequestOption) (*Response[struct{}], error) {
	req := repoRequest(http.MethodDelete, repoID, nil, "/repos/%s/changes/%s/discussions/%s/messages/%s", repoID, number, discussionID, messageID)
	return Call[struct{}](ctx, s.client, req, reqOpts...)
}
