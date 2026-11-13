package entire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

// Request describes one API call. The SDK's methods build requests for the
// endpoints they cover; Do and Call accept them directly for the endpoints
// the SDK does not cover, with the same authentication, routing, retry, and
// error handling.
type Request struct {
	// Method is the HTTP method, such as http.MethodGet.
	Method string
	// Path is the escaped request path relative to the client's base URL,
	// beginning with a slash, for example "/repos/" + url.PathEscape(id).
	Path string
	// Query holds query parameters. It may be nil.
	Query url.Values
	// Header holds request headers. It may be nil.
	Header http.Header
	// Body, when non-nil, is sent as JSON. Use WithRequestBody for other
	// media types.
	Body any
	// Anonymous suppresses the client's credentials for endpoints that must
	// not receive them. An explicit Authorization header is still sent.
	Anonymous bool
	// Idempotent marks the request as safe to repeat under the client's
	// RetryPolicy. GET, HEAD, and OPTIONS requests, and requests that carry an
	// Idempotency-Key header, are always treated as idempotent.
	Idempotent bool
	// AcceptStatus lists non-2xx status codes to return as results instead of
	// errors, such as 302 for a download redirect or 304 for a conditional
	// read. Every 2xx status is always a result.
	AcceptStatus []int
	// Route names the repository or project that the request addresses, so
	// the request reaches the cell that stores it.
	Route Route
	// Core sends the request to Core, the control plane that holds
	// accounts, organizations, projects, and the repository directory,
	// instead of the Entire API.
	Core bool

	// invalid, when set, fails the request when it is sent, after the
	// call's options are released.
	invalid error
	// base overrides the client's endpoint, for Core discovery reads.
	base *url.URL
}

// Response contains a decoded JSON value and the response metadata.
type Response[T any] struct {
	// Data is the decoded body, or the list or object that the body wraps.
	// An empty body leaves it as the zero value.
	Data T
	// Next continues a paginated listing: pass it as the PageToken option
	// to request the following page. It is empty on the last page and for
	// operations that do not paginate.
	Next string
	// Total is the number of items in the whole listing, for listings that
	// report it, and zero otherwise.
	Total int64
	// StatusCode is the HTTP status code.
	StatusCode int
	// Header holds the response headers, such as ETag. Keep ETags exactly as
	// returned, including their quotes.
	Header http.Header
	// HTTPResponse is the underlying response. Its body has already been read
	// and closed, and is replaced by http.NoBody.
	HTTPResponse *http.Response
}

// NewRequest returns a request for an endpoint that the SDK does not cover,
// ready for Do or Call. path is relative to the API's
// version prefix and must begin with a slash; escape its parameters with
// url.PathEscape. A non-nil body is sent as JSON. Query and Header are empty
// and ready to fill. Set Route when the endpoint reads or changes a
// repository or project, so the request reaches the cell that stores it.
func (c *Client) NewRequest(method, path string, body any) *Request {
	return &Request{Method: method, Path: path, Query: url.Values{}, Header: http.Header{}, Body: body}
}

type requestOptions struct {
	headers      http.Header
	query        url.Values
	body         io.Reader
	bodySet      bool
	responseBody io.Writer
	route        *Route
}

// RequestOption configures a single request. Options run after the method
// sets its own parameters, so an option can override a header or query value
// that the method sets.
type RequestOption func(*requestOptions) error

// WithHeader sets a request header, including conditional or idempotency
// headers.
func WithHeader(name, value string) RequestOption {
	return func(o *requestOptions) error {
		if name == "" {
			return errors.New("entire: header name must not be empty")
		}
		o.headers.Set(name, value)
		return nil
	}
}

// IfMatch makes a write conditional on the resource still being at version
// etag, such as the ETag header of an earlier write's response. If the
// resource has changed since, nothing is written and the call fails with 412
// Precondition Failed. Keep the ETag exactly as returned, including its quotes
// and W/ prefix.
func IfMatch(etag string) RequestOption {
	return WithHeader("If-Match", etag)
}

