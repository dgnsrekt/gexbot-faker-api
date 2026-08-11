package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectFields(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		fields []string
		want   string
	}{
		{"keeps requested keys in order", `{"a":1,"b":2,"c":3}`, []string{"c", "a"}, `{"c":3,"a":1}`},
		{"skips missing keys", `{"a":1}`, []string{"a", "zzz"}, `{"a":1}`},
		{"non-object passes through", `[1,2,3]`, []string{"a"}, `[1,2,3]`},
		{"empty selection yields empty object", `{"a":1}`, []string{"x"}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := projectFields(json.RawMessage(tt.raw), tt.fields)
			if string(got) != tt.want {
				t.Fatalf("projectFields(%s, %v) = %s, want %s", tt.raw, tt.fields, got, tt.want)
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" a, b ,,c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// The Authorization header must be sent on data routes (auth=true) and withheld
// on discovery/control routes (auth=false) — matching the faker's shape-based auth.
func TestAuthHeaderOnlyOnDataRoutes(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	flagURL, flagKey = srv.URL, "secret-token"
	c := newClient()

	if _, err := c.get(context.Background(), "/QQQ/classic/gex_zero", true, nil); err != nil {
		t.Fatalf("data get: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("data route Authorization = %q, want Bearer secret-token", gotAuth)
	}

	gotAuth = ""
	if _, err := c.get(context.Background(), "/tickers", false, nil); err != nil {
		t.Fatalf("discovery get: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("discovery route sent Authorization = %q, want none", gotAuth)
	}
}

func TestHTTPErrorHints(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://x/QQQ/classic/gex_zero", nil)
	tests := []struct {
		name       string
		status     int
		body       string
		wantSubstr string
	}{
		{"exhausted cursor", http.StatusNotFound, `{"error":"No more data available"}`, "reset"},
		{"missing auth", http.StatusBadRequest, `{"error":"Authorization header not found."}`, "--key"},
		{"reload in progress", http.StatusConflict, `{"error":"reload already in progress"}`, "retry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ae := httpError(req, tt.status, []byte(tt.body))
			if ae.Status != tt.status {
				t.Fatalf("status = %d, want %d", ae.Status, tt.status)
			}
			if !strings.Contains(ae.Hint, tt.wantSubstr) {
				t.Fatalf("hint %q does not mention %q", ae.Hint, tt.wantSubstr)
			}
		})
	}
}
