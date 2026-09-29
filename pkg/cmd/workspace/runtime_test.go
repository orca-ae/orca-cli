// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveRuntimeRequiresRegistryURL(t *testing.T) {
	opts := &Options{AccessToken: "token"}
	_, err := opts.resolveRuntime(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "--registry-url is required") {
		t.Fatalf("expected missing registry URL error, got %v", err)
	}
}

func TestResolveRuntimeRequiresAuthentication(t *testing.T) {
	opts := &Options{RegistryURL: "http://example.com/v1/registry"}
	_, err := opts.resolveRuntime(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "one of --access-token or --api-key is required") {
		t.Fatalf("expected missing authentication error, got %v", err)
	}
}

func TestResolveRuntimeRejectsMultipleAuthenticationMethods(t *testing.T) {
	opts := &Options{
		RegistryURL: "http://example.com",
		AccessToken: "token",
		APIKey:      "orca_key",
	}
	_, err := opts.resolveRuntime(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "--access-token and --api-key cannot be used together") {
		t.Fatalf("expected conflicting authentication error, got %v", err)
	}
}

func TestNewRegistryClientUsesAPIKeyAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "orca_test-key" {
			t.Fatalf("x-api-key = %q, want %q", got, "orca_test-key")
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("authorization = %q, want empty", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
	}))
	defer server.Close()

	opts := &Options{RegistryURL: server.URL, APIKey: "orca_test-key"}
	client, err := opts.newRegistryClient(context.Background(), nil, server.Client())
	if err != nil {
		t.Fatalf("newRegistryClient() error = %v", err)
	}
	if _, err := client.GetAPIGroups(context.Background()); err != nil {
		t.Fatalf("GetAPIGroups() error = %v", err)
	}
}

func TestResolveConnectionSelectionStandaloneUnsupported(t *testing.T) {
	_, err := (&Options{}).resolveConnectionSelection(context.Background(), "", true)
	if err == nil || !strings.Contains(err.Error(), "--use-connection is not supported; pass --connection") {
		t.Fatalf("expected unsupported use-connection error, got %v", err)
	}
}

func TestResolveConnectionSelectionUsesInjectedSelector(t *testing.T) {
	opts := &Options{SelectConnection: func(context.Context) (string, error) { return "conn-a", nil }}
	name, err := opts.resolveConnectionSelection(context.Background(), "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "conn-a" {
		t.Fatalf("expected conn-a, got %q", name)
	}
}

func TestNewRegistryClientRoutesLegacyWarningOnce(t *testing.T) {
	testCases := []struct {
		name       string
		newCommand func(*bytes.Buffer) *cobra.Command
		ioStreams  func(*bytes.Buffer) IOStreams
	}{
		{
			name: "IOStreams ErrOut",
			newCommand: func(*bytes.Buffer) *cobra.Command {
				return nil
			},
			ioStreams: func(warnings *bytes.Buffer) IOStreams {
				return IOStreams{ErrOut: warnings}
			},
		},
		{
			name: "cobra error stream",
			newCommand: func(warnings *bytes.Buffer) *cobra.Command {
				cmd := &cobra.Command{}
				cmd.SetErr(warnings)
				return cmd
			},
			ioStreams: func(*bytes.Buffer) IOStreams {
				return IOStreams{}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var warnings bytes.Buffer
			opts := &Options{
				IOStreams:   testCase.ioStreams(&warnings),
				RegistryURL: "https://registry.example.com/v1/registry",
				AccessToken: "token",
			}
			cmd := testCase.newCommand(&warnings)

			for attempt := range 2 {
				clientCmd := cmd
				if attempt > 0 {
					clientCmd = nil
				}
				if _, err := opts.newRegistryClient(context.Background(), clientCmd, nil); err != nil {
					t.Fatalf("newRegistryClient() error = %v", err)
				}
			}

			if count := strings.Count(warnings.String(), "warning: registry base URL"); count != 1 {
				t.Fatalf("legacy URL warning count = %d, want 1; output = %q", count, warnings.String())
			}
		})
	}
}
