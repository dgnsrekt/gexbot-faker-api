package server

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dgnsrekt/gexbot-downloader/web"
)

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
