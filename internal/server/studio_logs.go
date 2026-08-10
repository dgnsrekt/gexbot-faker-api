package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// logsBaseSelector picks the faker containers Alloy ships to Loki (labeled
// service=gex-faker-api / gex-daemon via observability.service).
const logsBaseSelector = `{service=~"gex-.*"}`

// errorLevelMatch matches error/warn zap lines in a raw log line, for the
// volume series (level is a JSON field, not a Loki label).
const errorLevelMatch = `"level":"(error|warn|fatal|panic)"`

const (
	logsPollInterval = 1500 * time.Millisecond
	logsBackfillMax  = 500
	logsPollMax      = 1000
)

// logRanges maps the UI's range chips to durations.
var logRanges = map[string]time.Duration{
	"5m": 5 * time.Minute,
	"1h": time.Hour,
	"6h": 6 * time.Hour,
}

func parseLogRange(s string) time.Duration {
	if d, ok := logRanges[s]; ok {
		return d
	}
	return 5 * time.Minute
}

// logLine is one entry streamed to the Studio Logs screen. Fields carries every
// structured zap field beyond level/ts/msg/caller so the UI can render them.
type logLine struct {
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Service string         `json:"service"`
	Msg     string         `json:"msg"`
	Caller  string         `json:"caller,omitempty"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// buildLogQuery assembles the LogQL query. service scopes the label selector
// (server-side, so a sparse service like the daemon isn't crowded out of the
// backfill by the chatty api); hideAccess drops the "request completed" access
// logs; search adds a case-insensitive substring filter. All filtering that
// changes *which* lines Loki returns lives here so the backfill window is spent
// on lines the user actually wants.
func buildLogQuery(service, search string, hideAccess bool) string {
	q := logsBaseSelector
	switch service {
	case "gex-faker-api":
		q = `{service="gex-faker-api"}`
	case "gex-daemon":
		q = `{service="gex-daemon"}`
	}
	if hideAccess {
		q += ` != "request completed"`
	}
	if s := strings.TrimSpace(search); s != "" {
		if len(s) > 200 {
			s = s[:200]
		}
		q += " |~ " + strconv.Quote("(?i)"+regexp.QuoteMeta(s))
	}
	return q
}

// handleLogs streams faker logs to the browser as SSE. The server queries Loki on
// the internal compose network (LOKI_URL) and forwards entries, so the browser
// never talks to Loki directly. Query params: range=5m|1h|6h, search=<text>.
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

	qp := r.URL.Query()
	query := buildLogQuery(qp.Get("service"), qp.Get("search"), qp.Get("hide_access") == "1")
	window := parseLogRange(qp.Get("range"))

	ctx := r.Context()
	client := &lokiClient{base: base, hc: &http.Client{Timeout: 8 * time.Second}}

	// Backfill the most recent lines in the window (direction=backward), shown
	// oldest→newest, then tail forward from the last-seen timestamp.
	nowNs := time.Now().UnixNano()
	lines, lastNs, err := client.queryRange(ctx, query, nowNs-int64(window), nowNs, logsBackfillMax, "backward")
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
			fresh, newLast, err := client.queryRange(ctx, query, lastNs+1, end, logsPollMax, "forward")
			if err != nil {
				continue
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

// handleLogsVolume returns an error/warn-per-minute series over the range, for
// the Logs screen's volume sparkline.
func (h *StudioHandlers) handleLogsVolume(w http.ResponseWriter, r *http.Request) {
	base := h.server.config.LokiURL
	if base == "" {
		writeStudioJSON(w, map[string]any{"points": []any{}})
		return
	}
	window := parseLogRange(r.URL.Query().Get("range"))
	client := &lokiClient{base: base, hc: &http.Client{Timeout: 8 * time.Second}}
	metric := `sum(count_over_time(` + logsBaseSelector + ` |~ ` + strconv.Quote(errorLevelMatch) + `[1m]))`
	points, err := client.metricRange(r.Context(), metric, window, time.Minute)
	if err != nil {
		writeStudioJSON(w, map[string]any{"points": []any{}})
		return
	}
	writeStudioJSON(w, map[string]any{"points": points})
}

// lokiClient does range queries against Loki's HTTP API.
type lokiClient struct {
	base string
	hc   *http.Client
}

// queryRange returns log lines in [startNs, endNs] ascending (regardless of the
// Loki fetch direction), plus the max Loki timestamp seen (0 if none). direction
// selects which limit lines Loki returns: "backward" = most recent, "forward" =
// earliest.
func (c *lokiClient) queryRange(ctx context.Context, query string, startNs, endNs int64, limit int, direction string) ([]logLine, int64, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("start", strconv.FormatInt(startNs, 10))
	q.Set("end", strconv.FormatInt(endNs, 10))
	q.Set("limit", strconv.Itoa(limit))
	q.Set("direction", direction)

	var payload struct {
		Data struct {
			Result []struct {
				Stream map[string]string `json:"stream"`
				Values [][2]string       `json:"values"` // [ns, line]
			} `json:"result"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/loki/api/v1/query_range?"+q.Encode(), &payload); err != nil {
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
	sort.Slice(all, func(i, j int) bool { return all[i].ns < all[j].ns })
	out := make([]logLine, 0, len(all))
	for _, t := range all {
		out = append(out, t.line)
	}
	return out, maxNs, nil
}

