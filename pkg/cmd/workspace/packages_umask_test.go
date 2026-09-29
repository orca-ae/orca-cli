// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package workspace

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPackagesDownloadHonorsRestrictiveUmask(t *testing.T) {
	originalClientFactory := newWorkspacePackagesClient
	defer func() { newWorkspacePackagesClient = originalClientFactory }()
	newWorkspacePackagesClient = func() (workspacePackagesClient, error) {
		return &workspacePackagesClientMock{
			downloadFn: func(_ context.Context, _, _, _ string, writer io.Writer) error {
				_, err := io.WriteString(writer, "payload")
				return err
			},
		}, nil
	}

	originalUmask := syscall.Umask(0077)
	defer syscall.Umask(originalUmask)

	outputPath := filepath.Join(t.TempDir(), "download.nar")
	o := &packagesOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}
	cmd := o.newDownloadCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"source://pkg-a@v1", "--path", outputPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}
