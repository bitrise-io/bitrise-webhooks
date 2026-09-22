// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package bindings

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"structs"

	"github.com/DataDog/go-libddwaf/v5/internal/unsafeutil"
	"github.com/DataDog/go-libddwaf/v5/waferrors"
)

const (
	MaxStringLength   uint16 = 4096
	MaxContainerDepth uint16 = 20
	MaxContainerSize  uint16 = 256
)

type WAFReturnCode int32

const (
	WAFErrInternal WAFReturnCode = iota - 3
	WAFErrInvalidObject
	WAFErrInvalidArgument
	WAFOK
	WAFMatch
)

// WAFObjectType represents the type discriminator for a WAFObject.
type WAFObjectType uint8

const (
	// WAFInvalidType represents an unknown or uninitialized type
	WAFInvalidType WAFObjectType = 0x00
	// WAFNilType represents a null type, used for its semantical value
	WAFNilType WAFObjectType = 0x01
	// WAFBoolType represents a boolean type
	WAFBoolType WAFObjectType = 0x02
	// WAFIntType represents a 64-bit signed integer (DDWAF_OBJ_SIGNED)
	WAFIntType WAFObjectType = 0x04
	// WAFUintType represents a 64-bit unsigned integer (DDWAF_OBJ_UNSIGNED)
	WAFUintType WAFObjectType = 0x06
	// WAFFloatType represents a 64-bit float (double)
	WAFFloatType WAFObjectType = 0x08
	// WAFStringType represents a dynamic UTF-8 string (up to uint32 length)
	WAFStringType WAFObjectType = 0x10
	// WAFLiteralStringType represents a literal UTF-8 string (never freed)
	WAFLiteralStringType WAFObjectType = 0x12
	// WAFSmallStringType represents a UTF-8 string up to 14 bytes (inline storage)
	WAFSmallStringType WAFObjectType = 0x14
	// WAFArrayType represents an array of ddwaf_object (up to uint16 capacity)
	WAFArrayType WAFObjectType = 0x20
	// WAFMapType represents an array of ddwaf_object_kv (up to uint16 capacity)
	WAFMapType WAFObjectType = 0x40
)

func (w WAFObjectType) String() string {
	switch w {
	case WAFInvalidType:
		return "invalid"
	case WAFNilType:
		return "nil"
	case WAFBoolType:
		return "bool"
	case WAFIntType:
		return "int"
	case WAFUintType:
		return "uint"
	case WAFFloatType:
		return "float"
	case WAFStringType:
		return "string"
	case WAFLiteralStringType:
		return "literal_string"
	case WAFSmallStringType:
		return "small_string"
	case WAFArrayType:
		return "array"
	case WAFMapType:
		return "map"
	default:
		return fmt.Sprintf("unknown(%d)", w)
	}
}

// WAFObject represents the ddwaf_object union type.
// It is exactly 16 bytes to match the C union layout.
//
// The 16-byte union layout supports multiple interpretations based on type:
//
//	struct _ddwaf_object_bool {      // type=0x02
//	    uint8_t type;                // offset 0
//	    bool val;                    // offset 1
//	};                               // + padding to 16 bytes
//
//	struct _ddwaf_object_signed {    // type=0x04
//	    uint8_t type;                // offset 0
//	    int64_t val;                 // offset 8 (7 bytes padding)
//	};
//
//	struct _ddwaf_object_unsigned {  // type=0x06
//	    uint8_t type;                // offset 0
//	    uint64_t val;                // offset 8 (7 bytes padding)
//	};
//
//	struct _ddwaf_object_float {     // type=0x08
//	    uint8_t type;                // offset 0
//	    double val;                  // offset 8 (7 bytes padding)
//	};
//
//	struct _ddwaf_object_string {    // type=0x10 or 0x12
//	    uint8_t type;                // offset 0
//	    uint32_t size;               // offset 4 (3 bytes padding)
//	    char *ptr;                   // offset 8
//	};
//
//	struct _ddwaf_object_small_string { // type=0x14
//	    uint8_t type;                // offset 0
//	    uint8_t size;                // offset 1
//	    char data[14];               // offset 2
//	};
//
//	struct _ddwaf_object_array {     // type=0x20
//	    uint8_t type;                // offset 0
//	    uint16_t size;               // offset 2 (1 byte padding)
//	    uint16_t capacity;           // offset 4
//	    ddwaf_object *ptr;           // offset 8 (2 bytes padding)
//	};
//
//	struct _ddwaf_object_map {       // type=0x40
//	    uint8_t type;                // offset 0
//	    uint16_t size;               // offset 2 (1 byte padding)
//	    uint16_t capacity;           // offset 4
//	    ddwaf_object_kv *ptr;        // offset 8 (2 bytes padding)
//	};
//
// All these offsets are sorted in the constants below and used to query the byte slice.
type WAFObject struct {
	_    structs.HostLayout
	_    [0]uintptr // force 8-byte alignment to match C union (contains pointer/int64/float64 at offset 8)
	data [16]byte
}

