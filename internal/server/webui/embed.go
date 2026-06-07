package webui

import "embed"

// EmbedFS contains the built frontend SPA assets.
// The dist/ directory must contain at least .gitkeep so go:embed compiles
// even when the frontend hasn't been built yet.
// Use "all:dist" to recursively embed subdirectories (e.g. dist/assets/).
//
//go:embed all:dist
var EmbedFS embed.FS
