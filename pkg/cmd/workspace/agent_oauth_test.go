// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/orca-ae/orca-cli/pkg/mcpoauth"
	registry "github.com/orca-ae/orca-sdk-go"
)

func TestVaultCredentialOAuthRegistersWithRegistry(t *testing.T) {
	var payload map[string]any
	var path, method, authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method, authorization = r.URL.EscapedPath(), r.Method, r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode credential: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"cred_123","auth":{"access_token":"secret-access"}}`)
	}))
	defer server.Close()
	original := newWorkspaceManagedAgentsClient
	t.Cleanup(func() { newWorkspaceManagedAgentsClient = original })
	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		client, err := registry.NewClient(server.URL, "registry-token", server.Client())
		if err != nil {
			return nil, err
		}
		return registry.NewManagedAgentsClient(client), nil
	}
	var stdout, stderr bytes.Buffer
	auth := map[string]any{
		"type": "mcp_oauth", "mcp_server_url": "https://mcp.example.com/mcp", "access_token": "secret-access",
		"refresh": map[string]any{"refresh_token": "secret-refresh", "token_endpoint": "https://issuer.example.com/token", "client_id": "client", "token_endpoint_auth": map[string]any{"type": "none"}},
	}
	called := false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := (&agentOptions{ioStreams: IOStreams{Out: &stdout, ErrOut: &stderr}}).newVaultCredentialCreateCommandWithOAuth(
		func(gotCtx context.Context, opts mcpoauth.Options, progress io.Writer) (map[string]any, error) {
			called = true
			if gotCtx != ctx {
				t.Error("OAuth did not inherit command context")
			}
			want := mcpoauth.Options{ServerURL: "https://mcp.example.com/mcp", Issuer: "https://issuer.example.com", ClientID: "client", Scopes: []string{"read write", "offline_access"}, Timeout: time.Minute, CallbackAddress: "127.0.0.1:53900", NoBrowser: true, NoRefresh: true, AllowIssuerMismatch: true}
			if !reflect.DeepEqual(opts, want) {
				t.Errorf("OAuth options = %#v, want %#v", opts, want)
			}
			fmt.Fprintln(progress, "Authorize in browser")
			return auth, nil
		})
	cmd.SetArgs([]string{"--vault", "vault 1/child", "--display-name", "Example MCP", "--mcp-server-url", "https://mcp.example.com/mcp", "--oauth-issuer", "https://issuer.example.com", "--oauth-allow-issuer-mismatch", "--oauth-client-id", "client", "--oauth-scope", "read write", "--oauth-scope", "offline_access", "--oauth-timeout", "1m", "--callback-address", "127.0.0.1:53900", "--no-browser", "--no-refresh", "--metadata", "team=platform", "--output", "json"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	if !called || method != http.MethodPost || path != "/v1/vaults/vault%201%2Fchild/credentials" || authorization != "Bearer registry-token" {
		t.Fatalf("request = %s %s, auth=%q, OAuth called=%v", method, path, authorization, called)
	}
	want := map[string]any{"display_name": "Example MCP", "auth": auth, "metadata": map[string]any{"team": "platform"}}
	if !reflect.DeepEqual(payload, want) {
		t.Error("credential payload does not match OAuth auth and CLI metadata")
	}
	if !strings.Contains(stdout.String(), "cred_123") || strings.Contains(stdout.String(), "secret-access") || strings.Contains(stdout.String(), "Authorize") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Authorize") {
		t.Fatal("missing progress on stderr")
	}
}

func TestVaultCredentialOAuthRejectsFlagsBeforeFlow(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing auth", nil},
		{"empty auth", []string{"--auth-json", " "}},
		{"empty MCP URL", []string{"--mcp-server-url", ""}},
		{"mutually exclusive", []string{"--auth-json", "{}", "--mcp-server-url", "https://example.com/mcp"}},
		{"OAuth flag without URL", []string{"--auth-json", "{}", "--no-browser"}},
		{"issuer override without URL", []string{"--auth-json", "{}", "--oauth-allow-issuer-mismatch"}},
		{"HTTP without opt-in", []string{"--mcp-server-url", "http://127.0.0.1/mcp"}},
		{"non-loopback HTTP", []string{"--mcp-server-url", "http://example.com/mcp", "--allow-http"}},
		{"invalid callback", []string{"--mcp-server-url", "https://example.com/mcp", "--callback-address", "0.0.0.0:1234"}},
		{"negative timeout", []string{"--mcp-server-url", "https://example.com/mcp", "--oauth-timeout", "-1s"}},
		{"invalid metadata", []string{"--mcp-server-url", "https://example.com/mcp", "--metadata", "invalid"}},
		{"invalid output", []string{"--mcp-server-url", "https://example.com/mcp", "-o", "invalid"}},
		{"long display name", []string{"--mcp-server-url", "https://example.com/mcp", "--display-name", strings.Repeat("a", 256)}},
		{"long metadata key", []string{"--mcp-server-url", "https://example.com/mcp", "--metadata", strings.Repeat("k", 65) + "=value"}},
		{"long metadata value", []string{"--mcp-server-url", "https://example.com/mcp", "--metadata", "key=" + strings.Repeat("v", 513)}},
	}
	tooManyMetadata := []string{"--mcp-server-url", "https://example.com/mcp"}
	for i := range 17 {
		tooManyMetadata = append(tooManyMetadata, "--metadata", fmt.Sprintf("key%d=value", i))
	}
	tests = append(tests, struct {
		name string
		args []string
	}{"too many metadata pairs", tooManyMetadata})
	original := newWorkspaceManagedAgentsClient
	t.Cleanup(func() { newWorkspaceManagedAgentsClient = original })
	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		t.Error("registry client initialized before flag validation")
		return &workspaceManagedAgentsClientMock{}, nil
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := (&agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}).newVaultCredentialCreateCommandWithOAuth(
				func(context.Context, mcpoauth.Options, io.Writer) (map[string]any, error) {
					t.Error("OAuth started for invalid flags")
					return nil, nil
				})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(append([]string{"--vault", "vlt_123"}, tt.args...))
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected flag validation error")
			}
		})
	}
}

func TestVaultCredentialOAuthFailures(t *testing.T) {
	for _, stage := range []string{"client", "authorization", "registration", "registration HTTP"} {
		t.Run(stage, func(t *testing.T) {
			created, authorized := false, false
			original := newWorkspaceManagedAgentsClient
			t.Cleanup(func() { newWorkspaceManagedAgentsClient = original })
			newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
				if stage == "client" {
					return nil, errors.New("registry not configured")
				}
				return &workspaceManagedAgentsClientMock{createFn: func(context.Context, string, any) (any, error) {
					created = true
					if stage == "registration HTTP" {
						return nil, &registry.APIError{StatusCode: http.StatusForbidden, Body: "echo secret-access secret-refresh"}
					}
					return nil, errors.New("echo secret-access secret-refresh")
				}}, nil
			}
			var stdout bytes.Buffer
			cmd := (&agentOptions{ioStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}}).newVaultCredentialCreateCommandWithOAuth(
				func(context.Context, mcpoauth.Options, io.Writer) (map[string]any, error) {
					authorized = true
					if stage == "authorization" {
						return nil, context.DeadlineExceeded
					}
					return map[string]any{"type": "mcp_oauth", "access_token": "secret-access"}, nil
				})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--vault", "vlt_123", "--display-name", "Example MCP", "--mcp-server-url", "https://example.com/mcp"})
			err := cmd.Execute()
			if err == nil || strings.Contains(err.Error(), "secret-") || stdout.Len() != 0 {
				t.Fatalf("unexpected result: error=%v stdout=%q", err, stdout.String())
			}
			if authorized != (stage != "client") || created != strings.HasPrefix(stage, "registration") {
				t.Fatalf("authorized=%v created=%v", authorized, created)
			}
			if stage == "registration HTTP" && !strings.Contains(err.Error(), "HTTP 403") {
				t.Fatal("missing registry HTTP status")
			}
			if stage == "authorization" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("lost cancellation cause")
			}
		})
	}
}

func TestVaultCredentialAuthJSONStillSupported(t *testing.T) {
	original := newWorkspaceManagedAgentsClient
	t.Cleanup(func() { newWorkspaceManagedAgentsClient = original })
	created := false
	newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
		return &workspaceManagedAgentsClientMock{createFn: func(_ context.Context, _ string, payload any) (any, error) {
			created = true
			auth := payload.(map[string]any)["auth"].(map[string]any)
			if auth["type"] != "static_bearer" || auth["token"] != "static-token" {
				t.Error("auth JSON changed")
			}
			return map[string]any{"id": "cred_static"}, nil
		}}, nil
	}
	cmd := (&agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}).newVaultCredentialCreateCommandWithOAuth(
		func(context.Context, mcpoauth.Options, io.Writer) (map[string]any, error) {
			t.Error("OAuth invoked for auth JSON")
			return nil, nil
		})
	cmd.SetArgs([]string{"--vault", "vlt_123", "--auth-json", `{"type":"static_bearer","token":"static-token"}`})
	if err := cmd.Execute(); err != nil || !created {
		t.Fatalf("created=%v error=%v", created, err)
	}
}

func TestOAuthCredentialPayloadLimits(t *testing.T) {
	for _, tt := range []struct {
		name    string
		payload map[string]any
		valid   bool
	}{
		{"optional display name", map[string]any{}, true},
		{"maximum name", map[string]any{"display_name": strings.Repeat("a", 255)}, true},
		{"maximum unicode name", map[string]any{"display_name": strings.Repeat("界", 255)}, true},
		{"UTF16 name too long", map[string]any{"display_name": strings.Repeat("𐐀", 128)}, false},
		{"maximum metadata", map[string]any{"metadata": map[string]string{strings.Repeat("k", 64): strings.Repeat("v", 512)}}, true},
		{"UTF16 metadata key too long", map[string]any{"metadata": map[string]string{strings.Repeat("𐐀", 33): "v"}}, false},
		{"UTF16 metadata value too long", map[string]any{"metadata": map[string]string{"k": strings.Repeat("𐐀", 257)}}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateOAuthCredentialPayload(tt.payload); (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}

func TestOAuthCredentialResultRedactsEmbeddedSecrets(t *testing.T) {
	for _, format := range []string{"json", "yaml", "text"} {
		t.Run(format, func(t *testing.T) {
			auth := map[string]any{"access_token": "secret-access", "refresh": map[string]any{"refresh_token": `secret-refresh"quoted`, "token_endpoint_auth": map[string]any{"client_secret": "secret-client"}}}
			encoded, err := json.Marshal(auth)
			if err != nil {
				t.Fatal(err)
			}
			result := map[string]any{
				"id":       "cred_123",
				"auth":     auth,
				"debug":    map[string]any{"request_body": string(encoded)},
				"metadata": map[string]any{"secret-access": string(encoded), "array": []any{"secret-access", map[string]any{"token": "secret-client"}}},
			}
			var out bytes.Buffer
			if err := renderJSONOrText(&out, format, redactOAuthCredentialResult(result, auth)); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "secret-") || strings.Contains(out.String(), "debug") || !strings.Contains(out.String(), "cred_123") {
				t.Fatalf("credential response was not safely rendered: %s", out.String())
			}
			if result["auth"] == "[REDACTED]" {
				t.Fatal("redaction mutated caller input")
			}
		})
	}
}
