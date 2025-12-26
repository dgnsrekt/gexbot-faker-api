package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const (
	primaryHistDomain  = "hist.gex.bot"
	fallbackHistDomain = "hist.gexbot.com"
)

// Client interface for testability
type Client interface {
	GetDownloadURL(ctx context.Context, ticker, pkg, category, date string) (string, error)
	DownloadFile(ctx context.Context, url string, dest io.Writer) (int64, error)
}

type HTTPClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	limiter    *rate.Limiter
	retryCount int
	retryDelay time.Duration
	logger     *zap.Logger
}

type HistoryResponse struct {
	URL string `json:"url"`
}

func NewClient(baseURL, apiKey string, ratePerSec int, timeout, retryDelay time.Duration, retryCount int, logger *zap.Logger) *HTTPClient {
	transport := &http.Transport{
		MaxIdleConns:       100,
		MaxConnsPerHost:    10,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: false,
	}

	return &HTTPClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		baseURL:    baseURL,
		apiKey:     apiKey,
		limiter:    rate.NewLimiter(rate.Limit(ratePerSec), ratePerSec*2),
		retryCount: retryCount,
		retryDelay: retryDelay,
		logger:     logger,
	}
}

func (c *HTTPClient) GetDownloadURL(ctx context.Context, ticker, pkg, category, date string) (string, error) {
	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limiter: %w", err)
	}

	url := fmt.Sprintf("%s/v2/hist/%s/%s/%s/%s?noredirect", c.baseURL, ticker, pkg, category, date)
	c.logger.Debug("requesting", zap.String("url", url))

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * time.Duration(1<<(attempt-1)) // Exponential backoff
			c.logger.Debug("retrying request", zap.Int("attempt", attempt), zap.Duration("delay", delay))

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Authorization", "Basic "+c.apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Read body before closing for error messages
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			return "", ErrNotFound
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = ErrRateLimited
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
		}

		var histResp HistoryResponse
		if err := json.Unmarshal(body, &histResp); err != nil {
			return "", fmt.Errorf("decoding response: %w", err)
		}

		return histResp.URL, nil
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *HTTPClient) DownloadFile(ctx context.Context, url string, dest io.Writer) (int64, error) {
	size, err := c.downloadFileOnce(ctx, url, dest)
	if err == nil {
		return size, nil
	}

	// Check if fallback is applicable
	if !strings.Contains(url, primaryHistDomain) {
		return 0, err
	}

	// Try fallback domain
	fallbackURL := strings.Replace(url, primaryHistDomain, fallbackHistDomain, 1)
	c.logger.Info("retrying with fallback domain",
		zap.String("original", url),
		zap.String("fallback", fallbackURL),
		zap.Error(err))

	return c.downloadFileOnce(ctx, fallbackURL, dest)
}

func (c *HTTPClient) downloadFileOnce(ctx context.Context, url string, dest io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Stream to destination
	return io.Copy(dest, resp.Body)
}
