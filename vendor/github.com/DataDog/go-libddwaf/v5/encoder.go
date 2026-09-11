// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package libddwaf

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strings"

	"github.com/DataDog/go-libddwaf/v5/internal/bindings"
	"github.com/DataDog/go-libddwaf/v5/internal/invariant"
	"github.com/DataDog/go-libddwaf/v5/timer"
	"github.com/DataDog/go-libddwaf/v5/waferrors"
)

// maxPointerIndirections limits traversal of chained pointers/interfaces
// to avoid infinite loops on self-referencing pointers during encoding.
const maxPointerIndirections = 8

// EncoderConfig provides configuration for an [encoder], including pinning,
// timing, and size limits for strings, containers, and object depth.
type EncoderConfig struct {
	// Pinner is used to pin the data referenced by the encoded wafObjects.
	Pinner *runtime.Pinner
	// Timer makes sure the encoder doesn't spend too much time doing its job.
	Timer timer.Timer
	// MaxContainerSize is the maximum number of elements in a container (list, map, struct) that will be encoded.
	MaxContainerSize uint16
	// MaxStringSize is the maximum length of a string that will be encoded.
	MaxStringSize uint16
	// MaxObjectDepth is the maximum depth of the object that will be encoded.
	MaxObjectDepth uint16
}

// EncoderOption mutates an [EncoderConfig] created by [newEncoderConfig].
type EncoderOption func(*EncoderConfig)

// Encoder is the public encoding helper exposed to [Encodable] implementers.
// It provides string-writing utilities, map/array builders, timer checking,
// and truncation accounting.
type Encoder struct {
	Config      EncoderConfig
	Truncations Truncations
}

func (e *Encoder) WriteString(obj *WAFObject, str string) {
	if len(str) > e.Config.maxStringSize() {
		e.Truncations.Record(StringTooLong, len(str))
		str = str[:e.Config.maxStringSize()]
	}
	obj.SetString(e.Config.Pinner, str)
}

func (e *Encoder) WriteLiteralString(obj *WAFObject, str string) {
	if len(str) > e.Config.maxStringSize() {
		e.Truncations.Record(StringTooLong, len(str))
		str = str[:e.Config.maxStringSize()]
	}
	obj.SetLiteralString(e.Config.Pinner, str)
}

func (e *Encoder) Timeout() bool {
	return e.Config.Timer != nil && e.Config.Timer.Exhausted()
}

// encoder encodes Go values into wafObjects. Only the subset of Go types representable into wafObjects
// will be encoded while ignoring the rest of it.
// The encoder allocates memory for wafObjects in Go memory, which must remain referenced for their
// lifetime in the C world. This lifetime depends on the ddwaf function being used with the encoded result.
// The encoder's config.Pinner is used to pin Go pointers that the C library will reference, ensuring they
// aren't moved or collected by the garbage collector. Users must ensure the pinner remains valid until
// the C library is done with the encoded data.
type encoder struct {
	enc *Encoder
}

// TruncationReason is a flag representing reasons why some input was not encoded in full.
type TruncationReason uint8

const (
	// StringTooLong indicates a string exceeded the maximum string length configured. The truncation
	// values indicate the actual length of truncated strings.
	StringTooLong TruncationReason = 1 << iota
	// ContainerTooLarge indicates a container (list, map, struct) exceeded the maximum number of
	// elements configured. The truncation values indicate the actual number of elements in the
	// truncated container.
	ContainerTooLarge
	// ObjectTooDeep indicates an overall object exceeded the maximum encoding depths configured. The
	// truncation values indicate an estimated actual depth of the truncated object. The value is
	// guaranteed to be less than or equal to the actual depth (it may not be more).
	ObjectTooDeep
)

func (reason TruncationReason) String() string {
	switch reason {
	case ObjectTooDeep:
		return "container_depth"
	case ContainerTooLarge:
		return "container_size"
	case StringTooLong:
		return "string_length"
	default:
		return fmt.Sprintf("TruncationReason(%v)", int(reason))
	}
}

