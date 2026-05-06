package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:ui/out
var uiFiles embed.FS

func getUIHandler() http.Handler {
	// The files are inside api/ui/out directory in the embedded FS.
	// We need to strip that prefix.
	fsys, err := fs.Sub(uiFiles, "ui/out")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fsys))
}
