package redact

import (
	"strings"
	"testing"
)

func TestURLStripsSignedQuery(t *testing.T) {
	signed := "https://hist.gexbot.com/v2/hist/SPX/classic/gex_full/2026-08-07?X-Amz-Signature=deadbeef&X-Amz-Credential=AKIA123/secret&token=abc"
	got := URL(signed)
	if strings.Contains(got, "Signature") || strings.Contains(got, "AKIA123") || strings.Contains(got, "token") || strings.Contains(got, "?") {
		t.Errorf("signed query not stripped: %q", got)
	}
	if got != "https://hist.gexbot.com/v2/hist/SPX/classic/gex_full/2026-08-07" {
		t.Errorf("unexpected redaction: %q", got)
	}
	// User info must go too.
	if u := URL("https://user:pass@example.com/x?y=1"); strings.Contains(u, "pass") || strings.Contains(u, "y=1") {
		t.Errorf("userinfo/query leaked: %q", u)
	}
	if URL("::::not a url") == "" {
		t.Error("unparseable URL should redact, not return empty")
	}
}

func TestTextRedactsEmbeddedURLQuery(t *testing.T) {
	line := `download fell back to https://hist.gexbot.com/f/x?X-Amz-Signature=abc&k=v after retries`
	got := Text(line)
	if strings.Contains(got, "Signature=abc") || strings.Contains(got, "k=v") {
		t.Errorf("embedded signed query not redacted: %q", got)
	}
	if !strings.Contains(got, "https://hist.gexbot.com/f/x?[redacted]") {
		t.Errorf("expected redacted marker, got: %q", got)
	}
	// URLs without a query are left alone.
	if Text("see https://ntfy.sh/mytopic now") != "see https://ntfy.sh/mytopic now" {
		t.Error("query-less URL should be unchanged by Text")
	}
}