// Truncations accumulates truncation events that occurred during encoding.
type Truncations struct {
	StringTooLong     []int
	ContainerTooLarge []int
	ObjectTooDeep     []int
}

func (t *Truncations) Record(reason TruncationReason, size int) {
	switch reason {
	case StringTooLong:
		t.StringTooLong = append(t.StringTooLong, size)
	case ContainerTooLarge:
		t.ContainerTooLarge = append(t.ContainerTooLarge, size)
	case ObjectTooDeep:
		t.ObjectTooDeep = append(t.ObjectTooDeep, size)
	}
}

func (t *Truncations) Merge(other Truncations) {
	t.StringTooLong = append(t.StringTooLong, other.StringTooLong...)
	t.ContainerTooLarge = append(t.ContainerTooLarge, other.ContainerTooLarge...)
	t.ObjectTooDeep = append(t.ObjectTooDeep, other.ObjectTooDeep...)
}

func (t Truncations) IsEmpty() bool {
	return len(t.StringTooLong) == 0 && len(t.ContainerTooLarge) == 0 && len(t.ObjectTooDeep) == 0
}

func (t Truncations) AsMap() map[TruncationReason][]int {
	if t.IsEmpty() {
		return nil
	}
	m := make(map[TruncationReason][]int, 3)
	if len(t.StringTooLong) > 0 {
		m[StringTooLong] = t.StringTooLong
	}
	if len(t.ContainerTooLarge) > 0 {
		m[ContainerTooLarge] = t.ContainerTooLarge
	}
	if len(t.ObjectTooDeep) > 0 {
		m[ObjectTooDeep] = t.ObjectTooDeep
	}
	return m
}

const (
	// AppsecFieldTag is the struct tag recognized by the encoder when
	// deciding how to process struct fields (for example, `ddwaf:"ignore"`).
	AppsecFieldTag = "ddwaf"
	// AppsecFieldTagValueIgnore is the struct tag value that causes the
	// annotated field to be skipped during encoding.
	AppsecFieldTagValueIgnore = "ignore"
)

// Encodable represents a type that can encode itself into a WAFObject.
//
// The encoder writes into obj using the provided [*Encoder] for size limits,
// pinning, timer, and truncation accounting.
//
// Best-effort encoding philosophy: implementers should self-recover from
// input parsing errors by leaving obj as invalid (zero value) or calling
// [WAFObject.SetInvalid]. Only fatal conditions should be returned as errors:
//   - [waferrors.ErrTimeout] when [Encoder.Timeout] returns true
//   - [waferrors.ErrMaxDepthExceeded] when depth budget is exhausted
//
// The depth parameter is the remaining recursion budget. Implementers
// must decrement and check before recursing into nested structures.
//
// The [*Encoder] is owned by the caller; implementations must NOT retain
// the pointer past return. Truncations accumulate in enc.Truncations
// (shared across nested calls).
type Encodable interface {
	Encode(enc *Encoder, obj *WAFObject, depth int) error
}

func newEncoder(config EncoderConfig) (*encoder, error) {
	if config.Pinner == nil {
		return nil, errors.New("pinner cannot be nil")
	}
	if config.Timer == nil {
		var timerErr error
		config.Timer, timerErr = timer.NewTimer(timer.WithUnlimitedBudget())
		invariant.Assert(timerErr == nil, "timer.NewTimer(WithUnlimitedBudget) should not fail: %v", timerErr)
	}

	return &encoder{enc: &Encoder{Config: config}}, nil
}

