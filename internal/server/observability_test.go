package server

import (
	"net/url"
	"strings"
	"testing"
)

func TestMaskQueryKeyRedactsSecret(t *testing.T) {
	masked := maskQueryKey("key=super-secret&ticker=SPX")
	if strings.Contains(masked, "super-secret") {
		t.Fatal("masked query contains API key")
	}
	values, err := url.ParseQuery(masked)
	if err != nil {
		t.Fatal(err)
	}
	if values.Get("key") != "[REDACTED]" || values.Get("ticker") != "SPX" {
		t.Fatalf("unexpected masked query: %q", masked)
	}
}
