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

// DownloadEODReport downloads the latest completed EOD ZIP for a ticker.
func (c *HTTPClient) DownloadEODReport(ctx context.Context, ticker string, dest io.Writer) (int64, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return 0, fmt.Errorf("rate limiter: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v2/hist/eod/%s", c.baseURL, ticker), nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/zip")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return 0, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return io.Copy(dest, resp.Body)
}

func (c *HTTPClient) DownloadFile(ctx context.Context, url string, dest io.Writer) (int64, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * time.Duration(1<<(attempt-1))
			c.logger.Debug("retrying download", zap.Int("attempt", attempt), zap.Duration("delay", delay))

			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(delay):
			}
		}

		size, err := c.downloadFileOnce(ctx, url, dest)
		if err == nil {
			return size, nil
		}

		// Don't retry if data was partially written — would corrupt the output
		if size > 0 {
			return size, err
		}

		lastErr = err
		c.logger.Warn("download attempt failed",
			zap.Int("attempt", attempt+1),
			zap.Int("max_attempts", c.retryCount+1),
			zap.Error(err))
	}

	// All retries exhausted — try fallback domain if applicable
	if strings.Contains(url, primaryHistDomain) {
		fallbackURL := strings.Replace(url, primaryHistDomain, fallbackHistDomain, 1)
		c.logger.Info("retrying with fallback domain",
			zap.String("original", url),
			zap.String("fallback", fallbackURL),
			zap.Error(lastErr))

		return c.downloadFileOnce(ctx, fallbackURL, dest)
	}

	return 0, fmt.Errorf("download failed after %d attempts: %w", c.retryCount+1, lastErr)
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