func newEncoderConfig(pinner *runtime.Pinner, opts ...EncoderOption) EncoderConfig {
	config := EncoderConfig{
		Pinner:           pinner,
		MaxContainerSize: bindings.MaxContainerSize,
		MaxStringSize:    bindings.MaxStringLength,
		MaxObjectDepth:   bindings.MaxContainerDepth,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	return config
}

// WithUnlimitedLimits configures [newEncoderConfig] to disable encoder truncation limits.
func WithUnlimitedLimits() EncoderOption {
	return func(config *EncoderConfig) {
		config.MaxContainerSize = math.MaxUint16
		config.MaxStringSize = math.MaxUint16
		config.MaxObjectDepth = math.MaxUint16
	}
}

// WithTimer configures [newEncoderConfig] to use the provided timer.
func WithTimer(timer timer.Timer) EncoderOption {
	return func(config *EncoderConfig) {
		config.Timer = timer
	}
}

func (config EncoderConfig) hasUnlimitedLimits() bool {
	return config.MaxContainerSize == math.MaxUint16 &&
		config.MaxStringSize == math.MaxUint16 &&
		config.MaxObjectDepth == math.MaxUint16
}

func (config EncoderConfig) maxContainerSize() int {
	if config.hasUnlimitedLimits() {
		return math.MaxInt
	}
	return int(config.MaxContainerSize)
}

func (config EncoderConfig) maxStringSize() int {
	if config.hasUnlimitedLimits() {
		return math.MaxInt
	}
	return int(config.MaxStringSize)
}

func (config EncoderConfig) maxObjectDepth() int {
	if config.hasUnlimitedLimits() {
		return math.MaxInt
	}
	return int(config.MaxObjectDepth)
}

// Encode converts a Go value into a WAFObject tree.
// It may return unsupported-value, timeout, max-depth, or too-many-indirections errors.
func (encoder *encoder) Encode(data any) (*WAFObject, error) {
	var obj WAFObject

	if handled, err := encoder.tryEncodeScalarFastPath(data, &obj); handled {
		return &obj, err
	}

	if handled, err := encoder.tryEncodeTypedSliceFastPath(data, &obj, encoder.enc.Config.maxObjectDepth()); handled {
		if len(encoder.enc.Truncations.ObjectTooDeep) > 0 && !encoder.enc.Config.Timer.Exhausted() {
			depth, _ := depthOf(encoder.enc.Config.Timer, reflect.ValueOf(data))
			encoder.enc.Truncations.ObjectTooDeep = []int{depth}
		}
		return &obj, err
	}

	if handled, err := encoder.tryEncodeTypedMapFastPath(data, &obj, encoder.enc.Config.maxObjectDepth()); handled {
		if len(encoder.enc.Truncations.ObjectTooDeep) > 0 && !encoder.enc.Config.Timer.Exhausted() {
			depth, _ := depthOf(encoder.enc.Config.Timer, reflect.ValueOf(data))
			encoder.enc.Truncations.ObjectTooDeep = []int{depth}
		}
		return &obj, err
	}

	value := reflect.ValueOf(data)

	err := encoder.encode(value, &obj, encoder.enc.Config.maxObjectDepth())

	if len(encoder.enc.Truncations.ObjectTooDeep) > 0 && !encoder.enc.Config.Timer.Exhausted() {
		depth, _ := depthOf(encoder.enc.Config.Timer, value)
		encoder.enc.Truncations.ObjectTooDeep = []int{depth}
	}

	return &obj, err
}

func (encoder *encoder) tryEncodeScalarFastPath(data any, obj *WAFObject) (bool, error) {
	switch v := data.(type) {
	case nil:
		obj.SetNil()
	case bool:
		obj.SetBool(v)
	case string:
		encoder.enc.WriteString(obj, v)
	case int:
		obj.SetInt(int64(v))
	case int8:
		obj.SetInt(int64(v))
	case int16:
		obj.SetInt(int64(v))
	case int32:
		obj.SetInt(int64(v))
	case int64:
		obj.SetInt(v)
	case uint:
		obj.SetUint(uint64(v))
	case uint8:
		obj.SetUint(uint64(v))
	case uint16:
		obj.SetUint(uint64(v))
	case uint32:
		obj.SetUint(uint64(v))
	case uint64:
		obj.SetUint(v)
	case float32:
		obj.SetFloat(float64(v))
	case float64:
		obj.SetFloat(v)
	case json.Number:
		encoder.encodeJSONNumber(v, obj)
	default:
		return false, nil
	}

	return true, nil
}

func (encoder *encoder) tryEncodeTypedSliceFastPath(data any, obj *WAFObject, depth int) (bool, error) {
	switch v := data.(type) {
	case []any:
		if v == nil {
			obj.SetNil()
			return true, nil
		}
		return true, encoder.encodeTypedAnySliceFastPath(v, obj, depth)
	case []string:
		if v == nil {
			obj.SetNil()
			return true, nil
		}
		return true, encoder.encodeTypedStringSliceFastPath(v, obj, depth)
	case []int:
		if v == nil {
			obj.SetNil()
			return true, nil
		}
		return true, encoder.encodeTypedIntSliceFastPath(v, obj, depth)
	case []float64:
		if v == nil {
			obj.SetNil()
			return true, nil
		}
		return true, encoder.encodeTypedFloat64SliceFastPath(v, obj, depth)
	case []bool:
		if v == nil {
			obj.SetNil()
			return true, nil
		}
		return true, encoder.encodeTypedBoolSliceFastPath(v, obj, depth)
	default:
		return false, nil
	}
}

func (encoder *encoder) tryEncodeTypedMapFastPath(data any, obj *WAFObject, depth int) (bool, error) {
	switch v := data.(type) {
	case map[string]any:
		if v == nil {
			obj.SetNil()
			return true, nil
		}
		return true, encoder.encodeTypedMapStringAnyFastPath(v, obj, depth)
	case map[string]string:
		if v == nil {
			obj.SetNil()
			return true, nil
		}
		return true, encoder.encodeTypedStringMapFastPath(v, obj, depth)
	case map[string][]string:
		if v == nil {
			obj.SetNil()
			return true, nil
		}
		return true, encoder.encodeTypedStringSliceMapFastPath(v, obj, depth)
	default:
		return false, nil
	}
}

func (encoder *encoder) encodeTypedMapStringAnyFastPath(values map[string]any, obj *WAFObject, depth int) error {
	mb, err := encoder.beginTypedMapFastPath(obj, len(values), depth)
	if err != nil {
		return err
	}
	defer mb.Close()

	for key, value := range values {
		if encoder.enc.Timeout() {
			return nil
		}

		slot := mb.NextValue(key)
		if slot == nil {
			mb.Skip()
			continue
		}

		if value == nil {
			slot.SetNil()
			continue
		}

		if err := encoder.encode(reflect.ValueOf(value), slot, depth-1); err != nil {
			// Preserve the key like the reflect map path does, but mark the value unusable.
			slot.SetInvalid()
		}
	}

	return nil
}

func (encoder *encoder) beginTypedSliceFastPath(obj *WAFObject, length int, depth int) (*ArrayBuilder, error) {
	if encoder.enc.Timeout() {
		return nil, waferrors.ErrTimeout
	}
	if depth <= 0 {
		encoder.enc.Truncations.Record(ObjectTooDeep, -1)
		return nil, waferrors.ErrMaxDepthExceeded
	}
	return encoder.enc.Array(obj, length), nil
}

func (encoder *encoder) beginTypedMapFastPath(obj *WAFObject, length int, depth int) (*MapBuilder, error) {
	if encoder.enc.Timeout() {
		return nil, waferrors.ErrTimeout
	}
	if depth <= 0 {
		encoder.enc.Truncations.Record(ObjectTooDeep, -1)
		return nil, waferrors.ErrMaxDepthExceeded
	}
	return encoder.enc.Map(obj, length), nil
}

func (encoder *encoder) encodeTypedAnySliceFastPath(values []any, obj *WAFObject, depth int) error {
	ab, err := encoder.beginTypedSliceFastPath(obj, len(values), depth)
	if err != nil {
		return err
	}
	defer ab.Close()

	for _, value := range values {
		if encoder.enc.Timeout() {
			return nil
		}

		slot := ab.NextValue()
		if slot == nil {
			ab.Skip()
			continue
		}

		if err := encoder.encode(reflect.ValueOf(value), slot, depth-1); err != nil {
			ab.DropLast()
			continue
		}

		if slot.IsUnusable() {
			ab.DropLast()
		}
	}

	return nil
}

func (encoder *encoder) encodeTypedStringSliceFastPath(values []string, obj *WAFObject, depth int) error {
	ab, err := encoder.beginTypedSliceFastPath(obj, len(values), depth)
	if err != nil {
		return err
	}
	defer ab.Close()

	for _, value := range values {
		if encoder.enc.Timeout() {
			return nil
		}

		slot := ab.NextValue()
		if slot == nil {
			ab.Skip()
			continue
		}

		encoder.enc.WriteString(slot, value)
	}

	return nil
}

func (encoder *encoder) encodeTypedIntSliceFastPath(values []int, obj *WAFObject, depth int) error {
	ab, err := encoder.beginTypedSliceFastPath(obj, len(values), depth)
	if err != nil {
		return err
	}
	defer ab.Close()

	for _, value := range values {
		if encoder.enc.Timeout() {
			return nil
		}

		slot := ab.NextValue()
		if slot == nil {
			ab.Skip()
			continue
		}

		slot.SetInt(int64(value))
	}

	return nil
}

func (encoder *encoder) encodeTypedFloat64SliceFastPath(values []float64, obj *WAFObject, depth int) error {
	ab, err := encoder.beginTypedSliceFastPath(obj, len(values), depth)
	if err != nil {
		return err
	}
	defer ab.Close()

	for _, value := range values {
		if encoder.enc.Timeout() {
			return nil
		}

		slot := ab.NextValue()
		if slot == nil {
			ab.Skip()
			continue
		}

		slot.SetFloat(value)
	}

	return nil
}

func (encoder *encoder) encodeTypedBoolSliceFastPath(values []bool, obj *WAFObject, depth int) error {
	ab, err := encoder.beginTypedSliceFastPath(obj, len(values), depth)
	if err != nil {
		return err
	}
	defer ab.Close()

	for _, value := range values {
		if encoder.enc.Timeout() {
			return nil
		}

		slot := ab.NextValue()
		if slot == nil {
			ab.Skip()
			continue
		}

		slot.SetBool(value)
	}

	return nil
}

func (encoder *encoder) encodeTypedStringMapFastPath(values map[string]string, obj *WAFObject, depth int) error {
	mb, err := encoder.beginTypedMapFastPath(obj, len(values), depth)
	if err != nil {
		return err
	}
	defer mb.Close()

	for key, value := range values {
		if encoder.enc.Timeout() {
			return nil
		}

		slot := mb.NextValue(key)
		if slot == nil {
			mb.Skip()
			continue
		}

		encoder.enc.WriteString(slot, value)
	}

	return nil
}

func (encoder *encoder) encodeTypedStringSliceMapFastPath(values map[string][]string, obj *WAFObject, depth int) error {
	mb, err := encoder.beginTypedMapFastPath(obj, len(values), depth)
	if err != nil {
		return err
	}
	defer mb.Close()

	for key, value := range values {
		if encoder.enc.Timeout() {
			return nil
		}

		slot := mb.NextValue(key)
		if slot == nil {
			mb.Skip()
			continue
		}

		if value == nil {
			slot.SetNil()
			continue
		}

		if err := encoder.encodeTypedStringSliceFastPath(value, slot, depth-1); err != nil {
			slot.SetInvalid()
		}
	}

	return nil
}

var (
	jsonNumberType = reflect.TypeFor[json.Number]()
	byteArrayType  = reflect.TypeFor[[]byte]()
)

// isNullableKind returns true if the given kind can be nil.
func isNullableKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Interface, reflect.Pointer, reflect.UnsafePointer,
		reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return true
	default:
		return false
	}
}

