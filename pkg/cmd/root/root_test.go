// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/orca-ae/orca-cli/pkg/cmd/workspace"
)

func TestRootHelpShowsFlatResourceCommands(t *testing.T) {
	out := &bytes.Buffer{}
	cmd := NewCommand(workspace.IOStreams{Out: out, ErrOut: out})
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"api-versions", "healthz", "readyz", "health", "local", "connections", "functions", "kafka-connect",
		"packages", "sources", "sinks", "guardrails", "model-prices", "api-groups", "api-resources",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("help missing %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "--api-key") {
		t.Fatalf("help missing --api-key:\n%s", text)
	}
	if strings.Contains(text, "agent-functions") {
		t.Fatalf("help unexpectedly contains removed agent-functions command:\n%s", text)
	}
	if strings.Contains(text, "workspace [command]") {
		t.Fatalf("standalone root should be flat, got help:\n%s", text)
	}
}

func TestRootHelpDoesNotExposeAuthenticationEnvironmentValues(t *testing.T) {
	t.Setenv("ORCA_ACCESS_TOKEN", "secret-access-token")
	t.Setenv("ORCA_API_KEY", "secret-api-key")

	out := &bytes.Buffer{}
	cmd := NewCommand(workspace.IOStreams{Out: out, ErrOut: out})
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}

	text := out.String()
	for _, secret := range []string{"secret-access-token", "secret-api-key"} {
		if strings.Contains(text, secret) {
			t.Fatalf("help exposed authentication secret %q:\n%s", secret, text)
		}
	}
}

func TestRootUsesAuthenticationEnvironmentValues(t *testing.T) {
	testCases := []struct {
		name       string
		envName    string
		envValue   string
		headerName string
		headerWant string
	}{
		{
			name:       "access token",
			envName:    "ORCA_ACCESS_TOKEN",
			envValue:   "env-access-token",
			headerName: "Authorization",
			headerWant: "Bearer env-access-token",
		},
		{
			name:       "API key",
			envName:    "ORCA_API_KEY",
			envValue:   "env-api-key",
			headerName: "x-api-key",
			headerWant: "env-api-key",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("ORCA_ACCESS_TOKEN", "")
			t.Setenv("ORCA_API_KEY", "")
			t.Setenv(testCase.envName, testCase.envValue)

			var gotHeader string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeader = r.Header.Get(testCase.headerName)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
			}))
			defer server.Close()
			t.Setenv("ORCA_REGISTRY_URL", server.URL)

			cmd := NewCommand(workspace.IOStreams{Out: io.Discard, ErrOut: io.Discard})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"api-groups"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if gotHeader != testCase.headerWant {
				t.Fatalf("%s = %q, want %q", testCase.headerName, gotHeader, testCase.headerWant)
			}
		})
	}
}

func TestRootRejectsMultipleAuthenticationMethods(t *testing.T) {
	out := &bytes.Buffer{}
	cmd := NewCommand(workspace.IOStreams{Out: out, ErrOut: out})
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{
		"--registry-url", "https://example.com",
		"--access-token", "token",
		"--api-key", "orca_key",
		"api-groups",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "[access-token api-key] were all set") {
		t.Fatalf("Execute() error = %v, want mutually exclusive authentication error", err)
	}
}