// WithRoute sends the request to the cell that route locates, in place of the
// one the method chooses. A caller granted only a repository, such as a
// service account, passes Route{RepoID: ...} to reach a project Trail with
// methods such as Trails.GetInProject and Trails.UpdateInProject: by default
// they locate the project, which such a caller cannot read. Core requests ignore it.
func WithRoute(route Route) RequestOption {
	return func(o *requestOptions) error {
		if route.key() == "" {
			return errors.New("entire: WithRoute needs a repository or project")
		}
		o.route = &route
		return nil
	}
}

// WithQueryParameter sets all values of a query parameter, replacing any
// that the method sets. Use it for a parameter that a method does not take.
func WithQueryParameter(name string, values ...string) RequestOption {
	values = append([]string(nil), values...)
	return func(o *requestOptions) error {
		if name == "" {
			return errors.New("entire: query parameter name must not be empty")
		}
		o.query[name] = append([]string(nil), values...)
		return nil
	}
}

// WithRequestBody replaces the method's JSON body, for example with a
// documented application/octet-stream alternative. From the moment the call
// starts, the SDK owns the body: it is closed, when it implements io.Closer,
// on every path, including validation errors before a request is sent.
// Bodies from bytes.Reader, bytes.Buffer, or strings.Reader can be replayed
// by a RetryPolicy; other bodies are sent at most once. Pass it to a single
// call: the All iterators and the Watch methods send each of their requests
// with the same options. Pass http.NoBody to send an empty body.
func WithRequestBody(contentType string, body io.Reader) RequestOption {
	return func(o *requestOptions) error {
		if contentType == "" {
			return errors.New("entire: request content type must not be empty")
		}
		if body == nil {
			return errors.New("entire: request body must not be nil")
		}
		if o.bodySet && !sameBody(o.body, body) {
			closeBody(o.body)
		}
		o.body, o.bodySet = body, true
		o.headers.Set("Content-Type", contentType)
		return nil
	}
}

// WithResponseBody copies the raw bytes of a successful response into w as
// the SDK reads them, alongside the decoded Data of a typed method. Use it to
// log a response, or to read members that the SDK does not model. A JSON
// method writes the body it decoded, up to the response size limit; a raw
// operation writes what the caller reads from the body; a stream writes the
// event stream as it is read. Error responses, and attempts that were
// retried or redirected to another endpoint, write nothing. An error from w
// ends the read and is returned.
func WithResponseBody(w io.Writer) RequestOption {
	return func(o *requestOptions) error {
		if w == nil {
			return errors.New("entire: response body writer must not be nil")
		}
		o.responseBody = w
		return nil
	}
}

// teeBody copies a response body into w as it is read.
type teeBody struct {
	io.ReadCloser
	w io.Writer
}

func (t *teeBody) Read(p []byte) (int, error) {
	n, err := t.ReadCloser.Read(p)
	if n > 0 {
		if written, werr := t.w.Write(p[:n]); werr != nil {
			return written, werr
		}
	}
	return n, err
}

// ResponseTooLargeError reports a buffered response that exceeded the limit.
type ResponseTooLargeError struct {
	// Limit is the size limit in bytes that the response exceeded, as set
	// by WithMaxResponseBytes.
	Limit int64
}

func (e *ResponseTooLargeError) Error() string {
	return fmt.Sprintf("entire: response exceeds %d bytes", e.Limit)
}

// Do sends req and returns the response when its status is 2xx or listed in
// req.AcceptStatus. The caller owns the body of a returned response and must
// close it. Any other status returns an *HTTPError together with the response,
// whose body has then already been read and closed.
func (c *Client) Do(ctx context.Context, req *Request, options ...RequestOption) (*http.Response, error) {
	return c.send(ctx, req, c.defaults, options)
}

