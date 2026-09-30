// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package mcpoauth

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func proxyFixture(t *testing.T) *oauthFixture {
	f := newOAuthFixture(t)
	f.issuerPath = "/proxy"
	f.metadataPath = "/.well-known/oauth-authorization-server/proxy"
	f.metadata["issuer"] = "https://identity.example/"
	f.callbackIssuer = "https://identity.example/"
	return f
}

func TestAuthorizeProxyIssuerPin(t *testing.T) {
	for _, tt := range []struct {
		name, issuer string
		valid        bool
	}{
		{"no implicit issuer change", "", false},
		{"explicit trusted issuer", "https://identity.example/", true},
		{"wrong explicit issuer", "https://other.example/", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := proxyFixture(t)
			opts := f.options()
			opts.Issuer = tt.issuer
			var progress bytes.Buffer
			auth, err := authorize(context.Background(), opts, &progress, testBrowser)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%t, error=%v", tt.valid, err)
			}
			if tt.valid {
				if auth["access_token"] != "access-secret" || len(f.exchanges) != 1 {
					t.Fatal("pinned proxy did not finish the authorization flow")
				}
			} else {
				if auth != nil || len(f.registered) != 0 || len(f.exchanges) != 0 {
					t.Fatal("issuer mismatch reached registration or token exchange")
				}
				if tt.issuer == "" && !strings.Contains(err.Error(), "--oauth-issuer") {
					t.Fatal("issuer mismatch lacks actionable pinning guidance")
				}
			}
		})
	}
}

func TestAuthorizePinnedProxyStillChecksCallbackIssuer(t *testing.T) {
	f := proxyFixture(t)
	f.callbackIssuer = "https://other.example/"
	opts := f.options()
	opts.Issuer = "https://identity.example/"
	_, err := authorize(context.Background(), opts, io.Discard, testBrowser)
	if err == nil || !strings.Contains(err.Error(), "callback issuer mismatch") || len(f.exchanges) != 0 {
		t.Fatalf("unexpected callback validation result: %v", err)
	}
}

func TestAuthorizeBasicClientAuthentication(t *testing.T) {
	for _, stage := range []string{"dynamic", "pre-registered", "metadata default", "registration default", "pinned proxy"} {
		t.Run(stage, func(t *testing.T) {
			var f *oauthFixture
			if stage == "pinned proxy" {
				f = proxyFixture(t)
			} else {
				f = newOAuthFixture(t)
			}
			f.metadata["token_endpoint_auth_methods_supported"] = []string{"client_secret_basic"}
			f.registration["token_endpoint_auth_method"] = "client_secret_basic"
			f.registration["client_id"] = "client:id +space"
			f.registration["client_secret"] = "client-secret:+ &space"
			f.tokenAuthMethod = "client_secret_basic"
			f.expectedBasicID = "client:id +space"
			f.expectedBasicSecret = "client-secret:+ &space"
			opts := f.options()
			if stage == "pre-registered" {
				opts.ClientID, opts.ClientSecret = f.expectedBasicID, f.expectedBasicSecret
			}
			if stage == "metadata default" {
				delete(f.metadata, "token_endpoint_auth_methods_supported")
			}
			if stage == "registration default" {
				delete(f.registration, "token_endpoint_auth_method")
			}
			if stage == "pinned proxy" {
				opts.Issuer = "https://identity.example/"
			}
			var progress bytes.Buffer
			auth, err := authorize(context.Background(), opts, &progress, testBrowser)
			if err != nil {
				t.Fatal(err)
			}
			refresh := auth["refresh"].(map[string]any)
			endpointAuth := refresh["token_endpoint_auth"].(map[string]any)
			if endpointAuth["type"] != "client_secret_basic" || endpointAuth["client_secret"] != f.expectedBasicSecret || refresh["client_id"] != f.expectedBasicID {
				t.Fatal("refresh settings do not preserve Basic client authentication")
			}
			if refresh["resource"] != f.server.URL || len(f.exchanges) != 1 {
				t.Fatal("resource binding or token exchange missing")
			}
			if stage == "pre-registered" {
				if len(f.registered) != 0 {
					t.Fatal("pre-registered client invoked dynamic registration")
				}
			} else if len(f.registered) != 1 || f.registered[0]["token_endpoint_auth_method"] != "client_secret_basic" {
				t.Fatal("dynamic registration did not request Basic client authentication")
			}
			for _, secret := range []string{f.expectedBasicSecret, "access-secret", "refresh-secret", "code-secret", f.exchanges[0].Get("code_verifier")} {
				if strings.Contains(progress.String(), secret) {
					t.Fatal("OAuth progress exposed credentials")
				}
			}
		})
	}
}

