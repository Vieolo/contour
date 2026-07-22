//go:build dev

package config

// Dev reports whether this is a development build.
const Dev = true

// Program is the installed binary name, used in help and guidance text.
const Program = "contour-dev"

// active selects the development store profile.
var active = developmentProfile
