package web

import "embed"

//go:embed static/*
var Assets embed.FS
