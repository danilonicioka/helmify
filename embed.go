package helmify

import "embed"

// ModelsFS embeds the models directory containing single and multi Helm charts.
//
//go:embed models/single/* models/single/templates/* models/multi/* models/multi/templates/*
var ModelsFS embed.FS

//go:embed gitlab-ci.yml
var GitLabCI []byte