const (
	wafObjectTypeOffset = 0  // uint8_t type at byte 0
	wafObjectBoolOffset = 1  // bool val at byte 1
	wafObjectSStrSize   = 1  // small string size at byte 1
	wafObjectSStrData   = 2  // small string data starts at byte 2
	wafObjectSStrMaxLen = 14 // max small string length

	wafObjectContainerSizeOffset     = 2 // uint16 size at byte 2 (for array/map)
	wafObjectContainerCapacityOffset = 4 // uint16 capacity at byte 4 (for array/map)
	wafObjectStringSizeOffset        = 4 // uint32 size at byte 4 (for string)
	wafObjectPtrOffset               = 8 // pointer at byte 8 (for string/array/map and int64/uint64/float64)
)

// WAFObjectKV represents a key-value pair in a map.
// Maps are now arrays of {key, value} pairs where both are full WAFObjects.
//
//	struct _ddwaf_object_kv {
//	    union _ddwaf_object key;    // 16 bytes
//	    union _ddwaf_object val;    // 16 bytes
//	};
type WAFObjectKV struct {
	_   structs.HostLayout
	Key WAFObject // 16 bytes
	Val WAFObject // 16 bytes
}

// Type returns the type discriminator of this WAFObject.
func (w *WAFObject) Type() WAFObjectType {
	return WAFObjectType(w.data[wafObjectTypeOffset])
}

// setType sets the type discriminator of this WAFObject.
func (w *WAFObject) setType(t WAFObjectType) {
	w.data[wafObjectTypeOffset] = byte(t)
}

// IsInvalid determines whether this WAF Object has the invalid type (which is the 0-value).
func (w *WAFObject) IsInvalid() bool {
	return w.Type() == WAFInvalidType
}

// IsNil determines whether this WAF Object is nil or not.
func (w *WAFObject) IsNil() bool {
	return w.Type() == WAFNilType
}

// IsArray determines whether this WAF Object is an array or not.
func (w *WAFObject) IsArray() bool {
	return w.Type() == WAFArrayType
}

// IsMap determines whether this WAF Object is a map or not.
func (w *WAFObject) IsMap() bool {
	return w.Type() == WAFMapType
}

// IsInt determines whether this WAF Object is a signed int or not.
func (w *WAFObject) IsInt() bool {
	return w.Type() == WAFIntType
}

// IsUint determines whether this WAF Object is a uint or not.
func (w *WAFObject) IsUint() bool {
	return w.Type() == WAFUintType
}

// IsBool determines whether this WAF Object is a bool or not.
func (w *WAFObject) IsBool() bool {
	return w.Type() == WAFBoolType
}

// IsFloat determines whether this WAF Object is a float or not.
func (w *WAFObject) IsFloat() bool {
	return w.Type() == WAFFloatType
}

// IsString determines whether this WAF Object is a string or not.
// Returns true for regular strings, literal strings, and small strings.
func (w *WAFObject) IsString() bool {
	t := w.Type()
	return t == WAFStringType || t == WAFLiteralStringType || t == WAFSmallStringType
}

// IsUnusable returns true if the wafObject has no impact on the WAF execution
// But we still need this kind of objects to forward map keys in case the value of the map is invalid
func (w *WAFObject) IsUnusable() bool {
	return w.Type() == WAFInvalidType || w.Type() == WAFNilType
}

// SetNil sets the receiving [WAFObject] to nil.
//
// C layout for nil (type=0x01):
//
//	byte 0: type (0x01)
//	bytes 1-15: unused (zeroed)
func (w *WAFObject) SetNil() {
	clear(w.data[:])
	w.setType(WAFNilType)
}

// SetInvalid sets the receiving [WAFObject] to invalid.
//
// C layout for invalid (type=0x00):
//
//	byte 0: type (0x00)
//	bytes 1-15: unused (zeroed)
func (w *WAFObject) SetInvalid() {
	clear(w.data[:])
	w.setType(WAFInvalidType)
}

