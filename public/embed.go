// Package public embeds the static files the API serves, so the server does not depend on its working directory.
package public

import "embed"

// FS holds favicon.ico, api/v1/index.html, api/v1/openapi.json, cct_turtle/startup.lua, cct_turtle/updating_startup.lua and mcstatus/icons/*.png.
//
//go:embed favicon.ico api cct_turtle mcstatus
var FS embed.FS
