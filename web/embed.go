// Package web embeds the built GEX Faker Studio SPA. The Vite build writes to
// web/dist; a checked-in placeholder index.html keeps this package compilable
// before the UI is built. See `just studio-build`.
package web

import "embed"

// Dist holds the built SPA (web/dist). `all:` includes files whose names start
// with "_" or "." (Vite emits assets under directories that are safe here, but
// the prefix keeps future-proof).
//
//go:embed all:dist
var Dist embed.FS
