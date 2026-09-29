// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	registry "github.com/orca-ae/orca-sdk-go"
)

type workspacePackagesClientMock struct {
	listFn           func(ctx context.Context, packageType string) ([]string, error)
	listVersionsFn   func(ctx context.Context, packageType, packageName string) ([]string, error)
	getMetadataFn    func(ctx context.Context, packageType, packageName, version string) (*registry.PackageMetadata, error)
	updateMetadataFn func(ctx context.Context, packageType, packageName, version string, metadata registry.PackageMetadata) error
	uploadFn         func(ctx context.Context, packageType, packageName, version, filePath string, metadata registry.PackageMetadata) error
	downloadFn       func(ctx context.Context, packageType, packageName, version string, writer io.Writer) error
	deleteFn         func(ctx context.Context, packageType, packageName, version string) error
}

func (m *workspacePackagesClientMock) List(ctx context.Context, packageType string) ([]string, error) {
	if m.listFn != nil {
		return m.listFn(ctx, packageType)
	}
	return nil, nil
}

func (m *workspacePackagesClientMock) ListVersions(ctx context.Context, packageType, packageName string) ([]string, error) {
	if m.listVersionsFn != nil {
		return m.listVersionsFn(ctx, packageType, packageName)
	}
	return nil, nil
}

func (m *workspacePackagesClientMock) GetMetadata(ctx context.Context, packageType, packageName, version string) (*registry.PackageMetadata, error) {
	if m.getMetadataFn != nil {
		return m.getMetadataFn(ctx, packageType, packageName, version)
	}
	return nil, nil
}

func (m *workspacePackagesClientMock) UpdateMetadata(ctx context.Context, packageType, packageName, version string, metadata registry.PackageMetadata) error {
	if m.updateMetadataFn != nil {
		return m.updateMetadataFn(ctx, packageType, packageName, version, metadata)
	}
	return nil
}

func (m *workspacePackagesClientMock) Upload(ctx context.Context, packageType, packageName, version, filePath string, metadata registry.PackageMetadata) error {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, packageType, packageName, version, filePath, metadata)
	}
	return nil
}

func (m *workspacePackagesClientMock) Download(ctx context.Context, packageType, packageName, version string, writer io.Writer) error {
	if m.downloadFn != nil {
		return m.downloadFn(ctx, packageType, packageName, version, writer)
	}
	return nil
}

func (m *workspacePackagesClientMock) Delete(ctx context.Context, packageType, packageName, version string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, packageType, packageName, version)
	}
	return nil
}

func TestParseWorkspacePackageReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    workspacePackageRef
		wantErr string
	}{
		{
			name: "function package with version",
			raw:  "function://pkg-a@v1",
			want: workspacePackageRef{
				LogicalType: "function",
				APIType:     "function",
				Name:        "pkg-a",
				Version:     "v1",
			},
		},
		{
			name: "source package without version",
			raw:  "source://pkg-source",
			want: workspacePackageRef{
				LogicalType: "source",
				APIType:     "source",
				Name:        "pkg-source",
			},
		},
		{
			name: "sink package with version",
			raw:  "sink://pkg-sink@latest",
			want: workspacePackageRef{
				LogicalType: "sink",
				APIType:     "sink",
				Name:        "pkg-sink",
				Version:     "latest",
			},
		},
		{
			name:    "reject plural type",
			raw:     "functions://pkg-a@v1",
			wantErr: `invalid package type "functions"`,
		},
		{
			name:    "reject slash in name",
			raw:     "function://pkg/a@v1",
			wantErr: "must not contain slashes or whitespace",
		},
		{
			name:    "reject empty version",
			raw:     "function://pkg-a@",
			wantErr: "package version cannot be empty",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseWorkspacePackageReference(tc.raw)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseWorkspacePackageReference() error = %v, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWorkspacePackageReference() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("parseWorkspacePackageReference() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestParseWorkspacePackageReferenceArgRequiresVersion(t *testing.T) {
	t.Parallel()

	if _, err := parseWorkspacePackageReferenceArg("function://pkg-a", true); err == nil || !strings.Contains(err.Error(), "must include a version") {
		t.Fatalf("parseWorkspacePackageReferenceArg() error = %v, want missing version error", err)
	}
	if _, err := parseWorkspacePackageReferenceArg("function://pkg-a@v1", false); err == nil || !strings.Contains(err.Error(), "must not include a version") {
		t.Fatalf("parseWorkspacePackageReferenceArg() error = %v, want extra version error", err)
	}
}

func TestPackagesListCommandRendersCanonicalURLs(t *testing.T) {
	original := newWorkspacePackagesClient
	defer func() {
		newWorkspacePackagesClient = original
	}()

	newWorkspacePackagesClient = func() (workspacePackagesClient, error) {
		return &workspacePackagesClientMock{
			listFn: func(_ context.Context, packageType string) ([]string, error) {
				if packageType != "function" {
					t.Fatalf("packageType = %q, want %q", packageType, "function")
				}
				return []string{"pkg-a", "pkg-b"}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	o := &packagesOptions{
		ioStreams: IOStreams{
			Out:    &stdout,
			ErrOut: io.Discard,
		},
	}

	cmd := o.newListCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--type", "function"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rendered := stdout.String()
	for _, expected := range []string{"function://pkg-a", "function://pkg-b"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered output missing %q: %s", expected, rendered)
		}
	}
}

func TestPackagesUpdateMetadataRejectsEmptyUpdate(t *testing.T) {
	t.Parallel()

	o := &packagesOptions{}
	cmd := o.newUpdateMetadataCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"function://pkg-a@v1"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "at least one metadata flag must be provided") {
		t.Fatalf("Execute() error = %v, want empty metadata error", err)
	}
}

func TestPackagesUpdateMetadataPreservesUnchangedFields(t *testing.T) {
	original := newWorkspacePackagesClient
	defer func() { newWorkspacePackagesClient = original }()

	var updated registry.PackageMetadata
	newWorkspacePackagesClient = func() (workspacePackagesClient, error) {
		return &workspacePackagesClientMock{
			getMetadataFn: func(context.Context, string, string, string) (*registry.PackageMetadata, error) {
				return &registry.PackageMetadata{
					Description:      "old",
					Contact:          "owner@example.com",
					CreateTime:       10,
					ModificationTime: 20,
					Properties:       map[string]string{"tier": "gold"},
				}, nil
			},
			updateMetadataFn: func(_ context.Context, _, _, _ string, metadata registry.PackageMetadata) error {
				updated = metadata
				return nil
			},
		}, nil
	}

	o := &packagesOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}
	cmd := o.newUpdateMetadataCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"function://pkg-a@v1", "--description", "new"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if updated.Description != "new" || updated.Contact != "owner@example.com" || updated.CreateTime != 10 || updated.ModificationTime != 20 || updated.Properties["tier"] != "gold" {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestPackagesUpdateMetadataSupportsExplicitClears(t *testing.T) {
	original := newWorkspacePackagesClient
	defer func() { newWorkspacePackagesClient = original }()

	var updated registry.PackageMetadata
	newWorkspacePackagesClient = func() (workspacePackagesClient, error) {
		return &workspacePackagesClientMock{
			getMetadataFn: func(context.Context, string, string, string) (*registry.PackageMetadata, error) {
				return &registry.PackageMetadata{Description: "old", Properties: map[string]string{"tier": "gold"}}, nil
			},
			updateMetadataFn: func(_ context.Context, _, _, _ string, metadata registry.PackageMetadata) error {
				updated = metadata
				return nil
			},
		}, nil
	}

	o := &packagesOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}
	cmd := o.newUpdateMetadataCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"function://pkg-a@v1", "--description", "", "--clear-properties"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if updated.Description != "" || len(updated.Properties) != 0 {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestPackagesDownloadCommandWritesFile(t *testing.T) {
	original := newWorkspacePackagesClient
	defer func() {
		newWorkspacePackagesClient = original
	}()

	newWorkspacePackagesClient = func() (workspacePackagesClient, error) {
		return &workspacePackagesClientMock{
			downloadFn: func(_ context.Context, packageType, packageName, version string, writer io.Writer) error {
				if packageType != "source" {
					t.Fatalf("packageType = %q, want %q", packageType, "source")
				}
				if packageName != "pkg-a" {
					t.Fatalf("packageName = %q, want %q", packageName, "pkg-a")
				}
				if version != "v1" {
					t.Fatalf("version = %q, want %q", version, "v1")
				}
				_, _ = io.WriteString(writer, "payload")
				return nil
			},
		}, nil
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "download.nar")

	var stdout bytes.Buffer
	o := &packagesOptions{
		ioStreams: IOStreams{
			Out:    &stdout,
			ErrOut: io.Discard,
		},
	}

	cmd := o.newDownloadCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"source://pkg-a@v1", "--path", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "payload" {
		t.Fatalf("downloaded content = %q, want %q", string(content), "payload")
	}
	if !strings.Contains(stdout.String(), `Downloaded package "source://pkg-a@v1" successfully`) {
		t.Fatalf("stdout = %q, want success message", stdout.String())
	}
}

func TestPackagesDownloadFailureDoesNotClobberDestination(t *testing.T) {
	original := newWorkspacePackagesClient
	defer func() { newWorkspacePackagesClient = original }()

	newWorkspacePackagesClient = func() (workspacePackagesClient, error) {
		return &workspacePackagesClientMock{
			downloadFn: func(_ context.Context, _, _, _ string, writer io.Writer) error {
				_, _ = io.WriteString(writer, "partial")
				return errors.New("stream failed")
			},
		}, nil
	}

	outputPath := filepath.Join(t.TempDir(), "download.nar")
	if err := os.WriteFile(outputPath, []byte("original"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	o := &packagesOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}
	cmd := o.newDownloadCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"source://pkg-a@v1", "--path", outputPath})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() expected error")
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "original" {
		t.Fatalf("destination content = %q, want original", content)
	}
}

func TestPackagesDownloadPreservesDestinationPermissions(t *testing.T) {
	original := newWorkspacePackagesClient
	defer func() { newWorkspacePackagesClient = original }()

	newWorkspacePackagesClient = func() (workspacePackagesClient, error) {
		return &workspacePackagesClientMock{
			downloadFn: func(_ context.Context, _, _, _ string, writer io.Writer) error {
				_, err := io.WriteString(writer, "replacement")
				return err
			},
		}, nil
	}

	outputPath := filepath.Join(t.TempDir(), "download.nar")
	if err := os.WriteFile(outputPath, []byte("original"), 0640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(outputPath, 0640); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

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
	if got := info.Mode().Perm(); got != 0640 {
		t.Fatalf("mode = %o, want 640", got)
	}
}

func TestNewCmdWorkspaceIncludesPackages(t *testing.T) {
	t.Parallel()

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{}})
	if _, _, err := cmd.Find([]string{"packages"}); err != nil {
		t.Fatalf("Find(packages) error = %v", err)
	}
}

func TestNewCmdPackagesProvidesDocDescriptions(t *testing.T) {
	t.Parallel()

	cmd := NewCmdPackages(&Options{IOStreams: IOStreams{}})

	if !strings.Contains(cmd.Long, "<type>://<package_name>[@<version>]") {
		t.Fatalf("packages root Long = %q, want package URL format description", cmd.Long)
	}
	if !strings.Contains(cmd.Long, "one of function, source, or sink") {
		t.Fatalf("packages root Long = %q, want singular package type description", cmd.Long)
	}

	tests := map[string]string{
		"list":            "requested type",
		"list-versions":   "without an @<version> suffix",
		"get-metadata":    "include both the package type and version",
		"update-metadata": "without re-uploading the package artifact",
		"upload":          "--path must point to the local artifact file",
		"download":        "--path selects where the downloaded artifact is written",
		"delete":          "include the version to remove",
	}

	for name, want := range tests {
		subCmd, _, err := cmd.Find([]string{name})
		if err != nil {
			t.Fatalf("Find(%s) error = %v", name, err)
		}
		if !strings.Contains(subCmd.Long, want) {
			t.Fatalf("%s Long = %q, want substring %q", name, subCmd.Long, want)
		}
	}

	listCmd, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("Find(list) error = %v", err)
	}
	if got := listCmd.Flag("type").Usage; !strings.Contains(got, "function, source, sink") {
		t.Fatalf("list --type usage = %q, want singular package type description", got)
	}
}
