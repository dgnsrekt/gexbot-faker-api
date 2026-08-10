package server

import (
	"bytes"
	"crypto/subtle"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dgnsrekt/gexbot-downloader/web"
)

// studioAuthMiddleware gates the Studio behind HTTP Basic auth when token is set
// (any username, password must equal the token). An empty token leaves the
// Studio open. Basic is used so the browser natively prompts and stores the
// credential, and the SPA's same-origin fetch/SSE calls carry it automatically;
// a Bearer header is also accepted for curl/programmatic use.
func studioAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" || studioTokenOK(r, token) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="GEX Faker Studio"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

func studioTokenOK(r *http.Request, token string) bool {
	if _, pass, ok := r.BasicAuth(); ok {
		return subtle.ConstantTimeCompare([]byte(pass), []byte(token)) == 1
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, "Bearer ")), []byte(token)) == 1
	}
	return false
}

// registerStudioUI serves the embedded SPA at /studio and /studio/*. Assets are
// served uncompressed (like the swagger-ui handlers — chi's Compress corrupts
// large JS bundles) with an SPA fallback: any /studio path that isn't a real
// asset returns index.html so client-side routes (e.g. /studio/library) deep-link.
//
// File bytes are written directly (rather than via http.FileServer) so the
// index.html fallback doesn't trigger FileServer's /index.html -> ./ redirect.
func registerStudioUI(r chi.Router) error {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return err
	}

	serve := func(w http.ResponseWriter, req *http.Request) {
		rel := strings.TrimPrefix(req.URL.Path, "/studio")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			rel = "index.html"
		}
		data, err := fs.ReadFile(dist, rel)
		if err != nil {
			// Not a real asset — hand the SPA its entry point for client routing.
			rel = "index.html"
			data, err = fs.ReadFile(dist, rel)
			if err != nil {
				// UI hasn't been built (only .gitkeep is embedded). Serve a hint.
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(studioNotBuiltHTML))
				return
			}
		}
		if ct := mime.TypeByExtension(path.Ext(rel)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		// Hashed Vite assets are immutable; index.html must always revalidate.
		if rel == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeContent(w, req, rel, time.Time{}, bytes.NewReader(data))
	}

	// Redirect the bare /studio to /studio/ so relative asset URLs resolve.
	r.Get("/studio", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/studio/", http.StatusMovedPermanently)
	})
	r.Get("/studio/*", serve)
	return nil
}

const studioNotBuiltHTML = `<!doctype html><meta charset="utf-8"><title>GEX Faker Studio</title>` +
	`<body style="background:#0d0e10;color:#e7e8ea;font-family:system-ui;padding:40px">` +
	`<h1>GEX Faker Studio</h1>` +
	`<p>The UI hasn't been built yet. Run <code>just studio-build</code> ` +
	`(or <code>cd web &amp;&amp; npm ci &amp;&amp; npm run build</code>) and rebuild the server.</p>` +
	`</body>`
