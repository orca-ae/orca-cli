// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

type renderWorkspaceResourceFixture struct {
	Name string `json:"name" yaml:"name"`
}

func TestRenderWorkspaceResourceText(t *testing.T) {
	t.Parallel()

	var builder strings.Builder
	called := false
	err := renderWorkspaceResource(&builder, "text", renderWorkspaceResourceFixture{Name: "fixture"}, func(writer io.Writer) error {
		called = true
		_, _ = writer.Write([]byte("text-output"))
		return nil
	})
	if err != nil {
		t.Fatalf("renderWorkspaceResource() error = %v", err)
	}
	if !called {
		t.Fatal("renderWorkspaceResource() did not invoke text renderer")
	}
	if builder.String() != "text-output" {
		t.Fatalf("builder.String() = %q, want %q", builder.String(), "text-output")
	}
}

func TestRenderWorkspaceResourceJSON(t *testing.T) {
	t.Parallel()

	var builder strings.Builder
	err := renderWorkspaceResource(&builder, "json", renderWorkspaceResourceFixture{Name: "fixture"}, func(io.Writer) error {
		return errors.New("text renderer should not be called for json output")
	})
	if err != nil {
		t.Fatalf("renderWorkspaceResource() error = %v", err)
	}

	rendered := builder.String()
	for _, expected := range []string{`"name": "fixture"`} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered output missing %q: %s", expected, rendered)
		}
	}
}

func TestRenderWorkspaceResourceYAML(t *testing.T) {
	t.Parallel()

	var builder strings.Builder
	err := renderWorkspaceResource(&builder, "yaml", renderWorkspaceResourceFixture{Name: "fixture"}, func(io.Writer) error {
		return errors.New("text renderer should not be called for yaml output")
	})
	if err != nil {
		t.Fatalf("renderWorkspaceResource() error = %v", err)
	}

	rendered := builder.String()
	for _, expected := range []string{"name: fixture"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered output missing %q: %s", expected, rendered)
		}
	}
}

func TestRenderWorkspaceResourceRejectsInvalidOutput(t *testing.T) {
	t.Parallel()

	called := false
	err := renderWorkspaceResource(&strings.Builder{}, "xml", renderWorkspaceResourceFixture{Name: "fixture"}, func(io.Writer) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("renderWorkspaceResource() expected error, got nil")
	}
	if called {
		t.Fatal("renderWorkspaceResource() invoked text renderer for invalid output")
	}
}

func TestEnsureExtensionAvailableGroupPresent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apis" {
			t.Fatalf("path = %q, want /apis", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io"}]}`))
	}))
	defer server.Close()

	client, err := registry.NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := ensureExtensionAvailable(context.Background(), client, registry.CloudExtensionGroup, io.Discard); err != nil {
		t.Fatalf("ensureExtensionAvailable() error = %v, want nil", err)
	}
}

func TestEnsureExtensionAvailableEmptyGroupsReportsNotAvailable(t *testing.T) {
	t.Parallel()

	// An empty groups list is a normal, fully-functional self-hosted engine with no extensions -
	// not an error, and not the same diagnosis as a 404. Both mean the caller cannot use the
	// extension, but the gate must still tell them apart (see TestEnsureExtensionAvailable
	// NotFoundWarnsAndReportsNotAvailable for the 404 case).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
	}))
	defer server.Close()

	client, err := registry.NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var warnOut bytes.Buffer
	gateErr := ensureExtensionAvailable(context.Background(), client, registry.CloudExtensionGroup, &warnOut)

	if gateErr == nil {
		t.Fatal("ensureExtensionAvailable() expected error for an empty groups list, got nil")
	}
	if !strings.Contains(gateErr.Error(), "not available on this deployment") {
		t.Fatalf("error = %v, want a \"not available\" message", gateErr)
	}
	if strings.Contains(gateErr.Error(), "404") {
		t.Fatalf("error = %v, must not read like a bare 404", gateErr)
	}
	if warnOut.Len() != 0 {
		t.Fatalf("warnOut = %q, want no version warning for a deployment that answers discovery normally", warnOut.String())
	}
}

func TestEnsureExtensionAvailableNotFoundWarnsAndReportsNotAvailable(t *testing.T) {
	t.Parallel()

	// A 404 from /apis itself is a different diagnosis from an empty groups list: the deployment
	// predates extension discovery entirely. Both produce a "not available" error, but only this
	// one should additionally warn about the server version.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := registry.NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var warnOut bytes.Buffer
	gateErr := ensureExtensionAvailable(context.Background(), client, registry.CloudExtensionGroup, &warnOut)

	if gateErr == nil {
		t.Fatal("ensureExtensionAvailable() expected error for a 404 from /apis, got nil")
	}
	if !strings.Contains(gateErr.Error(), "not available on this deployment") {
		t.Fatalf("error = %v, want a \"not available\" message", gateErr)
	}
	if !strings.Contains(warnOut.String(), "404") {
		t.Fatalf("warnOut = %q, want a version warning mentioning the 404", warnOut.String())
	}
}

