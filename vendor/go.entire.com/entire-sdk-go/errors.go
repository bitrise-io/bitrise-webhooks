package entire

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPError describes an unsuccessful HTTP response. It normalizes the API's
// {"error": ...} bodies and RFC 9457 problem documents
// (application/problem+json). Body retains the original response, up to
// 1 MiB, including unknown problem extension members. Use errors.As to obtain
// an HTTPError; transport and context errors remain wrapped separately.
type HTTPError struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// Code is the API's machine-readable error code, when supplied.
	Code string
	// Message is the human-readable summary: the error, message, or title
	// member, in that order of preference.
	Message string
	// Title is the problem document's title member.
	Title string
	// Detail explains this occurrence of the problem.
	Detail string
	// Type is the problem type URI.
	Type string
	// Instance identifies this occurrence of the problem.
	Instance string
	// Status is the problem document's status member, which RFC 9457 says
	// echoes the HTTP status. It differs from StatusCode when an intermediary
	// changed the response status, and is zero when absent.
	Status int
	// Errors lists field-level problems, such as validation failures. Core
	// lists each invalid field of a request it rejects; the Entire API's
	// errors don't carry it, and leave it empty.
	Errors []ErrorDetail
	// RequestID identifies the request for support: the body's request_id
	// member, the X-Request-ID header, or Cloudflare's CF-Ray header, in that
	// order of preference.
	RequestID string
	// Header holds the response headers.
	Header http.Header
	// Body is the raw response body, up to 1 MiB.
	Body []byte
	// BodyTruncated reports that Body stops at the 1 MiB limit.
	BodyTruncated bool
	// Cause is a failure encountered while reading the body, if any.
	Cause error
}

// ErrorDetail is one field-level problem in an error response, such as a
// validation failure.
type ErrorDetail struct {
	// Location is where the error occurred, such as "body.items[3].tags" or
	// "path.thing-id".
	Location *string `json:"location,omitempty"`
	// Message is the error message text.
	Message *string `json:"message,omitempty"`
	// Value is the value at the given location.
	Value json.RawMessage `json:"value,omitempty"`
}

func (e *HTTPError) Error() string {
	message := e.Message
	if e.Detail != "" {
		message = e.Detail
	}
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "entire: HTTP %d", e.StatusCode)
	if e.Code != "" {
		fmt.Fprintf(&b, " (%s)", e.Code)
	}
	b.WriteString(": ")
	b.WriteString(message)
	for i, detail := range e.Errors {
		if i == 3 {
			fmt.Fprintf(&b, "; and %d more", len(e.Errors)-i)
			break
		}
		location, text := "", ""
		if detail.Location != nil {
			location = *detail.Location
		}
		if detail.Message != nil {
			text = *detail.Message
		}
		switch {
		case location != "" && text != "":
			fmt.Fprintf(&b, "; %s: %s", location, text)
		case location+text != "":
			fmt.Fprintf(&b, "; %s", location+text)
		}
	}
	return b.String()
}

// Unwrap returns a failure encountered while reading the error body, if any.
func (e *HTTPError) Unwrap() error { return e.Cause }

// IsNotFound reports a 404 Not Found response.
func (e *HTTPError) IsNotFound() bool { return e.StatusCode == http.StatusNotFound }

// IsRateLimited reports a 429 Too Many Requests response. RetryAfter gives
// the server's suggested delay.
func (e *HTTPError) IsRateLimited() bool { return e.StatusCode == http.StatusTooManyRequests }

// IsConflict reports that the resource's state conflicts with the request:
// 409 Conflict, or 412 Precondition Failed when an If-Match ETag is stale.
// Read the resource again before retrying.
func (e *HTTPError) IsConflict() bool {
	return e.StatusCode == http.StatusConflict || e.StatusCode == http.StatusPreconditionFailed
}

// Temporary reports a status that a later retry may resolve: 408, 429, 502,
// 503, or 504. These are the statuses a RetryPolicy retries.
func (e *HTTPError) Temporary() bool { return retryableStatus(e.StatusCode) }

// Error codes that the API sends in HTTPError.Code. Branch on these and the
// status code; titles and details are for people and can change.
const (
	// CodeInvalidRequest: a field is present but its value is invalid.
	CodeInvalidRequest = "invalid_request"
	// CodeValidationError: a required field is missing, has the wrong type,
	// or a timestamp does not parse.
	CodeValidationError = "validation_error"
	// CodeUnsupportedPermission: a token request named no permissions, an
	// invalid one, or one the plugin did not register.
	CodeUnsupportedPermission = "unsupported_permission"
	// CodeUnauthorized: the credential is missing, expired, or invalid.
	CodeUnauthorized = "unauthorized"
	// CodeForbidden: the caller may not act on this installation or
	// repository, whether or not it exists.
	CodeForbidden = "forbidden"
	// CodeNotFound: the plugin, installation, or repository is unknown, or
	// the repository is outside the installation.
	CodeNotFound = "not_found"
	// CodeWrongCell: this host does not serve the repository. Look up the
	// repository's host again and retry there.
	CodeWrongCell = "wrong_cell"
	// CodeNoPrimary: the repository's primary is not yet known in this
	// region. Retry with backoff.
	CodeNoPrimary = "no_primary"
	// CodeUnavailable: an Entire dependency is down. Retry with backoff.
	CodeUnavailable = "unavailable"
	// CodeRepoNotCovered: the plugin installation does not cover the
	// repository.
	CodeRepoNotCovered = "repo_not_covered"
	// CodeDeploymentOwnerConflict: another live installation of the plugin
	// owns a deployment key in the write.
	CodeDeploymentOwnerConflict = "deployment_owner_conflict"
	// CodeRequestTooLarge: the request body is over the endpoint's limit.
	// Split it.
	CodeRequestTooLarge = "request_too_large"
)

