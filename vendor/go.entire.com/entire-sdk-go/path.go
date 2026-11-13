package entire

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// filePath is a path argument of pathf that spans segments, such as
// "cmd/app/main.go". Each segment is escaped separately.
type filePath string

// pathf formats a request path, escaping each argument as one path segment.
// Every verb in format is %s, and each argument is a string, an int64, or a
// filePath. An empty or dot segment and a number below one are rejected, so
// an argument can never change which endpoint a request reaches.
func pathf(format string, args ...any) (string, error) {
	escaped := make([]any, len(args))
	shown := make([]any, len(args))
	problem := ""
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			escaped[i], shown[i] = url.PathEscape(v), v
			switch {
			case v == "":
				shown[i], problem = "<empty>", "a path argument is empty"
			case v == "." || v == "..":
				problem = "a path argument is a dot segment"
			}
		case int64:
			escaped[i] = strconv.FormatInt(v, 10)
			shown[i] = escaped[i]
			if v < 1 {
				problem = "a number in the path is not positive"
			}
		case filePath:
			parts := strings.Split(string(v), "/")
			for j, part := range parts {
				if part == "" || part == "." || part == ".." {
					problem = "a file path has an empty or dot segment"
				}
				parts[j] = url.PathEscape(part)
			}
			escaped[i], shown[i] = strings.Join(parts, "/"), string(v)
		default:
			panic(fmt.Sprintf("entire: pathf argument of type %T", arg))
		}
	}
	if problem != "" {
		return "", fmt.Errorf("entire: invalid request path %s: %s", fmt.Sprintf(format, shown...), problem)
	}
	return fmt.Sprintf(format, escaped...), nil
}