// isValueNil check if the value is nullable and if it is actually nil
// we cannot directly use value.IsNil() because it panics on non-pointer values
func isValueNil(value reflect.Value) bool {
	return isNullableKind(value.Kind()) && value.IsNil()
}

func (encoder *encoder) encode(value reflect.Value, obj *bindings.WAFObject, depth int) error {
	if encoder.enc.Timeout() {
		return waferrors.ErrTimeout
	}

	if value.IsValid() && value.CanInterface() {
		if encodable, ok := value.Interface().(Encodable); ok {
			return encodable.Encode(encoder.enc, obj, depth)
		}
	}

	value, kind := resolvePointer(value)
	if (kind == reflect.Interface || kind == reflect.Pointer) && !value.IsNil() {
		// resolvePointer failed to resolve to something that's not a pointer, it
		// has indirected too many times...
		return waferrors.ErrTooManyIndirections
	}

	// Measure-only runs for leaves
	if obj == nil && kind != reflect.Array && kind != reflect.Slice && kind != reflect.Map && kind != reflect.Struct {
		// Nothing to do, we were only here to measure object depth!
		return nil
	}

	switch {
	// Terminal cases (leaves of the tree)
	//		Is invalid type: nil interfaces for example, cannot be used to run any reflect method or it's susceptible to panic
	case !value.IsValid() || kind == reflect.Invalid:
		return waferrors.ErrUnsupportedValue
	// 		Is nullable type: nil pointers, channels, maps or functions
	case isValueNil(value):
		obj.SetNil()
		return nil

	// 		Booleans
	case kind == reflect.Bool:
		obj.SetBool(value.Bool())
		return nil

	// 		Numbers
	case value.CanInt(): // any int type or alias
		obj.SetInt(value.Int())
		return nil
	case value.CanUint(): // any Uint type or alias
		obj.SetUint(value.Uint())
		return nil
	case value.CanFloat(): // any float type or alias
		obj.SetFloat(value.Float())
		return nil

	// 		json.Number -- string-represented arbitrary precision numbers
	case value.Type() == jsonNumberType:
		encoder.encodeJSONNumber(json.Number(value.String()), obj)
		return nil

	//		Strings
	case kind == reflect.String: // string type
		encoder.encodeString(value.String(), obj)
		return nil

	case (kind == reflect.Array || kind == reflect.Slice) && value.Type().Elem().Kind() == reflect.Uint8:
		// Byte Arrays are skipped voluntarily because they are often used
		// to do partial parsing which leads to false positives
		return nil

	// Containers (internal nodes of the tree)

	// 		All recursive cases can only execute if the depth is superior to 0.
	case depth <= 0:
		// Record that there was a truncation; we will try to measure the actual depth of the object afterwards.
		encoder.enc.Truncations.Record(ObjectTooDeep, -1)
		return waferrors.ErrMaxDepthExceeded

	// 		Either an array or a slice of an array
	case kind == reflect.Array || kind == reflect.Slice:
		encoder.encodeArray(value, obj, depth-1)
		return nil
	case kind == reflect.Map:
		encoder.encodeMap(value, obj, depth-1)
		return nil
	case kind == reflect.Struct:
		encoder.encodeStruct(value, obj, depth-1)
		return nil

	default:
		return waferrors.ErrUnsupportedValue
	}
}

