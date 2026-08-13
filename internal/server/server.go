package server

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/api"
	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
	"github.com/dgnsrekt/gexbot-downloader/internal/observability"
	"github.com/dgnsrekt/gexbot-downloader/internal/sync"
	"github.com/dgnsrekt/gexbot-downloader/internal/ws"
)

// WebSocketHubs holds all WebSocket hubs for routing.
type WebSocketHubs struct {
	Orderflow       *ws.Hub
	StateGex        *ws.Hub
	Classic         *ws.Hub
	StateGreeksZero *ws.Hub
	StateGreeksOne  *ws.Hub
}

func NewRouter(server *Server, wsHubs *WebSocketHubs, negotiateHandler *ws.NegotiateHandler, syncBroadcaster *sync.SyncBroadcaster, logger *zap.Logger) (http.Handler, error) {
	// Load OpenAPI spec for validation
	swagger, err := generated.GetSwagger()
	if err != nil {
		return nil, err
	}
	swagger.Servers = nil // Allow any host

	r := chi.NewRouter()

	// Global middleware (NO compression - applied selectively below)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)
	r.Use(zapLoggerMiddleware(logger))

	// Static assets - serve WITHOUT compression (compression corrupts large JS files)
	r.Get("/openapi.yaml", openapiHandler)
	r.Get("/docs", swaggerUIHandler)
	r.Get("/swagger-ui.js", swaggerUIBundleHandler)
	r.Get("/swagger-ui.css", swaggerUICSSHandler)

	// AsyncAPI documentation for WebSocket endpoints (renderer vendored/embedded so the
	// page works offline — see swagger-ui above)
	r.Get("/asyncapi.yaml", asyncapiHandler)
	r.Get("/asyncapi", asyncapiUIHandler)
	r.Get("/asyncapi-ui.js", asyncapiUIBundleHandler)
	r.Get("/asyncapi-ui.css", asyncapiUICSSHandler)

	// Brand favicon for the open doc pages (the Studio's copy sits behind auth at
	// /studio/favicon.svg, so serve an unauthenticated one here for /docs and /asyncapi).
	r.Get("/favicon.svg", faviconHandler)

	// Guides: the embedded Starlight docs site at /guides + the root
	// llms.txt/llms-full.txt agent files. Public (no secrets), so it sits outside
	// the Studio auth group and next to the other static docs handlers.
	if err := registerGuidesUI(r); err != nil {
		return nil, err
	}

	// GEX Faker Studio: embedded SPA at /studio + its control-plane JSON endpoints
	// at /studio/api/*. Registered before the API group so it skips the OpenAPI
	// auth/validation/compression (assets must not be compressed). Optionally gated
	// behind HTTP Basic auth (STUDIO_AUTH_TOKEN) since it exposes control endpoints
	// and container logs.
	var studioErr error
	r.Group(func(sr chi.Router) {
		sr.Use(studioAuthMiddleware(server.config.StudioAuthToken))
		studioErr = registerStudioUI(sr)
		RegisterStudioRoutes(sr, server, wsHubs)
	})
	if studioErr != nil {
		return nil, studioErr
	}

	// WebSocket routes (outside OpenAPI validation)
	if negotiateHandler != nil {
		r.Get("/negotiate", negotiateHandler.HandleNegotiate)
		r.Post("/negotiate", negotiateHandler.HandleNegotiatePost)
		r.Patch("/negotiate", negotiateHandler.HandleNegotiatePatch)
	}
	if wsHubs != nil {
		if wsHubs.Orderflow != nil {
			r.HandleFunc("/ws/orderflow", wsHubs.Orderflow.HandleOrderflowWS)
		}
		if wsHubs.StateGex != nil {
			r.HandleFunc("/ws/state_gex", wsHubs.StateGex.HandleOrderflowWS)
		}
		if wsHubs.Classic != nil {
			r.HandleFunc("/ws/classic", wsHubs.Classic.HandleOrderflowWS)
		}
		if wsHubs.StateGreeksZero != nil {
			r.HandleFunc("/ws/state_greeks_zero", wsHubs.StateGreeksZero.HandleOrderflowWS)
		}
		if wsHubs.StateGreeksOne != nil {
			r.HandleFunc("/ws/state_greeks_one", wsHubs.StateGreeksOne.HandleOrderflowWS)
		}
	}

	// Sync Broadcast System route (SSE stream, outside OpenAPI validation)
	if syncBroadcaster != nil {
		r.Get("/sync/stream", syncBroadcaster.HandleSSE)
	}

	// API routes with compression and OpenAPI validation
	r.Group(func(apiRouter chi.Router) {
		apiRouter.Use(middleware.Compress(5))
		// Auth runs before request validation so a missing Authorization header
		// fails with the real API's error before any path/param check.
		apiRouter.Use(authMiddleware)
		// Gate the state-changing faker control routes behind the Studio auth token
		// (no-op when the token is unset — local dev). Runs before validation so an
		// unauthorized mutating request is rejected before its body is parsed.
		apiRouter.Use(controlAuthMiddleware(server.config.StudioAuthToken))
		apiRouter.Use(oapimiddleware.OapiRequestValidator(swagger))

		strictHandler := generated.NewStrictHandler(server, nil)
		generated.HandlerFromMux(strictHandler, apiRouter)
	})

	return r, nil
}

