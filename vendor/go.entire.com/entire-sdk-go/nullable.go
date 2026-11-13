package entire

import (
	"bytes"
	"encoding/json"
	"reflect"
)

// Ptr returns a pointer to v, for optional parameters and fields.
func Ptr[T any](v T) *T { return &v }

// Deref returns the pointed-to value, or T's zero value when p is nil.
// Use a nil check instead when the absence of endpoint context matters.
func Deref[T any](p *T) T {
	if p != nil {
		return *p
	}
	var zero T
	return zero
}

// Nullable is a JSON value that may be null. Its zero value is unset.
//
// Optional nullable fields omit an unset value, so a PATCH body can leave a
// field unchanged (unset), clear it (Null), or set it (Value). Required
// nullable fields are always sent; an unset value is sent as null. On reads,
// IsSet, IsNull, and Get report which case the server returned.
type Nullable[T any] struct {
	value T
	set   bool
	null  bool
}

// Value constructs a present, non-null value, including false, zero, or an
// empty string. A nil slice or map is sent as an empty collection, [] or {};
// use Null to send null.
func Value[T any](v T) Nullable[T] { return Nullable[T]{value: v, set: true} }

// Null constructs an explicit JSON null.
func Null[T any]() Nullable[T] { return Nullable[T]{set: true, null: true} }

// IsSet reports whether the value was supplied, as a value or as null.
func (n Nullable[T]) IsSet() bool { return n.set }

// IsNull reports whether the value is an explicit JSON null.
func (n Nullable[T]) IsNull() bool { return n.set && n.null }

// IsZero reports whether the value is unset. The omitzero option of
// encoding/json uses it to omit unset optional fields.
func (n Nullable[T]) IsZero() bool { return !n.set }

// Get returns the value and whether a non-null value is present.
func (n Nullable[T]) Get() (T, bool) { return n.value, n.set && !n.null }

// Or returns the value when a non-null value is present, and fallback
// otherwise.
func (n Nullable[T]) Or(fallback T) T {
	if n.set && !n.null {
		return n.value
	}
	return fallback
}

var marshalerType = reflect.TypeFor[json.Marshaler]()

// MarshalJSON implements json.Marshaler. Outside a struct field with
// omitzero, an unset value encodes as null.
func (n Nullable[T]) MarshalJSON() ([]byte, error) {
	if !n.set || n.null {
		return []byte("null"), nil
	}
	// encoding/json writes a nil slice or map as null; a present value must
	// not be indistinguishable from Null, so send the empty collection.
	if t := reflect.TypeFor[T](); (t.Kind() == reflect.Slice || t.Kind() == reflect.Map) && !t.Implements(marshalerType) && !reflect.PointerTo(t).Implements(marshalerType) {
		if v := reflect.ValueOf(&n.value).Elem(); v.IsNil() {
			if t.Kind() == reflect.Map {
				return json.Marshal(reflect.MakeMap(t).Interface())
			}
			return json.Marshal(reflect.MakeSlice(t, 0, 0).Interface())
		}
	}
	return json.Marshal(n.value)
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *Nullable[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*n = Null[T]()
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*n = Value(value)
	return nil
}
