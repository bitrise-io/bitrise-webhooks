package entire

import (
	"context"
	"net/http"
	"net/url"
)

// This file holds what the service methods share: request builders, and
// helpers that take the result out of a response envelope.

// repoRequest builds an Entire API request about repository repoID, so that
// it reaches the cell that stores the repository. Its path is
// pathf(format, args...); an invalid argument fails the request when it is
// sent, after its options are released.
func repoRequest(method, repoID string, body any, format string, args ...any) *Request {
	return newRequest(method, Route{RepoID: repoID}, false, body, format, args)
}

// homeRequest builds an Entire API request about the caller rather than one
// repository, which the caller's home cell answers.
func homeRequest(method string, body any, format string, args ...any) *Request {
	return newRequest(method, Route{}, false, body, format, args)
}

// coreRequest builds a request for Core.
func coreRequest(method string, body any, format string, args ...any) *Request {
	return newRequest(method, Route{}, true, body, format, args)
}

func newRequest(method string, route Route, core bool, body any, format string, args []any) *Request {
	path, err := pathf(format, args...)
	return &Request{Method: method, Path: path, Query: url.Values{}, Header: http.Header{}, Body: body, Route: route, Core: core, invalid: err}
}

// unwrap sends req, decodes the JSON response into the envelope E, and
// returns the value that get takes out of it, with the response metadata.
func unwrap[E, T any](ctx context.Context, c *Client, req *Request, opts []RequestOption, get func(*E) T) (*Response[T], error) {
	resp, err := Call[E](ctx, c, req, opts...)
	if resp == nil {
		return nil, err
	}
	out := &Response[T]{StatusCode: resp.StatusCode, Header: resp.Header, HTTPResponse: resp.HTTPResponse}
	if err == nil {
		out.Data = get(&resp.Data)
	}
	return out, err
}

// page is what a list method takes out of a list envelope.
type page[T any] struct {
	items []T
	// next is the continuation token, empty on the last page.
	next string
	// total is the size of the whole listing, or zero when the API does not
	// report it.
	total int64
}

// list sends req and returns the items of the list envelope E, with
// Response.Next and Response.Total set from it.
func list[E, T any](ctx context.Context, c *Client, req *Request, opts []RequestOption, get func(*E) page[T]) (*Response[[]T], error) {
	resp, err := Call[E](ctx, c, req, opts...)
	if resp == nil {
		return nil, err
	}
	out := &Response[[]T]{StatusCode: resp.StatusCode, Header: resp.Header, HTTPResponse: resp.HTTPResponse}
	if err == nil {
		p := get(&resp.Data)
		out.Data, out.Next, out.Total = p.items, p.next, p.total
	}
	return out, err
}
