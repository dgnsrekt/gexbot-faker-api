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
}

// NegotiateHandler handles the /negotiate endpoint.
type NegotiateHandler struct {
	logger *zap.Logger
}

// NewNegotiateHandler creates a new NegotiateHandler.
func NewNegotiateHandler(logger *zap.Logger) *NegotiateHandler {
	return &NegotiateHandler{logger: logger}
}

// HandleNegotiate handles GET /negotiate
// Accepts API key via Authorization header (matching real GexBot API) or query param (fallback).
// Returns WebSocket URLs for available hubs.
func (h *NegotiateHandler) HandleNegotiate(w http.ResponseWriter, r *http.Request) {
	// Extract API key from Authorization header (matches real GexBot API)
	// Format: "Authorization: Basic <API_KEY>"
	var apiKey string
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Basic ") {
		apiKey = strings.TrimPrefix(authHeader, "Basic ")
	}

	if apiKey == "" {
		h.logger.Debug("negotiate request missing authorization")
		http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
		return
	}

	// Generate simple access token (apiKey:connectionID)
	// This is simplified for the faker - real API uses JWT
	connID := uuid.New().String()
	token := fmt.Sprintf("%s:%s", apiKey, connID)

	// Build WebSocket URL based on request
	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}
	baseURL := fmt.Sprintf("%s://%s/ws", scheme, r.Host)

	response := NegotiateResponse{
		WebsocketURLs: map[string]string{
			"orderflow": fmt.Sprintf("%s/orderflow?access_token=%s", baseURL, token),
			// Future hubs:
			// "classic":           fmt.Sprintf("%s/classic?access_token=%s", baseURL, token),
			// "state_gex":         fmt.Sprintf("%s/state_gex?access_token=%s", baseURL, token),
			// "state_greeks_zero": fmt.Sprintf("%s/state_greeks_zero?access_token=%s", baseURL, token),
			// "state_greeks_one":  fmt.Sprintf("%s/state_greeks_one?access_token=%s", baseURL, token),
		},
	}

	h.logger.Debug("negotiate successful",
		zap.String("connID", connID),
		zap.String("apiKey", maskAPIKey(apiKey)),
	)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode negotiate response", zap.Error(err))
	}
}

// maskAPIKey masks all but the first 4 characters of an API key for logging.
func maskAPIKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:4] + "****"
}