// send is Do with the default options passed explicitly. The SDK's own Core
// lookups send none.
func (c *Client) send(ctx context.Context, req *Request, defaults, options []RequestOption) (*http.Response, error) {
	o := requestOptions{headers: make(http.Header), query: make(url.Values)}
	fail := func(err error) (*http.Response, error) {
		if o.bodySet {
			closeBody(o.body)
		}
		return nil, err
	}
	if req != nil {
		for name, values := range req.Header {
			o.headers[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
		}
		for name, values := range req.Query {
			o.query[name] = append([]string(nil), values...)
		}
	}
	// Apply every option before validating anything, so that a body supplied
	// by an option is owned, and closed, on every failure path.
	var optionErr error
	for _, list := range [][]RequestOption{defaults, options} {
		for _, option := range list {
			err := errors.New("entire: nil request option")
			if option != nil {
				// Keep applying after a failure so that every body an
				// option supplies is captured and released.
				err = option(&o)
			}
			if optionErr == nil {
				optionErr = err
			}
		}
	}
	switch {
	case ctx == nil:
		return fail(errors.New("entire: context must not be nil"))
	case req == nil:
		return fail(errors.New("entire: request must not be nil"))
	case req.invalid != nil:
		return fail(req.invalid)
	case optionErr != nil:
		return fail(optionErr)
	case req.Method == "" || !strings.HasPrefix(req.Path, "/"):
		return fail(errors.New("entire: request needs a method and a path beginning with /"))
	}
	if o.route != nil && !req.Core {
		routed := *req
		routed.Route = *o.route
		req = &routed
	}
	var payload []byte
	if !o.bodySet && req.Body != nil {
		data, err := json.Marshal(req.Body)
		if err != nil {
			return fail(fmt.Errorf("entire: encode request body: %w", err))
		}
		payload = data
		if _, supplied := o.headers["Content-Type"]; !supplied {
			o.headers.Set("Content-Type", "application/json")
		}
	}
	if _, err := url.PathUnescape(req.Path); err != nil {
		return fail(fmt.Errorf("entire: invalid request path: %w", err))
	}
	base, cached, err := c.endpoint(ctx, req)
	if err != nil {
		return fail(err)
	}
	query := o.query.Encode()
	target := requestURL(base, req.Path, query)

	idempotent := req.Idempotent || req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodOptions || o.headers.Get("Idempotency-Key") != ""
	attempts := 1
	if idempotent && c.retry.MaxAttempts > 1 {
		attempts = c.retry.MaxAttempts
	}
	var replay func() (io.ReadCloser, error)
	if payload != nil {
		replay = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(payload)), nil }
	}
	rerouted, renewed := false, false
	for attempt := 1; ; attempt++ {
		var body io.Reader
		switch {
		case o.bodySet && attempt == 1:
			body = o.body
		case o.bodySet:
			if replay == nil {
				return nil, errors.New("entire: request body cannot be replayed")
			}
			if body, err = replay(); err != nil {
				return nil, fmt.Errorf("entire: replay request body: %w", err)
			}
		case payload != nil:
			body = bytes.NewReader(payload)
		}
		httpReq, err := http.NewRequestWithContext(ctx, req.Method, target, body)
		if err != nil {
			closeBody(body)
			return nil, fmt.Errorf("entire: create request: %w", err)
		}
		if o.bodySet && attempt == 1 {
			replay = httpReq.GetBody
		}
		httpReq.Header = o.headers.Clone()
		if _, supplied := httpReq.Header["User-Agent"]; !supplied && c.userAgent != "" {
			httpReq.Header.Set("User-Agent", c.userAgent)
		}
		sentToken := ""
		if _, supplied := httpReq.Header["Authorization"]; !supplied && !req.Anonymous {
			token, err := c.tokenSource(ctx)
			if err != nil {
				closeBody(httpReq.Body)
				return nil, fmt.Errorf("entire: obtain access token: %w", err)
			}
			if token != "" {
				httpReq.Header.Set("Authorization", "Bearer "+token)
				sentToken = token
			}
		}
		canRetry := attempt < attempts && (!o.bodySet || replay != nil)
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			err = fmt.Errorf("entire: send request: %w", err)
			if canRetry && ctx.Err() == nil && c.retry.wait(ctx, attempt, nil) {
				continue
			}
			return resp, err
		}
		if accepted(resp.StatusCode, req.AcceptStatus) {
			if o.responseBody != nil && resp.Body != nil && resp.Body != http.NoBody {
				resp.Body = &teeBody{ReadCloser: resp.Body, w: o.responseBody}
			}
			return resp, nil
		}
		apiErr := readError(resp)
		if resp.StatusCode == http.StatusUnauthorized && c.rejectToken != nil && sentToken != "" && !renewed {
			// The exchanged token may have been revoked or expired early:
			// drop it, so the next request exchanges again. Send this one
			// once more if repeating it is safe, or if the server says it
			// rejected the token itself (RFC 6750 invalid_token), so no
			// handler ran. Otherwise a 401 does not prove that nothing ran:
			// the Entire API answers 401 from inside some handlers, such
			// as a merge whose GitHub credential failed. Like a reroute,
			// the repeat is a new first attempt, not a retry.
			c.rejectToken(sentToken)
			renewed = true
			if (idempotent || invalidToken(resp.Header)) && (!o.bodySet || replay != nil) {
				attempts++
				continue
			}
		}
		if !rerouted && (!o.bodySet || replay != nil) {
			if next := c.reroute(ctx, req, base, cached, apiErr); next != nil {
				// A new endpoint is a new first attempt, not a retry.
				base, cached, rerouted = next, false, true
				target = requestURL(base, req.Path, query)
				attempts++
				continue
			}
		}
		if canRetry && retryableStatus(resp.StatusCode) && c.retry.wait(ctx, attempt, apiErr) {
			continue
		}
		return resp, apiErr
	}
}

