// Package redact strips capability-bearing material (signed-URL query strings,
// user info) from values before they reach logs. Signed history-download URLs
// and similar carry authorization in their query parameters; those must never be
// logged, especially now that logs are aggregated through an HTTP endpoint.
package redact

import (
	"net/url"
	"regexp"
)

// URL removes the query string, user info, and fragment from a URL, keeping only
// scheme://host/path — enough to identify the endpoint without leaking a signed
// token. Returns "[redacted-url]" if the input doesn't parse.
func URL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[redacted-url]"
	}
	u.RawQuery = ""
	u.User = nil
	u.Fragment = ""
	return u.String()
}

// urlInText matches an http(s) URL that carries a query string.
var urlInText = regexp.MustCompile(`(?i)(https?://[^\s"'?]+)\?[^\s"']*`)

// Text redacts the query string of any http(s) URL embedded in free text,
// leaving the scheme://host/path intact. Defense-in-depth for log lines whose
// producers may include signed URLs.
func Text(s string) string {
	return urlInText.ReplaceAllString(s, "$1?[redacted]")
}
