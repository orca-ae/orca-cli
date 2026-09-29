// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/orca-ae/orca-cli/pkg/cmd/root"
	"github.com/orca-ae/orca-cli/pkg/cmd/workspace"
)

func TestResolveVersion(t *testing.T) {
	buildInfo := func(version string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: version}}, true
		}
	}
	noBuildInfo := func() (*debug.BuildInfo, bool) { return nil, false }

	for _, tc := range []struct {
		name      string
		stamped   string
		buildInfo func() (*debug.BuildInfo, bool)
		want      string
	}{
		{name: "release build", stamped: "0.5.0-rc.1", buildInfo: buildInfo("v0.4.1"), want: "0.5.0-rc.1"},
		{name: "go install", buildInfo: buildInfo("v0.5.0"), want: "0.5.0"},
		{name: "git checkout", buildInfo: buildInfo("v0.5.1-0.20260927101112-0123456789ab+dirty"), want: "0.5.1-0.20260927101112-0123456789ab+dirty"},
		{name: "unversioned build", buildInfo: buildInfo("(devel)"), want: "dev"},
		{name: "no build info", buildInfo: noBuildInfo, want: "dev"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveVersion(tc.stamped, tc.buildInfo); got != tc.want {
				t.Fatalf("resolveVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestVersionFlagPrintsTheVersion(t *testing.T) {
	var out bytes.Buffer
	cmd := root.NewCommand(workspace.IOStreams{In: strings.NewReader(""), Out: &out, ErrOut: &out})
	cmd.Version = "0.5.0-rc.1"
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "ork version 0.5.0-rc.1\n" {
		t.Fatalf("ork --version printed %q", got)
	}
}
