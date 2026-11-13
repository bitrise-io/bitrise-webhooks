package entire

import (
	"context"
	"iter"
	"net/http"
)

// This file holds conversation on a project Trail. Its discussions and
// messages have the same shape as a Change's. Reading needs read access to
// the Trail, and writing needs permission to edit it. Only a person or a
// service account can write: an automation, such as a plugin installation
// token, fails with 403 Forbidden.

// ListDiscussions returns one page of the project Trail's discussions.
func (s *TrailsService) ListDiscussions(ctx context.Context, projectPath, trailID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]Discussion], error) {
	req := projectTrailRequest(http.MethodGet, projectPath, "", nil, "/%s/discussions", trailID)
	Deref(opts).add(req.Query, "pageSize", "pageToken")
	type envelope struct {
		Items         []Discussion `json:"items"`
		NextPageToken string       `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[Discussion] {
		return page[Discussion]{items: e.Items, next: e.NextPageToken}
	})
}

// ListDiscussionsAll iterates over every discussion that ListDiscussions
// returns, page by page.
func (s *TrailsService) ListDiscussionsAll(ctx context.Context, projectPath, trailID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[Discussion, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Discussion], error) {
		return s.ListDiscussions(ctx, projectPath, trailID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// GetDiscussion returns a discussion on a project Trail, with its messages.
func (s *TrailsService) GetDiscussion(ctx context.Context, projectPath, trailID, discussionID string, reqOpts ...RequestOption) (*Response[Discussion], error) {
	type detail struct {
		Discussion Discussion `json:"discussion"`
		Messages   []Message  `json:"messages"`
	}
	req := projectTrailRequest(http.MethodGet, projectPath, "", nil, "/%s/discussions/%s", trailID, discussionID)
	return unwrap(ctx, s.client, req, reqOpts, func(d *detail) Discussion {
		d.Discussion.Messages = d.Messages
		return d.Discussion
	})
}

// CreateDiscussion starts a discussion on a project Trail.
func (s *TrailsService) CreateDiscussion(ctx context.Context, projectPath, trailID string, body DiscussionCreateRequest, reqOpts ...RequestOption) (*Response[Discussion], error) {
	type created struct {
		Discussion Discussion `json:"discussion"`
		Message    Message    `json:"message"`
	}
	req := projectTrailRequest(http.MethodPost, projectPath, "", body, "/%s/discussions", trailID)
	return unwrap(ctx, s.client, req, reqOpts, func(c *created) Discussion {
		c.Discussion.Messages = []Message{c.Message}
		return c.Discussion
	})
}

// UpdateDiscussion retitles, resolves, or reopens a discussion on a project
// Trail. With IfMatch and the discussion's ETag, it fails with 412
// Precondition Failed if the discussion has changed since.
func (s *TrailsService) UpdateDiscussion(ctx context.Context, projectPath, trailID, discussionID string, body DiscussionUpdateRequest, reqOpts ...RequestOption) (*Response[Discussion], error) {
	type updated struct {
		Discussion Discussion `json:"discussion"`
	}
	req := projectTrailRequest(http.MethodPatch, projectPath, "", body, "/%s/discussions/%s", trailID, discussionID)
	return unwrap(ctx, s.client, req, reqOpts, func(u *updated) Discussion { return u.Discussion })
}

// CreateMessage adds a message to a discussion on a project Trail.
func (s *TrailsService) CreateMessage(ctx context.Context, projectPath, trailID, discussionID string, body MessageRequest, reqOpts ...RequestOption) (*Response[Message], error) {
	req := projectTrailRequest(http.MethodPost, projectPath, "", body, "/%s/discussions/%s/messages", trailID, discussionID)
	return unwrap(ctx, s.client, req, reqOpts, func(m *messageEnvelope) Message { return m.Message })
}

// UpdateMessage replaces the text of a message on a project Trail. With
// IfMatch and the message's ETag, it fails with 412 Precondition Failed if
// the message has changed since.
func (s *TrailsService) UpdateMessage(ctx context.Context, projectPath, trailID, discussionID, messageID string, body MessageRequest, reqOpts ...RequestOption) (*Response[Message], error) {
	req := projectTrailRequest(http.MethodPatch, projectPath, "", body, "/%s/discussions/%s/messages/%s", trailID, discussionID, messageID)
	return unwrap(ctx, s.client, req, reqOpts, func(m *messageEnvelope) Message { return m.Message })
}

// DeleteMessage deletes a message on a project Trail.
func (s *TrailsService) DeleteMessage(ctx context.Context, projectPath, trailID, discussionID, messageID string, reqOpts ...RequestOption) (*Response[struct{}], error) {
	req := projectTrailRequest(http.MethodDelete, projectPath, "", nil, "/%s/discussions/%s/messages/%s", trailID, discussionID, messageID)
	return Call[struct{}](ctx, s.client, req, reqOpts...)
}
