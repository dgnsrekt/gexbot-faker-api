package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// TestPostNegotiateAutoJoin proves the POST /negotiate contract end-to-end: a
// client that connects with a group-bearing access token (as buildWebsocketURLs
// produces for POST) is auto-joined to that hub group, so it will receive
// broadcasts without ever sending a joinGroup message.
func TestPostNegotiateAutoJoin(t *testing.T) {
	hub := NewHub("orderflow", zap.NewNop(), IsValidOrderflowGroup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleOrderflowWS))
	defer srv.Close()

	group := "blue_SPX_orderflow_orderflow"
	// Token shape that POST /negotiate embeds: apiKey:connID:group1,group2
	token := url.QueryEscape("testkey:conn-1:" + group)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/orderflow?access_token=" + token

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Auto-join runs in the connect handler after registration; poll until the
	// group is active (bounded).
	joined := false
	for i := 0; i < 100 && !joined; i++ {
		for _, g := range hub.GetActiveGroups() {
			if g == group {
				joined = true
			}
		}
		if !joined {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if !joined {
		t.Fatalf("client was not auto-joined to %q; active groups: %v", group, hub.GetActiveGroups())
	}
}

// TestPlainNegotiateNoAutoJoin confirms a token without a group field (the GET
// /negotiate flow) joins nothing on connect.
func TestPlainNegotiateNoAutoJoin(t *testing.T) {
	hub := NewHub("orderflow", zap.NewNop(), IsValidOrderflowGroup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleOrderflowWS))
	defer srv.Close()

	token := url.QueryEscape("testkey:conn-1")
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/orderflow?access_token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Give the connect path time to run, then assert no groups were joined.
	time.Sleep(150 * time.Millisecond)
	if groups := hub.GetActiveGroups(); len(groups) != 0 {
		t.Fatalf("expected no auto-joined groups, got %v", groups)
	}
}
