package entire

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// addQuery sets query parameters from the fields of opts, a pointer to an
// options struct, using each field's `url:"name"` tag. Untagged fields, such
// as an embedded ListOptions, and fields tagged "-" are skipped. Zero values
// are left out, a pointer sends the value it points to, and a slice repeats
// the parameter. Times are sent in RFC 3339 form. A nil opts adds nothing.
func addQuery(q url.Values, opts any) {
	v := reflect.ValueOf(opts)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	t := v.Type()
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("url"), ",")
		field := v.Field(i)
		if name == "" || name == "-" || field.IsZero() {
			continue
		}
		if field.Kind() == reflect.Pointer {
			field = field.Elem()
		}
		if field.Kind() == reflect.Slice {
			for j := range field.Len() {
				q.Add(name, queryValue(field.Index(j)))
			}
			continue
		}
		q.Set(name, queryValue(field))
	}
}

func queryValue(v reflect.Value) string {
	if t, ok := v.Interface().(time.Time); ok {
		return t.UTC().Format(time.RFC3339Nano)
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	}
	panic(fmt.Sprintf("entire: unsupported query field type %s", v.Type()))
}
