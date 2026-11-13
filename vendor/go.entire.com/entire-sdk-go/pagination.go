package entire

import (
	"errors"
	"fmt"
	"iter"
	"net/url"
	"reflect"
	"strconv"
)

// ListOptions selects a page of a listing. Every list method's options embed
// it, whatever the endpoint calls its page parameters.
type ListOptions struct {
	// PageSize is the number of items per page. Zero leaves it to the
	// server.
	PageSize int
	// PageToken requests the page that follows a response whose Next it is.
	// Leave it empty for the first page.
	PageToken string
}

// add sets the page parameters under the names that the endpoint uses for
// them.
func (o ListOptions) add(q url.Values, sizeName, tokenName string) {
	if o.PageSize > 0 {
		q.Set(sizeName, strconv.Itoa(o.PageSize))
	}
	if o.PageToken != "" {
		q.Set(tokenName, o.PageToken)
	}
}

// ErrRepeatedPageToken reports a server that returned a continuation value it
// had already returned. Iteration stops instead of looping forever.
var ErrRepeatedPageToken = errors.New("entire: server repeated a pagination token")

// Iterate returns an iterator over the items of a paginated listing, as the
// SDK's All methods do. Use it for endpoints that the SDK does not cover.
// fetch requests the page for a continuation value, the empty string
// requesting the first page, and returns it with Response.Next set; items
// extracts a page's entries. Iteration follows Next until it is empty, stops
// at the first error, which it yields once with the zero item, and stops when
// the caller stops ranging. Each range starts from the first page again.
func Iterate[T, R any](fetch func(next string) (*Response[R], error), items func(*Response[R]) []T) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		seen := make(map[string]struct{})
		token := ""
		for {
			resp, err := fetch(token)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range items(resp) {
				if !yield(item, nil) {
					return
				}
			}
			if resp.Next == "" {
				return
			}
			if _, repeated := seen[resp.Next]; repeated {
				var zero T
				yield(zero, fmt.Errorf("%w: %q", ErrRepeatedPageToken, resp.Next))
				return
			}
			seen[resp.Next] = struct{}{}
			token = resp.Next
		}
	}
}

// all iterates over every item of a listing, starting at the page that token
// selects (the first page when it is empty), and fetching each later page
// with the Next of the one before. Like Iterate, each range starts over.
// snapshot returns a copy of the options at p, or the zero options when p is
// nil. The copy has its own slices, so a lazy iterator holding it is
// unaffected by later changes to the caller's.
func snapshot[T any](p *T) T {
	o := Deref(p)
	v := reflect.ValueOf(&o).Elem()
	for i := range v.NumField() {
		if f := v.Field(i); f.Kind() == reflect.Slice && !f.IsNil() && f.CanSet() {
			c := reflect.MakeSlice(f.Type(), f.Len(), f.Len())
			reflect.Copy(c, f)
			f.Set(c)
		}
	}
	return o
}

func all[T any](token string, list func(token string) (*Response[[]T], error)) iter.Seq2[T, error] {
	return Iterate(func(next string) (*Response[[]T], error) {
		if next == "" {
			next = token
		}
		return list(next)
	}, func(r *Response[[]T]) []T { return r.Data })
}
