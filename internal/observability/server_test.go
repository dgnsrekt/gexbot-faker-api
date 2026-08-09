package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusHandler(t *testing.T) {
	for _, tc := range []struct {
		name string
		ok   bool
		want int
	}{{"ready", true, http.StatusOK}, {"unavailable", false, http.StatusServiceUnavailable}} {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			statusHandler(func() bool { return tc.ok })(rr, httptest.NewRequest(http.MethodGet, "/", nil))
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d", rr.Code, tc.want)
			}
		})
	}
}