// SetBool sets the receiving [WAFObject] value to the given bool.
//
// C layout for bool (type=0x02):
//
//	struct _ddwaf_object_bool {
//	    uint8_t type;    // offset 0
//	    bool val;        // offset 1
//	};
func (w *WAFObject) SetBool(b bool) {
	clear(w.data[:])
	w.setType(WAFBoolType)
	if b {
		w.data[wafObjectBoolOffset] = 1
	}
}

// SetInt sets the receiving [WAFObject] value to the given int64.
//
// C layout for signed int (type=0x04):
//
//	struct _ddwaf_object_signed {
//	    uint8_t type;    // offset 0
//	    // 7 bytes padding
//	    int64_t val;     // offset 8
//	};
func (w *WAFObject) SetInt(i int64) {
	clear(w.data[:])
	w.setType(WAFIntType)
	binary.NativeEndian.PutUint64(w.data[wafObjectPtrOffset:], uint64(i))
}

// SetUint sets the receiving [WAFObject] value to the given uint64.
//
// C layout for unsigned int (type=0x06):
//
//	struct _ddwaf_object_unsigned {
//	    uint8_t type;    // offset 0
//	    // 7 bytes padding
//	    uint64_t val;    // offset 8
//	};
func (w *WAFObject) SetUint(i uint64) {
	clear(w.data[:])
	w.setType(WAFUintType)
	binary.NativeEndian.PutUint64(w.data[wafObjectPtrOffset:], i)
}

// SetFloat sets the receiving [WAFObject] value to the given float64.
//
// C layout for float (type=0x08):
//
//	struct _ddwaf_object_float {
//	    uint8_t type;    // offset 0
//	    // 7 bytes padding
//	    double val;      // offset 8
//	};
func (w *WAFObject) SetFloat(f float64) {
	clear(w.data[:])
	w.setType(WAFFloatType)
	binary.NativeEndian.PutUint64(w.data[wafObjectPtrOffset:], math.Float64bits(f))
}

// SetString sets the receiving [WAFObject] value to the given string.
// Uses small string optimization for strings <= 14 bytes.
//
// C layout for string (type=0x10):
//
//	struct _ddwaf_object_string {
//	    uint8_t type;    // offset 0
//	    // 3 bytes padding
//	    uint32_t size;   // offset 4
//	    char *ptr;       // offset 8
//	};
//
// C layout for small string (type=0x14):
//
//	struct _ddwaf_object_small_string {
//	    uint8_t type;    // offset 0
//	    uint8_t size;    // offset 1
//	    char data[14];   // offset 2
//	};
func (w *WAFObject) SetString(pinner *runtime.Pinner, str string) {
	clear(w.data[:])

	header := unsafeutil.NativeStringUnwrap(str)
	length := header.Len

	// Use small string optimization for strings that fit in 14 bytes
	if length <= wafObjectSStrMaxLen {
		w.setType(WAFSmallStringType)
		w.data[wafObjectSStrSize] = byte(length)
		if length > 0 {
			copy(w.data[wafObjectSStrData:], unsafeutil.Slice((*byte)(unsafeutil.Pointer(header.Data)), uint64(length)))
		}
		return
	}

	// The C size field is uint32; clamp instead of letting the cast wrap
	// around for pathological (>4GiB) strings. WAFObject is public API, so
	// callers may reach this without the Encoder's truncation guard.
	if uint64(length) > math.MaxUint32 {
		length = math.MaxUint32
	}

	// Regular string: pin the data pointer
	w.setType(WAFStringType)
	binary.NativeEndian.PutUint32(w.data[wafObjectStringSizeOffset:], uint32(length))
	pinner.Pin(unsafeutil.Pointer(header.Data))
	unsafeutil.WritePtr(&w.data[wafObjectPtrOffset], unsafeutil.Pointer(header.Data))
}

// SetLiteralString sets the receiving [WAFObject] value to a literal string.
// Literal strings are never freed by the WAF.
//
// C layout for literal string (type=0x12):
//
//	struct _ddwaf_object_string {
//	    uint8_t type;    // offset 0
//	    // 3 bytes padding
//	    uint32_t size;   // offset 4
//	    char *ptr;       // offset 8
//	};
func (w *WAFObject) SetLiteralString(pinner *runtime.Pinner, str string) {
	clear(w.data[:])

	header := unsafeutil.NativeStringUnwrap(str)
	length := header.Len

	// See SetString for why the length is clamped to the uint32 size field.
	if uint64(length) > math.MaxUint32 {
		length = math.MaxUint32
	}

	w.setType(WAFLiteralStringType)
	binary.NativeEndian.PutUint32(w.data[wafObjectStringSizeOffset:], uint32(length))
	if length > 0 {
		pinner.Pin(unsafeutil.Pointer(header.Data))
		unsafeutil.WritePtr(&w.data[wafObjectPtrOffset], unsafeutil.Pointer(header.Data))
	}
}