// HasCode reports whether err wraps an HTTPError whose Code is code.
func HasCode(err error, code string) bool {
	var e *HTTPError
	return errors.As(err, &e) && e.Code == code
}

// IsWrongCell reports whether err is a 409 wrong_cell response: the host
// does not serve the repository, which has another processing primary.
func IsWrongCell(err error) bool {
	var e *HTTPError
	return errors.As(err, &e) && e.StatusCode == http.StatusConflict && e.Code == CodeWrongCell
}

// IsNoPrimary reports whether err is a 503 no_primary response: the
// repository's primary is not yet known in the region. A RetryPolicy retries
// it for idempotent requests.
func IsNoPrimary(err error) bool {
	var e *HTTPError
	return errors.As(err, &e) && e.StatusCode == http.StatusServiceUnavailable && e.Code == CodeNoPrimary
}

// IsNotFound reports whether err wraps an HTTPError with status 404.
func IsNotFound(err error) bool {
	var e *HTTPError
	return errors.As(err, &e) && e.IsNotFound()
}

// IsRateLimited reports whether err wraps an HTTPError with status 429.
func IsRateLimited(err error) bool {
	var e *HTTPError
	return errors.As(err, &e) && e.IsRateLimited()
}

// IsConflict reports whether err wraps an HTTPError with status 409 or 412.
func IsConflict(err error) bool {
	var e *HTTPError
	return errors.As(err, &e) && e.IsConflict()
}

// IsTemporary reports whether err wraps an HTTPError whose status a later
// retry may resolve. See HTTPError.Temporary.
func IsTemporary(err error) bool {
	var e *HTTPError
	return errors.As(err, &e) && e.Temporary()
}

// RetryAfter parses Retry-After as seconds or an HTTP date relative to now. A
// past HTTP date returns zero with ok=true; an absent or invalid header
// returns ok=false.
func (e *HTTPError) RetryAfter(now time.Time) (delay time.Duration, ok bool) {
	value := strings.TrimSpace(e.Header.Get("Retry-After"))
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds < 0 || seconds > int64((1<<63-1)/time.Second) {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	date, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	return max(date.Sub(now), 0), true
}

// readError consumes and closes an unsuccessful response's body, replacing it
// with http.NoBody, and describes it.
func readError(resp *http.Response) *HTTPError {
	body, readErr := readLimited(resp.Body, maxErrorBytes)
	if readErr != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	}
	_ = resp.Body.Close()
	resp.Body = http.NoBody
	err := &HTTPError{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: body, Cause: readErr}
	_, err.BodyTruncated = readErr.(*ResponseTooLargeError)
	var document struct {
		Error     json.RawMessage `json:"error"`
		Message   json.RawMessage `json:"message"`
		Title     json.RawMessage `json:"title"`
		Code      json.RawMessage `json:"code"`
		Detail    json.RawMessage `json:"detail"`
		Type      json.RawMessage `json:"type"`
		Instance  json.RawMessage `json:"instance"`
		RequestID json.RawMessage `json:"request_id"`
		Errors    json.RawMessage `json:"errors"`
		Status    json.RawMessage `json:"status"`
	}
	if json.Unmarshal(body, &document) == nil {
		text := func(raw json.RawMessage) string {
			var v string
			if json.Unmarshal(raw, &v) != nil {
				return ""
			}
			return v
		}
		err.Code, err.Title, err.Detail = text(document.Code), text(document.Title), text(document.Detail)
		err.Type, err.Instance, err.RequestID = text(document.Type), text(document.Instance), text(document.RequestID)
		for _, candidate := range []json.RawMessage{document.Error, document.Message, document.Title} {
			if v := text(candidate); v != "" {
				err.Message = v
				break
			}
		}
		var details []ErrorDetail
		if json.Unmarshal(document.Errors, &details) == nil {
			err.Errors = details
		}
		var status int
		if json.Unmarshal(document.Status, &status) == nil {
			err.Status = status
		}
	}
	for _, header := range []string{"X-Request-Id", "Cf-Ray"} {
		if err.RequestID == "" {
			err.RequestID = resp.Header.Get(header)
		}
	}
	return err
}
