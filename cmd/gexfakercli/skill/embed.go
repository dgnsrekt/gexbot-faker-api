// Package skill embeds the installable SKILL.md (and any future reference files)
// so `gexfakercli skill install` can write them into the Claude/Codex skills dirs
// without shipping a separate asset. Mirrors web/embed.go.
package skill

import "embed"

// Files holds the skill bundle as a walkable FS. Add reference files to the
// embed pattern here as they are created (e.g. `//go:embed SKILL.md references`).
//
//go:embed SKILL.md
var Files embed.FS
