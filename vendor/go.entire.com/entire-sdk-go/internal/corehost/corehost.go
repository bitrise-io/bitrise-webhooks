// Package corehost holds the rule that names Entire's production Cores, so
// the client and the plugin package share one copy.
package corehost

import (
	"net/url"
	"strings"
)

// Domains are the domains Entire's Cores live under: entire.com, and
// entire.io until Entire's move to entire.com ends.
var Domains = []string{"entire.com", "entire.io"}

// IsOrigin reports whether u is the origin of a production Core:
// https://{jurisdiction}.auth.entire.com, or the same under entire.io. Entire
// names every Core this way and keeps auth.entire.com and auth.entire.io for
// Cores alone, so a new jurisdiction needs no SDK release.
func IsOrigin(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	_, _, ok := Split(u.Hostname())
	return ok
}

// Split returns the jurisdiction and domain of a Core host such as
// us.auth.entire.io, and false when host is not one.
func Split(host string) (jurisdiction, domain string, ok bool) {
	host = strings.ToLower(host)
	for _, domain := range Domains {
		label, found := strings.CutSuffix(host, ".auth."+domain)
		if found && isDNSLabel(label) {
			return label, domain, true
		}
	}
	return "", "", false
}

// Origin returns the origin of the Core that serves jurisdiction under
// domain, such as https://eu.auth.entire.io.
func Origin(jurisdiction, domain string) *url.URL {
	return &url.URL{Scheme: "https", Host: jurisdiction + ".auth." + domain}
}

// isDNSLabel reports whether s is one lowercase DNS label, such as "us".
func isDNSLabel(s string) bool {
	if s == "" || len(s) > 63 || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}
