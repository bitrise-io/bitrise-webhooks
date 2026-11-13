package entire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Timestamp is a time in an API response. The API sends times as RFC 3339
// strings, and sends an empty string or null for a time it doesn't have,
// such as the merge time of a trail that hasn't merged. Both decode as the
// zero time, which IsZero reports, and so does a string that isn't an RFC
// 3339 time, such as one an agent recorded in a transcript, instead of
// failing the whole response. Timestamp embeds time.Time, so time's methods
// apply directly; use the Time field where a time.Time is needed.
type Timestamp struct{ time.Time }

// UnmarshalJSON accepts an RFC 3339 string; any other string, or null,
// decodes as the zero time. Anything else is an error, and leaves t
// unchanged.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var parsed time.Time
	if data = bytes.TrimSpace(data); !bytes.Equal(data, []byte("null")) {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("entire: time %s is not a string", data)
		}
		if p, err := time.Parse(time.RFC3339Nano, s); err == nil {
			parsed = p
		}
	}
	t.Time = parsed
	return nil
}

// MarshalJSON encodes the zero time as an empty string, as the API does, and
// any other time in RFC 3339 form.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte(`""`), nil
	}
	return t.Time.MarshalJSON()
}
