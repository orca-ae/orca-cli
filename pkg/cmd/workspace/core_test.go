// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAPIVersionsCommandUsesAuthenticatedCoreEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			t.Fatalf("path = %q, want /api", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		_, _ = w.Write([]byte(`{"kind":"APIVersions","versions":["v1"],"preferred_version":"v1"}`))
	}))
	defer server.Close()

	var output bytes.Buffer
	cmd := NewCmdAPIVersions(&Options{
		IOStreams:   IOStreams{Out: &output, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "test-token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(output.String(), `"preferred_version": "v1"`) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestCoreProbeCommandsDoNotRequireAccessToken(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		path   string
		status string
		newCmd func(*Options) *cobra.Command
	}{
		{
			name:   "healthz",
			path:   "/healthz",
			status: "ok",
			newCmd: NewCmdHealthz,
		},
		{
			name:   "readyz",
			path:   "/readyz",
			status: "ready",
			newCmd: NewCmdReadyz,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != testCase.path {
					t.Fatalf("path = %q, want %q", r.URL.Path, testCase.path)
				}
				if got := r.Header.Get("Authorization"); got != "" {
					t.Fatalf("Authorization = %q, want empty", got)
				}
				_, _ = w.Write([]byte(`{"status":"` + testCase.status + `","service":"managed-agents"}`))
			}))
			defer server.Close()

			var output bytes.Buffer
			cmd := testCase.newCmd(&Options{
				IOStreams:   IOStreams{Out: &output, ErrOut: io.Discard},
				RegistryURL: server.URL,
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(nil)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !strings.Contains(output.String(), "Status: "+testCase.status) {
				t.Fatalf("output = %q", output.String())
			}
		})
	}
}
