// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package commands_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orca-ae/orca-cli/pkg/cmd/root"
	"github.com/orca-ae/orca-cli/pkg/cmd/workspace"
)

// Follow the URL printed by --no-browser, just as a user would. This exercises
// the public Cobra command and real OAuth flow without launching a desktop app.
type oauthBrowserOutput struct {
	bytes.Buffer
	t      *testing.T
	origin string
	opened bool
}

func (w *oauthBrowserOutput) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if w.opened {
		return n, err
	}
	for _, line := range strings.Split(w.Buffer.String(), "\n") {
		if !strings.HasPrefix(line, w.origin+"/authorize?") {
			continue
		}
		w.opened = true
		resp, requestErr := (&http.Client{Timeout: 2 * time.Second}).Get(line)
		if requestErr != nil {
			w.t.Errorf("browser request failed: %v", requestErr)
			break
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			w.t.Errorf("browser HTTP %d", resp.StatusCode)
		}
		break
	}
	return n, err
}

func TestVaultCredentialOAuthE2E(t *testing.T) {
	t.Setenv("ORCA_API_KEY", "")
	for _, tt := range []struct {
		name, method    string
		mismatch, allow bool
	}{
		{"exact issuer with Basic", "client_secret_basic", false, false},
		{"cross-domain rejected by default", "none", true, false},
		{"override with public client", "none", true, true},
		{"override with Basic", "client_secret_basic", true, true},
		{"override with Post", "client_secret_post", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var base, actual, challenge string
			var registrations, exchanges, creations int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path != "/token" && r.URL.Path != "/v1/vaults/vault-1/credentials" && r.Header.Get("Authorization") != "" {
					t.Error("credentials leaked to discovery, registration or authorization")
				}
				switch r.URL.Path {
				case "/mcp":
					w.Header().Set("WWW-Authenticate", "Bearer resource_metadata="+base+"/resource-metadata")
					w.WriteHeader(http.StatusUnauthorized)
				case "/resource-metadata":
					json.NewEncoder(w).Encode(map[string]any{"resource": base + "/mcp", "authorization_servers": []string{base + "/tenant"}})
				case "/.well-known/oauth-authorization-server/tenant":
					json.NewEncoder(w).Encode(map[string]any{
						"issuer": actual, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "registration_endpoint": base + "/register",
						"code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{tt.method},
						"authorization_response_iss_parameter_supported": true, "scopes_supported": []string{"offline_access"},
					})
				case "/register":
					registrations++
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					if payload["token_endpoint_auth_method"] != tt.method {
						t.Error("wrong requested authentication method")
					}
					json.NewEncoder(w).Encode(map[string]any{"client_id": "client-id", "client_secret": "client-secret", "token_endpoint_auth_method": tt.method})
				case "/authorize":
					q := r.URL.Query()
					challenge = q.Get("code_challenge")
					if q.Get("code_challenge_method") != "S256" || q.Get("resource") != base+"/mcp" || q.Get("scope") != "offline_access" {
						t.Error("authorization parameters lost")
					}
					callback := q.Get("redirect_uri") + "?" + url.Values{"state": {q.Get("state")}, "code": {"code-secret"}, "iss": {actual}}.Encode()
					http.Redirect(w, r, callback, http.StatusFound)
				case "/token":
					exchanges++
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					digest := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
					if base64.RawURLEncoding.EncodeToString(digest[:]) != challenge || r.PostForm.Get("code") != "code-secret" || r.PostForm.Get("resource") != base+"/mcp" {
						t.Error("token request lost PKCE, code or resource")
					}
					switch tt.method {
					case "client_secret_basic":
						id, secret, ok := r.BasicAuth()
						if !ok || id != "client-id" || secret != "client-secret" || r.PostForm.Has("client_secret") {
							t.Error("invalid Basic authentication")
						}
					case "client_secret_post":
						if r.Header.Get("Authorization") != "" || r.PostForm.Get("client_secret") != "client-secret" {
							t.Error("invalid Post authentication")
						}
					default:
						if r.Header.Get("Authorization") != "" || r.PostForm.Has("client_secret") {
							t.Error("public client sent secret")
						}
					}
					json.NewEncoder(w).Encode(map[string]any{"access_token": "access-secret", "refresh_token": "refresh-secret", "token_type": "Bearer", "expires_in": 3600})
				case "/v1/vaults/vault-1/credentials":
					creations++
					if r.Header.Get("Authorization") != "Bearer registry-token" {
						t.Error("incorrect registry authentication")
					}
					var payload struct {
						Auth struct {
							Type        string `json:"type"`
							AccessToken string `json:"access_token"`
							Refresh     struct {
								Token    string `json:"refresh_token"`
								Resource string `json:"resource"`
								Scope    string `json:"scope"`
								Method   struct {
									Type   string `json:"type"`
									Secret string `json:"client_secret"`
								} `json:"token_endpoint_auth"`
							} `json:"refresh"`
						} `json:"auth"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					auth := payload.Auth
					if auth.Type != "mcp_oauth" || auth.AccessToken != "access-secret" || auth.Refresh.Token != "refresh-secret" || auth.Refresh.Resource != base+"/mcp" || auth.Refresh.Scope != "offline_access" || auth.Refresh.Method.Type != tt.method {
						t.Error("invalid vault OAuth payload")
					}
					expectedSecret := ""
					if tt.method != "none" {
						expectedSecret = "client-secret"
					}
					if auth.Refresh.Method.Secret != expectedSecret {
						t.Error("refresh client secret lost or public client persisted a secret")
					}
					// Simulate a registry echo: stdout must redact tokens and secrets.
					json.NewEncoder(w).Encode(map[string]any{"id": "credential-1", "auth": auth, "metadata": map[string]string{"echo": "access-secret refresh-secret " + expectedSecret}})
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			base, actual = server.URL, server.URL+"/tenant"
			if tt.mismatch {
				actual = "https://auth.example.com/"
			}
			var stdout bytes.Buffer
			stderr := &oauthBrowserOutput{t: t, origin: base}
			cmd := root.NewCommand(workspace.IOStreams{Out: &stdout, ErrOut: stderr})
			cmd.SetOut(&stdout)
			cmd.SetErr(stderr)
			args := []string{"--registry-url", base, "--access-token", "registry-token", "agent", "vaults", "credentials", "create", "--vault", "vault-1", "--mcp-server-url", base + "/mcp", "--allow-http", "--no-browser", "--oauth-timeout", "3s", "--output", "json"}
			if tt.allow {
				args = append(args, "--oauth-allow-issuer-mismatch")
			}
			cmd.SetArgs(args)
			err := cmd.Execute()
			mu.Lock()
			defer mu.Unlock()
			if tt.mismatch && !tt.allow {
				if err == nil || !strings.Contains(err.Error(), "issuer mismatch") || registrations != 0 || exchanges != 0 || creations != 0 || stderr.opened {
					t.Fatalf("default did not reject cross-domain discovery: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if registrations != 1 || exchanges != 1 || creations != 1 || !stderr.opened {
				t.Fatal("OAuth did not complete through the vault")
			}
			var result map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result["id"] != "credential-1" {
				t.Fatal("stdout is not a credential JSON result")
			}
			if strings.Contains(stderr.String(), "Warning: --oauth-allow-issuer-mismatch") != tt.allow {
				t.Error("issuer warning did not follow explicit opt-in")
			}
			for _, secret := range []string{"client-secret", "access-secret", "refresh-secret", "code-secret", "registry-token"} {
				if strings.Contains(stdout.String()+stderr.String(), secret) {
					t.Error("CLI output leaked a credential")
				}
			}
		})
	}
}