func TestEnsureExtensionAvailablePropagatesProbeFailureRaw(t *testing.T) {
	t.Parallel()

	// Anything other than a clean 404 (here: connection refused, from a server closed before any
	// request reaches it) must not be folded into "not available" - the probe could not determine
	// capability at all, so the caller sees what actually happened instead of a misleading verdict
	// either way.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	client, err := registry.NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = ensureExtensionAvailable(context.Background(), client, registry.CloudExtensionGroup, io.Discard)
	if err == nil {
		t.Fatal("ensureExtensionAvailable() expected error for a probe network failure, got nil")
	}
	if strings.Contains(err.Error(), "not available on this deployment") {
		t.Fatalf("error = %v, a probe failure must not be folded into the confirmed-unavailable message", err)
	}
}

func TestRequireCloudExtensionBlocksCommandWhenGroupMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apis" {
			t.Fatalf("unexpected request to %s; the real connections call must not fire when the gate blocks it", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
	}))
	defer server.Close()

	original := newWorkspaceConnectionsClient
	newWorkspaceConnectionsClient = func() (workspaceConnectionsClient, error) {
		t.Fatal("newWorkspaceConnectionsClient() should not be called when the gate blocks the command")
		return nil, nil
	}
	defer func() { newWorkspaceConnectionsClient = original }()

	cmd := NewCmdConnections(&Options{
		IOStreams:   IOStreams{Out: io.Discard, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"list"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not available on this deployment") {
		t.Fatalf("Execute() error = %v, want a \"not available\" message, not a bare 404", err)
	}
}

func TestRequireCloudExtensionAllowsCommandWhenGroupPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apis" {
			t.Fatalf("unexpected request to %s, want only the /apis probe to reach this server", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io"}]}`))
	}))
	defer server.Close()

	called := false
	original := newWorkspaceConnectionsClient
	newWorkspaceConnectionsClient = func() (workspaceConnectionsClient, error) {
		called = true
		return &workspaceConnectionsClientMock{
			listFn: func(context.Context) ([]registry.ConnectionConfig, error) {
				return nil, nil
			},
		}, nil
	}
	defer func() { newWorkspaceConnectionsClient = original }()

	cmd := NewCmdConnections(&Options{
		IOStreams:   IOStreams{Out: io.Discard, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !called {
		t.Fatal("newWorkspaceConnectionsClient() was not called; the gate should have let the command proceed")
	}
}

func TestRebasedCloudCommandGroupsRequireExtension(t *testing.T) {
	testCases := []struct {
		name string
		new  func(*Options) *cobra.Command
		args []string
	}{
		{name: "health", new: NewCmdHealth, args: []string{"ready"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/apis" {
					t.Fatalf("unexpected request to %s; capability gating must block the command request", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
			}))
			defer server.Close()

			cmd := testCase.new(&Options{
				IOStreams:   IOStreams{Out: io.Discard, ErrOut: io.Discard},
				RegistryURL: server.URL,
				AccessToken: "token",
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(testCase.args)

			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "not available on this deployment") {
				t.Fatalf("Execute() error = %v, want extension-not-available error", err)
			}
		})
	}
}

func TestCloudCommandGroupsShowHelpWithoutCapabilityProbe(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "connections", args: []string{"connections"}},
		{name: "health", args: []string{"health", "--help"}},
		{name: "api resources", args: []string{"api-resources", "--help"}},
		{name: "functions", args: []string{"functions"}},
		{name: "sources", args: []string{"sources"}},
		{name: "sinks", args: []string{"sinks"}},
		{name: "kafka connect", args: []string{"kafka-connect"}},
		{name: "packages", args: []string{"packages"}},
		{name: "agent providers", args: []string{"agent", "providers"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var runtimeCalls int
			var output bytes.Buffer
			cmd := NewGroupCommand(&Options{
				IOStreams: IOStreams{Out: &output, ErrOut: &output},
				ResolveRuntime: func(context.Context, *cobra.Command, *Options) (RegistryRuntime, error) {
					runtimeCalls++
					return RegistryRuntime{}, errors.New("capability probe should not run while showing help")
				},
			})
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(testCase.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v, want help without capability probing", err)
			}
			if runtimeCalls != 0 {
				t.Fatalf("ResolveRuntime() calls = %d, want 0", runtimeCalls)
			}
			if !strings.Contains(output.String(), "Usage:") {
				t.Fatalf("help output missing Usage section: %q", output.String())
			}
		})
	}
}

