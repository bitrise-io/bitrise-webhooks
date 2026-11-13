package entire

import (
	"context"
	"errors"
	"iter"
	"net"
	"time"
)

// WatchOptions configures Changes.WatchEvents and Git.WatchRefs. The
// zero value watches from now on with the default reconnection delays.
type WatchOptions struct {
	// LastEventID resumes a stream after this event, where the stream
	// supports replay. Git.WatchRefs ignores it: each of its connections
	// starts with a fresh snapshot.
	LastEventID string
	// MinReconnectDelay is the first wait before reconnecting when the server
	// suggests none; it doubles after each failed attempt. Zero selects one
	// second.
	MinReconnectDelay time.Duration
	// MaxReconnectDelay caps the wait between reconnection attempts. Zero
	// selects 30 seconds.
	MaxReconnectDelay time.Duration
}

// watch yields the events of the streams that open returns, reopening the
// stream with the last event ID whenever it ends or fails. It stops when ctx
// ends, when the caller stops ranging, or when a stream cannot be opened or
// read for a reason that reconnecting cannot fix, which it yields as the
// final error. The wait before reconnecting grows while connections fail,
// and starts over once one works.
func watch(ctx context.Context, options *WatchOptions, open func(lastEventID string) (*EventStream, error)) iter.Seq2[Event, error] {
	var settings WatchOptions
	if options != nil {
		settings = *options
	}
	minimum, maximum := settings.MinReconnectDelay, settings.MaxReconnectDelay
	if minimum <= 0 {
		minimum = time.Second
	}
	if maximum <= 0 {
		maximum = 30 * time.Second
	}
	return func(yield func(Event, error) bool) {
		lastEventID := settings.LastEventID
		backoff := minimum
		for ctx.Err() == nil {
			stream, err := open(lastEventID)
			var delay time.Duration
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if !reconnectable(err) {
					yield(Event{}, err)
					return
				}
				delay = backoff
				var apiErr *HTTPError
				if errors.As(err, &apiErr) {
					if wait, ok := apiErr.RetryAfter(time.Now()); ok {
						delay = wait
					}
				}
			} else {
				working := false
				for {
					event, err := stream.Next()
					if errors.Is(err, errEventTooLarge) {
						yield(Event{}, err)
						return
					}
					if err != nil {
						break
					}
					// Entire's streams report a failed connection with an
					// error or forbidden frame; any other frame shows that
					// the connection works.
					if event.Type != "error" && event.Type != "forbidden" {
						working = true
					}
					if !yield(event, nil) {
						_ = stream.Close()
						return
					}
				}
				_ = stream.Close()
				lastEventID = stream.LastEventID()
				if working {
					backoff = minimum
				}
				delay = backoff
				if suggested, ok := stream.Retry(); ok {
					delay = suggested
				}
			}
			backoff = min(backoff*2, maximum)
			timer := time.NewTimer(min(delay, maximum))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

// reconnectable reports whether a stream that failed to open may open on a
// later attempt: after a network failure, or an HTTP status that retrying can
// fix. Anything else, such as an invalid argument, a missing Change, or a
// failing token source, ends the watch.
func reconnectable(err error) bool {
	var apiErr *HTTPError
	if errors.As(err, &apiErr) {
		return apiErr.Temporary() || apiErr.StatusCode >= 500
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}