type ctxKey string

// authKeyCtxKey holds the API key parsed from the Authorization header. It seeds
// each client's playback position (see data.SharedCacheKey / data.CacheKey).
const authKeyCtxKey ctxKey = "authKey"

// authMiddleware mirrors the real GexBot API: market-data routes authenticate via
// the Authorization header (Basic or Bearer). The parsed token is stashed in the
// request context for the handlers; an absent header on a data route is rejected
// with the exact upstream error body. This middleware leaves faker
// control/metadata routes open (no market-data header); the mutating control
// routes are separately gated by controlAuthMiddleware (STUDIO_AUTH_TOKEN).
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := parseAuthToken(r.Header.Get("Authorization"))
		if token == "" && requiresAuth(r.URL.Path) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Authorization header not found."}`))
			return
		}
		ctx := context.WithValue(r.Context(), authKeyCtxKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isMutatingControlPath reports whether path is a state-changing faker control route that Part B
// gates behind the Studio auth token. These mutate global server state (what data is loaded, cursor
// positions), so on a LAN they should require the token. Read-only discovery (/dates, /current-load,
// /coverage, /available/{date}, /tickers, /load/status, /health), the market-data routes (their own
// any-token auth), and per-client playback (/seek) stay open.
func isMutatingControlPath(path string) bool {
	switch strings.Trim(path, "/") {
	case "load", "reset":
		return true
	}
	return false
}

// controlAuthMiddleware gates the mutating control routes behind the Studio auth token. It reuses
// STUDIO_AUTH_TOKEN and the same Basic/Bearer check as the Studio UI (studioTokenOK), so a client
// (e.g. gexsync) presents one token for both. Empty token = pass-through (local dev), matching
// studioAuthMiddleware; everything except the mutating control routes is untouched.
func controlAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token != "" && isMutatingControlPath(r.URL.Path) && !studioTokenOK(r, token) {
				w.Header().Set("WWW-Authenticate", `Basic realm="GEX Faker control"`)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"control route requires the Studio auth token"}`))
				return
			}
			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}

// parseAuthToken extracts the credential from "Basic <token>" or "Bearer <token>"
// (the real API accepts both), tolerating a bare token.
func parseAuthToken(header string) string {
	if header == "" {
		return ""
	}
	for _, prefix := range []string{"Basic ", "Bearer "} {
		if strings.HasPrefix(header, prefix) {
			return strings.TrimSpace(header[len(prefix):])
		}
	}
	return strings.TrimSpace(header)
}

// requiresAuth reports whether path is a real-API market-data route. It is keyed
// off the route SHAPE, not a ticker regex: /{ticker}/{classic|state|orderflow}/...
// and /hist/... require auth for any ticker (including underscore/futures tickers
// like ES_SPX). Faker extensions — /tickers, /tickers/quant, /{package}/categories,
// /health, /download/..., control routes — need no market-data header (their 2nd
// segment is never classic/state/orderflow, and /download carries a date there).
// The mutating control routes are still gated by controlAuthMiddleware.
func requiresAuth(path string) bool {
	seg := strings.Split(strings.Trim(path, "/"), "/")
	if len(seg) == 0 {
		return false
	}
	// Historical (/hist/...), options (/options/...) and futures (/futures/...)
	// routes require auth upstream.
	if seg[0] == "hist" || seg[0] == "options" || seg[0] == "futures" {
		return true
	}
	if len(seg) >= 2 {
		switch seg[1] {
		case "classic", "state", "orderflow":
			return true
		}
	}
	return false
}

// authKeyFromContext returns the API key parsed by authMiddleware (empty if none).
func authKeyFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(authKeyCtxKey).(string); ok {
		return v
	}
	return ""
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func zapLoggerMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			observability.HTTPInFlight.Inc()
			defer observability.HTTPInFlight.Dec()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			// Record in a defer so a panicking handler is still counted and logged.
			// The outer Recoverer (registered before this middleware) writes the 500
			// only after this frame unwinds, so the wrapped writer has no status yet
			// at defer time: treat an in-flight panic as 500, then re-panic so
			// Recoverer still produces the response. Without this, panic-generated
			// 5xx are absent from both request metrics and the access log.
			defer func() {
				panicked := recover()
				route := chi.RouteContext(r.Context()).RoutePattern()
				if route == "" {
					route = "unmatched"
				}
				status := ww.Status()
				if panicked != nil {
					status = http.StatusInternalServerError
				} else if status == 0 {
					status = http.StatusOK
				}
				statusClass := strconv.Itoa(status/100) + "xx"
				duration := time.Since(started)
				observability.HTTPRequests.WithLabelValues(r.Method, route, statusClass).Inc()
				observability.HTTPRequestDuration.WithLabelValues(r.Method, route).Observe(duration.Seconds())
				observability.HTTPResponseBytes.WithLabelValues(route).Add(float64(ww.BytesWritten()))
				logger.Info("request completed",
					zap.String("request_id", middleware.GetReqID(r.Context())),
					zap.String("method", r.Method),
					zap.String("route", route),
					zap.Int("status", status),
					zap.Duration("duration", duration),
					zap.Int("response_bytes", ww.BytesWritten()),
				)
				if panicked != nil {
					panic(panicked) // let the outer Recoverer write the 500
				}
			}()
			next.ServeHTTP(ww, r)
		})
	}
}

// maskQueryKey masks the "key" parameter in a query string
func maskQueryKey(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	if key := values.Get("key"); key != "" {
		values.Set("key", "[REDACTED]")
	}
	// Rebuild query string preserving order as much as possible
	var parts []string
	for k, vs := range values {
		for _, v := range vs {
			parts = append(parts, k+"="+v)
		}
	}
	return strings.Join(parts, "&")
}

func openapiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(api.OpenAPISpec)
}

// faviconSVG is the "Reflection" brand mark (matches web/public/favicon.svg), served at
// the open /favicon.svg for the doc pages.
const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96"><rect width="96" height="96" rx="20" fill="#0d0e10"/><path d="M14 42 L34 18 L54 36 L74 12 L84 12" fill="none" stroke="#e7e8ea" stroke-width="6" stroke-linecap="round" stroke-linejoin="round"/><rect x="10" y="46.6" width="76" height="2.8" rx="1.4" fill="#3a3e46"/><path d="M14 54 L34 78 L54 60 L74 84 L84 84" fill="none" stroke="#6fcf97" stroke-width="6" stroke-linecap="round" stroke-linejoin="round" stroke-dasharray="9 6"/></svg>`

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(faviconSVG))
}

func swaggerUIBundleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	_, _ = w.Write(api.SwaggerUIBundle)
}

func swaggerUICSSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	_, _ = w.Write(api.SwaggerUICSS)
}

func asyncapiUIBundleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	_, _ = w.Write(api.AsyncAPIUIBundle)
}

func asyncapiUICSSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	_, _ = w.Write(api.AsyncAPIUICSS)
}

// docsFontLinks loads the Studio's typefaces (Space Grotesk wordmark + JetBrains Mono)
// so the API docs match the GEX Faker Studio shell. No CSP is applied, so external font
// loads are unrestricted.
const docsFontLinks = `<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;700&family=Space+Grotesk:wght@400;500;700&display=swap" rel="stylesheet">`

// docsChromeCSS is the shared Studio palette + top-nav/brand styling used by both the
// Swagger UI and AsyncAPI pages. The palette tokens mirror web/src/app.css verbatim.
const docsChromeCSS = `:root{color-scheme:dark;--bg:#0d0e10;--panel:#111214;--panel-2:#141619;--input:#191b1f;--border:#212429;--border-2:#272a30;--text:#e7e8ea;--text-2:#d6d8db;--muted:#8b8f98;--muted-2:#7b808a;--green:#6fcf97;--green-2:#7fd1a8;--green-ink:#0d1a12;--green-bg:#16211b;--green-border:#2c4335;--amber:#e0b164;--blue:#7aa7e0;--red:#e08080;--mono:'JetBrains Mono',ui-monospace,SFMono-Regular,Menlo,monospace;--wordmark:'Space Grotesk','Helvetica Neue',Helvetica,Arial,sans-serif;--sans:'Helvetica Neue',Helvetica,Arial,sans-serif}
body{margin:0;background:var(--bg);color:var(--text);font-family:var(--sans)}
.docs-nav{position:sticky;top:0;z-index:20;height:46px;display:flex;align-items:center;gap:14px;padding:0 16px;background:var(--panel);border-bottom:1px solid var(--border)}
.docs-brand{display:flex;align-items:center;gap:9px}
.docs-brand svg{width:20px;height:20px;flex:none}
.docs-wordmark{font-family:var(--wordmark);font-size:15px;font-weight:400;letter-spacing:-0.01em;color:var(--text);white-space:nowrap}
.docs-wordmark b{font-weight:700}
.docs-wordmark .studio{color:var(--muted)}
.docs-sep{width:1px;height:20px;background:var(--border-2)}
.docs-links{display:flex;align-items:center;gap:6px}
.docs-links a,.docs-links .current{font-size:13px;text-decoration:none;padding:5px 10px;border-radius:6px;line-height:1}
.docs-links a{color:var(--muted)}
.docs-links a:hover{color:var(--text);background:#1c1f24}
.docs-links .current{color:#b7e6cd;background:var(--green-bg);font-weight:600}
.docs-spec{margin-left:auto;font-family:var(--mono);font-size:12px;color:var(--muted-2);text-decoration:none}
.docs-spec:hover{color:var(--green-2)}`

