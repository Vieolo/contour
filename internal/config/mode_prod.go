//go:build !dev

package config

// Dev reports whether this is a development build.
const Dev = false

// Program is the installed binary name, used in help and guidance text.
const Program = "contour"

// active selects the production store profile.
var active = productionProfile
