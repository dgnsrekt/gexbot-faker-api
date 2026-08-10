package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// logsQuery selects the faker containers Alloy ships to Loki (labeled
// service=gex-faker-api / gex-daemon via observability.service).
const logsQuery = `{service=~"gex-.*"}`

const (
	logsPollInterval = 1500 * time.Millisecond
	logsBackfill     = 5 * time.Minute
	logsBackfillMax  = 300
	logsPollMax      = 1000
)

// logLine is one entry streamed to the Studio Logs screen.
type logLine struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Service string `json:"service"`
	Msg     string `json:"msg"`
}

// handleLogs streams faker logs to the browser as SSE. The server queries Loki on
// the internal compose network (LOKI_URL) and forwards entries, so the browser
// never talks to Loki directly (no exposed port, no CORS). If Loki isn't
// configured or is unreachable, it emits a single error event the UI surfaces.
func (h *StudioHandlers) handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	sendEvent := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	base := h.server.config.LokiURL
	if base == "" {
		sendEvent(map[string]string{"error": "Logs need the observability stack. Loki isn't configured (LOKI_URL)."})
		return
	}

	ctx := r.Context()
	client := &lokiClient{base: base, hc: &http.Client{Timeout: 8 * time.Second}}

	// Backfill a recent window, then tail forward from the last-seen timestamp.
	nowNs := time.Now().UnixNano()
	lines, lastNs, err := client.queryRange(ctx, nowNs-int64(logsBackfill), nowNs, logsBackfillMax)
	if err != nil {
		sendEvent(map[string]string{"error": "Loki is unreachable. Is the observability stack running?"})
		return
	}
	for _, l := range lines {
		sendEvent(l)
	}
	if lastNs == 0 {
		lastNs = nowNs
	}

	ticker := time.NewTicker(logsPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			end := time.Now().UnixNano()
			fresh, newLast, err := client.queryRange(ctx, lastNs+1, end, logsPollMax)
			if err != nil {
				continue // transient; keep tailing
			}
			for _, l := range fresh {
				sendEvent(l)
			}
			if newLast > lastNs {
				lastNs = newLast
			}
		}
	}
}

// lokiClient does forward range queries against Loki's HTTP API.
type lokiClient struct {
	base string
	hc   *http.Client
}

// queryRange returns log lines in [startNs, endNs] ascending, and the max Loki
// timestamp seen (0 if none).
func (c *lokiClient) queryRange(ctx context.Context, startNs, endNs int64, limit int) ([]logLine, int64, error) {
	q := url.Values{}
	q.Set("query", logsQuery)
	q.Set("start", strconv.FormatInt(startNs, 10))
	q.Set("end", strconv.FormatInt(endNs, 10))
	q.Set("limit", strconv.Itoa(limit))
	q.Set("direction", "forward")
	u := c.base + "/loki/api/v1/query_range?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("loki status %d", resp.StatusCode)
	}

	var payload struct {
		Data struct {
			Result []struct {
				Stream map[string]string `json:"stream"`
				Values [][2]string       `json:"values"` // [ns, line]
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, err
	}

	type tsLine struct {
		ns   int64
		line logLine
	}
	var all []tsLine
	var maxNs int64
	for _, stream := range payload.Data.Result {
		service := stream.Stream["service"]
		for _, v := range stream.Values {
			ns, _ := strconv.ParseInt(v[0], 10, 64)
			if ns > maxNs {
				maxNs = ns
			}
			all = append(all, tsLine{ns: ns, line: parseLogLine(v[1], service, ns)})
		}
	}
	// Loki returns per-stream ordering; merge to a single ascending timeline.
	sort.Slice(all, func(i, j int) bool { return all[i].ns < all[j].ns })
	out := make([]logLine, 0, len(all))
	for _, t := range all {
		out = append(out, t.line)
	}
	return out, maxNs, nil
}

// parseLogLine turns a container stdout line (zap JSON) into a display entry,
// falling back to the raw text (level "info") when it isn't structured.
func parseLogLine(raw, service string, ns int64) logLine {
	entry := logLine{Service: service, Level: "info", Msg: raw}
	var zapEntry struct {
		Level string  `json:"level"`
		Ts    float64 `json:"ts"`
		Msg   string  `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &zapEntry); err == nil && zapEntry.Msg != "" {
		entry.Level = zapEntry.Level
		entry.Msg = zapEntry.Msg
		if zapEntry.Ts > 0 {
			entry.Time = time.Unix(int64(zapEntry.Ts), 0).UTC().Format("15:04:05")
			return entry
		}
	}
	entry.Time = time.Unix(0, ns).UTC().Format("15:04:05")
	return entry
}