func TestCoreCommandsRouteLegacyWarningsToCommandErrorStream(t *testing.T) {
	testCases := []struct {
		name         string
		args         []string
		expectedPath string
		response     string
	}{
		// A paginated envelope, like every other core list. The bare array this
		// used to return was not a shape the server can produce; the untyped
		// passthrough accepted it because it decoded into interface{}.
		{name: "managed agents", args: []string{"agent", "list", "--output", "json"}, expectedPath: "/v1/agents", response: `{"data":[],"next_page":null}`},
		{name: "triggers", args: []string{"agent", "triggers", "list", "--output", "json"}, expectedPath: "/v1/triggers", response: `{"data":[],"next_page":null}`},
		{name: "API groups", args: []string{"api-groups", "--output", "json"}, expectedPath: "/apis", response: `{"kind":"APIGroupList","groups":[]}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != testCase.expectedPath {
					t.Fatalf("path = %q, want %q", r.URL.Path, testCase.expectedPath)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(testCase.response))
			}))
			defer server.Close()

			var warnings bytes.Buffer
			cmd := NewGroupCommand(&Options{
				IOStreams:   IOStreams{Out: io.Discard},
				RegistryURL: server.URL + "/v1/registry",
				AccessToken: "token",
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(&warnings)
			cmd.SetArgs(testCase.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if count := strings.Count(warnings.String(), "warning: registry base URL"); count != 1 {
				t.Fatalf("legacy URL warning count = %d, want 1; output = %q", count, warnings.String())
			}
		})
	}
}

func TestRequireCloudExtensionCachesDiscoveryResult(t *testing.T) {
	var probeCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apis" {
			t.Fatalf("path = %q, want /apis", r.URL.Path)
		}
		probeCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io"}]}`))
	}))
	defer server.Close()

	original := newWorkspaceConnectionsClient
	newWorkspaceConnectionsClient = func() (workspaceConnectionsClient, error) {
		return &workspaceConnectionsClientMock{
			listFn: func(context.Context) ([]registry.ConnectionConfig, error) {
				return nil, nil
			},
		}, nil
	}
	defer func() { newWorkspaceConnectionsClient = original }()

	cmd := NewCmdConnections(&Options{
		IOStreams:   IOStreams{Out: io.Discard, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"list"})

	for range 2 {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}
	}
	if got := probeCount.Load(); got != 1 {
		t.Fatalf("GET /apis calls = %d, want 1", got)
	}
}

func TestRequireCloudExtensionCachesDiscoveryError(t *testing.T) {
	var probeCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apis" {
			t.Fatalf("path = %q, want /apis", r.URL.Path)
		}
		probeCount.Add(1)
		http.Error(w, "discovery failed", http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := NewCmdConnections(&Options{
		IOStreams:   IOStreams{Out: io.Discard, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"list"})

	for range 2 {
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "failed to probe deployment capabilities") {
			t.Fatalf("Execute() error = %v, want cached discovery failure", err)
		}
	}
	if got := probeCount.Load(); got != 1 {
		t.Fatalf("GET /apis calls = %d, want 1", got)
	}
}

func TestLifecycleActionLabel(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		action string
		want   string
	}{
		{name: "start", action: "start", want: "Start"},
		{name: "stop", action: "stop", want: "Stop"},
		{name: "restart", action: "restart", want: "Restart"},
		{name: "unknown", action: "pause", want: "pause"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := lifecycleActionLabel(tc.action); got != tc.want {
				t.Fatalf("lifecycleActionLabel(%q) = %q, want %q", tc.action, got, tc.want)
			}
		})
	}
}

func TestSplitPackageLocation(t *testing.T) {
	for _, tc := range []struct {
		location string
		wantFile string
		wantURL  string
	}{
		{location: ""},
		{location: "  "},
		{location: "./word-count.jar", wantFile: "./word-count.jar"},
		{location: " /tmp/archive.nar ", wantFile: "/tmp/archive.nar"},
		{location: "builtin://kafka", wantURL: "builtin://kafka"},
		{location: "http://example.com/fn.jar", wantURL: "http://example.com/fn.jar"},
		{location: "https://example.com/fn.jar", wantURL: "https://example.com/fn.jar"},
		{location: "file:///opt/fn.jar", wantURL: "file:///opt/fn.jar"},
		{location: "function://public/default/word-count@1", wantURL: "function://public/default/word-count@1"},
		{location: "sink://public/default/archive@1", wantURL: "sink://public/default/archive@1"},
		{location: "source://public/default/ingest@1", wantURL: "source://public/default/ingest@1"},
	} {
		file, url := splitPackageLocation(tc.location)
		if file != tc.wantFile || url != tc.wantURL {
			t.Errorf("splitPackageLocation(%q) = (%q, %q), want (%q, %q)", tc.location, file, url, tc.wantFile, tc.wantURL)
		}
	}
}
