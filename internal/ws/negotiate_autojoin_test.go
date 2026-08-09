package ws

import (
	"context"
	"encoding/json"
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

// TestPatchReplacesMemberships proves PATCH /negotiate's replace semantics: an
// omitted group is left (unsubscribed) and the requested group is joined, against
// the live connection for the API key.
func TestPatchReplacesMemberships(t *testing.T) {
	hub := NewHub("classic", zap.NewNop(), IsValidClassicGroup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleOrderflowWS))
	defer srv.Close()

	initial := "blue_SPX_classic_gex_zero"
	token := url.QueryEscape("testkey:conn-1:" + initial)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/classic?access_token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Wait for the initial auto-join.
	active := func() map[string]bool {
		m := map[string]bool{}
		for _, g := range hub.GetActiveGroups() {
			m[g] = true
		}
		return m
	}
	for i := 0; i < 100 && !active()[initial]; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if !active()[initial] {
		t.Fatalf("initial group %q never joined", initial)
	}

	// PATCH: replace gex_zero with gex_one for this key.
	replacement := "blue_SPX_classic_gex_one"
	if n := hub.SetKeyGroups("testkey", []string{replacement}); n != 1 {
		t.Fatalf("expected 1 active group after replace, got %d", n)
	}
	a := active()
	if a[initial] {
		t.Errorf("omitted group %q should have been left", initial)
	}
	if !a[replacement] {
		t.Errorf("requested group %q should be joined", replacement)
	}
}

func activeSet(h *Hub) map[string]bool {
	m := map[string]bool{}
	for _, g := range h.GetActiveGroups() {
		m[g] = true
	}
	return m
}

func dialAutoJoin(t *testing.T, srv *httptest.Server, path, group string) *websocket.Conn {
	t.Helper()
	token := url.QueryEscape("testkey:conn-1:" + group)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + path + "?access_token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	return conn
}

func waitActive(t *testing.T, h *Hub, group string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if activeSet(h)[group] {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("group %q never became active on hub %q", group, h.name)
}

// TestNegotiateBodyValidation covers PR #26 review: POST requires groups
// (minItems:1); both methods reject malformed JSON; PATCH accepts an explicit
// empty array (clear-all) but rejects an absent groups field.
func TestNegotiateBodyValidation(t *testing.T) {
	h := NewNegotiateHandler(zap.NewNop(), "blue")
	cases := []struct {
		name, method, body string
		want               int
	}{
		{"post empty groups", "POST", `{"groups":[]}`, http.StatusBadRequest},
		{"post missing groups", "POST", `{}`, http.StatusBadRequest},
		{"post malformed", "POST", `{`, http.StatusBadRequest},
		{"post valid", "POST", `{"groups":["SPX_classic_gex_zero"]}`, http.StatusOK},
		{"patch missing groups", "PATCH", `{}`, http.StatusBadRequest},
		{"patch malformed", "PATCH", `{`, http.StatusBadRequest},
		{"patch empty array clears", "PATCH", `{"groups":[]}`, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/negotiate", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Basic testkey")
			rec := httptest.NewRecorder()
			if tc.method == "POST" {
				h.HandleNegotiatePost(rec, req)
			} else {
				h.HandleNegotiatePatch(rec, req)
			}
			if rec.Code != tc.want {
				t.Errorf("got %d, want %d (body: %s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

// TestPatchClearsOmittedHub covers PR #26 review: the PATCH payload is the
// complete desired set, so a hub absent from the request must be cleared.
func TestPatchClearsOmittedHub(t *testing.T) {
	classic := NewHub("classic", zap.NewNop(), IsValidClassicGroup)
	orderflow := NewHub("orderflow", zap.NewNop(), IsValidOrderflowGroup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go classic.Run(ctx)
	go orderflow.Run(ctx)

	neg := NewNegotiateHandler(zap.NewNop(), "blue")
	neg.SetHubs(map[string]*Hub{"classic": classic, "orderflow": orderflow})

	csrv := httptest.NewServer(http.HandlerFunc(classic.HandleOrderflowWS))
	defer csrv.Close()
	osrv := httptest.NewServer(http.HandlerFunc(orderflow.HandleOrderflowWS))
	defer osrv.Close()

	cconn := dialAutoJoin(t, csrv, "/ws/classic", "blue_SPX_classic_gex_zero")
	defer func() { _ = cconn.Close() }()
	oconn := dialAutoJoin(t, osrv, "/ws/orderflow", "blue_SPX_orderflow_orderflow")
	defer func() { _ = oconn.Close() }()

	waitActive(t, classic, "blue_SPX_classic_gex_zero")
	waitActive(t, orderflow, "blue_SPX_orderflow_orderflow")

	// PATCH lists only classic; orderflow is omitted and must be cleared.
	body := `{"groups":[{"hub":"classic","group":"SPX_classic_gex_one"}]}`
	req := httptest.NewRequest("PATCH", "/negotiate", strings.NewReader(body))
	req.Header.Set("Authorization", "Basic testkey")
	rec := httptest.NewRecorder()
	neg.HandleNegotiatePatch(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", rec.Code, rec.Body.String())
	}

	c := activeSet(classic)
	if c["blue_SPX_classic_gex_zero"] || !c["blue_SPX_classic_gex_one"] {
		t.Errorf("classic memberships not replaced: %v", c)
	}
	if o := activeSet(orderflow); len(o) != 0 {
		t.Errorf("omitted hub orderflow should be cleared, got %v", o)
	}
}

// TestPatchCountDedupsAndValidates covers PR #26 review: updated_groups is the
// active group count after replacement, so duplicate entries and hub-rejected
// groups must not inflate it.
func TestPatchCountDedupsAndValidates(t *testing.T) {
	classic := NewHub("classic", zap.NewNop(), IsValidClassicGroup)
	orderflow := NewHub("orderflow", zap.NewNop(), IsValidOrderflowGroup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go classic.Run(ctx)
	go orderflow.Run(ctx)

	neg := NewNegotiateHandler(zap.NewNop(), "blue")
	neg.SetHubs(map[string]*Hub{"classic": classic, "orderflow": orderflow})

	csrv := httptest.NewServer(http.HandlerFunc(classic.HandleOrderflowWS))
	defer csrv.Close()
	cconn := dialAutoJoin(t, csrv, "/ws/classic", "blue_SPX_classic_gex_zero")
	defer func() { _ = cconn.Close() }()
	waitActive(t, classic, "blue_SPX_classic_gex_zero")

	// 3 entries: the same valid group twice + one invalid group for the hub.
	body := `{"groups":[` +
		`{"hub":"classic","group":"SPX_classic_gex_zero"},` +
		`{"hub":"classic","group":"SPX_classic_gex_zero"},` +
		`{"hub":"classic","group":"bogus"}]}`
	req := httptest.NewRequest("PATCH", "/negotiate", strings.NewReader(body))
	req.Header.Set("Authorization", "Basic testkey")
	rec := httptest.NewRecorder()
	neg.HandleNegotiatePatch(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		UpdatedGroups int            `json:"updated_groups"`
		Hubs          map[string]int `json:"hubs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// One distinct valid group is active — not 3.
	if resp.UpdatedGroups != 1 {
		t.Errorf("updated_groups = %d, want 1 (deduped, valid-only)", resp.UpdatedGroups)
	}
	if resp.Hubs["classic"] != 1 {
		t.Errorf("hubs[classic] = %d, want 1", resp.Hubs["classic"])
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
