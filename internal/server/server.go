package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/api"
	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
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

func NewRouter(server *Server, wsHubs *WebSocketHubs, negotiateHandler *ws.NegotiateHandler, logger *zap.Logger) (http.Handler, error) {
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

	// WebSocket routes (outside OpenAPI validation)
	if negotiateHandler != nil {
		r.Get("/negotiate", negotiateHandler.HandleNegotiate)
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

	// API routes with compression and OpenAPI validation
	r.Group(func(apiRouter chi.Router) {
		apiRouter.Use(middleware.Compress(5))
		apiRouter.Use(oapimiddleware.OapiRequestValidator(swagger))

		strictHandler := generated.NewStrictHandler(server, nil)
		generated.HandlerFromMux(strictHandler, apiRouter)
	})

	return r, nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
			logger.Debug("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", maskQueryKey(r.URL.RawQuery)),
			)
			next.ServeHTTP(w, r)
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
		if len(key) > 4 {
			values.Set("key", key[:4]+"****")
		}
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
	w.Write(api.OpenAPISpec)
}

func swaggerUIBundleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Write(api.SwaggerUIBundle)
}

func swaggerUICSSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Write(api.SwaggerUICSS)
}

func swaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>GEX Faker API</title>
    <link rel="stylesheet" href="/swagger-ui.css">
</head>
<body>
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
	w.Write([]byte(html))
}