func (encoder *encoder) encodeJSONNumber(num json.Number, obj *bindings.WAFObject) {
	// Important to attempt int64 first, as this is lossless. Values that are either too small or too
	// large to be represented as int64 can be represented as float64, but this can be lossy.
	if i, err := num.Int64(); err == nil {
		obj.SetInt(i)
		return
	}

	if f, err := num.Float64(); err == nil {
		obj.SetFloat(f)
		return
	}

	// Could not store as int64 nor float, so we'll store it as a string...
	encoder.encodeString(num.String(), obj)
}

func (encoder *encoder) encodeString(str string, obj *bindings.WAFObject) {
	encoder.enc.WriteString(obj, str)
}

var xmlNameType = reflect.TypeFor[xml.Name]()

func getFieldNameFromType(field reflect.StructField) (string, bool) {
	fieldName := field.Name

	// Private and synthetics fields
	if !field.IsExported() {
		return "", false
	}

	// This is the XML namespace/name pair, this isn't technically part of the data.
	if field.Type == xmlNameType {
		return "", false
	}

	// Use the encoding tag name as field name if present
	var contentTypeTag bool
	for _, tagName := range []string{"json", "yaml", "xml", "toml"} {
		tag, ok := field.Tag.Lookup(tagName)
		if !ok {
			continue
		}
		if tag == "-" {
			// Explicitly ignored, note that only "-" causes the field to be ignored,
			// any qualifier ("-,omitempty" or event "-,") will cause the field to be
			// actually named "-" instead of being ignored.
			return "", false
		}
		contentTypeTag = true
		tag, _, _ = strings.Cut(tag, ",")
		switch tag {
		case "":
			continue
		default:
			return tag, true
		}
	}

	// If none of the content-type tags are set, the field name is used; but we
	// specifically exclude those fields tagged as coming from a header, path
	// parameter or query parameter (this is used by labstack/echo.v4, see
	// https://echo.labstack.com/docs/binding).
	if !contentTypeTag {
		for _, tagName := range []string{"header", "path", "query"} {
			if _, ok := field.Tag.Lookup(tagName); ok {
				return "", false
			}
		}
	}

	return fieldName, true
}