// SetArray sets the receiving [WAFObject] to a new array with the given capacity.
// Returns a slice of WAFObjects that can be populated.
//
// C layout for array (type=0x20):
//
//	struct _ddwaf_object_array {
//	    uint8_t type;         // offset 0
//	    // 1 byte padding
//	    uint16_t size;        // offset 2
//	    uint16_t capacity;    // offset 4
//	    // 2 bytes padding
//	    ddwaf_object *ptr;    // offset 8
//	};
func (w *WAFObject) SetArray(pinner *runtime.Pinner, capacity uint16) []WAFObject {
	clear(w.data[:])
	w.setType(WAFArrayType)

	if capacity == 0 {
		return nil
	}

	arr := make([]WAFObject, 0, capacity)
	ptr := unsafeutil.Pointer(unsafeutil.SliceData(arr))
	pinner.Pin(ptr)

	binary.NativeEndian.PutUint16(w.data[wafObjectContainerSizeOffset:], 0)
	binary.NativeEndian.PutUint16(w.data[wafObjectContainerCapacityOffset:], capacity)
	unsafeutil.WritePtr(&w.data[wafObjectPtrOffset], ptr)

	return arr[:capacity]
}

// SetArrayData sets the receiving [WAFObject] to the provided array items.
//
// C layout for array (type=0x20): see SetArray
func (w *WAFObject) SetArrayData(pinner *runtime.Pinner, data []WAFObject) error {
	clear(w.data[:])
	w.setType(WAFArrayType)

	length := len(data)
	if length == 0 {
		return nil
	}
	if length > math.MaxUint16 || cap(data) > math.MaxUint16 {
		return fmt.Errorf("array too large for WAFObject: len=%d cap=%d (max %d)", length, cap(data), math.MaxUint16)
	}

	ptr := unsafeutil.Pointer(unsafeutil.SliceData(data))
	pinner.Pin(ptr)

	binary.NativeEndian.PutUint16(w.data[wafObjectContainerSizeOffset:], uint16(length))
	binary.NativeEndian.PutUint16(w.data[wafObjectContainerCapacityOffset:], uint16(cap(data)))
	unsafeutil.WritePtr(&w.data[wafObjectPtrOffset], ptr)
	return nil
}

// SetArraySize updates the size field of an array object.
// This is used after populating array elements.
func (w *WAFObject) SetArraySize(size uint16) {
	capacity := binary.NativeEndian.Uint16(w.data[wafObjectContainerCapacityOffset:])
	if size > capacity {
		size = capacity
	}
	binary.NativeEndian.PutUint16(w.data[wafObjectContainerSizeOffset:], size)
}

// SetMap sets the receiving [WAFObject] to a new map with the given capacity.
// Returns a slice of WAFObjectKV pairs that can be populated.
//
// C layout for map (type=0x40):
//
//	struct _ddwaf_object_map {
//	    uint8_t type;           // offset 0
//	    // 1 byte padding
//	    uint16_t size;          // offset 2
//	    uint16_t capacity;      // offset 4
//	    // 2 bytes padding
//	    ddwaf_object_kv *ptr;   // offset 8
//	};
func (w *WAFObject) SetMap(pinner *runtime.Pinner, capacity uint16) []WAFObjectKV {
	clear(w.data[:])
	w.setType(WAFMapType)

	if capacity == 0 {
		return nil
	}

	kvs := make([]WAFObjectKV, 0, capacity)
	ptr := unsafeutil.Pointer(unsafeutil.SliceData(kvs))
	pinner.Pin(ptr)

	binary.NativeEndian.PutUint16(w.data[wafObjectContainerSizeOffset:], 0)
	binary.NativeEndian.PutUint16(w.data[wafObjectContainerCapacityOffset:], capacity)
	unsafeutil.WritePtr(&w.data[wafObjectPtrOffset], ptr)

	return kvs[:capacity]
}