func TestAuthorizePrefersPublicClientWhenAvailable(t *testing.T) {
	f := newOAuthFixture(t)
	f.metadata["token_endpoint_auth_methods_supported"] = []string{"client_secret_basic", "none"}
	auth, err := authorize(context.Background(), f.options(), io.Discard, testBrowser)
	if err != nil {
		t.Fatal(err)
	}
	endpointAuth := auth["refresh"].(map[string]any)["token_endpoint_auth"].(map[string]any)
	if endpointAuth["type"] != "none" || f.registered[0]["token_endpoint_auth_method"] != "none" {
		t.Fatal("public client was not preferred")
	}
}

func TestAuthorizeBasicRegistrationRequiresSecret(t *testing.T) {
	for _, value := range []any{nil, "", 42} {
		f := newOAuthFixture(t)
		f.metadata["token_endpoint_auth_methods_supported"] = []string{"client_secret_basic"}
		f.registration["token_endpoint_auth_method"] = "client_secret_basic"
		if value != nil {
			f.registration["client_secret"] = value
		}
		_, err := authorize(context.Background(), f.options(), io.Discard, func(context.Context, string) error {
			t.Error("browser opened without a valid client secret")
			return nil
		})
		if err == nil || len(f.exchanges) != 0 {
			t.Fatal("invalid Basic registration reached token exchange")
		}
	}
}

func TestAuthorizeBasicPreRegisteredClientRequiresSecret(t *testing.T) {
	f := newOAuthFixture(t)
	f.metadata["token_endpoint_auth_methods_supported"] = []string{"client_secret_basic"}
	opts := f.options()
	opts.ClientID = "registered-client"
	_, err := authorize(context.Background(), opts, io.Discard, testBrowser)
	if err == nil || !strings.Contains(err.Error(), "requires a client secret") || len(f.registered) != 0 {
		t.Fatalf("unexpected validation result: %v", err)
	}
}

func TestClientSecretRequiresClientID(t *testing.T) {
	if err := ValidateOptions(Options{ServerURL: "https://mcp.example.com/mcp", ClientSecret: "private-secret"}); err == nil {
		t.Fatal("client secret without client ID was accepted")
	}
}

func TestPinnedProxyEndpointsStayOnProxyOrigin(t *testing.T) {
	for _, field := range []string{"authorization_endpoint", "token_endpoint", "registration_endpoint"} {
		t.Run(field, func(t *testing.T) {
			f := proxyFixture(t)
			f.metadata[field] = "https://other.example/endpoint"
			opts := f.options()
			opts.Issuer = "https://identity.example/"
			_, err := authorize(context.Background(), opts, io.Discard, testBrowser)
			if err == nil || !strings.Contains(err.Error(), "different origin") || len(f.registered) != 0 || len(f.exchanges) != 0 {
				t.Fatalf("unexpected endpoint validation result: %v", err)
			}
		})
	}
}

func TestMultipleAuthorizationServersRequireExplicitSelection(t *testing.T) {
	for _, stage := range []string{"missing selection", "unadvertised issuer", "selected server"} {
		t.Run(stage, func(t *testing.T) {
			f := newOAuthFixture(t)
			f.issuerPath = "/selected"
			f.metadataPath = "/.well-known/oauth-authorization-server/selected"
			f.metadata["issuer"] = f.server.URL + "/selected"
			f.authorizationServers = []string{f.server.URL + "/other", f.server.URL + "/selected"}
			opts := f.options()
			if stage == "unadvertised issuer" {
				opts.Issuer = "https://identity.example/"
			} else if stage == "selected server" {
				opts.Issuer = f.server.URL + "/selected"
			}
			auth, err := authorize(context.Background(), opts, io.Discard, testBrowser)
			if stage == "selected server" {
				if err != nil || auth["access_token"] != "access-secret" {
					t.Fatalf("advertised server selection failed: %v", err)
				}
			} else if err == nil || len(f.registered) != 0 || len(f.exchanges) != 0 {
				t.Fatal("ambiguous or unadvertised selection reached authorization")
			}
		})
	}
}