// volumePoint is one bucket of the volume sparkline.
type volumePoint struct {
	T int64   `json:"t"` // unix seconds
	V float64 `json:"v"` // count in the bucket
}

// metricRange runs a Loki metric query over [now-window, now] at the given step.
func (c *lokiClient) metricRange(ctx context.Context, metric string, window, step time.Duration) ([]volumePoint, error) {
	now := time.Now()
	q := url.Values{}
	q.Set("query", metric)
	q.Set("start", strconv.FormatInt(now.Add(-window).UnixNano(), 10))
	q.Set("end", strconv.FormatInt(now.UnixNano(), 10))
	q.Set("step", strconv.Itoa(int(step.Seconds())))

	var payload struct {
		Data struct {
			Result []struct {
				Values [][2]any `json:"values"` // [unixSec(float), "count"]
			} `json:"result"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/loki/api/v1/query_range?"+q.Encode(), &payload); err != nil {
		return nil, err
	}
	points := []volumePoint{}
	if len(payload.Data.Result) > 0 {
		for _, v := range payload.Data.Result[0].Values {
			t, _ := v[0].(float64)
			var val float64
			if s, ok := v[1].(string); ok {
				val, _ = strconv.ParseFloat(s, 64)
			}
			points = append(points, volumePoint{T: int64(t), V: val})
		}
	}
	return points, nil
}

func (c *lokiClient) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loki status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// parseLogLine turns a container stdout line (zap JSON) into a display entry with
// all structured fields preserved, falling back to raw text when unstructured.
func parseLogLine(raw, service string, ns int64) logLine {
	entry := logLine{Service: service, Level: "info", Msg: raw}
	var o map[string]any
	if err := json.Unmarshal([]byte(raw), &o); err == nil {
		if lvl, ok := o["level"].(string); ok && lvl != "" {
			entry.Level = lvl
		}
		if msg, ok := o["msg"].(string); ok && msg != "" {
			entry.Msg = msg
		}
		if c, ok := o["caller"].(string); ok {
			entry.Caller = c
		}
		fields := map[string]any{}
		for k, v := range o {
			switch k {
			case "level", "ts", "msg", "caller":
				continue
			}
			fields[k] = v
		}
		if len(fields) > 0 {
			entry.Fields = fields
		}
		if ts, ok := o["ts"].(float64); ok && ts > 0 {
			entry.Time = time.Unix(int64(ts), 0).UTC().Format("15:04:05")
			return entry
		}
	}
	entry.Time = time.Unix(0, ns).UTC().Format("15:04:05")
	return entry
}
