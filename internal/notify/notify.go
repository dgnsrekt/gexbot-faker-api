package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/download"
)

// Notifier is the interface for sending download notifications.
type Notifier interface {
	SendSuccess(ctx context.Context, result *download.BatchResult, date string, duration time.Duration) error
	SendFailure(ctx context.Context, result *download.BatchResult, date string, duration time.Duration, err error) error
}

// Client implements the ntfy notification client.
type Client struct {
	httpClient *http.Client
	config     *Config
	logger     *zap.Logger
}

// NewClient creates a new ntfy client.
func NewClient(cfg *Config, logger *zap.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: cfg,
		logger: logger,
	}
}

// SendSuccess sends a success notification.
func (c *Client) SendSuccess(ctx context.Context, result *download.BatchResult, date string, duration time.Duration) error {
	if !c.config.Enabled {
		return nil
	}

	title := fmt.Sprintf("Download Complete: %s", date)
	message := FormatSuccessMessage(result, duration)
	tags := c.config.Tags + ",white_check_mark"

	return c.send(ctx, title, message, tags, c.config.Priority)
}

// SendFailure sends a failure notification.
func (c *Client) SendFailure(ctx context.Context, result *download.BatchResult, date string, duration time.Duration, err error) error {
	if !c.config.Enabled {
		return nil
	}

	title := fmt.Sprintf("Download Failed: %s", date)
	message := FormatFailureMessage(result, duration, err)
	tags := c.config.Tags + ",x"
	priority := "high" // Override to high priority for failures

	return c.send(ctx, title, message, tags, priority)
}

func (c *Client) send(ctx context.Context, title, message, tags, priority string) error {
	url := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.config.Server, "/"), c.config.Topic)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Title", title)
	req.Header.Set("Priority", priority)
	req.Header.Set("Tags", tags)

	if c.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("failed to send notification", zap.Error(err))
		return fmt.Errorf("sending notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Drain response body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Warn("notification failed",
			zap.Int("status", resp.StatusCode),
			zap.String("url", url),
		)
		return fmt.Errorf("notification failed with status: %d", resp.StatusCode)
	}

	c.logger.Debug("notification sent", zap.String("title", title))
	return nil
}

// NoopNotifier is a no-op implementation for when notifications are disabled.
type NoopNotifier struct{}

// SendSuccess is a no-op.
func (n *NoopNotifier) SendSuccess(_ context.Context, _ *download.BatchResult, _ string, _ time.Duration) error {
	return nil
}

// SendFailure is a no-op.
func (n *NoopNotifier) SendFailure(_ context.Context, _ *download.BatchResult, _ string, _ time.Duration, _ error) error {
	return nil
}

// New creates the appropriate notifier based on config.
func New(cfg *Config, logger *zap.Logger) Notifier {
	if !cfg.Enabled {
		return &NoopNotifier{}
	}
	return NewClient(cfg, logger)
}
