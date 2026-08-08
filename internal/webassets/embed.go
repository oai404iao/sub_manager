package webassets

import "embed"

// Dist contains the production Vite build.
//
//go:embed all:dist
var Dist embed.FS
