package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestDownloadEODReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/hist/eod/SPY" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("unexpected auth %s", got)
		}
		_, _ = w.Write([]byte("zip"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", 10, 30*time.Second, time.Millisecond, 0, zap.NewNop())
	var out bytes.Buffer
	if _, err := client.DownloadEODReport(context.Background(), "SPY", &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "zip" {
		t.Fatalf("unexpected body %q", out.String())
	}
}

func TestGetDownloadURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth != "Basic test-key" {
			t.Errorf("expected Basic test-key, got %s", auth)
		}

		// Verify path
		expectedPath := "/v2/hist/SPX/state/gex_full/2025-11-14"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify noredirect param exists (it's a flag, not a key-value pair)
		if !r.URL.Query().Has("noredirect") {
			t.Error("expected noredirect query param")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(HistoryResponse{URL: "https://storage.example.com/file.json"})
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	client := NewClient(server.URL, "test-key", 10, 30*time.Second, 1*time.Second, 3, logger)

	url, err := client.GetDownloadURL(context.Background(), "SPX", "state", "gex_full", "2025-11-14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url != "https://storage.example.com/file.json" {
		t.Errorf("unexpected URL: %s", url)
	}
}

func TestGetDownloadURL_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	client := NewClient(server.URL, "test-key", 10, 30*time.Second, 1*time.Second, 0, logger)

	_, err := client.GetDownloadURL(context.Background(), "SPX", "state", "gex_full", "2025-11-14")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetDownloadURL_RateLimited(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	client := NewClient(server.URL, "test-key", 10, 30*time.Second, 10*time.Millisecond, 2, logger)

	_, err := client.GetDownloadURL(context.Background(), "SPX", "state", "gex_full", "2025-11-14")
	if err == nil {
		t.Error("expected error for rate limiting")
	}

	// Should have attempted 3 times (initial + 2 retries)
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}
