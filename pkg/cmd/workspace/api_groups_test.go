// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	registry "github.com/orca-ae/orca-sdk-go"
)

func TestAPIGroupsCommandUsesAuthenticatedDiscovery(t *testing.T) {
	var gotAuthorization string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io","preferred_version":{"group_version":"cloud.sn.io/v1","version":"v1"}}]}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	cmd := NewCmdAPIGroups(&Options{
		IOStreams:   IOStreams{Out: &stdout, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "test-token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotPath != "/apis" {
		t.Fatalf("path = %q, want %q", gotPath, "/apis")
	}
	if gotAuthorization != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want %q", gotAuthorization, "Bearer test-token")
	}
	if !strings.Contains(stdout.String(), "cloud.sn.io") {
		t.Fatalf("stdout = %q, want it to mention cloud.sn.io", stdout.String())
	}
}

func TestAPIGroupsCommandRequiresAuthentication(t *testing.T) {
	cmd := NewCmdAPIGroups(&Options{
		IOStreams:   IOStreams{Out: io.Discard, ErrOut: io.Discard},
		RegistryURL: "https://example.com",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "one of --access-token or --api-key is required") {
		t.Fatalf("Execute() error = %v, want missing authentication error", err)
	}
}

func TestAPIGroupsCommandRendersExtensionFreeDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	cmd := NewCmdAPIGroups(&Options{
		IOStreams:   IOStreams{Out: &stdout, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "test-token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "No extension groups advertised") {
		t.Fatalf("stdout = %q, want a message describing an extension-free deployment", stdout.String())
	}
}

func TestAPIGroupsCommandRejectsInvalidOutputBeforeClientCall(t *testing.T) {
	original := newWorkspaceDiscoveryClient
	newWorkspaceDiscoveryClient = func() (*registry.Client, error) {
		t.Fatal("newWorkspaceDiscoveryClient() should not be called when output is invalid")
		return nil, nil
	}
	defer func() { newWorkspaceDiscoveryClient = original }()

	cmd := NewCmdAPIGroups(&Options{IOStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--output", "xml"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--output must be one of: text, json, yaml") {
		t.Fatalf("Execute() error = %v, want output validation error", err)
	}
}

func TestAPIGroupsCommandJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io","preferred_version":{"group_version":"cloud.sn.io/v1","version":"v1"}}]}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	cmd := NewCmdAPIGroups(&Options{
		IOStreams:   IOStreams{Out: &stdout, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "test-token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{
		`"kind": "APIGroupList"`,
		`"name": "cloud.sn.io"`,
		`"preferred_version"`,
		`"group_version": "cloud.sn.io/v1"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %s", want, stdout.String())
		}
	}
}

func TestAPIResourcesCommandUsesCloudResourceDiscovery(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apis":
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io"}]}`))
		case "/apis/cloud.sn.io/v1/":
			_, _ = w.Write([]byte(`{"kind":"APIResourceList","group_version":"cloud.sn.io/v1","resources":[{"name":"connections","kind":"Connection","namespaced":true}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	cmd := NewCmdAPIResources(&Options{
		IOStreams:   IOStreams{Out: &stdout, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "test-token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := strings.Join(paths, ","), "/apis,/apis/cloud.sn.io/v1/"; got != want {
		t.Fatalf("paths = %q, want %q", got, want)
	}
	for _, want := range []string{"connections", "Connection", "true"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %s", want, stdout.String())
		}
	}
}

func TestAPIResourcesCommandJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/apis" {
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"kind":"APIResourceList","group_version":"cloud.sn.io/v1","resources":[]}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	cmd := NewCmdAPIResources(&Options{
		IOStreams:   IOStreams{Out: &stdout, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "test-token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{`"kind": "APIResourceList"`, `"group_version": "cloud.sn.io/v1"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %s", want, stdout.String())
		}
	}
}

func TestAPIResourcesCommandSupportsPolicyAndPricingGroups(t *testing.T) {
	for _, testCase := range []struct {
		group        string
		resourceName string
		kind         string
	}{
		{group: policyExtensionGroup, resourceName: "guardrails", kind: "Guardrail"},
		{group: pricingExtensionGroup, resourceName: "modelprices", kind: "ModelPrice"},
	} {
		t.Run(testCase.group, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/apis" {
					_, _ = fmt.Fprintf(w, `{"kind":"APIGroupList","groups":[{"name":%q}]}`, testCase.group)
					return
				}
				_, _ = fmt.Fprintf(w,
					`{"kind":"APIResourceList","group_version":%q,"resources":[{"name":%q,"kind":%q,"namespaced":true}]}`,
					testCase.group+"/v1", testCase.resourceName, testCase.kind)
			}))
			defer server.Close()

			var stdout bytes.Buffer
			cmd := NewCmdAPIResources(&Options{
				IOStreams:   IOStreams{Out: &stdout, ErrOut: io.Discard},
				RegistryURL: server.URL,
				AccessToken: "test-token",
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--group", testCase.group})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			wantPath := "/apis/" + testCase.group + "/v1"
			if got, want := strings.Join(paths, ","), "/apis,"+wantPath; got != want {
				t.Fatalf("paths = %q, want %q", got, want)
			}
			for _, want := range []string{testCase.resourceName, testCase.kind} {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout missing %q: %s", want, stdout.String())
				}
			}
		})
	}
}
