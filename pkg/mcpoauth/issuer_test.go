// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package mcpoauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type oauthRoundTripper func(*http.Request) (*http.Response, error)

func (r oauthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return r(req) }

// Exercise real discovery against protocol responses, without DNS or TLS overrides
// in the production Authorize transport. No request may follow the returned issuer.
func discoverIssuer(t *testing.T, advertised string, actual any, opts Options) (config, error) {
	t.Helper()
	const server = "https://mcp.example.com/mcp"
	u, err := url.Parse(advertised)
	if err != nil {
		t.Fatal(err)
	}
	metadataURL := u.Scheme + "://" + u.Host + "/.well-known/oauth-authorization-server" + strings.TrimRight(u.EscapedPath(), "/")
	metadata, err := url.Parse(metadataURL)
	if err != nil {
		t.Fatal(err)
	}
	metadataURL = metadata.String()
	opts.ServerURL = server
	client := &http.Client{Transport: oauthRoundTripper(func(r *http.Request) (*http.Response, error) {
		var body any
		status := http.StatusOK
		switch r.URL.String() {
		case server:
			status = http.StatusUnauthorized
			body = map[string]any{}
		case "https://mcp.example.com/.well-known/oauth-protected-resource/mcp":
			body = map[string]any{"resource": server, "authorization_servers": []string{advertised}}
		case metadataURL:
			body = map[string]any{
				"issuer": actual, "authorization_endpoint": "https://mcp.example.com/authorize",
				"token_endpoint": "https://mcp.example.com/token", "registration_endpoint": "https://mcp.example.com/register",
				"code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"none"},
			}
		default:
			t.Fatalf("unexpected discovery request: %s", r.URL)
		}
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})}
	f := flow{opts: opts, client: client}
	return f.discover(context.Background())
}

func TestDiscoveryIssuerCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name, advertised, actual string
		valid                    bool
	}{
		{"exact", "https://mcp.example.com/tenant", "https://mcp.example.com/tenant", true},
		{"delegated issuer", "https://mcp.example.com/tenant", "https://auth.example.com/", true},
		{"same host different path", "https://mcp.example.com/tenant", "https://mcp.example.com/", true},
		{"multi-label suffix", "https://mcp.example.co.uk/tenant", "https://auth.example.co.uk/", true},
		{"different registrants", "https://mcp.example.co.uk", "https://auth.other.co.uk", false},
		{"private suffix", "https://alice.github.io", "https://bob.github.io", false},
		{"same private tenant", "https://mcp.alice.github.io", "https://auth.alice.github.io", true},
		{"suffix spoof", "https://mcp.example.com", "https://auth.example.com.attacker.org", false},
		{"case and default port", "https://MCP.EXAMPLE.COM:443/tenant", "https://auth.example.com/", true},
		{"different port", "https://mcp.example.com", "https://auth.example.com:8443/", false},
		{"IDNA", "https://mcp.bücher.de", "https://auth.xn--bcher-kva.de/", true},
		{"exact IP", "https://127.0.0.1/tenant", "https://127.0.0.1/tenant", true},
		{"IP path mismatch", "https://127.0.0.1/tenant", "https://127.0.0.1/", false},
		{"IP address mismatch", "https://127.0.0.1", "https://127.0.0.2", false},
		{"localhost", "https://localhost/tenant", "https://localhost/", false},
		{"unknown suffix", "https://mcp.example.internal", "https://auth.example.internal", false},
		{"public suffix only", "https://co.uk/tenant", "https://co.uk/", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := discoverIssuer(t, tt.advertised, tt.actual, Options{})
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if tt.valid && cfg.issuer != tt.actual {
				t.Fatalf("pinned issuer = %q, want %q", cfg.issuer, tt.actual)
			}
		})
	}
}
