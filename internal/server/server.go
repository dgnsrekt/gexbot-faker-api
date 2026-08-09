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

	// AsyncAPI documentation for WebSocket endpoints
	r.Get("/asyncapi.yaml", asyncapiHandler)
	r.Get("/asyncapi", asyncapiUIHandler)

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
// with the exact upstream error body. Faker control/metadata routes stay open.
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
// /health, /download/..., control routes — stay open (their 2nd segment is never
// classic/state/orderflow, and /download carries a date there).
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

func swaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>GEX Faker API</title>
    <link rel="stylesheet" href="/swagger-ui.css">
    <style>
        :root { color-scheme: dark; }
        body, .swagger-ui { background: #0d1117; color: #e6edf3; }
        .nav-bar { background: #1b1b1b; padding: 10px 20px; display: flex; gap: 20px; align-items: center; }
        .nav-bar a { color: #61affe; text-decoration: none; font-family: sans-serif; font-size: 14px; }
        .nav-bar a:hover { text-decoration: underline; }
        .nav-bar .current { color: #fff; font-weight: bold; }
        .swagger-ui .info .title,
        .swagger-ui .info p,
        .swagger-ui .info li,
        .swagger-ui .opblock-tag,
        .swagger-ui .opblock-description-wrapper p,
        .swagger-ui .opblock-summary-description,
        .swagger-ui .parameter__name,
        .swagger-ui .parameter__type,
        .swagger-ui .response-col_status,
        .swagger-ui .response-col_description,
        .swagger-ui .responses-inner h4,
        .swagger-ui .responses-inner h5,
        .swagger-ui .model,
        .swagger-ui .model-title,
        .swagger-ui label,
        .swagger-ui table thead tr td,
        .swagger-ui table thead tr th { color: #e6edf3; }
        .swagger-ui .opblock-summary-path,
        .swagger-ui .opblock-summary-description,
        .swagger-ui .opblock-tag small { color: #c9d1d9 !important; }
        .swagger-ui .info .base-url,
        .swagger-ui .scheme-container,
        .swagger-ui .model-container,
        .swagger-ui section.models,
        .swagger-ui .opblock .opblock-section-header { background: #161b22; }
        .swagger-ui .scheme-container { box-shadow: none; }
        .swagger-ui section.models,
        .swagger-ui section.models .model-container,
        .swagger-ui .opblock .opblock-section-header { border-color: #30363d; }
        .swagger-ui input,
        .swagger-ui select,
        .swagger-ui textarea {
            background: #0d1117;
            color: #e6edf3;
            border-color: #30363d;
        }
    </style>
</head>
<body>
    <div class="nav-bar">
        <span class="current">REST API Docs</span>
        <a href="/asyncapi">WebSocket Docs</a>
    </div>
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

func asyncapiUIHandler(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>GEX Faker WebSocket API</title>
    <link rel="stylesheet" href="https://unpkg.com/@asyncapi/react-component@latest/styles/default.min.css">
    <style>
        :root { color-scheme: dark; }
        body, #asyncapi { background: #0d1117; color: #e6edf3; min-height: 100vh; }
        .nav-bar { background: #263238; padding: 10px 20px; display: flex; gap: 20px; align-items: center; }
        .nav-bar a { color: #87ceeb; text-decoration: none; font-family: sans-serif; font-size: 14px; }
        .nav-bar a:hover { text-decoration: underline; }
        .nav-bar .current { color: #fff; font-weight: bold; }
        #asyncapi .bg-white,
        #asyncapi .bg-gray-50,
        #asyncapi .bg-gray-100 { background-color: #161b22 !important; }
        #asyncapi .text-gray-500,
        #asyncapi .text-gray-600 { color: #8b949e !important; }
        #asyncapi .text-gray-700,
        #asyncapi .text-gray-800,
        #asyncapi .text-gray-900 { color: #e6edf3 !important; }
        #asyncapi h1,
        #asyncapi h2,
        #asyncapi h3,
        #asyncapi h4,
        #asyncapi h5,
        #asyncapi h6,
        #asyncapi p,
        #asyncapi li { color: #c9d1d9 !important; }
        #asyncapi .bg-gray-200 { background-color: #21262d !important; }
        #asyncapi .border-gray-200,
        #asyncapi .border-gray-300 { border-color: #30363d !important; }
        #asyncapi code,
        #asyncapi pre { background-color: #161b22 !important; color: #e6edf3 !important; }
    </style>
</head>
<body>
    <div class="nav-bar">
        <a href="/docs">REST API Docs</a>
        <span class="current">WebSocket Docs</span>
        <a href="/asyncapi.yaml">/asyncapi.yaml</a>
    </div>
    <div id="asyncapi"></div>
    <script src="https://unpkg.com/@asyncapi/react-component@latest/browser/standalone/index.js"></script>
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
