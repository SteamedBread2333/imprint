package imprint

import (
	"runtime/debug"
	"strings"
)

const modulePath = "github.com/SteamedBread2333/imprint"

// Version is the build version. The source of truth is the git tag
// (Go module versioning). Local trees report "devel" unless stamped:
//
//	go build -ldflags "-X github.com/SteamedBread2333/imprint/pkg/imprint.Version=X.Y.Z"
//
// `go install …@vX.Y.Z` fills this from module metadata when ldflags are absent.
var Version = "devel"

func init() {
	Version = resolveVersion(Version)
}

func resolveVersion(stamped string) string {
	stamped = strings.TrimPrefix(strings.TrimSpace(stamped), "v")
	if stamped != "" && stamped != "devel" {
		return stamped
	}
	if v := versionFromBuildInfo(); v != "" {
		return v
	}
	return "devel"
}

func versionFromBuildInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if info.Main.Path == modulePath {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	for _, d := range info.Deps {
		if d != nil && d.Path == modulePath && d.Version != "" && d.Version != "(devel)" {
			return strings.TrimPrefix(d.Version, "v")
		}
	}
	return ""
}