// encodeStruct takes a reflect.Value and a wafObject pointer and iterates on the struct field to build
// a wafObject map of type wafMapType. The specificities are the following:
// - It will only take the first encoder.ContainerMaxSize elements of the struct
// - If the field has a json tag it will become the field name
// - Private fields and also values producing an error at encoding will be skipped
// - Even if the element values are invalid or null we still keep them to report the field name
func (encoder *encoder) encodeStruct(value reflect.Value, obj *bindings.WAFObject, depth int) {
	if encoder.enc.Timeout() {
		return
	}

	typ := value.Type()
	nbFields := typ.NumField()
	mb := encoder.enc.Map(obj, nbFields)
	defer mb.Close()

	for i := range nbFields {
		if encoder.enc.Timeout() {
			return
		}

		fieldType := typ.Field(i)
		fieldName, usable := getFieldNameFromType(fieldType)
		if tag, ok := fieldType.Tag.Lookup(AppsecFieldTag); !usable || ok && tag == AppsecFieldTagValueIgnore {
			// Either the struct field is ignored by json marshaling so can we,
			// 		or the field was explicitly set with `ddwaf:ignore`
			continue
		}

		slot := mb.NextValue(fieldName)
		if slot == nil {
			mb.Skip()
			continue
		}

		if err := encoder.encode(value.Field(i), slot, depth); err != nil {
			// We still need to keep the map key, so we can't discard the full object, instead, we make the value a noop
			slot.SetInvalid()
		}
	}
}

