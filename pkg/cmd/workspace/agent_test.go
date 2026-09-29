// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

func TestNewCmdWorkspaceIncludesAgent(t *testing.T) {
	t.Parallel()

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{}})
	if _, _, err := cmd.Find([]string{"agent"}); err != nil {
		t.Fatalf("Find(agent) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "archive"}); err != nil {
		t.Fatalf("Find(agent archive) error = %v", err)
	}
	if deleteCmd, _, err := cmd.Find([]string{"agent", "delete"}); err == nil && deleteCmd.Name() == "delete" {
		t.Fatalf("Find(agent delete) unexpectedly found delete command")
	}
	if _, _, err := cmd.Find([]string{"agent", "sessions"}); err != nil {
		t.Fatalf("Find(agent sessions) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "sessions", "outcome"}); err != nil {
		t.Fatalf("Find(agent sessions outcome) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "sessions", "resources"}); err != nil {
		t.Fatalf("Find(agent sessions resources) error = %v", err)
	}
	for _, args := range [][]string{
		{"agent", "sessions", "files", "list"},
		{"agent", "sessions", "files", "get"},
		{"agent", "sessions", "files", "content"},
		{"agent", "sessions", "files", "download"},
		{"agent", "sessions", "files", "delete"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
	}
	if _, _, err := cmd.Find([]string{"agent", "sessions", "events"}); err != nil {
		t.Fatalf("Find(agent sessions events) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "sessions", "events", "stream"}); err != nil {
		t.Fatalf("Find(agent sessions events stream) error = %v", err)
	}
	for _, args := range [][]string{
		{"agent", "sessions", "events", "send", "message"},
		{"agent", "sessions", "events", "send", "outcome"},
		{"agent", "sessions", "events", "send", "tool-confirmation"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
	}
	if _, _, err := cmd.Find([]string{"agent", "sessions", "threads"}); err != nil {
		t.Fatalf("Find(agent sessions threads) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "sessions", "threads", "events"}); err != nil {
		t.Fatalf("Find(agent sessions threads events) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "sessions", "threads", "events", "list"}); err != nil {
		t.Fatalf("Find(agent sessions threads events list) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "sessions", "threads", "events", "stream"}); err != nil {
		t.Fatalf("Find(agent sessions threads events stream) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "memory-stores"}); err != nil {
		t.Fatalf("Find(agent memory-stores) error = %v", err)
	}
	for _, args := range [][]string{
		{"agent", "memory-stores", "memories", "list"},
		{"agent", "memory-stores", "memories", "get"},
		{"agent", "memory-stores", "memories", "create"},
		{"agent", "memory-stores", "memories", "update"},
		{"agent", "memory-stores", "memories", "delete"},
		{"agent", "memory-versions", "list"},
		{"agent", "memory-versions", "get"},
		{"agent", "memory-versions", "redact"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
	}
	if _, _, err := cmd.Find([]string{"agent", "vaults"}); err != nil {
		t.Fatalf("Find(agent vaults) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "vaults", "credentials"}); err != nil {
		t.Fatalf("Find(agent vaults credentials) error = %v", err)
	}
	for _, args := range [][]string{
		{"agent", "vaults", "credentials", "list"},
		{"agent", "vaults", "credentials", "get"},
		{"agent", "vaults", "credentials", "create"},
		{"agent", "vaults", "credentials", "update"},
		{"agent", "vaults", "credentials", "delete"},
		{"agent", "vaults", "credentials", "archive"},
		{"agent", "vaults", "credentials", "validate"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
	}
	if _, _, err := cmd.Find([]string{"agent", "environments"}); err != nil {
		t.Fatalf("Find(agent environments) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "files"}); err != nil {
		t.Fatalf("Find(agent files) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "files", "content"}); err != nil {
		t.Fatalf("Find(agent files content) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "skills"}); err != nil {
		t.Fatalf("Find(agent skills) error = %v", err)
	}
	if _, _, err := cmd.Find([]string{"agent", "skills", "versions", "content"}); err != nil {
		t.Fatalf("Find(agent skills versions content) error = %v", err)
	}
	for _, args := range [][]string{
		{"agent", "providers"},
		{"agent", "provider"},
		{"agent", "providers", "list"},
		{"agent", "provider", "list"},
		{"agent", "providers", "get"},
		{"agent", "provider", "get"},
		{"agent", "triggers"},
		{"agent", "trigger"},
		{"agent", "triggers", "list"},
		{"agent", "triggers", "get"},
		{"agent", "triggers", "create"},
		{"agent", "triggers", "update"},
		{"agent", "triggers", "delete"},
		{"agent", "triggers", "pause"},
		{"agent", "triggers", "unpause"},
		{"agent", "triggers", "sessions"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
	}
}

func TestManagedAgentCommandFlagsFollowPortableContract(t *testing.T) {
	cmd := NewCmdAgent(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})

	for _, test := range []struct {
		args  []string
		flags []string
	}{
		{args: []string{"list"}, flags: []string{"provider", "metadata", "created-at-gte", "created-at-lte"}},
		{args: []string{"versions"}, flags: []string{"include-archived", "created-at-gte", "created-at-lte"}},
		{args: []string{"sessions", "list"}, flags: []string{
			"provider", "metadata", "agent-version", "created-at-gt", "created-at-gte", "created-at-lt", "created-at-lte",
			"deployment-id", "memory-store", "order", "status",
		}},
		{args: []string{"memory-stores", "list"}, flags: []string{"provider", "metadata", "created-at-gte", "created-at-lte"}},
		{args: []string{"memory-versions", "list"}, flags: []string{"session-id"}},
		{args: []string{"vaults", "list"}, flags: []string{"provider", "metadata"}},
		{args: []string{"environments", "list"}, flags: []string{"provider", "metadata"}},
		{args: []string{"files", "list"}, flags: []string{"provider", "metadata", "page", "include-archived", "scope-id"}},
		{args: []string{"skills", "list"}, flags: []string{"provider", "metadata", "include-archived", "source"}},
		{args: []string{"vaults", "credentials", "list"}, flags: []string{"metadata"}},
		{args: []string{"create"}, flags: []string{"provider", "beta-version"}},
		{args: []string{"sessions", "create"}, flags: []string{"provider", "beta-version"}},
		{args: []string{"memory-stores", "create"}, flags: []string{"provider", "beta-version"}},
		{args: []string{"vaults", "create"}, flags: []string{"provider", "beta-version"}},
		{args: []string{"environments", "create"}, flags: []string{"provider", "beta-version"}},
		{args: []string{"files", "create"}, flags: []string{"provider", "beta-version"}},
		{args: []string{"skills", "create"}, flags: []string{"provider", "beta-version"}},
		{args: []string{"sessions", "resources", "add"}, flags: []string{"beta-version"}},
		{args: []string{"sessions", "resources", "update"}, flags: []string{"beta-version"}},
		{args: []string{"skills", "versions", "create"}, flags: []string{"beta-version"}},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			subcommand, _, err := cmd.Find(test.args)
			if err != nil {
				t.Fatalf("Find(%v) error = %v", test.args, err)
			}
			for _, flag := range test.flags {
				if subcommand.Flags().Lookup(flag) != nil {
					t.Fatalf("Find(%v) unexpectedly exposes --%s", test.args, flag)
				}
			}
		})
	}

	filesList, _, err := cmd.Find([]string{"files", "list"})
	if err != nil {
		t.Fatalf("Find(files list) error = %v", err)
	}
	for _, flag := range []string{"limit", "after-id", "before-id"} {
		if filesList.Flags().Lookup(flag) == nil {
			t.Fatalf("files list is missing --%s", flag)
		}
	}

	vaultCredentialList, _, err := cmd.Find([]string{"vaults", "credentials", "list"})
	if err != nil {
		t.Fatalf("Find(vaults credentials list) error = %v", err)
	}
	if vaultCredentialList.Flags().Lookup("include-archived") == nil {
		t.Fatal("vaults credentials list does not expose --include-archived")
	}
}

func TestAgentProvidersListCommandForwardsAndRendersText(t *testing.T) {
	original := newWorkspaceProvidersClient
	defer func() {
		newWorkspaceProvidersClient = original
	}()

	called := false
	newWorkspaceProvidersClient = func() (workspaceProvidersClient, error) {
		return &workspaceProvidersClientMock{
			listFn: func(context.Context) ([]registry.AgentProviderInfo, error) {
				called = true
				return []registry.AgentProviderInfo{{
					Name:             "openai",
					Type:             "llm",
					APIVersion:       "v1",
					BetaVersion:      "managed-agents-2026-04-01",
					APIKeyEnv:        "OPENAI_API_KEY",
					APIKeyConfigured: true,
				}}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	o := &agentOptions{ioStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}}
	cmd := o.newAgentProviderListCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("List() was not called")
	}
	output := stdout.String()
	for _, expected := range []string{"openai", "OPENAI_API_KEY", "true"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output missing %q: %s", expected, output)
		}
	}
}

func TestAgentProvidersGetCommandForwardsAndRendersText(t *testing.T) {
	original := newWorkspaceProvidersClient
	defer func() {
		newWorkspaceProvidersClient = original
	}()

	var gotName string
	newWorkspaceProvidersClient = func() (workspaceProvidersClient, error) {
		return &workspaceProvidersClientMock{
			getFn: func(_ context.Context, name string) (*registry.AgentProviderInfo, error) {
				gotName = name
				return &registry.AgentProviderInfo{
					Name:             "openai",
					Type:             "llm",
					APIVersion:       "v1",
					APIKeyEnv:        "OPENAI_API_KEY",
					APIKeyConfigured: true,
				}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	o := &agentOptions{ioStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}}
	cmd := o.newAgentProviderGetCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"openai"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotName != "openai" {
		t.Fatalf("provider name = %q, want %q", gotName, "openai")
	}
	output := stdout.String()
	for _, expected := range []string{"openai", "OPENAI_API_KEY", "true"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output missing %q: %s", expected, output)
		}
	}
}

func TestAgentProvidersListCommandRejectsInvalidOutputBeforeClientCall(t *testing.T) {
	original := newWorkspaceProvidersClient
	defer func() {
		newWorkspaceProvidersClient = original
	}()

	newWorkspaceProvidersClient = func() (workspaceProvidersClient, error) {
		t.Fatal("newWorkspaceProvidersClient() should not be called when output is invalid")
		return nil, nil
	}

	o := &agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}
	cmd := o.newAgentProviderListCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--output", "xml"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--output must be one of: text, json, yaml") {
		t.Fatalf("Execute() error = %v, want output validation error", err)
	}
}

type workspaceProvidersClientMock struct {
	listFn func(context.Context) ([]registry.AgentProviderInfo, error)
	getFn  func(context.Context, string) (*registry.AgentProviderInfo, error)
}

func (m *workspaceProvidersClientMock) List(ctx context.Context) ([]registry.AgentProviderInfo, error) {
	if m.listFn == nil {
		return nil, nil
	}
	return m.listFn(ctx)
}

func (m *workspaceProvidersClientMock) Get(ctx context.Context, name string) (*registry.AgentProviderInfo, error) {
	if m.getFn == nil {
		return nil, nil
	}
	return m.getFn(ctx, name)
}

type workspaceManagedAgentsClientMock struct {
	getFn         func(context.Context, string) (interface{}, error)
	createFn      func(context.Context, string, interface{}) (interface{}, error)
	updateFn      func(context.Context, string, string, interface{}) (interface{}, error)
	deleteFn      func(context.Context, string) (interface{}, error)
	archiveFn     func(context.Context, string) (interface{}, error)
	doMultipartFn func(context.Context, string, string, registry.MultipartRequest) (interface{}, error)
	getToWriterFn func(context.Context, string, io.Writer) error
	getStreamFn   func(context.Context, string, string, func(io.Reader) error) error
}

func (m *workspaceManagedAgentsClientMock) Get(ctx context.Context, path string) (interface{}, error) {
	if m.getFn == nil {
		return nil, nil
	}
	return m.getFn(ctx, path)
}

func (m *workspaceManagedAgentsClientMock) Create(
	ctx context.Context,
	path string,
	payload interface{},
) (interface{}, error) {
	if m.createFn == nil {
		return nil, nil
	}
	return m.createFn(ctx, path, payload)
}

func (m *workspaceManagedAgentsClientMock) Update(
	ctx context.Context,
	method string,
	path string,
	payload interface{},
) (interface{}, error) {
	if m.updateFn == nil {
		return nil, nil
	}
	return m.updateFn(ctx, method, path, payload)
}

func (m *workspaceManagedAgentsClientMock) Delete(ctx context.Context, path string) (interface{}, error) {
	if m.deleteFn == nil {
		return nil, nil
	}
	return m.deleteFn(ctx, path)
}

func (m *workspaceManagedAgentsClientMock) Archive(ctx context.Context, path string) (interface{}, error) {
	if m.archiveFn == nil {
		return nil, nil
	}
	return m.archiveFn(ctx, path)
}

func (m *workspaceManagedAgentsClientMock) DoMultipart(
	ctx context.Context,
	method string,
	path string,
	payload registry.MultipartRequest,
) (interface{}, error) {
	if m.doMultipartFn == nil {
		return nil, nil
	}
	return m.doMultipartFn(ctx, method, path, payload)
}

func (m *workspaceManagedAgentsClientMock) GetToWriter(ctx context.Context, path string, writer io.Writer) error {
	if m.getToWriterFn == nil {
		return nil
	}
	return m.getToWriterFn(ctx, path, writer)
}

func (m *workspaceManagedAgentsClientMock) GetStream(
	ctx context.Context,
	path string,
	accept string,
	handle func(io.Reader) error,
) error {
	if m.getStreamFn == nil {
		return nil
	}
	return m.getStreamFn(ctx, path, accept, handle)
}

func TestBuildManagedAgentSessionCollectionPath(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "sessions", sessions: true}
	opts := agentListOptions{
		limit:           0,
		includeArchived: false,
	}
	cmd := &cobra.Command{Use: "list"}
	addManagedAgentListFlags(cmd, &opts, resource)
	if err := cmd.Flags().Set("agent", "agent-1"); err != nil {
		t.Fatalf("Set(agent) error = %v", err)
	}
	if err := cmd.Flags().Set("limit", "25"); err != nil {
		t.Fatalf("Set(limit) error = %v", err)
	}
	if err := cmd.Flags().Set("include-archived", "true"); err != nil {
		t.Fatalf("Set(include-archived) error = %v", err)
	}
	if err := cmd.Flags().Set("page", "page-1"); err != nil {
		t.Fatalf("Set(page) error = %v", err)
	}
	got, err := buildManagedAgentCollectionPath(cmd, resource, opts)
	if err != nil {
		t.Fatalf("buildManagedAgentCollectionPath() error = %v", err)
	}

	for _, want := range []string{
		"/v1/sessions?",
		"agent_id=agent-1",
		"limit=25",
		"page=page-1",
		"include_archived=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("path = %q, want substring %q", got, want)
		}
	}
}

func TestBuildManagedAgentFileCollectionPathUsesIDCursor(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "files", basePath: "/files", file: true}
	opts := agentListOptions{}
	cmd := &cobra.Command{Use: "list"}
	addManagedAgentListFlags(cmd, &opts, resource)
	for name, value := range map[string]string{
		"limit":     "25",
		"after-id":  "file-after",
		"before-id": "file-before",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	got, err := buildManagedAgentCollectionPath(cmd, resource, opts)
	if err != nil {
		t.Fatalf("buildManagedAgentCollectionPath() error = %v", err)
	}
	if want := "/v1/files?after_id=file-after&before_id=file-before&limit=25"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestAppendAgentVersionQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		version int
		want    string
	}{
		{
			name:    "adds version",
			path:    "/agents/agent%201%2Fchild",
			version: 2,
			want:    "/agents/agent%201%2Fchild?version=2",
		},
		{
			name:    "replaces existing version",
			path:    "/agents/agent-1?version=1&metadata_source=manual-test",
			version: 2,
			want:    "/agents/agent-1?metadata_source=manual-test&version=2",
		},
		{
			name:    "preserves fragment",
			path:    "/agents/agent-1?include_archived=true#details",
			version: 2,
			want:    "/agents/agent-1?include_archived=true&version=2#details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := appendAgentVersionQuery(tt.path, tt.version); got != tt.want {
				t.Fatalf("appendAgentVersionQuery() = %q, want %q", got, tt.want)
			}
		})
	}

}

// withTypedClientServer points the typed commands at an httptest server and
// returns the requests it received.
//
// The commands build their requests through the SDK now, so a test that asserts
// on a hand-built path string would only be checking a seam that no longer
// exists. Asserting on what actually arrives at a server checks the whole
// chain: the CLI's arguments, the SDK's path construction, and the escaping in
// between.
func withTypedClientServer(t *testing.T, handler http.HandlerFunc) *[]*http.Request {
	t.Helper()

	var requests []*http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clone := r.Clone(context.Background())
		requests = append(requests, clone)
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	original := newWorkspaceTypedClient
	t.Cleanup(func() { newWorkspaceTypedClient = original })
	newWorkspaceTypedClient = func() (*registry.Client, error) {
		return registry.NewClient(server.URL, "token", server.Client())
	}
	return &requests
}

func TestManagedAgentGetPathIncludesVersionForAgent(t *testing.T) {
	requests := withTypedClientServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"agent 1/child"}`))
	})

	resource := managedAgentResource{
		use:      "agent",
		singular: "agent",
		plural:   "agents",
		idName:   "agent-id",
		basePath: "/agents",
	}
	cmd := (&agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}).newManagedAgentGetCommand(resource)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent 1/child", "--version", "2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(*requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(*requests))
	}
	got := (*requests)[0]
	// The id is escaped whole: a slash in it must not become a path segment,
	// and a space must not end the path.
	if want := "/v1/agents/agent%201%2Fchild"; got.URL.EscapedPath() != want {
		t.Errorf("path = %q, want %q", got.URL.EscapedPath(), want)
	}
	if want := "2"; got.URL.Query().Get("version") != want {
		t.Errorf("version = %q, want %q", got.URL.Query().Get("version"), want)
	}
}

func TestManagedAgentGetRejectsInvalidVersionBeforeClientCall(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		t.Fatal("newWorkspaceManagedAgentsClient() should not be called when --version is invalid")
		return nil, nil
	}

	resource := managedAgentResource{
		use:      "agent",
		singular: "agent",
		plural:   "agents",
		idName:   "agent-id",
		basePath: "/agents",
	}
	cmd := (&agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}).newManagedAgentGetCommand(resource)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent-1", "--version", "0"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--version must be greater than 0") {
		t.Fatalf("Execute() error = %v, want version validation error", err)
	}
}

func TestManagedAgentGetVersionFlagIsAgentOnly(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{
		use:      "sessions",
		singular: "session",
		plural:   "sessions",
		idName:   "session-id",
	}
	cmd := (&agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}).newManagedAgentGetCommand(resource)
	if cmd.Flags().Lookup("version") != nil {
		t.Fatalf("sessions get unexpectedly exposes --version")
	}
}

func TestAgentVersionsCommandForwardsPagination(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getFn: func(_ context.Context, path string) (interface{}, error) {
				want := "/v1/agents/agent%2F1/versions?limit=25&page=page-1"
				if path != want {
					t.Fatalf("path = %q, want %q", path, want)
				}
				return map[string]interface{}{"data": []interface{}{}}, nil
			},
		}, nil
	}

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"agent", "versions", "agent/1",
		"--limit", "25",
		"--page", "page-1",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestSkillVersionGetUsesIntegerVersionPath(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getFn: func(_ context.Context, path string) (interface{}, error) {
				want := "/v1/skills/sk%201%2Fchild/versions/1"
				if path != want {
					t.Fatalf("path = %q, want %q", path, want)
				}
				return map[string]interface{}{"id": "sklv_123"}, nil
			},
		}, nil
	}

	cmd := (&agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}).newSkillVersionGetCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"sk 1/child", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestSkillVersionGetRejectsSkillVersionIDBeforeClientCall(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		t.Fatal("newWorkspaceManagedAgentsClient() should not be called when version is not an integer")
		return nil, nil
	}

	cmd := (&agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}).newSkillVersionGetCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"sk_123", "sklv_123"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "positive integer version number") || !strings.Contains(err.Error(), "sklv_123") {
		t.Fatalf("Execute() error = %v, want integer version validation error", err)
	}
}

func TestBuildSessionThreadEventPaths(t *testing.T) {
	t.Parallel()

	opts := agentSessionChildOptions{
		sessionID: "session/1",
		threadID:  "thread/1",
	}
	got, err := buildSessionThreadEventCollectionPath(opts, agentListOptions{
		limit: 25,
		page:  "page-1",
	})
	if err != nil {
		t.Fatalf("buildSessionThreadEventCollectionPath() error = %v", err)
	}
	for _, want := range []string{
		"/v1/sessions/session%2F1/threads/thread%2F1/events?",
		"limit=25",
		"page=page-1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("path = %q, want substring %q", got, want)
		}
	}

	opts.fromCursor = "cursor/1"
	opts.eventDeltas = []string{"agent.message", "agent.thinking"}
	streamPath, err := buildSessionThreadEventStreamPath(opts)
	if err != nil {
		t.Fatalf("buildSessionThreadEventStreamPath() error = %v", err)
	}
	wantStreamPath := "/v1/sessions/session%2F1/threads/thread%2F1/stream?event_deltas=agent.message&event_deltas=agent.thinking&from_cursor=cursor%2F1"
	if streamPath != wantStreamPath {
		t.Fatalf("stream path = %q, want %q", streamPath, wantStreamPath)
	}
}

func TestBuildSessionEventCollectionPathForwardsOpenAPIFilters(t *testing.T) {
	t.Parallel()

	got, err := buildSessionChildCollectionPath(
		agentSessionChildOptions{sessionID: "session/1"},
		"events",
		agentListOptions{
			limit:        25,
			page:         "page-1",
			createdAtGT:  "2026-08-01T00:00:00Z",
			createdAtGTE: "2026-08-02T00:00:00Z",
			createdAtLT:  "2026-08-30T00:00:00Z",
			createdAtLTE: "2026-08-31T00:00:00Z",
			order:        "asc",
			eventTypes:   []string{"user.message", "agent.message"},
			subpath:      "child/path",
		},
	)
	if err != nil {
		t.Fatalf("buildSessionChildCollectionPath() error = %v", err)
	}
	for _, want := range []string{
		"created_at%5Bgt%5D=2026-08-01T00%3A00%3A00Z",
		"created_at%5Bgte%5D=2026-08-02T00%3A00%3A00Z",
		"created_at%5Blt%5D=2026-08-30T00%3A00%3A00Z",
		"created_at%5Blte%5D=2026-08-31T00%3A00%3A00Z",
		"order=asc",
		"subpath=child%2Fpath",
		"types=user.message",
		"types=agent.message",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("path = %q, want substring %q", got, want)
		}
	}
}

func TestManagedAgentQueryRejectsInvalidEnums(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		opts agentListOptions
		want string
	}{
		{name: "order", opts: agentListOptions{order: "newest"}, want: "--order"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildManagedAgentQuery(nil, test.opts); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("buildManagedAgentQuery() error = %v, want %s validation error", err, test.want)
			}
		})
	}
}

func TestMemoryAndStreamQueriesRejectInvalidEnums(t *testing.T) {
	t.Parallel()

	if _, err := buildMemoryEntryItemPath(agentMemoryEntryOptions{memoryStoreID: "store-1", view: "expanded"}, "mem-1"); err == nil || !strings.Contains(err.Error(), "--view") {
		t.Fatalf("buildMemoryEntryItemPath() error = %v, want --view validation error", err)
	}
	if _, err := buildMemoryEntryCollectionPath(agentMemoryEntryOptions{memoryStoreID: "store-1", depth: "2"}); err == nil || !strings.Contains(err.Error(), "--depth") {
		t.Fatalf("buildMemoryEntryCollectionPath() error = %v, want --depth validation error", err)
	}
	if _, err := buildSessionEventStreamPath(agentSessionChildOptions{sessionID: "session-1", eventDeltas: []string{"tool.delta"}}); err == nil || !strings.Contains(err.Error(), "--event-delta") {
		t.Fatalf("buildSessionEventStreamPath() error = %v, want --event-delta validation error", err)
	}
}

func TestBuildMemoryAndFilePaths(t *testing.T) {
	t.Parallel()

	memoryOpts := agentMemoryEntryOptions{
		memoryStoreID: "store/1",
		limit:         25,
		page:          "page-1",
		depth:         "1",
		pathPrefix:    "notes/",
		view:          "full",
	}
	collectionPath, err := buildMemoryEntryCollectionPath(memoryOpts)
	if err != nil {
		t.Fatalf("buildMemoryEntryCollectionPath() error = %v", err)
	}
	for _, want := range []string{
		"/v1/memory_stores/store%2F1/memories?",
		"limit=25",
		"page=page-1",
		"depth=1",
		"path_prefix=notes%2F",
		"view=full",
	} {
		if !strings.Contains(collectionPath, want) {
			t.Fatalf("collection path = %q, want substring %q", collectionPath, want)
		}
	}

	itemPath, err := buildMemoryEntryItemPath(agentMemoryEntryOptions{memoryStoreID: memoryOpts.memoryStoreID}, "mem/1")
	if err != nil {
		t.Fatalf("buildMemoryEntryItemPath() error = %v", err)
	}
	if itemPath != "/v1/memory_stores/store%2F1/memories/mem%2F1" {
		t.Fatalf("item path = %q", itemPath)
	}

	viewPath, err := buildMemoryEntryItemPath(agentMemoryEntryOptions{memoryStoreID: "store/1", view: "full"}, "mem/1")
	if err != nil {
		t.Fatalf("buildMemoryEntryItemPath(view) error = %v", err)
	}
	if viewPath != "/v1/memory_stores/store%2F1/memories/mem%2F1?view=full" {
		t.Fatalf("view path = %q", viewPath)
	}

	deletePath, err := buildMemoryEntryDeletePath(agentMemoryEntryOptions{memoryStoreID: "store/1", expectedSHA: "abc"}, "mem/1")
	if err != nil {
		t.Fatalf("buildMemoryEntryDeletePath() error = %v", err)
	}
	if deletePath != "/v1/memory_stores/store%2F1/memories/mem%2F1?expected_content_sha256=abc" {
		t.Fatalf("delete path = %q", deletePath)
	}

	versionPath, err := buildMemoryVersionRedactPath(agentMemoryVersionOptions{memoryStoreID: "store/1"}, "ver/1")
	if err != nil {
		t.Fatalf("buildMemoryVersionRedactPath() error = %v", err)
	}
	if versionPath != "/v1/memory_stores/store%2F1/memory_versions/ver%2F1/redact" {
		t.Fatalf("version path = %q", versionPath)
	}

	versionCollectionPath, err := buildMemoryVersionCollectionPath(agentMemoryVersionOptions{
		memoryStoreID: "store/1",
		memoryID:      "memory-1",
		apiKeyID:      "key-1",
		operation:     "modified",
		createdAtGTE:  "2026-08-01T00:00:00Z",
		createdAtLTE:  "2026-08-31T00:00:00Z",
		view:          "basic",
		limit:         25,
		page:          "page-1",
	})
	if err != nil {
		t.Fatalf("buildMemoryVersionCollectionPath() error = %v", err)
	}
	for _, want := range []string{
		"api_key_id=key-1",
		"created_at%5Bgte%5D=2026-08-01T00%3A00%3A00Z",
		"created_at%5Blte%5D=2026-08-31T00%3A00%3A00Z",
		"memory_id=memory-1",
		"operation=modified",
		"view=basic",
	} {
		if !strings.Contains(versionCollectionPath, want) {
			t.Fatalf("version collection path = %q, want substring %q", versionCollectionPath, want)
		}
	}

	filePath, err := buildFileContentPath("file/1")
	if err != nil {
		t.Fatalf("buildFileContentPath() error = %v", err)
	}
	if filePath != "/v1/files/file%2F1/content" {
		t.Fatalf("file path = %q", filePath)
	}

	sessionFilePath, err := buildSessionFileContentPath(agentSessionChildOptions{sessionID: "session/1"}, "file/1")
	if err != nil {
		t.Fatalf("buildSessionFileContentPath() error = %v", err)
	}
	if sessionFilePath != "/v1/sessions/session%2F1/files/file%2F1/content" {
		t.Fatalf("session file path = %q", sessionFilePath)
	}
}

func TestBuildMemoryEntryPayload(t *testing.T) {
	t.Parallel()

	payload, err := buildMemoryEntryPayload(agentMemoryEntryOptions{path: "notes/todo.md", content: "hello"}, false)
	if err != nil {
		t.Fatalf("buildMemoryEntryPayload() error = %v", err)
	}
	if payload["path"] != "notes/todo.md" || payload["content"] != "hello" {
		t.Fatalf("payload = %#v", payload)
	}
	payload, err = buildMemoryEntryPayload(agentMemoryEntryOptions{contentJSON: `{"path":"notes/todo.md","content":null}`}, false)
	if err != nil {
		t.Fatalf("buildMemoryEntryPayload(json) error = %v", err)
	}
	if payload["path"] != "notes/todo.md" {
		t.Fatalf("json payload = %#v", payload)
	}

	payload, err = buildMemoryEntryPayload(agentMemoryEntryOptions{
		path:            "notes/done.md",
		preconditionSHA: "abc123",
	}, true)
	if err != nil {
		t.Fatalf("buildMemoryEntryPayload(update) error = %v", err)
	}
	precondition, ok := payload["precondition"].(map[string]interface{})
	if payload["path"] != "notes/done.md" || !ok || precondition["type"] != "content_sha256" || precondition["content_sha256"] != "abc123" {
		t.Fatalf("update payload = %#v", payload)
	}
}

func TestBuildMemoryEntryCreatePayloadRequiresPath(t *testing.T) {
	t.Parallel()

	_, err := buildMemoryEntryPayload(agentMemoryEntryOptions{content: "hello"}, false)
	if err == nil || !strings.Contains(err.Error(), "--path is required") {
		t.Fatalf("buildMemoryEntryPayload() error = %v, want path validation", err)
	}
}

func TestMemoryEntryCreateCommandForwardsPayload(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			createFn: func(_ context.Context, path string, payload interface{}) (interface{}, error) {
				if path != "/v1/memory_stores/store-1/memories" {
					t.Fatalf("path = %q, want /v1/memory_stores/store-1/memories", path)
				}
				payloadMap, ok := payload.(map[string]interface{})
				if !ok || payloadMap["path"] != "notes/todo.md" || payloadMap["content"] != "hello" {
					t.Fatalf("payload = %#v", payload)
				}
				return map[string]interface{}{"id": "mem-1"}, nil
			},
		}, nil
	}

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "memory-stores", "memories", "create", "--memory-store", "store-1", "--path", "notes/todo.md", "--content", "hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestMemoryEntryWriteCommandsRejectInvalidOutput(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		t.Fatal("newWorkspaceManagedAgentsClient() should not be called when output is invalid")
		return nil, nil
	}

	for _, args := range [][]string{
		{"agent", "memory-stores", "memories", "create", "--memory-store", "store-1", "--path", "notes/todo.md", "--content", "hello", "--output", "jsno"},
		{"agent", "memory-stores", "memories", "update", "mem-1", "--memory-store", "store-1", "--content", "hello", "--output", "jsno"},
	} {
		cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--output must be one of") {
			t.Fatalf("Execute(%v) error = %v, want output validation error", args, err)
		}
	}
}

func TestMemoryEntryGetCommandForwardsView(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getFn: func(_ context.Context, path string) (interface{}, error) {
				if path != "/v1/memory_stores/store-1/memories/mem-1?view=full" {
					t.Fatalf("path = %q, want /v1/memory_stores/store-1/memories/mem-1?view=full", path)
				}
				return map[string]interface{}{"id": "mem-1"}, nil
			},
		}, nil
	}

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "memory-stores", "memories", "get", "mem-1", "--memory-store", "store-1", "--view", "full"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestMemoryEntryDeleteCommandForwardsExpectedSHA(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			deleteFn: func(_ context.Context, path string) (interface{}, error) {
				if path != "/v1/memory_stores/store-1/memories/mem-1?expected_content_sha256=abc" {
					t.Fatalf("path = %q, want /v1/memory_stores/store-1/memories/mem-1?expected_content_sha256=abc", path)
				}
				return map[string]interface{}{"id": "mem-1"}, nil
			},
		}, nil
	}

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "memory-stores", "memories", "delete", "mem-1", "--memory-store", "store-1", "--expected-content-sha256", "abc"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestMemoryVersionRedactCommandUsesPostAction(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			archiveFn: func(_ context.Context, path string) (interface{}, error) {
				if path != "/v1/memory_stores/store-1/memory_versions/ver-1/redact" {
					t.Fatalf("path = %q, want /v1/memory_stores/store-1/memory_versions/ver-1/redact", path)
				}
				return map[string]interface{}{"id": "ver-1"}, nil
			},
		}, nil
	}

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "memory-versions", "redact", "ver-1", "--memory-store", "store-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestFileContentCommandDownloadsToOutputFile(t *testing.T) {
	original := newWorkspaceManagedAgentsStreamClient
	defer func() {
		newWorkspaceManagedAgentsStreamClient = original
	}()

	newWorkspaceManagedAgentsStreamClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getToWriterFn: func(_ context.Context, path string, writer io.Writer) error {
				if path != "/v1/files/file-1/content" {
					t.Fatalf("path = %q, want /v1/files/file-1/content", path)
				}
				_, err := writer.Write([]byte("file bytes"))
				return err
			},
		}, nil
	}

	outputPath := filepath.Join(t.TempDir(), "download.txt")
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "files", "content", "file-1", "--output-file", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "file bytes" {
		t.Fatalf("content = %q", string(content))
	}
}

func TestSessionOutcomeCommandUsesCorePath(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() { newWorkspaceManagedAgentsClient = original }()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getFn: func(_ context.Context, path string) (interface{}, error) {
				if path != "/v1/sessions/session%2F1/outcome" {
					t.Fatalf("path = %q, want session outcome path", path)
				}
				return map[string]interface{}{"type": "outcome_evaluation", "result": "pass"}, nil
			},
		}, nil
	}

	var output bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &output, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "sessions", "outcome", "session/1", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(output.String(), `"result": "pass"`) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestSkillVersionContentCommandDownloadsBundle(t *testing.T) {
	original := newWorkspaceManagedAgentsStreamClient
	defer func() { newWorkspaceManagedAgentsStreamClient = original }()

	newWorkspaceManagedAgentsStreamClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getToWriterFn: func(_ context.Context, path string, writer io.Writer) error {
				if path != "/v1/skills/skill%2F1/versions/latest/content" {
					t.Fatalf("path = %q, want skill version content path", path)
				}
				_, err := writer.Write([]byte("zip bytes"))
				return err
			},
		}, nil
	}

	var output bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &output, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "skills", "versions", "content", "skill/1", "latest"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.String() != "zip bytes" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestBuildCoreSingletonPaths(t *testing.T) {
	t.Parallel()

	outcomePath, err := buildSessionOutcomePath("session/1")
	if err != nil || outcomePath != "/v1/sessions/session%2F1/outcome" {
		t.Fatalf("outcome path = %q, error = %v", outcomePath, err)
	}
	contentPath, err := buildSkillVersionContentPath("skill/1", "version/1")
	if err != nil || contentPath != "/v1/skills/skill%2F1/versions/version%2F1/content" {
		t.Fatalf("content path = %q, error = %v", contentPath, err)
	}
}

func TestManagedAgentFileListHelpOmitsIncludeArchived(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "files", "list", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(stdout.String(), "--include-archived") {
		t.Fatalf("help unexpectedly contains --include-archived:\n%s", stdout.String())
	}
}

func TestSessionFileMetadataCommandsUseScopedPaths(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	var gotPaths []string
	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getFn: func(_ context.Context, path string) (interface{}, error) {
				gotPaths = append(gotPaths, "GET "+path)
				return map[string]interface{}{"id": "file-1"}, nil
			},
			deleteFn: func(_ context.Context, path string) (interface{}, error) {
				gotPaths = append(gotPaths, "DELETE "+path)
				return map[string]interface{}{"deleted": true}, nil
			},
		}, nil
	}

	for _, args := range [][]string{
		{"agent", "sessions", "files", "list", "--session", "session/1", "--limit", "25", "--after-id", "file-0", "--before-id", "file-9"},
		{"agent", "sessions", "files", "get", "file/1", "--session", "session/1"},
		{"agent", "sessions", "files", "delete", "file/1", "--session", "session/1"},
	} {
		cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
	}

	want := []string{
		"GET /v1/sessions/session%2F1/files?after_id=file-0&before_id=file-9&limit=25",
		"GET /v1/sessions/session%2F1/files/file%2F1",
		"DELETE /v1/sessions/session%2F1/files/file%2F1",
	}
	if !reflect.DeepEqual(gotPaths, want) {
		t.Fatalf("paths = %#v, want %#v", gotPaths, want)
	}
}

func TestSessionFileContentCommandDownloadsScopedFile(t *testing.T) {
	original := newWorkspaceManagedAgentsStreamClient
	defer func() {
		newWorkspaceManagedAgentsStreamClient = original
	}()

	newWorkspaceManagedAgentsStreamClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getToWriterFn: func(_ context.Context, path string, writer io.Writer) error {
				if path != "/v1/sessions/session%2F1/files/file%2F1/content" {
					t.Fatalf("path = %q, want scoped session file content path", path)
				}
				_, err := writer.Write([]byte("session file bytes"))
				return err
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "sessions", "files", "content", "file/1", "--session", "session/1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout.String() != "session file bytes" {
		t.Fatalf("content = %q, want %q", stdout.String(), "session file bytes")
	}
}

func TestFileContentCommandOverwritesOutputFile(t *testing.T) {
	original := newWorkspaceManagedAgentsStreamClient
	defer func() {
		newWorkspaceManagedAgentsStreamClient = original
	}()

	newWorkspaceManagedAgentsStreamClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getToWriterFn: func(_ context.Context, _ string, writer io.Writer) error {
				_, err := writer.Write([]byte("new file bytes"))
				return err
			},
		}, nil
	}

	outputPath := filepath.Join(t.TempDir(), "download.txt")
	if err := os.WriteFile(outputPath, []byte("old file bytes"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "files", "content", "file-1", "--output-file", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "new file bytes" {
		t.Fatalf("content = %q", string(content))
	}
}

func TestBuildVaultCredentialCollectionPath(t *testing.T) {
	t.Parallel()

	got, err := buildVaultCredentialCollectionPath(vaultCredentialOptions{
		vaultID:         "vault-1",
		limit:           25,
		page:            "page-1",
		includeArchived: true,
	})
	if err != nil {
		t.Fatalf("buildVaultCredentialCollectionPath() error = %v", err)
	}

	for _, want := range []string{
		"/v1/vaults/vault-1/credentials?",
		"limit=25",
		"page=page-1",
		"include_archived=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("path = %q, want substring %q", got, want)
		}
	}
}

func TestVaultCredentialListHelpMatchesOpenAPI(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "vaults", "credentials", "list", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "--include-archived") {
		t.Fatalf("help does not contain --include-archived:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "--metadata") {
		t.Fatalf("help unexpectedly contains --metadata:\n%s", stdout.String())
	}
}

func TestBuildVaultCredentialActionPath(t *testing.T) {
	t.Parallel()

	got, err := buildVaultCredentialActionPath(vaultCredentialOptions{vaultID: "vault 1"}, "credential/1", "archive")
	if err != nil {
		t.Fatalf("buildVaultCredentialActionPath() error = %v", err)
	}
	want := "/v1/vaults/vault%201/credentials/credential%2F1/archive"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestBuildVaultCredentialCreatePayload(t *testing.T) {
	t.Parallel()

	opts := vaultCredentialOptions{output: "text"}
	cmd := &cobra.Command{Use: "create"}
	addVaultCredentialPayloadFlags(cmd, &opts, false)
	for name, value := range map[string]string{
		"display-name": "test credential",
		"auth-json":    `{"type":"static_bearer","mcp_server_url":"https://example.com/mcp","token":"test-token"}`,
		"metadata":     "source=opsclaw",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	payload, err := buildVaultCredentialPayload(cmd, opts, false)
	if err != nil {
		t.Fatalf("buildVaultCredentialPayload() error = %v", err)
	}

	if payload["display_name"] != "test credential" {
		t.Fatalf("display_name = %#v", payload["display_name"])
	}
	auth, ok := payload["auth"].(map[string]interface{})
	if !ok || auth["type"] != "static_bearer" || auth["token"] != "test-token" {
		t.Fatalf("auth = %#v", payload["auth"])
	}
	if !reflect.DeepEqual(payload["metadata"], map[string]string{"source": "opsclaw"}) {
		t.Fatalf("metadata = %#v", payload["metadata"])
	}
}

func TestBuildAgentTriggerCreatePayloadFromFlags(t *testing.T) {
	t.Parallel()

	opts := agentTriggerOptions{output: "text", sessionMode: "SESSION_PER_TOPIC", sourceType: "pulsar"}
	cmd := &cobra.Command{Use: "create"}
	addAgentTriggerPayloadFlags(cmd, &opts, false)
	for name, value := range map[string]string{
		"name":                "orders-trigger",
		"agent":               "agent-1",
		"agent-version":       "3",
		"session-mode":        "SESSION_PER_KEY",
		"source-type":         "KAFKA",
		"connection":          "kafka-1",
		"topic":               "orders",
		"subscription-name":   "orders-sub",
		"type-class-name":     "com.example.Order",
		"schema-type":         "JSON",
		"consumer-config":     "auto.offset.reset=earliest",
		"input-schema-config": `orders={"type":"avro","subject":"orders-value","version":2}`,
		"environment-id":      "env-1",
		"title-template":      "Order ${payload}",
		"metadata":            "source=manual-test",
		"vault-id":            "vault-1",
		"replicas":            "2",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	payload, err := buildAgentTriggerPayload(cmd, opts, false)
	if err != nil {
		t.Fatalf("buildAgentTriggerPayload() error = %v", err)
	}

	if payload["name"] != "orders-trigger" {
		t.Fatalf("name = %#v", payload["name"])
	}
	agentRef, ok := payload["agent"].(map[string]interface{})
	if !ok || agentRef["type"] != "agent" || agentRef["id"] != "agent-1" || agentRef["version"] != 3 {
		t.Fatalf("agent = %#v", payload["agent"])
	}
	if payload["session_mode"] != "SESSION_PER_KEY" {
		t.Fatalf("session_mode = %#v", payload["session_mode"])
	}
	source, ok := payload["source"].(map[string]interface{})
	if !ok || source["type"] != "kafka" || source["connection"] != "kafka-1" {
		t.Fatalf("source = %#v", payload["source"])
	}
	if !reflect.DeepEqual(source["topics"], []string{"orders"}) {
		t.Fatalf("source.topics = %#v", source["topics"])
	}
	if source["subscription_name"] != "orders-sub" {
		t.Fatalf("source.subscription_name = %#v", source["subscription_name"])
	}
	if source["type_class_name"] != "com.example.Order" {
		t.Fatalf("source.type_class_name = %#v", source["type_class_name"])
	}
	if source["schema_type"] != "JSON" {
		t.Fatalf("source.schema_type = %#v", source["schema_type"])
	}
	if !reflect.DeepEqual(source["consumer_additional_config"], map[string]interface{}{"auto.offset.reset": "earliest"}) {
		t.Fatalf("source.consumer_additional_config = %#v", source["consumer_additional_config"])
	}
	if !reflect.DeepEqual(source["input_schema_configs"], map[string]map[string]interface{}{
		"orders": {"type": "avro", "subject": "orders-value", "version": float64(2)},
	}) {
		t.Fatalf("source.input_schema_configs = %#v", source["input_schema_configs"])
	}
	session, ok := payload["session"].(map[string]interface{})
	if !ok || session["environment_id"] != "env-1" {
		t.Fatalf("session = %#v", payload["session"])
	}
	if session["title_template"] != "Order ${payload}" {
		t.Fatalf("session.title_template = %#v", session["title_template"])
	}
	if !reflect.DeepEqual(session["metadata"], map[string]string{"source": "manual-test"}) {
		t.Fatalf("session.metadata = %#v", session["metadata"])
	}
	if !reflect.DeepEqual(session["vault_ids"], []string{"vault-1"}) {
		t.Fatalf("session.vault_ids = %#v", session["vault_ids"])
	}
	if payload["replicas"] != 2 {
		t.Fatalf("replicas = %#v", payload["replicas"])
	}
}

func TestBuildAgentTriggerPayloadFromConfigJSON(t *testing.T) {
	t.Parallel()

	opts := agentTriggerOptions{
		configJSON: `{"name":"json-trigger","agent":{"type":"agent","id":"agent-1"},"source":{"type":"pulsar"}}`,
	}
	payload, err := buildAgentTriggerPayload(&cobra.Command{Use: "create"}, opts, false)
	if err != nil {
		t.Fatalf("buildAgentTriggerPayload() error = %v", err)
	}
	if payload["name"] != "json-trigger" {
		t.Fatalf("name = %#v", payload["name"])
	}
	if _, ok := payload["agent"].(map[string]interface{}); !ok {
		t.Fatalf("agent = %#v", payload["agent"])
	}
}

func TestBuildAgentTriggerCreatePayloadValidatesRequiredFields(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		flags   map[string]string
		wantErr string
	}{
		{name: "name", flags: map[string]string{}, wantErr: "--name is required"},
		{name: "agent", flags: map[string]string{"name": "trigger"}, wantErr: "--agent is required"},
		{name: "environment", flags: map[string]string{"name": "trigger", "agent": "agent-1"}, wantErr: "--environment-id is required"},
		{name: "connection", flags: map[string]string{"name": "trigger", "agent": "agent-1", "environment-id": "env-1"}, wantErr: "--connection is required"},
		{name: "topic", flags: map[string]string{"name": "trigger", "agent": "agent-1", "environment-id": "env-1", "connection": "conn-1"}, wantErr: "exactly one of --topic or --topic-pattern is required"},
		{name: "topic conflict", flags: map[string]string{"name": "trigger", "agent": "agent-1", "environment-id": "env-1", "connection": "conn-1", "topic": "orders", "topic-pattern": "orders-.*"}, wantErr: "cannot be used together"},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts := newAgentTriggerOptions()
			cmd := &cobra.Command{Use: "create"}
			addAgentTriggerPayloadFlags(cmd, &opts, false)
			for name, value := range test.flags {
				if err := cmd.Flags().Set(name, value); err != nil {
					t.Fatalf("Set(%s) error = %v", name, err)
				}
			}
			_, err := buildAgentTriggerPayload(cmd, opts, false)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("buildAgentTriggerPayload() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestBuildCronAgentTriggerPayload(t *testing.T) {
	t.Parallel()

	opts := newAgentTriggerOptions()
	cmd := &cobra.Command{Use: "create"}
	addAgentTriggerPayloadFlags(cmd, &opts, false)
	for name, value := range map[string]string{
		"name":           "daily-trigger",
		"agent":          "agent-1",
		"source-type":    "cron",
		"session-mode":   "SHARED",
		"schedule":       "0 9 * * *",
		"timezone":       "UTC",
		"payload":        `{"task":"daily"}`,
		"environment-id": "env-1",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}
	payload, err := buildAgentTriggerPayload(cmd, opts, false)
	if err != nil {
		t.Fatalf("buildAgentTriggerPayload() error = %v", err)
	}
	source := payload["source"].(map[string]interface{})
	if source["type"] != "cron" || source["schedule"] != "0 9 * * *" || source["timezone"] != "UTC" {
		t.Fatalf("source = %#v", source)
	}
}

func TestBuildAgentTriggerUpdateOmitsAgentFields(t *testing.T) {
	t.Parallel()

	opts := newAgentTriggerOptions()
	cmd := &cobra.Command{Use: "update"}
	addAgentTriggerPayloadFlags(cmd, &opts, true)
	if cmd.Flags().Lookup("agent") != nil || cmd.Flags().Lookup("agent-version") != nil {
		t.Fatalf("update command exposes immutable agent flags")
	}
	if cmd.Flags().Lookup("source-type") == nil {
		t.Fatal("update command does not expose the required source discriminator")
	}
	if cmd.Flags().Lookup("paused") != nil {
		t.Fatal("update command exposes paused instead of using pause and unpause actions")
	}
	if err := cmd.Flags().Set("name", "updated-trigger"); err != nil {
		t.Fatalf("Set(name) error = %v", err)
	}
	payload, err := buildAgentTriggerPayload(cmd, opts, true)
	if err != nil {
		t.Fatalf("buildAgentTriggerPayload() error = %v", err)
	}
	if !reflect.DeepEqual(payload, map[string]interface{}{"name": "updated-trigger"}) {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestBuildAgentTriggerUpdateSourceRequiresType(t *testing.T) {
	t.Parallel()

	opts := newAgentTriggerOptions()
	cmd := &cobra.Command{Use: "update"}
	addAgentTriggerPayloadFlags(cmd, &opts, true)
	if err := cmd.Flags().Set("schedule", "30 8 * * 1-5"); err != nil {
		t.Fatalf("Set(schedule) error = %v", err)
	}
	if _, err := buildAgentTriggerPayload(cmd, opts, true); err == nil ||
		!strings.Contains(err.Error(), "--source-type is required") {
		t.Fatalf("buildAgentTriggerPayload() error = %v, want source type requirement", err)
	}
	if err := cmd.Flags().Set("source-type", "cron"); err != nil {
		t.Fatalf("Set(source-type) error = %v", err)
	}
	payload, err := buildAgentTriggerPayload(cmd, opts, true)
	if err != nil {
		t.Fatalf("buildAgentTriggerPayload() error = %v", err)
	}
	if !reflect.DeepEqual(payload["source"], map[string]interface{}{
		"type":     "cron",
		"schedule": "30 8 * * 1-5",
	}) {
		t.Fatalf("source = %#v", payload["source"])
	}
}

func TestBuildAgentTriggerPayloadRejectsScalarInputSchemaConfig(t *testing.T) {
	t.Parallel()

	opts := newAgentTriggerOptions()
	cmd := &cobra.Command{Use: "create"}
	addAgentTriggerPayloadFlags(cmd, &opts, false)
	for name, value := range map[string]string{
		"name":                "trigger",
		"agent":               "agent-1",
		"environment-id":      "env-1",
		"connection":          "conn-1",
		"topic":               "orders",
		"input-schema-config": "orders=orders-value",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}
	_, err := buildAgentTriggerPayload(cmd, opts, false)
	if err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("buildAgentTriggerPayload() error = %v, want JSON object error", err)
	}
}

func TestBuildAgentTriggerPayloadRejectsTypeClassDefinitionConflict(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "type.py")
	if err := os.WriteFile(file, []byte("class Event: pass"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	opts := newAgentTriggerOptions()
	cmd := &cobra.Command{Use: "create"}
	addAgentTriggerPayloadFlags(cmd, &opts, false)
	for name, value := range map[string]string{
		"name":                       "trigger",
		"agent":                      "agent-1",
		"environment-id":             "env-1",
		"connection":                 "pulsar-1",
		"topic":                      "orders",
		"type-class-definition":      "class Inline: pass",
		"type-class-definition-file": file,
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	_, err := buildAgentTriggerPayload(cmd, opts, false)
	if err == nil || !strings.Contains(err.Error(), "--type-class-definition and --type-class-definition-file cannot be used together") {
		t.Fatalf("buildAgentTriggerPayload() error = %v, want type class definition conflict", err)
	}
}

func TestBuildAgentTriggerPaths(t *testing.T) {
	t.Parallel()

	list, err := buildAgentTriggerCollectionPath(nil, agentListOptions{
		limit:   10,
		page:    "page-1",
		agentID: "agent/1",
	})
	if err != nil {
		t.Fatalf("buildAgentTriggerCollectionPath() error = %v", err)
	}
	for _, want := range []string{"/v1/triggers?", "agent_id=agent%2F1", "limit=10", "page=page-1"} {
		if !strings.Contains(list, want) {
			t.Fatalf("list path = %q, want substring %q", list, want)
		}
	}

	item, err := buildAgentTriggerItemPath("trigger/1")
	if err != nil {
		t.Fatalf("buildAgentTriggerItemPath() error = %v", err)
	}
	if item != "/v1/triggers/trigger%2F1" {
		t.Fatalf("item path = %q", item)
	}
	action, err := buildAgentTriggerActionPath("trigger/1", "pause")
	if err != nil {
		t.Fatalf("buildAgentTriggerActionPath() error = %v", err)
	}
	if action != "/v1/triggers/trigger%2F1/pause" {
		t.Fatalf("action path = %q", action)
	}
	sessions, err := buildAgentTriggerSessionsPath("trigger/1", agentListOptions{
		limit:           10,
		page:            "page-1",
		includeArchived: true,
	})
	if err != nil {
		t.Fatalf("buildAgentTriggerSessionsPath() error = %v", err)
	}
	for _, want := range []string{"/v1/triggers/trigger%2F1/sessions?", "include_archived=true", "limit=10", "page=page-1"} {
		if !strings.Contains(sessions, want) {
			t.Fatalf("sessions path = %q, want substring %q", sessions, want)
		}
	}
}

func TestAgentTriggerCreateCommandUsesCorePath(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	var gotPath string
	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			createFn: func(_ context.Context, path string, _ interface{}) (interface{}, error) {
				gotPath = path
				return map[string]interface{}{"id": "trigger-1"}, nil
			},
		}, nil
	}

	o := &agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}
	cmd := o.newAgentTriggerCreateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--name", "trigger-1", "--agent", "agent-1", "--connection", "kafka-1", "--topic", "events", "--environment-id", "env-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotPath != "/v1/triggers" {
		t.Fatalf("path = %q, want %q", gotPath, "/v1/triggers")
	}
}

func TestAgentTriggerUpdateCommandUsesPost(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	var gotMethod string
	var gotPath string
	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			updateFn: func(_ context.Context, method string, path string, _ interface{}) (interface{}, error) {
				gotMethod = method
				gotPath = path
				return map[string]interface{}{"id": "trigger-1"}, nil
			},
		}, nil
	}

	o := &agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}
	cmd := o.newAgentTriggerUpdateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"trigger-1", "--name", "updated"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/triggers/trigger-1" {
		t.Fatalf("request = %s %s, want POST /v1/triggers/trigger-1", gotMethod, gotPath)
	}
}

func TestBuildVaultCredentialUpdatePayload(t *testing.T) {
	t.Parallel()

	opts := vaultCredentialOptions{output: "text"}
	cmd := &cobra.Command{Use: "update"}
	addVaultCredentialPayloadFlags(cmd, &opts, true)
	if err := cmd.Flags().Set("display-name", "updated credential"); err != nil {
		t.Fatalf("Set(display-name) error = %v", err)
	}

	payload, err := buildVaultCredentialPayload(cmd, opts, true)
	if err != nil {
		t.Fatalf("buildVaultCredentialPayload() error = %v", err)
	}

	if !reflect.DeepEqual(payload, map[string]interface{}{"display_name": "updated credential"}) {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestBuildVaultCredentialUpdatePayloadIncludesMetadata(t *testing.T) {
	t.Parallel()

	opts := vaultCredentialOptions{output: "text"}
	cmd := &cobra.Command{Use: "update"}
	addVaultCredentialPayloadFlags(cmd, &opts, true)
	if err := cmd.Flags().Set("metadata", "source=opsclaw"); err != nil {
		t.Fatalf("Set(metadata) error = %v", err)
	}

	payload, err := buildVaultCredentialPayload(cmd, opts, true)
	if err != nil {
		t.Fatalf("buildVaultCredentialPayload() error = %v", err)
	}

	if !reflect.DeepEqual(payload, map[string]interface{}{
		"metadata": map[string]string{"source": "opsclaw"},
	}) {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestBuildManagedAgentSessionCreatePayloadIncludesAgent(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{sessions: true}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	addSessionAgentFlag(cmd, resource)
	if err := cmd.Flags().Set("agent", "agent-1"); err != nil {
		t.Fatalf("Set(agent) error = %v", err)
	}
	if err := cmd.Flags().Set("environment-id", "env-1"); err != nil {
		t.Fatalf("Set(environment-id) error = %v", err)
	}
	if err := cmd.Flags().Set("title", "test session"); err != nil {
		t.Fatalf("Set(title) error = %v", err)
	}
	if err := cmd.Flags().Set("vault-id", "vault-1"); err != nil {
		t.Fatalf("Set(vault-id) error = %v", err)
	}
	if err := cmd.Flags().Set("vault-id", "vault-2"); err != nil {
		t.Fatalf("Set(vault-id) error = %v", err)
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, false)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	if payload["agent"] != "agent-1" {
		t.Fatalf("agent = %#v", payload["agent"])
	}
	if payload["environment_id"] != "env-1" {
		t.Fatalf("environment_id = %#v", payload["environment_id"])
	}
	if payload["title"] != "test session" {
		t.Fatalf("title = %#v", payload["title"])
	}
	if !reflect.DeepEqual(payload["vault_ids"], []string{"vault-1", "vault-2"}) {
		t.Fatalf("vault_ids = %#v", payload["vault_ids"])
	}
}

func TestBuildManagedAgentSessionUpdatePayloadIncludesVaultIDs(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{sessions: true}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "update"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, true)
	addSessionAgentFlag(cmd, resource, true)
	if err := cmd.Flags().Set("vault-id", "vault-1"); err != nil {
		t.Fatalf("Set(vault-id) error = %v", err)
	}
	if err := cmd.Flags().Set("agent-json", `{"tools":[]}`); err != nil {
		t.Fatalf("Set(agent-json) error = %v", err)
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, true)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	agent, ok := payload["agent"].(map[string]interface{})
	if !ok || !reflect.DeepEqual(agent["tools"], []interface{}{}) {
		t.Fatalf("agent = %#v", payload["agent"])
	}
	if !reflect.DeepEqual(payload["vault_ids"], []string{"vault-1"}) {
		t.Fatalf("vault_ids = %#v", payload["vault_ids"])
	}
}

func TestBuildManagedAgentSessionUpdatePayloadIncludesAgentRef(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{sessions: true}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "update"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, true)
	addSessionAgentFlag(cmd, resource, true)
	if err := cmd.Flags().Set("agent", "agent-2"); err != nil {
		t.Fatalf("Set(agent) error = %v", err)
	}
	if err := cmd.Flags().Set("agent-version", "3"); err != nil {
		t.Fatalf("Set(agent-version) error = %v", err)
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, true)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	wantAgent := map[string]interface{}{"type": "agent", "id": "agent-2", "version": 3}
	if !reflect.DeepEqual(payload["agent"], wantAgent) {
		t.Fatalf("agent = %#v", payload["agent"])
	}
}

func TestBuildManagedAgentSessionCreatePayloadSupportsAgentOverridesAndInitialEvents(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{sessions: true}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	addSessionAgentFlag(cmd, resource)
	for name, value := range map[string]string{
		"environment-id":     "env-1",
		"agent-json":         `{"type":"agent_with_overrides","id":"agent-1","system":"Be concise"}`,
		"initial-event-json": `{"type":"user.message","content":[{"type":"text","text":"hello"}]}`,
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, false)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	agent, ok := payload["agent"].(map[string]interface{})
	if !ok || agent["type"] != "agent_with_overrides" || agent["id"] != "agent-1" {
		t.Fatalf("agent = %#v", payload["agent"])
	}
	events, ok := payload["initial_events"].([]map[string]interface{})
	if !ok || len(events) != 1 || events[0]["type"] != "user.message" {
		t.Fatalf("initial_events = %#v", payload["initial_events"])
	}
}

func TestBuildManagedAgentSessionCreatePayloadRequiresEnvironment(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{sessions: true}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	addSessionAgentFlag(cmd, resource)

	_, err := buildManagedAgentPayload(cmd, resource, opts, false)
	if err == nil || !strings.Contains(err.Error(), "--environment-id is required") {
		t.Fatalf("buildManagedAgentPayload() error = %v, want environment validation", err)
	}
}

func TestBuildManagedAgentSessionUpdatePayloadOmitsCreateOnlyFields(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{sessions: true}
	opts := newManagedAgentPayloadOptions(resource)
	opts.environmentID = "env-1"
	opts.resources = []string{`{"type":"file","file_id":"file-1"}`}
	cmd := &cobra.Command{Use: "update"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, true)
	addSessionAgentFlag(cmd, resource, true)

	payload, err := buildManagedAgentPayload(cmd, resource, opts, true)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	for _, key := range []string{"environment_id", "resources"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("%s unexpectedly set in update payload: %#v", key, payload)
		}
	}
}

func TestManagedAgentSessionUpdateOnlyExposesOpenAPIFields(t *testing.T) {
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{}})
	updateCmd, _, err := cmd.Find([]string{"agent", "sessions", "update"})
	if err != nil {
		t.Fatalf("Find(agent sessions update) error = %v", err)
	}
	for _, disallowed := range []string{"environment-id", "resource-json", "initial-event-json"} {
		if updateCmd.Flags().Lookup(disallowed) != nil {
			t.Fatalf("session update unexpectedly exposes --%s", disallowed)
		}
	}
	for _, allowed := range []string{"agent", "agent-version", "agent-json", "vault-id", "title", "metadata"} {
		if updateCmd.Flags().Lookup(allowed) == nil {
			t.Fatalf("session update missing --%s", allowed)
		}
	}
}

func TestManagedAgentSessionCreateHelpIncludesVaultID(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "sessions", "create", "-h"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(help) error = %v", err)
	}
	help := stdout.String()
	for _, want := range []string{
		"workspace agent sessions create --environment-id <environment-id> [--agent <agent-id>] [--vault-id <vault-id>]",
		"--environment-id string",
		"--vault-id stringArray",
		"Sent as vault_ids",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

func TestManagedAgentSessionUpdateHelpExcludesCreateOnlyFlags(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "sessions", "update", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(help) error = %v", err)
	}
	help := stdout.String()
	for _, unwanted := range []string{"--environment-id", "--resource-json", "--initial-event-json"} {
		if strings.Contains(help, unwanted) {
			t.Fatalf("help unexpectedly contains %q:\n%s", unwanted, help)
		}
	}
}

func TestManagedAgentUpdateHelpDocumentsOptionalVersion(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "update", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(help) error = %v", err)
	}
	help := stdout.String()
	for _, want := range []string{
		"--version int",
		"optional optimistic concurrency",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

func TestManagedAgentUpdateUsesPost(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			updateFn: func(_ context.Context, method, path string, payload interface{}) (interface{}, error) {
				if method != http.MethodPost {
					t.Fatalf("method = %q, want POST", method)
				}
				if path != "/v1/agents/agent-1" {
					t.Fatalf("path = %q, want /v1/agents/agent-1", path)
				}
				payloadMap, ok := payload.(map[string]interface{})
				if !ok {
					t.Fatalf("payload type = %T, want map[string]interface{}", payload)
				}
				if payloadMap["version"] != 2 {
					t.Fatalf("version = %#v, want 2", payloadMap["version"])
				}
				if payloadMap["name"] != "updated-agent" {
					t.Fatalf("name = %#v, want updated-agent", payloadMap["name"])
				}
				return map[string]interface{}{"id": "agent-1"}, nil
			},
		}, nil
	}

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "update", "agent-1", "--version", "2", "--name", "updated-agent"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestManagedAgentSessionUpdateUsesPost(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			updateFn: func(_ context.Context, method, path string, payload interface{}) (interface{}, error) {
				if method != http.MethodPost {
					t.Fatalf("method = %q, want POST", method)
				}
				if path != "/v1/sessions/session-1" {
					t.Fatalf("path = %q, want /v1/sessions/session-1", path)
				}
				payloadMap, ok := payload.(map[string]interface{})
				if !ok {
					t.Fatalf("payload type = %T, want map[string]interface{}", payload)
				}
				agent, ok := payloadMap["agent"].(map[string]interface{})
				if !ok {
					t.Fatalf("agent type = %T, want map[string]interface{}", payloadMap["agent"])
				}
				if agent["id"] != "agent-2" || agent["version"] != 3 {
					t.Fatalf("agent = %#v", agent)
				}
				return map[string]interface{}{"id": "session-1"}, nil
			},
		}, nil
	}

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "sessions", "update", "session-1", "--agent", "agent-2", "--agent-version", "3"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestManagedAgentResourceUpdatesUsePost(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	tests := []struct {
		name string
		args []string
		path string
	}{
		{
			name: "session",
			args: []string{"agent", "sessions", "update", "session-1", "--title", "updated"},
			path: "/v1/sessions/session-1",
		},
		{
			name: "memory store",
			args: []string{"agent", "memory-stores", "update", "store-1", "--name", "updated"},
			path: "/v1/memory_stores/store-1",
		},
		{
			name: "vault",
			args: []string{"agent", "vaults", "update", "vault-1", "--display-name", "updated"},
			path: "/v1/vaults/vault-1",
		},
		{
			name: "environment",
			args: []string{"agent", "environments", "update", "env-1", "--name", "updated"},
			path: "/v1/environments/env-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
				return &workspaceManagedAgentsClientMock{
					updateFn: func(_ context.Context, method, path string, _ interface{}) (interface{}, error) {
						if method != http.MethodPost {
							t.Fatalf("method = %q, want POST", method)
						}
						if path != tt.path {
							t.Fatalf("path = %q, want %q", path, tt.path)
						}
						return map[string]interface{}{"id": "updated"}, nil
					},
				}, nil
			}

			cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
}

func TestAgentTriggersHelpIncludesCron(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "triggers", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(help) error = %v", err)
	}
	help := stdout.String()
	for _, want := range []string{
		"Pulsar, Kafka, and cron",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

func TestSessionAgentFlagScope(t *testing.T) {
	t.Parallel()

	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{}})
	for _, args := range [][]string{
		{"agent", "sessions", "create"},
	} {
		sessionCmd, _, err := cmd.Find(args)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
		if sessionCmd.Flags().Lookup("agent") == nil {
			t.Fatalf("%v missing --agent flag", args)
		}
	}

	listCmd, _, err := cmd.Find([]string{"agent", "sessions", "list"})
	if err != nil {
		t.Fatalf("Find(agent sessions list) error = %v", err)
	}
	listAgentFlag := listCmd.Flags().Lookup("agent")
	if listAgentFlag == nil {
		t.Fatalf("agent sessions list missing --agent flag")
	}
	if values := listAgentFlag.Annotations[cobra.BashCompOneRequiredFlag]; len(values) > 0 {
		t.Fatalf("agent sessions list --agent required annotation = %v, want optional", values)
	}

	updateCmd, _, err := cmd.Find([]string{"agent", "sessions", "update"})
	if err != nil {
		t.Fatalf("Find(agent sessions update) error = %v", err)
	}
	updateAgentFlag := updateCmd.Flags().Lookup("agent")
	if updateAgentFlag == nil {
		t.Fatalf("agent sessions update missing --agent flag")
	}
	if values := updateAgentFlag.Annotations[cobra.BashCompOneRequiredFlag]; len(values) > 0 {
		t.Fatalf("agent sessions update --agent required annotation = %v, want optional", values)
	}

	for _, args := range [][]string{
		{"agent", "sessions", "get"},
		{"agent", "sessions", "delete"},
		{"agent", "sessions", "archive"},
		{"agent", "sessions", "resources", "add"},
		{"agent", "sessions", "resources", "list"},
		{"agent", "sessions", "resources", "get"},
		{"agent", "sessions", "resources", "delete"},
		{"agent", "sessions", "events", "send"},
		{"agent", "sessions", "events", "list"},
		{"agent", "sessions", "events", "stream"},
		{"agent", "sessions", "threads", "list"},
		{"agent", "sessions", "threads", "get"},
		{"agent", "sessions", "threads", "archive"},
		{"agent", "sessions", "threads", "events", "list"},
		{"agent", "sessions", "threads", "events", "stream"},
	} {
		sessionCmd, _, err := cmd.Find(args)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
		if sessionCmd.Flags().Lookup("agent") != nil {
			t.Fatalf("%v unexpectedly has --agent flag", args)
		}
	}
}

func TestBuildSessionEventsPayload(t *testing.T) {
	t.Parallel()

	payload, err := buildSessionEventsPayload(agentSessionChildOptions{
		eventJSONs: []string{`{"type":"user.message","content":[{"type":"text","text":"ping"}]}`},
	})
	if err != nil {
		t.Fatalf("buildSessionEventsPayload() error = %v", err)
	}

	events, ok := payload["events"].([]map[string]interface{})
	if !ok || len(events) != 1 {
		t.Fatalf("events = %#v", payload["events"])
	}
	if events[0]["type"] != "user.message" {
		t.Fatalf("event type = %#v", events[0]["type"])
	}
}

func TestBuildTypedSessionEventPayloads(t *testing.T) {
	t.Parallel()

	message, err := buildSessionMessageEventPayload("ping")
	if err != nil {
		t.Fatalf("buildSessionMessageEventPayload() error = %v", err)
	}
	messageEvents := message["events"].([]map[string]interface{})
	if messageEvents[0]["type"] != "user.message" {
		t.Fatalf("message event = %#v", messageEvents[0])
	}
	content := messageEvents[0]["content"].([]map[string]interface{})
	if content[0]["text"] != "ping" {
		t.Fatalf("message content = %#v", content)
	}

	outcome, err := buildSessionOutcomeEventPayload("ship", "done", 3)
	if err != nil {
		t.Fatalf("buildSessionOutcomeEventPayload() error = %v", err)
	}
	outcomeEvents := outcome["events"].([]map[string]interface{})
	if outcomeEvents[0]["type"] != "user.define_outcome" || outcomeEvents[0]["max_iterations"] != 3 {
		t.Fatalf("outcome event = %#v", outcomeEvents[0])
	}

	confirmation, err := buildSessionToolConfirmationEventPayload("toolu-1", "ALLOW", "")
	if err != nil {
		t.Fatalf("buildSessionToolConfirmationEventPayload() error = %v", err)
	}
	confirmationEvents := confirmation["events"].([]map[string]interface{})
	if confirmationEvents[0]["type"] != "user.tool_confirmation" || confirmationEvents[0]["result"] != "allow" {
		t.Fatalf("confirmation event = %#v", confirmationEvents[0])
	}
}

func TestSessionEventSendMessageCommandUsesTypedPayload(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			createFn: func(_ context.Context, path string, payload interface{}) (interface{}, error) {
				if path != "/v1/sessions/session-1/events" {
					t.Fatalf("path = %q, want /v1/sessions/session-1/events", path)
				}
				payloadMap, ok := payload.(map[string]interface{})
				if !ok {
					t.Fatalf("payload type = %T", payload)
				}
				events := payloadMap["events"].([]map[string]interface{})
				if events[0]["type"] != "user.message" {
					t.Fatalf("event = %#v", events[0])
				}
				return map[string]interface{}{"ok": true}, nil
			},
		}, nil
	}

	for _, args := range [][]string{
		{"agent", "sessions", "events", "send", "message", "--session", "session-1", "--text", "ping"},
		{"agent", "sessions", "events", "send", "--session", "session-1", "message", "--text", "ping"},
	} {
		cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
	}
}

func TestSessionEventStreamParsesSSE(t *testing.T) {
	original := newWorkspaceManagedAgentsStreamClient
	defer func() {
		newWorkspaceManagedAgentsStreamClient = original
	}()

	newWorkspaceManagedAgentsStreamClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{
			getStreamFn: func(_ context.Context, path string, accept string, handle func(io.Reader) error) error {
				if path != "/v1/sessions/session-1/events/stream" {
					t.Fatalf("path = %q, want /v1/sessions/session-1/events/stream", path)
				}
				if accept != "text/event-stream" {
					t.Fatalf("accept = %q, want text/event-stream", accept)
				}
				return handle(strings.NewReader(
					"id: 42\n" +
						"event: session.status_idle\n" +
						"data: {\"id\":\"evt_1\",\"type\":\"session.status_idle\",\"stop_reason\":{\"type\":\"end_turn\"}}\n\n",
				))
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"agent", "sessions", "events", "stream", "--session", "session-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := `{"data":{"id":"evt_1","stop_reason":{"type":"end_turn"},"type":"session.status_idle"},"event":"session.status_idle","id":"42"}`
	if strings.TrimSpace(stdout.String()) != want {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestSessionEventStreamCursorHelpSaysInclusive(t *testing.T) {
	t.Parallel()

	root := NewGroupCommand(&Options{IOStreams: IOStreams{}})
	for _, args := range [][]string{
		{"agent", "sessions", "events", "stream"},
		{"agent", "sessions", "threads", "events", "stream"},
	} {
		cmd, _, err := root.Find(args)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
		flag := cmd.Flags().Lookup("from-cursor")
		if flag == nil {
			t.Fatalf("Find(%v) has no --from-cursor flag", args)
		}
		want := "Resume streaming at this SSE frame cursor (inclusive)"
		if flag.Usage != want {
			t.Fatalf("Find(%v) --from-cursor usage = %q, want %q", args, flag.Usage, want)
		}
	}
}

func TestBuildSessionEventStreamPath(t *testing.T) {
	t.Parallel()

	got, err := buildSessionEventStreamPath(agentSessionChildOptions{
		sessionID:  "session-1",
		fromCursor: "cursor/1",
		subpath:    "child/path",
		eventDeltas: []string{
			"agent.message",
			"agent.thinking",
		},
	})
	if err != nil {
		t.Fatalf("buildSessionEventStreamPath() error = %v", err)
	}

	want := "/v1/sessions/session-1/events/stream?event_deltas=agent.message&event_deltas=agent.thinking&from_cursor=cursor%2F1&subpath=child%2Fpath"
	if got != want {
		t.Fatalf("buildSessionEventStreamPath() = %q, want %q", got, want)
	}
}

func TestBuildSessionEventStreamContext(t *testing.T) {
	t.Parallel()

	ctx, cancel, hasTimeout, err := buildSessionEventStreamContext(context.Background(), "5s")
	if err != nil {
		t.Fatalf("buildSessionEventStreamContext() error = %v", err)
	}
	defer cancel()
	if !hasTimeout {
		t.Fatalf("hasTimeout = false, want true")
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("Deadline() ok = false, want true")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > 5*time.Second {
		t.Fatalf("deadline remaining = %v, want within 5s", remaining)
	}

	if _, cancel, hasTimeout, err = buildSessionEventStreamContext(context.Background(), ""); err != nil {
		t.Fatalf("buildSessionEventStreamContext(empty) error = %v", err)
	} else {
		cancel()
		if hasTimeout {
			t.Fatalf("hasTimeout = true, want false")
		}
	}

	if _, _, _, err = buildSessionEventStreamContext(context.Background(), "bad"); err == nil {
		t.Fatalf("buildSessionEventStreamContext(bad) error = nil, want error")
	}
}

func TestBuildManagedAgentSkillMultipartRequest(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "manual-skill")
	if err := os.MkdirAll(skillDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("# Skill\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	request, err := buildManagedAgentMultipartRequest(managedAgentResource{skill: true}, agentMultipartOptions{
		files:        []string{skillPath},
		displayTitle: "test-skill",
	})
	if err != nil {
		t.Fatalf("buildManagedAgentMultipartRequest() error = %v", err)
	}

	if request.Fields["display_title"] != "test-skill" {
		t.Fatalf("display_title field = %q", request.Fields["display_title"])
	}
	if len(request.Files) != 1 {
		t.Fatalf("len(request.Files) = %d, want 1", len(request.Files))
	}
	if request.Files[0].FieldName != "files[]" {
		t.Fatalf("file field name = %q, want files[]", request.Files[0].FieldName)
	}
	if request.Files[0].FileName != "manual-skill/SKILL.md" {
		t.Fatalf("file name = %q, want manual-skill/SKILL.md", request.Files[0].FileName)
	}
	if request.Files[0].ContentType != "text/markdown" {
		t.Fatalf("content type = %q, want text/markdown", request.Files[0].ContentType)
	}
}

func TestBuildManagedAgentFileMultipartRequestUsesPartContentTypeOnly(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "input.bin")
	if err := os.WriteFile(filePath, []byte("file bytes"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	request, err := buildManagedAgentMultipartRequest(managedAgentResource{file: true}, agentMultipartOptions{
		files:       []string{filePath},
		contentType: "application/x-test",
	})
	if err != nil {
		t.Fatalf("buildManagedAgentMultipartRequest() error = %v", err)
	}
	if len(request.Fields) != 0 {
		t.Fatalf("fields = %#v, want no extra multipart fields", request.Fields)
	}
	if request.File == nil || request.File.ContentType != "application/x-test" {
		t.Fatalf("file = %#v", request.File)
	}
}

func TestBuildManagedAgentSkillMultipartRequestAllowsZip(t *testing.T) {
	t.Parallel()

	zipPath := filepath.Join(t.TempDir(), "skill.zip")
	if err := os.WriteFile(zipPath, []byte("zip bytes"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	request, err := buildManagedAgentMultipartRequest(managedAgentResource{skill: true}, agentMultipartOptions{
		files: []string{zipPath},
	})
	if err != nil {
		t.Fatalf("buildManagedAgentMultipartRequest() error = %v", err)
	}
	if len(request.Files) != 1 {
		t.Fatalf("len(request.Files) = %d, want 1", len(request.Files))
	}
	if request.Files[0].FileName != "skill.zip" {
		t.Fatalf("file name = %q, want skill.zip", request.Files[0].FileName)
	}
	if string(request.Files[0].Content) != "zip bytes" {
		t.Fatalf("content = %q", string(request.Files[0].Content))
	}
}

func TestReadManagedAgentMultipartFileNormalizesSkillContentType(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	skillPath := filepath.Join(tempDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("# Skill\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	file, err := readManagedAgentMultipartFile(skillPath, "", managedAgentResource{skill: true})
	if err != nil {
		t.Fatalf("readManagedAgentMultipartFile() error = %v", err)
	}
	if file.ContentType != "text/markdown" {
		t.Fatalf("content type = %q, want text/markdown", file.ContentType)
	}
}

func TestBuildManagedAgentSkillMultipartRequestPreservesSkillDirectory(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "manual-skill")
	scriptsDir := filepath.Join(skillDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	scriptPath := filepath.Join(scriptsDir, "run.py")
	if err := os.WriteFile(skillPath, []byte("# Skill\n"), 0600); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("print('ok')\n"), 0600); err != nil {
		t.Fatalf("WriteFile(run.py) error = %v", err)
	}

	request, err := buildManagedAgentMultipartRequest(managedAgentResource{skill: true}, agentMultipartOptions{
		files: []string{skillPath, scriptPath},
	})
	if err != nil {
		t.Fatalf("buildManagedAgentMultipartRequest() error = %v", err)
	}

	if len(request.Files) != 2 {
		t.Fatalf("len(request.Files) = %d, want 2", len(request.Files))
	}
	if request.Files[0].FileName != "manual-skill/SKILL.md" {
		t.Fatalf("first file name = %q, want manual-skill/SKILL.md", request.Files[0].FileName)
	}
	if request.Files[1].FileName != "manual-skill/scripts/run.py" {
		t.Fatalf("second file name = %q, want manual-skill/scripts/run.py", request.Files[1].FileName)
	}
}

func TestBuildManagedAgentSkillMultipartRequestRejectsFilesOutsideSkillDirectory(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "manual-skill")
	if err := os.MkdirAll(skillDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	outsidePath := filepath.Join(tempDir, "outside.txt")
	if err := os.WriteFile(skillPath, []byte("# Skill\n"), 0600); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(outsidePath, []byte("outside\n"), 0600); err != nil {
		t.Fatalf("WriteFile(outside.txt) error = %v", err)
	}

	_, err := buildManagedAgentMultipartRequest(managedAgentResource{skill: true}, agentMultipartOptions{
		files: []string{skillPath, outsidePath},
	})
	if err == nil || !strings.Contains(err.Error(), "same top-level directory") {
		t.Fatalf("buildManagedAgentMultipartRequest() error = %v, want same top-level directory error", err)
	}
}

func TestBuildManagedAgentPayloadFromAgentFlags(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "agent"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	for name, value := range map[string]string{
		"name":             "test-agent",
		"model":            "claude-sonnet-4-6",
		"description":      "test description",
		"system":           "test system",
		"metadata":         "source=manual-test",
		"mcp-server":       "name=example,type=url,url=https://example.com/mcp",
		"tool-json":        `{"type":"agent_toolset_20260401"}`,
		"skill":            "skill-1@1",
		"multiagent-type":  "coordinator",
		"multiagent-agent": "agent-worker@2",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, false)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}

	if _, ok := payload["provider"]; ok {
		t.Fatalf("provider unexpectedly present: %#v", payload)
	}
	if _, ok := payload["beta_version"]; ok {
		t.Fatalf("beta_version unexpectedly present: %#v", payload)
	}
	if payload["model"] != "claude-sonnet-4-6" {
		t.Fatalf("model = %v", payload["model"])
	}
	if !reflect.DeepEqual(payload["metadata"], map[string]string{"source": "manual-test"}) {
		t.Fatalf("metadata = %#v", payload["metadata"])
	}
	skills, ok := payload["skills"].([]map[string]interface{})
	if !ok || len(skills) != 1 || skills[0]["type"] != "anthropic" || skills[0]["skill_id"] != "skill-1" || skills[0]["version"] != "1" {
		t.Fatalf("skills = %#v", payload["skills"])
	}
	multiagent, ok := payload["multiagent"].(map[string]interface{})
	if !ok || multiagent["type"] != "coordinator" {
		t.Fatalf("multiagent = %#v", payload["multiagent"])
	}
	agents, ok := multiagent["agents"].([]map[string]interface{})
	if !ok || len(agents) != 1 || agents[0]["id"] != "agent-worker" || agents[0]["version"] != 2 {
		t.Fatalf("multiagent.agents = %#v", multiagent["agents"])
	}
}

func TestParseSkillReferencesUsesOpenAPIDiscriminators(t *testing.T) {
	t.Parallel()

	skills, err := parseSkillReferences([]string{
		"pdfs@latest",
		"skill_custom_1@2",
		"custom:skill_explicit@3",
		"anthropic:spreadsheets",
	})
	if err != nil {
		t.Fatalf("parseSkillReferences() error = %v", err)
	}
	want := []map[string]interface{}{
		{"type": "anthropic", "skill_id": "pdfs", "version": "latest"},
		{"type": "custom", "skill_id": "skill_custom_1", "version": "2"},
		{"type": "custom", "skill_id": "skill_explicit", "version": "3"},
		{"type": "anthropic", "skill_id": "spreadsheets"},
	}
	if !reflect.DeepEqual(skills, want) {
		t.Fatalf("skills = %#v, want %#v", skills, want)
	}
}

func TestBuildManagedAgentPayloadRequiresAgentModelOnCreate(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "agent"}
	for _, tt := range []struct {
		name  string
		model string
	}{
		{name: "missing"},
		{name: "blank", model: "   "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := newManagedAgentPayloadOptions(resource)
			cmd := &cobra.Command{Use: "create"}
			addManagedAgentPayloadFlags(cmd, &opts, resource, false)
			if tt.model != "" {
				if err := cmd.Flags().Set("model", tt.model); err != nil {
					t.Fatalf("Set(model) error = %v", err)
				}
			}

			_, err := buildManagedAgentPayload(cmd, resource, opts, false)
			if err == nil || !strings.Contains(err.Error(), "--model is required") {
				t.Fatalf("buildManagedAgentPayload() error = %v, want required model error", err)
			}
		})
	}
}

func TestBuildManagedAgentPayloadRequiresCreateNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resource managedAgentResource
		model    string
		want     string
	}{
		{
			name:     "agent",
			resource: managedAgentResource{use: "agent"},
			model:    "claude-sonnet-4-6",
			want:     "--name is required for agent create",
		},
		{
			name:     "environment",
			resource: managedAgentResource{use: "environments"},
			want:     "--name is required for environment create",
		},
		{
			name:     "memory store",
			resource: managedAgentResource{use: "memory-stores"},
			want:     "--name is required for memory store create",
		},
	}
	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := newManagedAgentPayloadOptions(tc.resource)
			cmd := &cobra.Command{Use: "create"}
			addManagedAgentPayloadFlags(cmd, &opts, tc.resource, false)
			if tc.model != "" {
				if err := cmd.Flags().Set("model", tc.model); err != nil {
					t.Fatalf("Set(model) error = %v", err)
				}
			}
			_, err := buildManagedAgentPayload(cmd, tc.resource, opts, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("buildManagedAgentPayload() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBuildManagedAgentPayloadAcceptsSymbolicSkillVersion(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "agent"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	for name, value := range map[string]string{
		"name":  "test-agent",
		"model": "claude-sonnet-4-6",
		"skill": "skill-1@latest",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, false)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	skills, ok := payload["skills"].([]map[string]interface{})
	if !ok || len(skills) != 1 || skills[0]["version"] != "latest" {
		t.Fatalf("skills = %#v", payload["skills"])
	}
}

func TestBuildManagedAgentPayloadRejectsFractionalMultiagentVersion(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "agent"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	for name, value := range map[string]string{
		"name":             "test-agent",
		"model":            "claude-sonnet-4-6",
		"multiagent-agent": "type=agent,id=agent-worker,version=1.5",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	_, err := buildManagedAgentPayload(cmd, resource, opts, false)
	if err == nil || !strings.Contains(err.Error(), "--multiagent-agent version must be an integer") {
		t.Fatalf("buildManagedAgentPayload() error = %v, want integer version error", err)
	}
}

func TestBuildManagedAgentPayloadAllowsAgentUpdateWithoutVersion(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "agent"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "update"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, true)
	if err := cmd.Flags().Set("name", "updated-agent"); err != nil {
		t.Fatalf("Set(name) error = %v", err)
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, true)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	if _, ok := payload["version"]; ok {
		t.Fatalf("version unexpectedly present: %#v", payload)
	}
	if payload["name"] != "updated-agent" {
		t.Fatalf("name = %#v, want updated-agent", payload["name"])
	}
}

func TestBuildManagedAgentPayloadIncludesAgentVersionOnUpdate(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "agent"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "update"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, true)
	for name, value := range map[string]string{
		"version": "1",
		"name":    "updated-agent",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, true)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	if payload["version"] != 1 {
		t.Fatalf("version = %#v, want 1", payload["version"])
	}
	if payload["name"] != "updated-agent" {
		t.Fatalf("name = %#v, want updated-agent", payload["name"])
	}
}

func TestBuildManagedAgentEnvironmentPayloadDefaultsConfig(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "environments"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	if err := cmd.Flags().Set("name", "test-env"); err != nil {
		t.Fatalf("Set(name) error = %v", err)
	}
	if err := cmd.Flags().Set("scope", "account"); err != nil {
		t.Fatalf("Set(scope) error = %v", err)
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, false)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	config, ok := payload["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("config = %#v", payload["config"])
	}
	if config["type"] != "cloud" {
		t.Fatalf("config.type = %v", config["type"])
	}
	networking, ok := config["networking"].(map[string]interface{})
	if !ok || networking["type"] != "unrestricted" {
		t.Fatalf("config.networking = %#v", config["networking"])
	}
	if _, ok := config["packages"]; ok {
		t.Fatalf("config.packages unexpectedly set: %#v", config["packages"])
	}
	if payload["scope"] != "account" {
		t.Fatalf("scope = %#v, want account", payload["scope"])
	}
}

func TestBuildManagedAgentEnvironmentPayloadRejectsInvalidScope(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "environments"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	for name, value := range map[string]string{"name": "test-env", "scope": "workspace"} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}
	if _, err := buildManagedAgentPayload(cmd, resource, opts, false); err == nil || !strings.Contains(err.Error(), "--scope") {
		t.Fatalf("buildManagedAgentPayload() error = %v, want --scope validation error", err)
	}
}

func TestBuildManagedAgentEnvironmentPayloadIncludesPackageFlags(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "environments"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	if err := cmd.Flags().Set("name", "test-env"); err != nil {
		t.Fatalf("Set(name) error = %v", err)
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{name: "package-apt", value: "git"},
		{name: "package-apt", value: "curl"},
		{name: "package-npm", value: "typescript"},
		{name: "package-pip", value: "pytest==8.0.0"},
	} {
		if err := cmd.Flags().Set(item.name, item.value); err != nil {
			t.Fatalf("Set(%s) error = %v", item.name, err)
		}
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, false)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	config, ok := payload["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("config = %#v", payload["config"])
	}
	packages, ok := config["packages"].(map[string]interface{})
	if !ok {
		t.Fatalf("config.packages = %#v", config["packages"])
	}
	if packages["type"] != "packages" {
		t.Fatalf("config.packages.type = %#v", packages["type"])
	}
	if !reflect.DeepEqual(packages["apt"], []string{"git", "curl"}) {
		t.Fatalf("config.packages.apt = %#v", packages["apt"])
	}
	if !reflect.DeepEqual(packages["npm"], []string{"typescript"}) {
		t.Fatalf("config.packages.npm = %#v", packages["npm"])
	}
	if !reflect.DeepEqual(packages["pip"], []string{"pytest==8.0.0"}) {
		t.Fatalf("config.packages.pip = %#v", packages["pip"])
	}
}

func TestBuildManagedAgentEnvironmentPayloadMergesConfigJSONPackages(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "environments"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "update"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, true)
	if err := cmd.Flags().Set("config-json", `{"type":"cloud","packages":{"pip":["pytest"],"npm":["eslint"]}}`); err != nil {
		t.Fatalf("Set(config-json) error = %v", err)
	}
	if err := cmd.Flags().Set("package-npm", "typescript"); err != nil {
		t.Fatalf("Set(package-npm) error = %v", err)
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, true)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	config, ok := payload["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("config = %#v", payload["config"])
	}
	packages, ok := config["packages"].(map[string]interface{})
	if !ok {
		t.Fatalf("config.packages = %#v", config["packages"])
	}
	if packages["type"] != "packages" {
		t.Fatalf("config.packages.type = %#v", packages["type"])
	}
	if !reflect.DeepEqual(packages["pip"], []string{"pytest"}) {
		t.Fatalf("config.packages.pip = %#v", packages["pip"])
	}
	if !reflect.DeepEqual(packages["npm"], []string{"typescript"}) {
		t.Fatalf("config.packages.npm = %#v", packages["npm"])
	}
}

func TestBuildManagedAgentEnvironmentPayloadMapsLegacyPackagesToApt(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "environments"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "update"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, true)
	if err := cmd.Flags().Set("config-json", `{"packages":["git","curl"]}`); err != nil {
		t.Fatalf("Set(config-json) error = %v", err)
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, true)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	config, ok := payload["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("config = %#v", payload["config"])
	}
	packages, ok := config["packages"].(map[string]interface{})
	if !ok {
		t.Fatalf("config.packages = %#v", config["packages"])
	}
	if packages["type"] != "packages" {
		t.Fatalf("config.packages.type = %#v", packages["type"])
	}
	if !reflect.DeepEqual(packages["apt"], []string{"git", "curl"}) {
		t.Fatalf("config.packages.apt = %#v", packages["apt"])
	}
}

func TestBuildManagedAgentMemoryStorePayload(t *testing.T) {
	t.Parallel()

	resource := managedAgentResource{use: "memory-stores"}
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{Use: "create"}
	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	for name, value := range map[string]string{
		"name":        "test-memory",
		"description": "test memory store",
		"metadata":    "source=manual-test",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s) error = %v", name, err)
		}
	}

	payload, err := buildManagedAgentPayload(cmd, resource, opts, false)
	if err != nil {
		t.Fatalf("buildManagedAgentPayload() error = %v", err)
	}
	if payload["name"] != "test-memory" {
		t.Fatalf("payload = %#v", payload)
	}
	if _, ok := payload["provider"]; ok {
		t.Fatalf("provider unexpectedly present: %#v", payload)
	}
	if _, ok := payload["beta_version"]; ok {
		t.Fatalf("beta_version unexpectedly present: %#v", payload)
	}
	if payload["description"] != "test memory store" {
		t.Fatalf("description = %#v", payload["description"])
	}
}

func TestManagedAgentVaultCreateRequiresDisplayNameBeforeClientCall(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		t.Fatal("newWorkspaceManagedAgentsClient() should not be called when --display-name is missing")
		return nil, nil
	}

	resource := managedAgentResource{
		use:      "vaults",
		singular: "vault",
		plural:   "vaults",
		basePath: "/vaults",
	}

	for _, args := range [][]string{
		{},
		{"--display-name", "   "},
	} {
		cmd := (&agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}).newManagedAgentCreateCommand(resource)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--display-name is required") {
			t.Fatalf("Execute(%v) error = %v, want display name validation error", args, err)
		}
	}
}

func TestManagedAgentCreateRequiresModelBeforeClientCall(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	defer func() {
		newWorkspaceManagedAgentsClient = original
	}()

	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		t.Fatal("newWorkspaceManagedAgentsClient() should not be called when --model is missing")
		return nil, nil
	}

	resource := managedAgentResource{
		use:      "agent",
		singular: "agent",
		plural:   "agents",
		basePath: "/agents",
	}

	cmd := (&agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}).newManagedAgentCreateCommand(resource)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--name", "test-agent"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "model") || !strings.Contains(err.Error(), "required") {
		t.Fatalf("Execute() error = %v, want required model validation error", err)
	}
}

func TestBuildSessionResourcePayload(t *testing.T) {
	t.Parallel()

	payload, err := buildSessionResourcePayload(agentSessionChildOptions{
		resourceType:       "github-repository",
		url:                "https://github.com/acme/repo",
		authorizationToken: "token-1",
		branch:             "main",
		mountPath:          "/workspace/repo",
		access:             "read_only",
		instructions:       "Read repository files",
	}, false)
	if err != nil {
		t.Fatalf("buildSessionResourcePayload() error = %v", err)
	}
	if payload["type"] != "github_repository" {
		t.Fatalf("type = %#v", payload["type"])
	}
	checkout, ok := payload["checkout"].(map[string]interface{})
	if !ok || checkout["type"] != "branch" || checkout["name"] != "main" {
		t.Fatalf("checkout = %#v", payload["checkout"])
	}
	if payload["authorization_token"] != "token-1" || payload["access"] != "read_only" || payload["instructions"] != "Read repository files" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestBuildSessionResourcePayloadIncludesFileFields(t *testing.T) {
	t.Parallel()

	filePayload, err := buildSessionResourcePayload(agentSessionChildOptions{
		resourceType:  "file",
		fileID:        "file-1",
		access:        "read_only",
		mountPath:     "/workspace/input.txt",
		mountStrategy: "tarball_prefetch",
	}, false)
	if err != nil {
		t.Fatalf("buildSessionResourcePayload(file) error = %v", err)
	}
	for key, want := range map[string]interface{}{
		"type":           "file",
		"file_id":        "file-1",
		"access":         "read_only",
		"mount_path":     "/workspace/input.txt",
		"mount_strategy": "tarball_prefetch",
	} {
		if filePayload[key] != want {
			t.Fatalf("file %s = %#v, want %#v", key, filePayload[key], want)
		}
	}
}

func TestBuildSessionResourcePayloadRequiresTypeSpecificFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts agentSessionChildOptions
		want string
	}{
		{name: "file id", opts: agentSessionChildOptions{resourceType: "file"}, want: "--file-id is required"},
		{name: "memory store id", opts: agentSessionChildOptions{resourceType: "memory-store"}, want: "--memory-store-id is required"},
		{name: "repository url", opts: agentSessionChildOptions{resourceType: "github-repository"}, want: "--url is required"},
		{
			name: "repository token",
			opts: agentSessionChildOptions{resourceType: "github-repository", url: "https://github.com/acme/repo"},
			want: "--authorization-token is required",
		},
	}
	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := buildSessionResourcePayload(tc.opts, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("buildSessionResourcePayload() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBuildSessionResourceUpdatePayloadMatchesOpenAPI(t *testing.T) {
	t.Parallel()

	payload, err := buildSessionResourcePayload(agentSessionChildOptions{
		authorizationToken: "token-1",
	}, true)
	if err != nil {
		t.Fatalf("buildSessionResourcePayload() error = %v", err)
	}
	want := map[string]interface{}{"authorization_token": "token-1"}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("payload = %#v, want %#v", payload, want)
	}
}

func TestBuildSessionResourceUpdatePayloadRequiresAuthorizationToken(t *testing.T) {
	t.Parallel()

	_, err := buildSessionResourcePayload(agentSessionChildOptions{}, true)
	if err == nil || !strings.Contains(err.Error(), "--authorization-token is required") {
		t.Fatalf("buildSessionResourcePayload() error = %v, want authorization token validation", err)
	}
}

func TestSessionResourceUpdateOnlyExposesOpenAPIFields(t *testing.T) {
	cmd := NewGroupCommand(&Options{IOStreams: IOStreams{}})
	updateCmd, _, err := cmd.Find([]string{"agent", "sessions", "resources", "update"})
	if err != nil {
		t.Fatalf("Find(session resource update) error = %v", err)
	}
	for _, allowed := range []string{"session", "authorization-token", "output"} {
		if updateCmd.Flags().Lookup(allowed) == nil {
			t.Fatalf("session resource update missing --%s", allowed)
		}
	}
	for _, disallowed := range []string{"type", "file-id", "memory-store-id", "url", "mount-path", "access", "instructions", "checkout-type", "property"} {
		if updateCmd.Flags().Lookup(disallowed) != nil {
			t.Fatalf("session resource update unexpectedly exposes --%s", disallowed)
		}
	}
}

func TestBuildSessionChildPaths(t *testing.T) {
	t.Parallel()

	opts := agentSessionChildOptions{sessionID: "session-1"}
	path, err := buildSessionChildCollectionPath(opts, "threads", agentListOptions{limit: 10, page: "page-1"})
	if err != nil {
		t.Fatalf("buildSessionChildCollectionPath() error = %v", err)
	}
	for _, want := range []string{"/v1/sessions/session-1/threads?", "limit=10", "page=page-1"} {
		if !strings.Contains(path, want) {
			t.Fatalf("path = %q, want substring %q", path, want)
		}
	}

	itemPath, err := buildSessionChildItemPath(opts, "resources", "resource-1")
	if err != nil {
		t.Fatalf("buildSessionChildItemPath() error = %v", err)
	}
	if itemPath != "/v1/sessions/session-1/resources/resource-1" {
		t.Fatalf("item path = %q", itemPath)
	}
}
