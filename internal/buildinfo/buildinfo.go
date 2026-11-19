// Package buildinfo holds the product version stamped into every binary.
//
// These values are overridden at link time with -ldflags from Makefile and
// scripts/release.sh. The defaults match the development version of the
// current checkout so plain "go build" and "go test" stay consistent.
package buildinfo

var (
	Version   = "1.5.2-dev"
	Commit    = "dev"
	BuildDate = ""
)
