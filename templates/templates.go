// Package templates embeds every Go source template featherctl ships.
//
// Templates are compiled into the binary so `feather init` and
// `feather new feature` never depend on files next to the executable.
package templates

import "embed"

// FS holds the project and feature templates.
//
//go:embed project feature
var FS embed.FS

// Project is the directory inside [FS] that holds the templates used by
// `feather init`.
const Project = "project"

// Feature is the directory inside [FS] that holds the templates used by
// `feather new feature`.
const Feature = "feature"