// encodeMap takes a reflect.Value and a wafObject pointer and iterates on the map elements and returns
// a wafObject map of type wafMapType. The specificities are the following:
// - It will only take the first encoder.ContainerMaxSize elements of the map
// - Even if the element values are invalid or null we still keep them to report the map key
func (encoder *encoder) encodeMap(value reflect.Value, obj *bindings.WAFObject, depth int) {
	mb := encoder.enc.Map(obj, value.Len())
	defer mb.Close()

	for iter := value.MapRange(); iter.Next(); {
		if encoder.enc.Timeout() {
			return
		}

		keyStr, err := encoder.resolveMapKey(iter.Key())
		if err != nil {
			continue
		}

		slot := mb.NextValue(keyStr)
		if slot == nil {
			mb.Skip()
			continue
		}

		if err := encoder.encode(iter.Value(), slot, depth); err != nil {
			// We still need to keep the map key, so we can't discard the full object, instead, we make the value a noop
			slot.SetInvalid()
		}
	}
}

// resolveMapKey resolves value to a WAF map key string. Only strings and
// []byte are valid WAF map keys because the C libddwaf API requires map
// keys to be null-terminated C strings; all other types return
// [waferrors.ErrInvalidMapKey].
func (encoder *encoder) resolveMapKey(value reflect.Value) (string, error) {
	value, kind := resolvePointer(value)

	switch {
	case kind == reflect.Invalid:
		return "", waferrors.ErrInvalidMapKey
	case kind == reflect.String:
		return value.String(), nil
	case value.Type() == byteArrayType:
		return string(value.Bytes()), nil
	default:
		return "", waferrors.ErrInvalidMapKey
	}
}

