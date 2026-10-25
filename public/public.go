// Package public embeds the browser frontend. public/app is the output of
// `make web`; commit it so `go build` needs no Node toolchain.
package public

import (
	"embed"
	"net/http"
)

//go:embed app images index.html game.css ioquake3.js manifest.json browserconfig.xml favicon.ico
var assets embed.FS

// Files serves the embedded frontend.
var Files http.FileSystem = http.FS(assets)