// SetMapData sets the receiving [WAFObject] to the provided map items.
//
// C layout for map (type=0x40): see SetMap
func (w *WAFObject) SetMapData(pinner *runtime.Pinner, data []WAFObjectKV) error {
	clear(w.data[:])
	w.setType(WAFMapType)

	length := len(data)
	if length == 0 {
		return nil
	}
	if length > math.MaxUint16 || cap(data) > math.MaxUint16 {
		return fmt.Errorf("map too large for WAFObject: len=%d cap=%d (max %d)", length, cap(data), math.MaxUint16)
	}

	ptr := unsafeutil.Pointer(unsafeutil.SliceData(data))
	pinner.Pin(ptr)

	binary.NativeEndian.PutUint16(w.data[wafObjectContainerSizeOffset:], uint16(length))
	binary.NativeEndian.PutUint16(w.data[wafObjectContainerCapacityOffset:], uint16(cap(data)))
	unsafeutil.WritePtr(&w.data[wafObjectPtrOffset], ptr)
	return nil
}

// SetMapSize updates the size field of a map object.
// This is used after populating map entries.
func (w *WAFObject) SetMapSize(size uint16) {
	capacity := binary.NativeEndian.Uint16(w.data[wafObjectContainerCapacityOffset:])
	if size > capacity {
		size = capacity
	}
	binary.NativeEndian.PutUint16(w.data[wafObjectContainerSizeOffset:], size)
}

// BoolValue returns the boolean value of this WAFObject.
func (w *WAFObject) BoolValue() (bool, error) {
	if !w.IsBool() {
		return false, fmt.Errorf("value is not a boolean but %s", w.Type().String())
	}
	return w.data[wafObjectBoolOffset] != 0, nil
}

// IntValue returns the int64 value of this WAFObject.
func (w *WAFObject) IntValue() (int64, error) {
	if !w.IsInt() {
		return 0, fmt.Errorf("value is not a signed int but %s", w.Type().String())
	}
	return int64(binary.NativeEndian.Uint64(w.data[wafObjectPtrOffset:])), nil
}

// UIntValue returns the uint64 value of this WAFObject.
func (w *WAFObject) UIntValue() (uint64, error) {
	if !w.IsUint() {
		return 0, fmt.Errorf("value is not an unsigned int but %s", w.Type().String())
	}
	return binary.NativeEndian.Uint64(w.data[wafObjectPtrOffset:]), nil
}

// FloatValue returns the float64 value of this WAFObject.
func (w *WAFObject) FloatValue() (float64, error) {
	if !w.IsFloat() {
		return 0, fmt.Errorf("value is not a float but %s", w.Type().String())
	}
	return math.Float64frombits(binary.NativeEndian.Uint64(w.data[wafObjectPtrOffset:])), nil
}

// StringValue returns the string value of this WAFObject.
// Works for regular strings, literal strings, and small strings.
func (w *WAFObject) StringValue() (string, error) {
	t := w.Type()
	switch t {
	case WAFSmallStringType:
		size := int(w.data[wafObjectSStrSize])
		if size > wafObjectSStrMaxLen {
			return "", fmt.Errorf("%w: small string size %d exceeds max %d", waferrors.ErrInvalidObjectType, size, wafObjectSStrMaxLen)
		}
		if size == 0 {
			return "", nil
		}
		return string(w.data[wafObjectSStrData : wafObjectSStrData+size]), nil

	case WAFStringType, WAFLiteralStringType:
		size := binary.NativeEndian.Uint32(w.data[wafObjectStringSizeOffset:])
		if size == 0 {
			return "", nil
		}
		if binary.NativeEndian.Uint64(w.data[wafObjectPtrOffset:]) == 0 {
			return "", fmt.Errorf("nil pointer in string object with size %d", size)
		}
		return string(unsafeutil.Slice(unsafeutil.ReadPtr[byte](&w.data[wafObjectPtrOffset]), uint64(size))), nil

	default:
		return "", fmt.Errorf("%w: value is not a string but %s", waferrors.ErrInvalidObjectType, t.String())
	}
}

// ArraySize returns the number of elements in an array.
func (w *WAFObject) ArraySize() (uint16, error) {
	if !w.IsArray() {
		return 0, fmt.Errorf("%w: value is not an array but %s", waferrors.ErrInvalidObjectType, w.Type().String())
	}
	return binary.NativeEndian.Uint16(w.data[wafObjectContainerSizeOffset:]), nil
}

