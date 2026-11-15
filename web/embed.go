// Package web embeds the built Excalidraw frontend (web/dist).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// Assets is the built bundle rooted at dist/assets.
func Assets() fs.FS {
	sub, err := fs.Sub(embedded, "dist/assets")
	if err != nil {
		panic("web: dist/assets missing from embed: " + err.Error())
	}
	return sub
}
