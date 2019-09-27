package security

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// SsoTokenVerifierInterface ...
type SsoTokenVerifierInterface interface {
	Verify(timestamp, ssoToken, appSlug string) (bool, error)
}

// SsoTokenVerifier ...
type SsoTokenVerifier struct {
	ValidTimeInterval time.Duration
	SsoSecret         string
}

// Verify ...
func (v *SsoTokenVerifier) Verify(timestamp, ssoToken, appSlug string) (bool, error) {
	unixTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false, errors.WithStack(err)
	}

	ssoTimestamp := time.Unix(unixTimestamp, 0)
	if time.Now().After(ssoTimestamp.Add(v.ValidTimeInterval)) {
		return false, nil
	}

	hashPrefix := "sha256-"
	var hash hash.Hash
	if strings.HasPrefix(ssoToken, hashPrefix) {
		ssoToken = strings.TrimPrefix(ssoToken, hashPrefix)
		hash = sha256.New()
	} else {
		hash = sha1.New()
	}

	_, err = hash.Write([]byte(fmt.Sprintf("%s:%s:%s", appSlug, v.SsoSecret, timestamp)))
	if err != nil {
		return false, errors.Wrap(err, "Failed to write into sha1 buffer")
	}
	refToken := fmt.Sprintf("%x", hash.Sum(nil))
	if ssoToken != refToken {
		return false, nil
	}
	return true, nil
}
