package entire

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"math/rand/v2"
	"net/http"
	"time"
)

// RetryPolicy configures automatic retries. The zero value disables them.
//
// Only requests that are safe to repeat are retried: GET, HEAD, and OPTIONS
// requests, requests carrying an Idempotency-Key header (operations that
// accept one send a generated key when the caller supplies none), and
// requests marked Request.Idempotent. A request is retried after a transport
// error or a 408, 429, 502, 503, or 504 response. A Retry-After header sets
// the delay; otherwise delays grow exponentially with jitter. Retries stop
// early when the next delay would pass the context's deadline.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of attempts per call, including the
	// first. Values below 2 disable retries.
	MaxAttempts int
	// InitialBackoff is the delay before the first retry, before jitter. Zero
	// selects 250 milliseconds.
	InitialBackoff time.Duration
	// MaxBackoff caps computed delays. Zero selects 20 seconds.
	MaxBackoff time.Duration
	// MaxRetryAfter caps a server-requested Retry-After delay. A longer
	// request ends the retries and returns the error. Zero selects 1 minute.
	MaxRetryAfter time.Duration
}

// WithRetryPolicy enables automatic retries for safe requests.
func WithRetryPolicy(policy RetryPolicy) ClientOption {
	return func(c *clientConfig) error {
		c.retry = policy
		return c.retry.validate()
	}
}

func (p RetryPolicy) validate() error {
	if p.MaxAttempts < 0 || p.InitialBackoff < 0 || p.MaxBackoff < 0 || p.MaxRetryAfter < 0 {
		return errors.New("entire: retry policy values must not be negative")
	}
	return nil
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// delay returns the wait before the retry that follows attempt, and false
// when the server asked for a longer wait than the policy allows.
func (p RetryPolicy) delay(attempt int, failure *HTTPError) (time.Duration, bool) {
	initial, maximum, maxRetryAfter := p.InitialBackoff, p.MaxBackoff, p.MaxRetryAfter
	if initial == 0 {
		initial = 250 * time.Millisecond
	}
	if maximum == 0 {
		maximum = 20 * time.Second
	}
	if maxRetryAfter == 0 {
		maxRetryAfter = time.Minute
	}
	if failure != nil {
		if wait, ok := failure.RetryAfter(time.Now()); ok {
			return wait, wait <= maxRetryAfter
		}
	}
	backoff := min(initial, maximum)
	for i := 1; i < attempt && backoff < maximum; i++ {
		if backoff > maximum/2 {
			backoff = maximum
			break
		}
		backoff *= 2
	}
	// Equal jitter: wait at least half the backoff so retries stay spaced.
	half := backoff / 2
	return half + rand.N(half+1), true
}

// wait sleeps before the retry that follows attempt. It reports false when
// the retry should not happen: the delay is longer than allowed, would pass
// the context's deadline, or the context ends while waiting.
func (p RetryPolicy) wait(ctx context.Context, attempt int, failure *HTTPError) bool {
	d, ok := p.delay(attempt, failure)
	if !ok {
		return false
	}
	if deadline, has := ctx.Deadline(); has && time.Until(deadline) <= d {
		return false
	}
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// NewIdempotencyKey returns a random UUID (version 4) for an Idempotency-Key
// header. Reuse one key, with an identical body, when you retry a write
// yourself; the server then replays the first result instead of repeating the
// change.
func NewIdempotencyKey() string {
	var b [16]byte
	_, _ = crand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:], b[10:])
	return string(out[:])
}