// encodeArray takes a reflect.Value and a wafObject pointer and iterates on the elements and returns
// a wafObject array of type wafArrayType. The specificities are the following:
// - It will only take the first encoder.ContainerMaxSize elements of the array
// - Elements producing an error at encoding or null values will be skipped
func (encoder *encoder) encodeArray(value reflect.Value, obj *bindings.WAFObject, depth int) {
	length := value.Len()
	ab := encoder.enc.Array(obj, length)
	overCapacity := false
	defer func() {
		ab.Close()
		if overCapacity {
			encoder.enc.Truncations.Record(ContainerTooLarge, length)
		}
	}()

	for i := range length {
		if encoder.enc.Timeout() {
			return
		}

		slot := ab.NextValue()
		if slot == nil {
			overCapacity = true
			continue
		}

		if err := encoder.encode(value.Index(i), slot, depth); err != nil {
			ab.DropLast()
			continue
		}

		// If the element is null or invalid it has no impact on the waf execution, therefore we can skip its
		// encoding. In this specific case we just overwrite it at the next loop iteration.
		if slot.IsUnusable() {
			ab.DropLast()
			continue
		}
	}
}

// depthOf returns the depth of the provided object. This is 0 for scalar values,
// such as strings.
func depthOf(t timer.Timer, obj reflect.Value) (depth int, err error) {
	if t.Exhausted() {
		return 0, waferrors.ErrTimeout
	}

	obj, kind := resolvePointer(obj)

	var itemDepth int
	switch kind {
	case reflect.Array, reflect.Slice:
		if obj.Type() == byteArrayType {
			// We treat byte slices as strings
			return 0, nil
		}
		for i := range obj.Len() {
			itemDepth, err = depthOf(t, obj.Index(i))
			depth = max(depth, itemDepth)
			if err != nil {
				break
			}
		}
		return depth + 1, err
	case reflect.Map:
		for iter := obj.MapRange(); iter.Next(); {
			itemDepth, err = depthOf(t, iter.Value())
			depth = max(depth, itemDepth)
			if err != nil {
				break
			}
		}
		return depth + 1, err
	case reflect.Struct:
		typ := obj.Type()
		for i := range obj.NumField() {
			fieldType := typ.Field(i)
			_, usable := getFieldNameFromType(fieldType)
			if !usable {
				continue
			}

			itemDepth, err = depthOf(t, obj.Field(i))
			depth = max(depth, itemDepth)
			if err != nil {
				break
			}
		}
		return depth + 1, err
	default:
		return 0, nil
	}
}

// resolvePointer attempts to resolve a pointer while limiting the pointer depth
// to be traversed, so that this is not susceptible to an infinite loop when
// provided a self-referencing pointer.
func resolvePointer(obj reflect.Value) (reflect.Value, reflect.Kind) {
	kind := obj.Kind()
	for limit := maxPointerIndirections; limit > 0 && (kind == reflect.Pointer || kind == reflect.Interface); limit-- {
		if obj.IsNil() {
			return obj, kind
		}
		obj = obj.Elem()
		kind = obj.Kind()
	}
	return obj, kind
}