// docsNavBar renders the shared top bar: the Reflection brand mark + wordmark, the
// REST/WebSocket links (active one as a green pill), and a raw-spec link on the right.
// active is "rest" or "ws".
func docsNavBar(active string) string {
	mark := `<svg viewBox="0 0 96 96" aria-hidden="true" focusable="false"><path d="M14 42 L34 18 L54 36 L74 12 L84 12" fill="none" stroke="#e7e8ea" stroke-width="6" stroke-linecap="round" stroke-linejoin="round"/><rect x="10" y="46.6" width="76" height="2.8" rx="1.4" fill="#7b808a"/><path d="M14 54 L34 78 L54 60 L74 84 L84 84" fill="none" stroke="#6fcf97" stroke-width="6" stroke-linecap="round" stroke-linejoin="round" stroke-dasharray="9 6"/></svg>`
	rest := `<a href="/docs">REST API Docs</a>`
	ws := `<a href="/asyncapi">WebSocket Docs</a>`
	spec := `<a class="docs-spec" href="/openapi.yaml">/openapi.yaml</a>`
	if active == "ws" {
		ws = `<span class="current">WebSocket Docs</span>`
		spec = `<a class="docs-spec" href="/asyncapi.yaml">/asyncapi.yaml</a>`
	} else {
		rest = `<span class="current">REST API Docs</span>`
	}
	return `<div class="docs-nav"><span class="docs-brand">` + mark +
		`<span class="docs-wordmark">GEX <b>Faker</b> <span class="studio">Studio</span></span></span>` +
		`<span class="docs-sep"></span><span class="docs-links">` + rest + ws + `</span>` + spec + `</div>`
}