func requestURL(base *url.URL, path, query string) string {
	u := *base
	u.RawPath = strings.TrimRight(u.EscapedPath(), "/") + path
	u.Path, _ = url.PathUnescape(u.RawPath)
	u.RawQuery = query
	return u.String()
}

// reroute returns the endpoint to send a refused request to instead, or nil.
// Core answers 421 with the resource's home Core. A cell answers 409
// wrong_cell when the repository moved, and 404 when it holds no copy, which
// matters only when the cell came from the cache.
func (c *Client) reroute(ctx context.Context, req *Request, base *url.URL, cached bool, apiErr error) *url.URL {
	var httpErr *HTTPError
	if req.base != nil || !errors.As(apiErr, &httpErr) {
		return nil
	}
	if req.Core {
		if httpErr.StatusCode == http.StatusMisdirectedRequest {
			return c.misdirected(ctx, base, apiErr)
		}
		return nil
	}
	if c.baseURL != nil || req.Route.key() == "" {
		return nil
	}
	wrongCell := httpErr.StatusCode == http.StatusConflict && httpErr.Code == "wrong_cell"
	if !wrongCell && (httpErr.StatusCode != http.StatusNotFound || !cached) {
		return nil
	}
	c.routing.forget(req.Route)
	next, _, err := c.locate(ctx, req.Route, false)
	if err != nil || next.String() == base.String() {
		return nil
	}
	return next
}

// Call sends req with Do and decodes a JSON response into T. It requests
// JSON unless req sets Accept. An empty body leaves Data as the zero value.
// When T is struct{}, the body is discarded without decoding.
func Call[T any](ctx context.Context, c *Client, req *Request, options ...RequestOption) (*Response[T], error) {
	if c == nil {
		return nil, errors.New("entire: client must not be nil")
	}
	if req != nil && req.Header.Get("Accept") == "" && !isNoContent[T]() {
		copied := *req
		copied.Header = req.Header.Clone()
		if copied.Header == nil {
			copied.Header = make(http.Header)
		}
		copied.Header.Set("Accept", "application/json")
		req = &copied
	}
	resp, err := c.Do(ctx, req, options...)
	return decode[T](resp, err, c.maxResponseBytes)
}

func isNoContent[T any]() bool {
	var zero T
	_, ok := any(&zero).(*struct{})
	return ok
}

