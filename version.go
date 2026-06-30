package main

import "runtime/debug"

// VERSION is injected at build time via -ldflags "-X main.VERSION=<tag>"
// (see the Makefile and .goreleaser.yaml, which derive it from the git tag).
// It is intentionally NOT hardcoded in the repository.
var VERSION string

// APP is the name of the application
const APP = "bb"

// PACKAGE is the name of the package (used to create artifacts)
const PACKAGE = "bitbucket-cli"

// Version gets the current version of the application.
//
// Precedence: the build-time -ldflags value, then the module version recorded
// in the build info (set when installed via `go install <module>@<version>`),
// then "dev" for a plain local build.
func Version() string {
	if VERSION != "" {
		return VERSION
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}
