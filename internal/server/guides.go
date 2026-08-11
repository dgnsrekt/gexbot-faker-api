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

	"github.com/dgnsrekt/gexbot-downloader/website"
)

// registerGuidesUI serves the embedded Astro Starlight docs site ("guides") at
// /guides and /guides/*, plus the root llms.txt / llms-full.txt agent files. It
// is public — docs carry no secrets — so it is registered outside the Studio's
// auth group. Mirrors registerStudioUI's embedded-FS serving (assets served
// uncompressed; the site is prerendered static HTML with per-page directories).
func registerGuidesUI(r chi.Router) error {
	dist, err := fs.Sub(website.Dist, "dist")
	if err != nil {
		return err
	}

	serve := func(w http.ResponseWriter, req *http.Request) {
		rel := strings.TrimPrefix(req.URL.Path, "/guides")
		rel = strings.TrimPrefix(rel, "/")
		// Directory URLs (e.g. /guides/ or /guides/overview/) map to index.html.
		if rel == "" || strings.HasSuffix(rel, "/") {
			rel = path.Join(rel, "index.html")
		}

		data, ferr := fs.ReadFile(dist, rel)
		if ferr != nil {
			// Extensionless page URL (/guides/overview) -> its directory index.
			if d2, e2 := fs.ReadFile(dist, path.Join(rel, "index.html")); e2 == nil {
				rel, data, ferr = path.Join(rel, "index.html"), d2, nil
			}
		}
		if ferr != nil {
			guidesFallback(w, dist)
			return
		}

		if ct := mime.TypeByExtension(path.Ext(rel)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		// Astro emits content-hashed assets under _astro/ (immutable); HTML must
		// always revalidate so a rebuild is picked up.
		if strings.HasSuffix(rel, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeContent(w, req, rel, time.Time{}, bytes.NewReader(data))
	}

	r.Get("/guides", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/guides/", http.StatusMovedPermanently)
	})
	r.Get("/guides/*", serve)

	// llms.txt / llms-full.txt at the origin root (llmstxt.org convention), served
	// from the same embedded build so they stay in sync with the docs.
	llms := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			data, e := fs.ReadFile(dist, name)
			if e != nil {
				http.NotFound(w, req)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			http.ServeContent(w, req, name, time.Time{}, bytes.NewReader(data))
		}
	}
	r.Get("/llms.txt", llms("llms.txt"))
	r.Get("/llms-full.txt", llms("llms-full.txt"))
	return nil
}

// guidesFallback serves the site's 404 page, or a build hint when the docs have
// not been built (only dist/.gitkeep is embedded).
func guidesFallback(w http.ResponseWriter, dist fs.FS) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if nf, err := fs.ReadFile(dist, "404.html"); err == nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(nf)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(guidesNotBuiltHTML))
}

const guidesNotBuiltHTML = `<!doctype html><meta charset="utf-8"><title>GEX Faker guides</title>` +
	`<body style="background:#0d0e10;color:#e7e8ea;font-family:system-ui;padding:40px">` +
	`<h1>GEX Faker guides</h1>` +
	`<p>The docs site hasn't been built yet. Run <code>just docs-build</code> ` +
	`(or <code>cd website &amp;&amp; npm ci &amp;&amp; npm run build</code>) and rebuild the server.</p>` +
	`<p>The knowledge source is in <code>knowledge/</code>; agent files are at ` +
	`<code>/llms.txt</code> and <code>/llms-full.txt</code>.</p></body>`
