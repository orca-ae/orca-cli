// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/orca-ae/orca-cli/pkg/cmd/root"
	"github.com/orca-ae/orca-cli/pkg/cmd/workspace"
)

// version is stamped by release builds: -ldflags "-X main.version=<version>".
var version string

func main() {
	cmd := root.NewCommand(workspace.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	cmd.Version = resolveVersion(version, debug.ReadBuildInfo)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// resolveVersion prefers the version a release build stamped. Otherwise it uses
// the module version Go records, which is the release for `go install …@vX.Y.Z`
// and a pseudo-version for a build from a git checkout. It falls back to "dev".
func resolveVersion(stamped string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if stamped != "" {
		return stamped
	}
	if info, ok := readBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return "dev"
}
