// Package website embeds the built Astro Starlight docs site (the "guides").
// The build writes to website/dist; a checked-in dist/.gitkeep keeps this package
// compilable before the site is built. See `just docs-build`.
package website

import "embed"

// Dist holds the built docs site (website/dist). `all:` includes files whose
// names start with "_" (Astro emits hashed assets under _astro/) or ".".
//
//go:embed all:dist
var Dist embed.FS