// swaggerThemeCSS restyles Swagger UI 3.x to the Studio look: neutral operation panels
// with Studio-token method chips, Space Grotesk titles/tags, JetBrains Mono paths/code,
// green accents and Execute button.
const swaggerThemeCSS = `.swagger-ui{background:var(--bg);color:var(--text);font-family:var(--sans)}
.swagger-ui .wrapper{max-width:1100px}
.swagger-ui .info{margin:28px 0}
.swagger-ui .info .title{font-family:var(--wordmark);color:var(--text);font-weight:700;letter-spacing:-0.01em}
.swagger-ui .info .title small{background:var(--green-bg);border-radius:6px}
.swagger-ui .info .title small pre{color:var(--green);background:transparent;font-family:var(--mono)}
.swagger-ui .info p,.swagger-ui .info li,.swagger-ui .info table td{color:var(--text-2)}
.swagger-ui .info a,.swagger-ui a.nostyle,.swagger-ui .info .base-url a{color:var(--green-2)}
.swagger-ui .info .base-url{color:var(--muted);font-family:var(--mono)}
.swagger-ui .scheme-container{background:var(--panel);box-shadow:none;border:1px solid var(--border);border-radius:8px;margin:0 0 24px}
.swagger-ui .scheme-container .schemes>label,.swagger-ui .servers>label{color:var(--text)}
.swagger-ui .opblock-tag{color:var(--text);font-family:var(--wordmark);border-bottom:1px solid var(--border)}
.swagger-ui .opblock-tag:hover{background:rgba(255,255,255,.02)}
.swagger-ui .opblock-tag small{color:var(--muted);font-family:var(--sans)}
.swagger-ui .opblock{background:var(--panel);border:1px solid var(--border);border-radius:8px;box-shadow:none;margin:0 0 10px}
.swagger-ui .opblock .opblock-summary{border-color:var(--border)}
.swagger-ui .opblock .opblock-summary-path,.swagger-ui .opblock .opblock-summary-path a,.swagger-ui .opblock .opblock-summary-path__deprecated{color:var(--text);font-family:var(--mono)}
.swagger-ui .opblock .opblock-summary-description{color:var(--muted)}
.swagger-ui .opblock .opblock-summary-method{border-radius:6px;font-family:var(--mono);font-weight:600;text-shadow:none;min-width:74px}
.swagger-ui .opblock.opblock-get .opblock-summary-method{background:var(--green-bg);color:var(--green)}
.swagger-ui .opblock.opblock-post .opblock-summary-method{background:#241f16;color:var(--amber)}
.swagger-ui .opblock.opblock-put .opblock-summary-method,.swagger-ui .opblock.opblock-patch .opblock-summary-method{background:#161c24;color:var(--blue)}
.swagger-ui .opblock.opblock-delete .opblock-summary-method{background:#241617;color:var(--red)}
.swagger-ui .opblock.opblock-head .opblock-summary-method,.swagger-ui .opblock.opblock-options .opblock-summary-method{background:var(--input);color:var(--muted)}
.swagger-ui .opblock.opblock-get{border-color:var(--green-border)}
.swagger-ui .opblock.opblock-post{border-color:#3a3320}
.swagger-ui .opblock.opblock-delete{border-color:#3a2020}
.swagger-ui .opblock.opblock-put,.swagger-ui .opblock.opblock-patch{border-color:#20293a}
.swagger-ui .opblock .opblock-section-header{background:var(--panel-2);border-color:var(--border);box-shadow:none}
.swagger-ui .opblock .opblock-section-header h4,.swagger-ui .opblock .opblock-section-header label,.swagger-ui .opblock-description-wrapper p,.swagger-ui .opblock-external-docs-wrapper p,.swagger-ui .parameter__type,.swagger-ui table thead tr td,.swagger-ui table thead tr th,.swagger-ui .response-col_status,.swagger-ui .response-col_description,.swagger-ui .responses-inner h4,.swagger-ui .responses-inner h5,.swagger-ui label{color:var(--text-2)}
.swagger-ui .parameter__name{color:var(--text)}
.swagger-ui .parameter__in,.swagger-ui .prop-format{color:var(--muted)}
.swagger-ui table tbody tr td{border-color:var(--border);color:var(--text-2)}
.swagger-ui .response-col_status{color:var(--text)}
.swagger-ui section.models{background:var(--panel);border:1px solid var(--border)}
.swagger-ui section.models.is-open h4{border-color:var(--border)}
.swagger-ui section.models .model-container,.swagger-ui .model-box{background:var(--panel-2);border-radius:6px}
.swagger-ui section.models h4 span,.swagger-ui .model-title,.swagger-ui .model,.swagger-ui .prop-type{color:var(--text-2)}
.swagger-ui .model-toggle:after{filter:invert(.9)}
.swagger-ui svg.arrow,.swagger-ui .expand-operation svg,.swagger-ui .expand-methods svg{fill:var(--muted)}
.swagger-ui input[type=text],.swagger-ui input[type=email],.swagger-ui input[type=password],.swagger-ui select,.swagger-ui textarea{background:var(--input);color:var(--text);border:1px solid var(--border)}
.swagger-ui .btn{color:var(--text);border-color:var(--border);box-shadow:none}
.swagger-ui .btn.execute{background:var(--green);color:var(--green-ink);border-color:var(--green)}
.swagger-ui .btn.cancel{color:var(--red);border-color:var(--red)}
.swagger-ui .btn.authorize{color:var(--green);border-color:var(--green-border)}
.swagger-ui .btn.authorize svg{fill:var(--green)}
.swagger-ui .highlight-code,.swagger-ui .microlight,.swagger-ui .opblock-body pre{background:var(--bg)!important}
.swagger-ui .opblock-body pre.microlight,.swagger-ui .opblock-body pre code{color:var(--text)!important;font-family:var(--mono)!important}
.swagger-ui .copy-to-clipboard{background:var(--input)}
.swagger-ui .tab li{color:var(--muted)}
.swagger-ui .tab li.active{color:var(--text)}
.swagger-ui .loading-container .loading:after{color:var(--muted)}`

func swaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <title>GEX Faker Studio · REST API</title>
    <link rel="icon" href="/favicon.svg" type="image/svg+xml">
    <link rel="stylesheet" href="/swagger-ui.css">
    ` + docsFontLinks + `
    <style>` + docsChromeCSS + `
` + swaggerThemeCSS + `</style>
</head>
<body>
    ` + docsNavBar("rest") + `
    <div id="swagger-ui"></div>
    <script src="/swagger-ui.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/openapi.yaml",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis
                ],
                layout: "BaseLayout"
            });
        };
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(html))
}

func asyncapiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(api.AsyncAPISpec)
}

// asyncapiThemeCSS restyles the AsyncAPI React component (Tailwind utility classes) to
// the Studio look: Space Grotesk headings, JetBrains Mono code, Studio surfaces/borders,
// and green accents. Overrides stay !important since the component paints inline classes.
const asyncapiThemeCSS = `#asyncapi{background:var(--bg);color:var(--text);min-height:100vh;font-family:var(--sans)}
#asyncapi h1,#asyncapi h2,#asyncapi h3,#asyncapi h4,#asyncapi h5,#asyncapi h6{font-family:var(--wordmark)!important;color:var(--text)!important;letter-spacing:-0.01em}
#asyncapi p,#asyncapi li,#asyncapi td,#asyncapi .prop-name{color:var(--text-2)!important}
#asyncapi .bg-white,#asyncapi .bg-gray-50,#asyncapi .bg-gray-100{background-color:var(--panel)!important}
#asyncapi .bg-gray-200,#asyncapi .bg-gray-800,#asyncapi .bg-gray-900{background-color:var(--panel-2)!important}
#asyncapi .text-gray-300,#asyncapi .text-gray-400,#asyncapi .text-gray-500,#asyncapi .text-gray-600{color:var(--muted)!important}
#asyncapi .text-gray-700,#asyncapi .text-gray-800,#asyncapi .text-gray-900{color:var(--text)!important}
#asyncapi .border,#asyncapi .border-gray-100,#asyncapi .border-gray-200,#asyncapi .border-gray-300{border-color:var(--border)!important}
#asyncapi a{color:var(--green-2)!important}
#asyncapi code,#asyncapi pre{background-color:var(--panel-2)!important;color:var(--text)!important;font-family:var(--mono)!important}
#asyncapi .bg-blue-100,#asyncapi .bg-blue-500,#asyncapi .bg-green-500,#asyncapi .bg-green-600,#asyncapi .bg-teal-500{background-color:var(--green)!important;color:var(--green-ink)!important;border-color:var(--green)!important}
#asyncapi .text-blue-500,#asyncapi .text-blue-600,#asyncapi .text-teal-500{color:var(--green-2)!important}
#asyncapi .aui-root .sidebar,#asyncapi ul[class*=sidebar]{background-color:var(--panel)!important;border-color:var(--border)!important}`

func asyncapiUIHandler(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <title>GEX Faker Studio · WebSocket API</title>
    <link rel="icon" href="/favicon.svg" type="image/svg+xml">
    <link rel="stylesheet" href="/asyncapi-ui.css">
    ` + docsFontLinks + `
    <style>` + docsChromeCSS + `
` + asyncapiThemeCSS + `</style>
</head>
<body>
    ` + docsNavBar("ws") + `
    <div id="asyncapi"></div>
    <script src="/asyncapi-ui.js"></script>
    <script>
        AsyncApiStandalone.render({
            schema: {
                url: '/asyncapi.yaml',
                options: { method: "GET", mode: "cors" },
            },
            config: {
                show: { sidebar: true }
            },
        }, document.getElementById('asyncapi'));
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(html))
}
