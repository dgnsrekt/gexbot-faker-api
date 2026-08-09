package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NegotiateResponse matches the real GexBot API negotiate response format.
type NegotiateResponse struct {
	WebsocketURLs map[string]string `json:"websocket_urls"`
	Prefix        string            `json:"prefix"`
}

// NegotiateHandler handles the /negotiate endpoint.
type NegotiateHandler struct {
	logger *zap.Logger
	prefix string
	hubs   map[string]*Hub // hub name -> hub, for PATCH group management
}

// NewNegotiateHandler creates a new NegotiateHandler.
func NewNegotiateHandler(logger *zap.Logger, prefix string) *NegotiateHandler {
	return &NegotiateHandler{logger: logger, prefix: prefix}
}

// SetHubs wires the WebSocket hubs (keyed by hub name) so PATCH /negotiate can
// replace live group memberships. Safe to call once at startup before serving.
func (h *NegotiateHandler) SetHubs(hubs map[string]*Hub) {
	h.hubs = hubs
}

// apiKeyFromAuthHeader extracts the key from "Basic <key>" or "Bearer <key>"
// (the real API accepts both).
func apiKeyFromAuthHeader(r *http.Request) string {
	h := r.Header.Get("Authorization")
	for _, prefix := range []string{"Basic ", "Bearer "} {
		if strings.HasPrefix(h, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(h, prefix))
		}
	}
	return ""
}

// buildWebsocketURLs returns the per-hub WebSocket URLs for a connection, each
// carrying a simplified access token (real API uses JWT). Any groups are
// prefixed to internal form and embedded in the token so the hubs auto-join them
// on connect (POST /negotiate contract); pass nil for the plain GET flow.
func (h *NegotiateHandler) buildWebsocketURLs(r *http.Request, apiKey string, groups []string) map[string]string {
	connID := uuid.New().String()
	token := fmt.Sprintf("%s:%s", apiKey, connID)
	if prefixed := h.prefixGroups(groups); len(prefixed) > 0 {
		token += ":" + strings.Join(prefixed, ",")
	}
	token = url.QueryEscape(token)
	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}
	baseURL := fmt.Sprintf("%s://%s/ws", scheme, r.Host)
	return map[string]string{
		"orderflow":         fmt.Sprintf("%s/orderflow?access_token=%s", baseURL, token),
		"state_gex":         fmt.Sprintf("%s/state_gex?access_token=%s", baseURL, token),
		"classic":           fmt.Sprintf("%s/classic?access_token=%s", baseURL, token),
		"state_greeks_zero": fmt.Sprintf("%s/state_greeks_zero?access_token=%s", baseURL, token),
		"state_greeks_one":  fmt.Sprintf("%s/state_greeks_one?access_token=%s", baseURL, token),
	}
}

// prefixGroups converts unprefixed upstream group names (e.g. "SPX_state_gamma_zero")
// to the faker's internal form ("{prefix}_SPX_state_gamma_zero"), dropping empties.
func (h *NegotiateHandler) prefixGroups(groups []string) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		if g != "" {
			out = append(out, h.prefix+"_"+g)
		}
	}
	return out
}

// HandleNegotiate handles GET /negotiate. Accepts the API key via the
// Authorization header (Basic or Bearer). Returns WebSocket URLs for the hubs.
func (h *NegotiateHandler) HandleNegotiate(w http.ResponseWriter, r *http.Request) {
	apiKey := apiKeyFromAuthHeader(r)
	if apiKey == "" {
		h.logger.Debug("negotiate request missing authorization")
		http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
		return
	}

	response := NegotiateResponse{
		WebsocketURLs: h.buildWebsocketURLs(r, apiKey, nil),
		Prefix:        h.prefix,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode negotiate response", zap.Error(err))
	}
}

// NegotiatePostResponse matches the live POST /negotiate response.
type NegotiatePostResponse struct {
	WebsocketURLs map[string]string `json:"websocket_urls"`
}

// HandleNegotiatePost handles POST /negotiate: given a set of groups to join, it
// returns the WebSocket URLs for the connection (mirrors the live API shape).
func (h *NegotiateHandler) HandleNegotiatePost(w http.ResponseWriter, r *http.Request) {
	apiKey := apiKeyFromAuthHeader(r)
	if apiKey == "" {
		http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
		return
	}
	// Body carries {"groups": [...]} to auto-join; embedded in the token so the
	// hubs join them on connect. Upstream requires groups (minItems: 1).
	var body struct {
		Groups []string `json:"groups"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if len(body.Groups) == 0 {
		http.Error(w, `{"error":"groups is required"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(NegotiatePostResponse{
		WebsocketURLs: h.buildWebsocketURLs(r, apiKey, body.Groups),
	}); err != nil {
		h.logger.Error("failed to encode negotiate post response", zap.Error(err))
	}
}

// NegotiatePatchRequest is the body of PATCH /negotiate. Groups is a pointer so
// an absent field (nil -> 400) is distinguishable from an explicit empty array
// ([] -> clear all memberships).
type NegotiatePatchRequest struct {
	Groups *[]struct {
		Hub   string `json:"hub"`
		Group string `json:"group"`
	} `json:"groups"`
}

// NegotiatePatchResponse matches the live PATCH /negotiate response: updated_groups
// is the number of group entries applied, and hubs maps each touched hub to its
// active group count for the API key.
type NegotiatePatchResponse struct {
	UpdatedGroups int            `json:"updated_groups"`
	Hubs          map[string]int `json:"hubs"`
}

// HandleNegotiatePatch handles PATCH /negotiate: it replaces the API key's group
// memberships on each named hub against the connected clients and returns the
// resulting active group count per hub.
func (h *NegotiateHandler) HandleNegotiatePatch(w http.ResponseWriter, r *http.Request) {
	apiKey := apiKeyFromAuthHeader(r)
	if apiKey == "" {
		http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
		return
	}
	var req NegotiatePatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Groups == nil {
		http.Error(w, `{"error":"groups is required"}`, http.StatusBadRequest)
		return
	}
	// An explicit empty array is allowed and clears all memberships.

	// Group the requested memberships by hub, prefixing to internal form.
	byHub := map[string][]string{}
	for _, g := range *req.Groups {
		if g.Hub != "" && g.Group != "" {
			byHub[g.Hub] = append(byHub[g.Hub], h.prefix+"_"+g.Group)
		}
	}

	// The payload is the complete desired set: iterate every configured hub so a
	// hub absent from the request is cleared (SetKeyGroups with an empty slice),
	// not left with stale memberships.
	// updated_groups is the total ACTIVE group count after replacement — use the
	// count SetKeyGroups actually applies (deduped, hub-validated), not the raw
	// request-entry count which would over-report duplicates and rejected groups.
	hubs := map[string]int{}
	updated := 0
	for hubName, hub := range h.hubs {
		count := hub.SetKeyGroups(apiKey, byHub[hubName])
		hubs[hubName] = count
		updated += count
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(NegotiatePatchResponse{
		UpdatedGroups: updated,
		Hubs:          hubs,
	}); err != nil {
		h.logger.Error("failed to encode negotiate patch response", zap.Error(err))
	}
}

// maskAPIKey fully redacts an API key for logging. The observability stack ships
// logs to Loki/Grafana, so no portion of the key is emitted — matching the server
// and sync packages.
func maskAPIKey(string) string { return "[REDACTED]" }
