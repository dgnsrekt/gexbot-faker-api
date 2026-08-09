package ws

import (
	"context"
	"testing"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/observability"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

func newTestClient(h *Hub, apiKey string) *Client {
	return &Client{
		hub:    h,
		send:   make(chan []byte, 1),
		apiKey: apiKey,
		connID: "conn",
		groups: map[string]bool{},
		logger: zap.NewNop(),
	}
}

func waitFor(t *testing.T, cond func() bool, desc string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", desc)
}

// TestDuplicateUnregisterKeepsConnectionsGaugeNonNegative covers the PR #29
// review note: unregister can be queued twice (a failed broadcast schedules it,
// then readPump's defer schedules it again). Map deletion is idempotent but the
// gauge is not, so it must only decrement when the client was actually present.
func TestDuplicateUnregisterKeepsConnectionsGaugeNonNegative(t *testing.T) {
	const hubName = "test_dup_unregister"
	hub := NewHub(hubName, zap.NewNop(), func(string) bool { return true })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	gauge := func() float64 {
		return testutil.ToFloat64(observability.WSConnections.WithLabelValues(hubName))
	}
	client := newTestClient(hub, "k")

	hub.register <- client
	waitFor(t, func() bool { return gauge() == 1 }, "connections==1 after register")

	hub.unregister <- client
	waitFor(t, func() bool { return gauge() == 0 }, "connections==0 after unregister")

	// Duplicate unregister for an already-removed client must be a no-op.
	hub.unregister <- client
	time.Sleep(50 * time.Millisecond) // let the loop process it
	if g := gauge(); g != 0 {
		t.Fatalf("duplicate unregister drove connections gauge to %v, want 0", g)
	}
}

// TestSetKeyGroupsRefreshesActiveGroupsGauge covers the PR #29 review note: PATCH
// /negotiate goes through SetKeyGroups, which mutates h.groups directly. The
// faker_ws_active_groups gauge must be refreshed there, not left stale until the
// next join/leave/unregister.
func TestSetKeyGroupsRefreshesActiveGroupsGauge(t *testing.T) {
	const hubName = "test_setkeygroups"
	hub := NewHub(hubName, zap.NewNop(), func(string) bool { return true })
	client := newTestClient(hub, "k")

	hub.JoinGroup(client, "g1")
	hub.JoinGroup(client, "g2")
	gauge := func() float64 {
		return testutil.ToFloat64(observability.WSActiveGroups.WithLabelValues(hubName))
	}
	if g := gauge(); g != 2 {
		t.Fatalf("setup: expected active_groups==2, got %v", g)
	}

	// Replace {g1,g2} with {g1}: the gauge must drop to 1 as SetKeyGroups returns.
	if n := hub.SetKeyGroups("k", []string{"g1"}); n != 1 {
		t.Fatalf("SetKeyGroups returned %d, want 1", n)
	}
	if g := gauge(); g != 1 {
		t.Fatalf("active_groups gauge stale after SetKeyGroups: got %v, want 1", g)
	}
}
