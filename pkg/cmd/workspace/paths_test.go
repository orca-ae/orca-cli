// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	registry "github.com/orca-ae/orca-sdk-go"
)

// TestCoreCallResolvesUnderV1FromHostRootBaseURL proves that a core managed-agent call, built by
// this package's own path builders, resolves to {base}/v1/agents from a base URL with no path at
// all - the shape every deployment now expects. The expected path is a literal string, not built
// from corePathPrefix or any other constant the implementation shares: a bug that hardcoded the
// wrong prefix into that constant would still pass a test that reused it.
func TestCoreCallResolvesUnderV1FromHostRootBaseURL(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	// server.URL is a bare "http://127.0.0.1:PORT" - a host root with no path component.
	baseClient, err := registry.NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client := registry.NewManagedAgentsClient(baseClient)

	agentResource := managedAgentResource{
		use:      "agent",
		singular: "agent",
		plural:   "agents",
		idName:   "agent-id",
		basePath: "/agents",
	}
	path, err := buildManagedAgentCollectionPath(nil, agentResource, agentListOptions{})
	if err != nil {
		t.Fatalf("buildManagedAgentCollectionPath() error = %v", err)
	}

	if _, err := client.Get(context.Background(), path); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if gotPath != "/v1/agents" {
		t.Fatalf("path = %q, want %q", gotPath, "/v1/agents")
	}
}

// TestTriggerCallResolvesUnderV1FromHostRootBaseURL proves the same for a Trigger call: it resolves
// to {base}/v1/triggers from a base URL with no path, again asserted against a literal string.
func TestTriggerCallResolvesUnderV1FromHostRootBaseURL(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	baseClient, err := registry.NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client := registry.NewManagedAgentsClient(baseClient)

	path, err := buildAgentTriggerCollectionPath(nil, agentListOptions{})
	if err != nil {
		t.Fatalf("buildAgentTriggerCollectionPath() error = %v", err)
	}

	if _, err := client.Get(context.Background(), path); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if gotPath != "/v1/triggers" {
		t.Fatalf("path = %q, want %q", gotPath, "/v1/triggers")
	}
}

// TestExtensionRegistryClientResolvesUnderCloudExtensionBasePathFromHostRootBaseURL proves the
// same shape for the extension-only registry.*Client wrappers (connections, catalog, functions,
// sources, sinks, Kafka Connect, packages, providers), which apply their prefix via
// Client.WithPathPrefix rather than a literal string baked in by this package's own builders.
func TestExtensionRegistryClientResolvesUnderCloudExtensionBasePathFromHostRootBaseURL(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	baseClient, err := registry.NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, err := registry.NewConnectionsClient(baseClient).List(context.Background()); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if gotPath != "/apis/cloud.sn.io/v1/connections" {
		t.Fatalf("path = %q, want %q", gotPath, "/apis/cloud.sn.io/v1/connections")
	}
}
