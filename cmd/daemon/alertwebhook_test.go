package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/download"
)

type capturedAlert struct{ title, message, priority string }

type fakeNotifier struct{ alerts []capturedAlert }

func (f *fakeNotifier) SendSuccess(context.Context, *download.BatchResult, string, time.Duration) error {
	return nil
}
func (f *fakeNotifier) SendFailure(context.Context, *download.BatchResult, string, time.Duration, error) error {
	return nil
}
func (f *fakeNotifier) SendAlert(_ context.Context, title, message, priority string) error {
	f.alerts = append(f.alerts, capturedAlert{title, message, priority})
	return nil
}

func TestAlertWebhookHandler(t *testing.T) {
	fn := &fakeNotifier{}
	h := alertWebhookHandler(fn, zap.NewNop())

	body := `{"alerts":[
		{"status":"firing","labels":{"alertname":"FakerDataVolumeCritical","severity":"critical"},"annotations":{"summary":"disk critically low"}},
		{"status":"firing","labels":{"alertname":"FakerHTTPErrorRateHigh","severity":"warning"},"annotations":{"summary":"5xx high"}},
		{"status":"resolved","labels":{"alertname":"FakerReloadFailed","severity":"warning"},"annotations":{"summary":"reload failed"}}
	]}`
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/alerts/ntfy", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(fn.alerts) != 3 {
		t.Fatalf("forwarded %d alerts, want 3", len(fn.alerts))
	}
	if got := fn.alerts[0]; got.priority != "urgent" || got.title != "FakerDataVolumeCritical" {
		t.Errorf("critical alert = %+v, want urgent/FakerDataVolumeCritical", got)
	}
	if got := fn.alerts[1]; got.priority != "high" {
		t.Errorf("warning priority = %q, want high", got.priority)
	}
	if got := fn.alerts[2]; !strings.HasPrefix(got.title, "Resolved:") || got.priority != "default" {
		t.Errorf("resolved alert = %+v, want 'Resolved:' title + default priority", got)
	}

	// A GET is rejected (webhook is POST-only).
	rec2 := httptest.NewRecorder()
	h(rec2, httptest.NewRequest(http.MethodGet, "/alerts/ntfy", nil))
	if rec2.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET status = %d, want 405", rec2.Code)
	}
}