func decode[T any](resp *http.Response, requestErr error, limit int64) (*Response[T], error) {
	if resp == nil {
		return nil, requestErr
	}
	result := &Response[T]{StatusCode: resp.StatusCode, Header: resp.Header, HTTPResponse: resp}
	if requestErr != nil {
		return result, requestErr
	}
	defer closeResponse(resp)
	if isNoContent[T]() || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified {
		return result, nil
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		media, _, err := mime.ParseMediaType(contentType)
		if err != nil || (media != "application/json" && !strings.HasSuffix(media, "+json")) {
			var probe [1]byte
			if n, _ := io.ReadFull(resp.Body, probe[:]); n == 0 {
				return result, nil
			}
			return result, fmt.Errorf("entire: expected JSON response, got %q; read other content types with Do", contentType)
		}
	}
	data, err := readLimited(resp.Body, limit)
	if err != nil {
		return result, fmt.Errorf("entire: read response: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(data, &result.Data); err != nil {
		return result, fmt.Errorf("entire: decode response: %w", err)
	}
	return result, nil
}

func accepted(status int, extra []int) bool {
	if status >= 200 && status < 300 {
		return true
	}
	for _, code := range extra {
		if status == code {
			return true
		}
	}
	return false
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	if limit == 0 {
		return io.ReadAll(reader)
	}
	// Read limit bytes, then probe one more, so limit+1 cannot overflow.
	data, err := io.ReadAll(io.LimitReader(reader, limit))
	if err != nil {
		return data, err
	}
	var extra [1]byte
	n, err := io.ReadFull(reader, extra[:])
	if n > 0 {
		return data, &ResponseTooLargeError{Limit: limit}
	}
	if err != nil && err != io.EOF {
		return data, err
	}
	return data, nil
}

// closeResponse drains a bounded remainder so the connection can be reused,
// closes the body, and replaces it with http.NoBody.
func closeResponse(resp *http.Response) {
	if tee, ok := resp.Body.(*teeBody); ok {
		// The drained remainder is not part of what the caller read.
		resp.Body = tee.ReadCloser
	}
	if resp.Body == nil || resp.Body == http.NoBody {
		resp.Body = http.NoBody
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
	resp.Body = http.NoBody
}

// sameBody reports whether a and b are the same body. Unlike a == b, it does
// not panic on a reader whose type cannot be compared.
func sameBody(a, b io.Reader) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	return va.Type() == vb.Type() && va.Comparable() && va.Equal(vb)
}

func closeBody(body io.Reader) {
	if closer, ok := body.(io.Closer); ok && closer != nil {
		_ = closer.Close()
	}
}

// invalidToken reports whether a 401 response says the server rejected the
// access token itself, with an RFC 6750 WWW-Authenticate challenge carrying
// error="invalid_token". Authentication fails before any handler runs, so
// the request did nothing and may be sent again with a new token.
func invalidToken(h http.Header) bool {
	for _, header := range h.Values("WWW-Authenticate") {
		scheme := ""
		for _, item := range splitUnquoted(header) {
			item = strings.TrimSpace(item)
			// An item that starts with a token and a space opens a new
			// challenge (RFC 9110, section 11.6.1); any other item is a
			// parameter of the current one.
			if name, rest, ok := strings.Cut(item, " "); ok && !strings.Contains(name, "=") {
				scheme, item = name, strings.TrimSpace(rest)
			}
			name, value, ok := strings.Cut(item, "=")
			if ok && strings.EqualFold(scheme, "Bearer") && strings.EqualFold(strings.TrimSpace(name), "error") && unquote(strings.TrimSpace(value)) == "invalid_token" {
				return true
			}
		}
	}
	return false
}

// splitUnquoted splits s at commas outside quoted strings.
func splitUnquoted(s string) []string {
	var items []string
	quoted, escaped, start := false, false, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case escaped:
			escaped = false
		case quoted && c == '\\':
			escaped = true
		case c == '"':
			quoted = !quoted
		case c == ',' && !quoted:
			items = append(items, s[start:i])
			start = i + 1
		}
	}
	return append(items, s[start:])
}

// unquote returns a token, or the content of a quoted string with its
// escapes removed.
func unquote(v string) string {
	if len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
		return v
	}
	var b strings.Builder
	for i := 1; i < len(v)-1; i++ {
		if v[i] == '\\' && i+1 < len(v)-1 {
			i++
		}
		b.WriteByte(v[i])
	}
	return b.String()
}
