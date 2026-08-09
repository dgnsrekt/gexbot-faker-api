package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
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
}

// NewNegotiateHandler creates a new NegotiateHandler.
func NewNegotiateHandler(logger *zap.Logger, prefix string) *NegotiateHandler {
	return &NegotiateHandler{logger: logger, prefix: prefix}
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
// carrying a simplified access token (real API uses JWT).
func (h *NegotiateHandler) buildWebsocketURLs(r *http.Request, apiKey string) map[string]string {
	connID := uuid.New().String()
	token := fmt.Sprintf("%s:%s", apiKey, connID)
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
		WebsocketURLs: h.buildWebsocketURLs(r, apiKey),
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
	// Body carries {"groups": [...]}; the faker's URLs are per-hub, so we accept
	// and ignore the specific groups (drain the body for a clean request).
	_ = json.NewDecoder(r.Body).Decode(&struct {
		Groups []string `json:"groups"`
	}{})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(NegotiatePostResponse{
		WebsocketURLs: h.buildWebsocketURLs(r, apiKey),
	}); err != nil {
		h.logger.Error("failed to encode negotiate post response", zap.Error(err))
	}
}

// NegotiatePatchRequest is the body of PATCH /negotiate.
type NegotiatePatchRequest struct {
	Groups []struct {
		Hub   string `json:"hub"`
		Group string `json:"group"`
	} `json:"groups"`
}

// NegotiatePatchResponse matches the live PATCH /negotiate response.
type NegotiatePatchResponse struct {
	UpdatedGroups int                 `json:"updated_groups"`
	Hubs          map[string][]string `json:"hubs"`
}

// HandleNegotiatePatch handles PATCH /negotiate: it updates group subscriptions
// and reports how many were updated plus the resulting hub->groups mapping.
func (h *NegotiateHandler) HandleNegotiatePatch(w http.ResponseWriter, r *http.Request) {
	apiKey := apiKeyFromAuthHeader(r)
	if apiKey == "" {
		http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
		return
	}
	var req NegotiatePatchRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	hubs := map[string][]string{}
	for _, g := range req.Groups {
		hubs[g.Hub] = append(hubs[g.Hub], g.Group)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(NegotiatePatchResponse{
		UpdatedGroups: len(req.Groups),
		Hubs:          hubs,
	}); err != nil {
		h.logger.Error("failed to encode negotiate patch response", zap.Error(err))
	}
}

// maskAPIKey masks all but the first 4 characters of an API key for logging.
func maskAPIKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:4] + "****"
}