// ArrayValues returns the array elements as a slice of WAFObjects as-is using unsafe conversion.
func (w *WAFObject) ArrayValues() ([]WAFObject, error) {
	if !w.IsArray() {
		return nil, fmt.Errorf("%w: value is not an array but %s", waferrors.ErrInvalidObjectType, w.Type().String())
	}

	size := binary.NativeEndian.Uint16(w.data[wafObjectContainerSizeOffset:])
	if size == 0 {
		return nil, nil
	}

	if binary.NativeEndian.Uint64(w.data[wafObjectPtrOffset:]) == 0 {
		return nil, fmt.Errorf("nil pointer in array object with size %d", size)
	}
	return unsafeutil.Slice(unsafeutil.ReadPtr[WAFObject](&w.data[wafObjectPtrOffset]), uint64(size)), nil
}

// MapSize returns the number of entries in a map.
func (w *WAFObject) MapSize() (uint16, error) {
	if !w.IsMap() {
		return 0, fmt.Errorf("%w: value is not a map but %s", waferrors.ErrInvalidObjectType, w.Type().String())
	}
	return binary.NativeEndian.Uint16(w.data[wafObjectContainerSizeOffset:]), nil
}

// MapEntries returns the map entries as a slice of WAFObjectKV pairs as-is using unsafe conversion.
func (w *WAFObject) MapEntries() ([]WAFObjectKV, error) {
	if !w.IsMap() {
		return nil, fmt.Errorf("%w: value is not a map but %s", waferrors.ErrInvalidObjectType, w.Type().String())
	}

	size := binary.NativeEndian.Uint16(w.data[wafObjectContainerSizeOffset:])
	if size == 0 {
		return nil, nil
	}

	if binary.NativeEndian.Uint64(w.data[wafObjectPtrOffset:]) == 0 {
		return nil, fmt.Errorf("nil pointer in map object with size %d", size)
	}
	return unsafeutil.Slice(unsafeutil.ReadPtr[WAFObjectKV](&w.data[wafObjectPtrOffset]), uint64(size)), nil
}

// ArrayValue returns the array as a slice of any values (recursive conversion) making a copy of the whole.
func (w *WAFObject) ArrayValue() ([]any, error) {
	if w.IsNil() {
		return nil, nil
	}

	items, err := w.ArrayValues()
	if err != nil {
		return nil, err
	}

	res := make([]any, len(items))
	for i, item := range items {
		res[i], err = item.AnyValue()
		if err != nil {
			return nil, fmt.Errorf("while decoding item at index %d: %w", i, err)
		}
	}
	return res, nil
}

// MapValue returns the map as a Go map (recursive conversion) making a copy of the whole.
func (w *WAFObject) MapValue() (map[string]any, error) {
	if w.IsNil() {
		return nil, nil
	}

	entries, err := w.MapEntries()
	if err != nil {
		return nil, err
	}

	res := make(map[string]any, len(entries))
	for _, entry := range entries {
		key, err := entry.Key.StringValue()
		if err != nil {
			return nil, fmt.Errorf("while decoding map key: %w", err)
		}
		res[key], err = entry.Val.AnyValue()
		if err != nil {
			return nil, fmt.Errorf("while decoding value at %q: %w", key, err)
		}
	}
	return res, nil
}

// AnyValue returns the value of this WAFObject as a Go any type.
func (w *WAFObject) AnyValue() (any, error) {
	switch w.Type() {
	case WAFArrayType:
		return w.ArrayValue()
	case WAFBoolType:
		return w.BoolValue()
	case WAFFloatType:
		return w.FloatValue()
	case WAFIntType:
		return w.IntValue()
	case WAFMapType:
		return w.MapValue()
	case WAFStringType, WAFLiteralStringType, WAFSmallStringType:
		return w.StringValue()
	case WAFUintType:
		return w.UIntValue()
	case WAFNilType:
		return nil, nil
	default:
		return nil, fmt.Errorf("%w: %s (0x%02x)", waferrors.ErrUnsupportedValue, w.Type(), byte(w.Type()))
	}
}

// WAFBuilder is a forward declaration in ddwaf.h header.
type WAFBuilder uintptr

// WAFHandle is a forward declaration in ddwaf.h header.
type WAFHandle uintptr

// WAFContext is a forward declaration in ddwaf.h header.
type WAFContext uintptr

// WAFSubcontext is a forward declaration in ddwaf.h header for subcontexts.
type WAFSubcontext uintptr

// WAFAllocator is an opaque handle to a WAF allocator.
type WAFAllocator uintptr
