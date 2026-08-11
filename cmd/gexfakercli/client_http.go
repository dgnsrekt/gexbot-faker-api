package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// apiClient is a thin JSON HTTP client over the faker. It is deliberately
// untyped: responses are passed through as raw JSON so new/changed server fields
// reach the agent without a client rebuild. The generated models in
// internal/api/generated document the shapes (surfaced via `describe`).
type apiClient struct {
	base         string
	key          string
	controlToken string
	http         *http.Client
}

func newClient() *apiClient {
	return &apiClient{
		base:         strings.TrimRight(flagURL, "/"),
		key:          flagKey,
		controlToken: flagToken,
		http:         &http.Client{Timeout: 30 * time.Second},
	}
}

// apiError is a structured, agent-actionable failure. It marshals to a compact
// JSON object on stderr and carries a hint for the well-known faker error shapes.
type apiError struct {
	Msg    string `json:"error"`
	Status int    `json:"status,omitempty"`
	URL    string `json:"url,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

func (e *apiError) Error() string { return e.Msg }

// get issues a GET. When auth is true the Authorization header is sent (data
// routes require it; discovery/control routes do not — sending it there is
// harmless but we keep it off to mirror real-client behavior).
func (c *apiClient) get(ctx context.Context, path string, auth bool, q url.Values) (json.RawMessage, error) {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, &apiError{Msg: err.Error(), URL: u}
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	return c.do(req)
}

// postJSON issues a POST with a JSON body and no Studio-token auth. Mutating
// control routes that may require the token (load/reset on a gated faker) use
// postControlJSON instead; per-client /seek stays open, so it uses postJSON.
func (c *apiClient) postJSON(ctx context.Context, path string, auth bool, body any) (json.RawMessage, error) {
	u := c.base + path
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, &apiError{Msg: err.Error(), URL: u}
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, buf)
	if err != nil {
		return nil, &apiError{Msg: err.Error(), URL: u}
	}
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	return c.do(req)
}

// postControlJSON issues a POST to a mutating control route (load, reset).
// It presents the control token as Bearer when one is configured so the CLI works
// against a token-gated faker (Part B); with no token it sends no auth header, matching an open
// (local dev) faker.
func (c *apiClient) postControlJSON(ctx context.Context, path string, body any) (json.RawMessage, error) {
	u := c.base + path
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, &apiError{Msg: err.Error(), URL: u}
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, buf)
	if err != nil {
		return nil, &apiError{Msg: err.Error(), URL: u}
	}
	req.Header.Set("Content-Type", "application/json")
	if c.controlToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.controlToken)
	}
	return c.do(req)
}

func (c *apiClient) do(req *http.Request) (json.RawMessage, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		hint := ""
		if isConnRefused(err) {
			hint = "no faker reachable at this URL — run `gexfakercli setup` or start the server"
		}
		return nil, &apiError{Msg: err.Error(), URL: req.URL.String(), Hint: hint}
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpError(req, resp.StatusCode, raw)
	}
	return json.RawMessage(raw), nil
}

// httpError converts a non-2xx response into a structured apiError, extracting
// the server's {"error": ...} message and attaching a hint for known cases.
func httpError(req *http.Request, status int, body []byte) *apiError {
	msg := strings.TrimSpace(string(body))
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		msg = e.Error
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	ae := &apiError{Msg: msg, Status: status, URL: req.URL.String()}
	switch {
	case status == http.StatusBadRequest && strings.Contains(msg, "Authorization"):
		ae.Hint = "data routes need a key — pass --key or set GEXFAKER_KEY (any non-empty token works)"
	case status == http.StatusNotFound && strings.Contains(msg, "No more data"):
		ae.Hint = "this key's cursor reached the end (cache_mode=exhaust) — run `gexfakercli reset` to replay"
	case status == http.StatusConflict:
		ae.Hint = "a reload is already in progress — retry shortly"
	case status == http.StatusUnauthorized:
		ae.Hint = "this control route requires the faker's Studio auth token — pass --token or set GEXFAKER_TOKEN"
	}
	return ae
}

func isConnRefused(err error) bool {
	return err != nil && strings.Contains(err.Error(), "connection refused")
}

// ---- output ----------------------------------------------------------------

// emit prints a raw JSON response to stdout, applying the --fields projection and
// --pretty formatting. Non-object responses (arrays, scalars) are printed as-is.
func emit(raw json.RawMessage) error {
	out := raw
	if flagFields != "" {
		p, err := projectFields(raw, splitCSV(flagFields))
		if err == nil {
			out = p
		}
	}
	return writeJSON(os.Stdout, out)
}

// emitValue marshals an arbitrary Go value and emits it (used for locally
// assembled objects like merged status and describe).
func emitValue(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return &apiError{Msg: err.Error()}
	}
	return emit(b)
}

func writeJSON(w io.Writer, raw json.RawMessage) error {
	if flagPretty {
		var buf bytes.Buffer
		if err := json.Indent(&buf, raw, "", "  "); err == nil {
			buf.WriteByte('\n')
			_, err = w.Write(buf.Bytes())
			return err
		}
	}
	_, err := fmt.Fprintln(w, string(bytes.TrimSpace(raw)))
	return err
}

// projectFields keeps only the requested top-level keys, preserving the order
// the caller listed them. If raw is not a JSON object it is returned unchanged.
func projectFields(raw json.RawMessage, fields []string) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw, err // not an object — leave as-is
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	n := 0
	for _, f := range fields {
		v, ok := obj[f]
		if !ok {
			continue
		}
		if n > 0 {
			buf.WriteByte(',')
		}
		key, _ := json.Marshal(f)
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(v)
		n++
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// fail writes a structured JSON error to stderr and returns err so cobra exits
// nonzero. Any error is normalized into an apiError first.
func fail(err error) error {
	var ae *apiError
	if !errors.As(err, &ae) {
		ae = &apiError{Msg: err.Error()}
	}
	b, _ := json.Marshal(ae)
	fmt.Fprintln(os.Stderr, string(b))
	return err
}

// progress writes a JSON progress line to stderr (suppressed by --quiet) so
// stdout stays a single clean JSON document for the agent to parse.
func progress(step, msg string, kv ...any) {
	if flagQuiet {
		return
	}
	obj := map[string]any{"step": step, "msg": msg}
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok {
			obj[k] = kv[i+1]
		}
	}
	b, _ := json.Marshal(obj)
	fmt.Fprintln(os.Stderr, string(b))
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
