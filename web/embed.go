package web

import (
	"embed"
)

//go:embed home.html
var HomeHTML []byte

//go:embed instructions.html
var InstructionsHTML []byte

//go:embed converter.html
var ConverterHTML []byte

//go:embed index.html
var WizardHTML []byte

//go:embed static/*
var StaticFS embed.FS
