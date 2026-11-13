package entire

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxEventBytes = 8 << 20

// errEventTooLarge reports an event, or a line of one, larger than
// maxEventBytes.
var errEventTooLarge = fmt.Errorf("entire: SSE event exceeds %d bytes", maxEventBytes)

// Event is one server-sent event. Type defaults to "message". Data preserves the
// event's UTF-8 payload, with multiple data lines joined by newline characters.
// Event names and payloads are open to future additions to the API.
type Event struct {
	// ID is the stream's last event ID as of this event: the event's own id
	// field, or the most recent one before it when it has none. Before the
	// stream sends any, it is the request's Last-Event-ID header, if set.
	ID string
	// Type is the event's name from its event field, or "message" when it
	// has none.
	Type string
	// Data is the event's payload: its data lines joined by newline
	// characters, without a trailing newline.
	Data []byte
}

// Decode unmarshals an event's JSON payload into value.
func (e Event) Decode(value any) error { return json.Unmarshal(e.Data, value) }

// EventStream reads server-sent events without buffering the whole response.
// Call Close to release the connection. Next must be called by one goroutine;
// Close can be called concurrently to interrupt a blocked read. Cancellation of
// the request context also interrupts Next. Streams do not reconnect by
// themselves; use LastEventID and the endpoint's resume parameter when reopening.
type EventStream struct {
	// Response is the HTTP response the stream reads. The stream owns its
	// body: read events with Next, and release it with Close.
	Response    *http.Response
	scanner     *bufio.Scanner
	firstLine   bool
	lastEventID string
	retry       time.Duration
	hasRetry    bool
	err         error
}

// NewEventStream takes ownership of an SSE response. On an invalid content type
// it closes the body and returns an error. A line or event larger than 8 MiB is
// rejected; read other streaming formats or larger events with Client.Do.
func NewEventStream(resp *http.Response) (*EventStream, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("entire: event stream response and body must not be nil")
	}
	media, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || media != "text/event-stream" {
		resp.Body.Close()
		return nil, fmt.Errorf("entire: expected text/event-stream, got %q", resp.Header.Get("Content-Type"))
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), maxEventBytes)
	scanner.Split(splitEventLines)
	stream := &EventStream{Response: resp, scanner: scanner, firstLine: true}
	if resp.Request != nil {
		stream.lastEventID = resp.Request.Header.Get("Last-Event-ID")
	}
	return stream, nil
}

// Close closes the underlying HTTP response body.
func (s *EventStream) Close() error { return s.Response.Body.Close() }

// LastEventID is the latest valid SSE id, including id-only blocks. Pass it as
// Last-Event-ID when reconnecting to an endpoint that supports replay.
func (s *EventStream) LastEventID() string { return s.lastEventID }

// Retry returns the most recent server-suggested reconnect delay, if supplied.
func (s *EventStream) Retry() (time.Duration, bool) { return s.retry, s.hasRetry }

// Next returns the next complete event or io.EOF. Heartbeats and comment-only
// blocks are skipped. An unterminated final event is discarded as required by
// the SSE protocol. EOF and read errors automatically close the response body.
func (s *EventStream) Next() (Event, error) {
	if s.err != nil {
		return Event{}, s.err
	}
	var data bytes.Buffer
	pendingID := s.lastEventID
	eventType := ""
	hasData := false
	size := 0
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if s.firstLine {
			line = strings.TrimPrefix(line, "\uFEFF")
			s.firstLine = false
		}
		if line == "" {
			s.lastEventID = pendingID
			if hasData {
				if eventType == "" {
					eventType = "message"
				}
				payload := data.Bytes()
				return Event{ID: s.lastEventID, Type: eventType, Data: payload[:len(payload)-1]}, nil
			}
			eventType = ""
			size = 0
			continue
		}
		size += len(line)
		if size > maxEventBytes {
			s.err = errEventTooLarge
			s.Close()
			return Event{}, s.err
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "data":
			data.WriteString(value)
			data.WriteByte('\n')
			hasData = true
		case "event":
			eventType = value
		case "id":
			if !strings.ContainsRune(value, 0) {
				pendingID = value
			}
		case "retry":
			if value == "" || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
				continue
			}
			if ms, err := strconv.ParseInt(value, 10, 64); err == nil && ms <= int64((1<<63-1)/time.Millisecond) {
				s.retry, s.hasRetry = time.Duration(ms)*time.Millisecond, true
			}
		}
	}
	switch err := s.scanner.Err(); {
	case err == nil:
		s.err = io.EOF
	case errors.Is(err, bufio.ErrTooLong):
		s.err = errEventTooLarge
	default:
		s.err = fmt.Errorf("entire: read event stream: %w", err)
	}
	s.Close()
	return Event{}, s.err
}

// SSE accepts CR, LF, and CRLF line endings, including a CRLF split across reads.
func splitEventLines(data []byte, atEOF bool) (int, []byte, error) {
	for i, c := range data {
		if c == '\n' {
			return i + 1, data[:i], nil
		}
		if c == '\r' {
			if i+1 == len(data) && !atEOF {
				return 0, nil, nil
			}
			advance := i + 1
			if advance < len(data) && data[advance] == '\n' {
				advance++
			}
			return advance, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
