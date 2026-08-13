package api

import "embed"

// Fonts holds the self-hosted webfonts (Space Grotesk + JetBrains Mono, latin, weights
// 400/500/700) as .woff2, vendored from @fontsource so the Studio and the docs pages
// render in the brand typefaces fully offline instead of loading from Google Fonts.
// Served at /fonts/* and referenced by @font-face in both the SPA (web/src/app.css) and
// the docs chrome (internal/server docsChromeCSS).
//
//go:embed fonts/*.woff2
var Fonts embed.FS
