// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"github.com/maloquacious/semver"
)

var (
	version = semver.Version{
		Major:      0,
		Minor:      12,
		Patch:      0,
		PreRelease: "beta",
	}
)

func Version() semver.Version {
	return version
}
