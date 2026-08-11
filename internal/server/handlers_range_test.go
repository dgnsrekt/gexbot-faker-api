package server

import (
	"context"
	"testing"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
)

// Load must reject a body with more than one selector (date + from/to + dates) at the handler
// boundary, independent of the OpenAPI oneOf validation. The check returns before touching loader
// state, so a bare Server suffices.
func TestLoadRejectsConflictingSelectors(t *testing.T) {
	s := &Server{}
	resp, err := s.Load(context.Background(), generated.LoadRequestObject{Body: &generated.LoadRequest{
		Date: ptr("2026-08-07"),
		From: ptr("2026-08-06"),
		To:   ptr("2026-08-10"),
	}})
	if err != nil {
		t.Fatalf("Load returned a transport error: %v", err)
	}
	if _, ok := resp.(generated.Load400JSONResponse); !ok {
		t.Errorf("conflicting selectors → %T, want Load400JSONResponse", resp)
	}
}
