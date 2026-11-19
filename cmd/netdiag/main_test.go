package main

import (
	"testing"

	"github.com/kr3s4l4/netdiag/internal/buildinfo"
)

// TestBuildInfoAliases ensures the CLI has no private version literals.
func TestBuildInfoAliases(t *testing.T) {
	if version != buildinfo.Version {
		t.Errorf("version=%q, buildinfo.Version=%q", version, buildinfo.Version)
	}
	if commit != buildinfo.Commit {
		t.Errorf("commit=%q, buildinfo.Commit=%q", commit, buildinfo.Commit)
	}
	if buildDate != buildinfo.BuildDate {
		t.Errorf("buildDate=%q, buildinfo.BuildDate=%q", buildDate, buildinfo.BuildDate)
	}
}
