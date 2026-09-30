// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package mcpoauth

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestDiscoveryIssuerOverrideStillValidatesMetadata(t *testing.T) {
	for _, actual := range []any{nil, "", 42, "http://auth.other.org/", "https://auth.other.org/?q=1", "https://auth.other.org/?", "https://auth.other.org/#", "https://user:secret@auth.other.org/", "https://auth.other.org:99999/"} {
		for _, allow := range []bool{false, true} {
			_, err := discoverIssuer(t, "https://mcp.example.com/tenant", actual, Options{AllowIssuerMismatch: allow})
			if err == nil {
				t.Fatalf("accepted invalid issuer %v (override=%v)", actual, allow)
			}
		}
	}
	actual := "https://auth.other.org/"
	cfg, err := discoverIssuer(t, "https://mcp.example.com/tenant", actual, Options{AllowIssuerMismatch: true})
	if err != nil || cfg.issuer != actual {
		t.Fatalf("override did not pin actual issuer: %q, %v", cfg.issuer, err)
	}
	_, err = discoverIssuer(t, "https://mcp.example.com/tenant", actual, Options{AllowIssuerMismatch: true, Issuer: "https://unadvertised.example.com"})
	if err == nil || !strings.Contains(err.Error(), "must match a protected-resource") {
		t.Fatalf("override changed issuer selection: %v", err)
	}
}

func TestAuthorizePinsMetadataIssuerAfterOverride(t *testing.T) {
	for _, callbackIssuer := range []string{"actual", "advertised", "same-domain", "missing", "empty"} {
		t.Run(callbackIssuer, func(t *testing.T) {
			f := newOAuthFixture(t)
			f.metadata["issuer"] = "https://auth.example.com/"
			opts := f.options()
			opts.AllowIssuerMismatch = true
			var progress bytes.Buffer
			auth, err := authorize(context.Background(), opts, &progress, func(ctx context.Context, target string) error {
				u, err := url.Parse(target)
				if err != nil {
					return err
				}
				q := u.Query()
				callback := url.Values{"state": {q.Get("state")}, "code": {"code-secret"}}
				switch callbackIssuer {
				case "actual":
					callback.Set("iss", "https://auth.example.com/")
				case "advertised":
					callback.Set("iss", f.server.URL)
				case "same-domain":
					callback.Set("iss", "https://another.example.com/")
				case "empty":
					callback.Set("iss", "")
				}
				return testBrowser(ctx, q.Get("redirect_uri")+"?"+callback.Encode())
			})
			if !strings.Contains(progress.String(), "Warning:") || !strings.Contains(progress.String(), "--oauth-allow-issuer-mismatch") {
				t.Fatal("missing issuer override warning")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if callbackIssuer == "actual" {
				if err != nil || auth["access_token"] != "access-secret" || len(f.exchanges) != 1 {
					t.Fatalf("valid callback failed: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "issuer") || len(f.exchanges) != 0 {
				t.Fatalf("invalid callback reached token exchange: %v", err)
			}
		})
	}
}
