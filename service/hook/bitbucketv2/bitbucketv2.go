package bitbucketv2

import (
	"fmt"
	"net/http"

	"github.com/bitrise-io/go-utils/httputil"
)

func detectContentTypeUserAgentAndEventKey(header http.Header) (string, string, string, error) {
	contentType, err := httputil.GetSingleValueFromHeader("Content-Type", header)
	if err != nil {
		return "", "", "", fmt.Errorf("Issue with Content-Type Header: %s", err)
	}

	userAgent, err := httputil.GetSingleValueFromHeader("User-Agent", header)
	if err != nil {
		return "", "", "", fmt.Errorf("Issue with User-Agent Header: %s", err)
	}

	eventKey, err := httputil.GetSingleValueFromHeader("X-Event-Key", header)
	if err != nil {
		return "", "", "", fmt.Errorf("Issue with X-Event-Key Header: %s", err)
	}

	return contentType, userAgent, eventKey, nil
}
