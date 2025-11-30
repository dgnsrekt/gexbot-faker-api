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
)

func NewRouter(server *Server, logger *zap.Logger) (http.Handler, error) {
	// Load OpenAPI spec for validation
	swagger, err := generated.GetSwagger()
	if err != nil {
		return nil, err
	}
	swagger.Servers = nil // Allow any host

	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(corsMiddleware)
	r.Use(zapLoggerMiddleware(logger))

	// Non-validated routes
	r.Get("/openapi.yaml", openapiHandler)
	r.Get("/docs", swaggerUIHandler)

	// API routes with OpenAPI validation
	r.Group(func(apiRouter chi.Router) {
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

func swaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>GEX Faker API</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.10.3/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.3/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/openapi.yaml",
                dom_id: '#swagger-ui',
            });
        };
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
