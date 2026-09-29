// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package mcpoauth

import (
	"bytes"
	"context"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTokenExpiry(t *testing.T) {
	now := time.Date(2026, 9, 10, 4, 0, 0, 123456789, time.FixedZone("test", 3600))
	for _, seconds := range []float64{0, 1, 3600, float64(int64(math.MaxInt64) / int64(time.Second))} {
		got, err := tokenExpiry(seconds, now)
		want := now.UTC().Add(time.Duration(int64(seconds)) * time.Second).Format(time.RFC3339Nano)
		if err != nil || got != want {
			t.Fatalf("expiry(%v) = %q, %v; want %q", seconds, got, err, want)
		}
	}
	for _, value := range []any{nil, "3600", true, float64(-1), 0.5, math.NaN(), math.Inf(1), math.Inf(-1), float64(int64(math.MaxInt64)/int64(time.Second) + 1), 1e100} {
		if _, err := tokenExpiry(value, now); err == nil {
			t.Fatalf("accepted invalid expiry %v", value)
		}
	}
}

func TestAuthorizeOptionalExpiry(t *testing.T) {
	for _, provided := range []bool{false, true} {
		name := "absent"
		if provided {
			name = "zero"
		}
		t.Run(name, func(t *testing.T) {
			f := newOAuthFixture(t)
			delete(f.tokens, "refresh_token")
			delete(f.tokens, "expires_in")
			if provided {
				f.tokens["expires_in"] = 0
			}
			started := time.Now()
			auth, err := authorize(context.Background(), f.options(), io.Discard, testBrowser)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"type": "mcp_oauth", "mcp_server_url": f.options().ServerURL, "access_token": "access-secret"}
			if provided {
				expires, ok := auth["expires_at"].(string)
				parsed, err := time.Parse(time.RFC3339Nano, expires)
				if !ok || err != nil || parsed.Before(started) || parsed.After(time.Now()) {
					t.Fatalf("invalid immediate expiry: %v", auth)
				}
				want["expires_at"] = expires
			}
			if !reflect.DeepEqual(auth, want) {
				t.Fatalf("auth=%v want=%v", auth, want)
			}
		})
	}
}

func TestAuthorizeGrantedRefreshScope(t *testing.T) {
	for _, tt := range []struct {
		name        string
		provided    bool
		scope, want string
	}{
		{"fallback", false, "", "read write offline_access"},
		{"narrower-grant", true, "read", "read"},
		{"grant-with-offline", true, "read offline_access", "read offline_access"},
		{"empty-grant", true, "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newOAuthFixture(t)
			delete(f.tokens, "expires_in")
			if tt.provided {
				f.tokens["scope"] = tt.scope
			}
			opts := f.options()
			opts.Scopes = []string{"read write"}
			auth, err := authorize(context.Background(), opts, io.Discard, testBrowser)
			if err != nil {
				t.Fatal(err)
			}
			refresh := map[string]any{"refresh_token": "refresh-secret", "token_endpoint": f.server.URL + "/token", "client_id": "dynamic-client", "token_endpoint_auth": map[string]any{"type": "none"}, "resource": f.server.URL}
			if tt.want != "" {
				refresh["scope"] = tt.want
			}
			want := map[string]any{"type": "mcp_oauth", "mcp_server_url": opts.ServerURL, "access_token": "access-secret", "refresh": refresh}
			if !reflect.DeepEqual(auth, want) {
				t.Fatalf("auth=%v want=%v", auth, want)
			}
		})
	}
}

func TestAuthorizeOmitsEmptyRequestedRefreshScope(t *testing.T) {
	f := newOAuthFixture(t)
	f.advertisePRM = false
	f.resourceScopes = []string{}
	delete(f.tokens, "expires_in")
	opts := f.options()
	opts.NoRefresh = true
	auth, err := authorize(context.Background(), opts, io.Discard, testBrowser)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"type": "mcp_oauth", "mcp_server_url": opts.ServerURL, "access_token": "access-secret",
		"refresh": map[string]any{"refresh_token": "refresh-secret", "token_endpoint": f.server.URL + "/token", "client_id": "dynamic-client", "token_endpoint_auth": map[string]any{"type": "none"}, "resource": f.server.URL},
	}
	if !reflect.DeepEqual(auth, want) {
		t.Fatalf("auth=%v want=%v", auth, want)
	}
}

func TestAuthorizeRejectsInvalidTokenMetadata(t *testing.T) {
	for key, values := range map[string][]any{
		"expires_in": {nil, "secret-invalid-expiry", true, -1, 1.5, 9223372037, 1e100, []any{1}},
		"scope":      {nil, true, 42, []string{"read"}, "read\nsecret-invalid-scope", "read\twrite", "read  write", " read", "read ", "read\"write", "read\\write", "非ASCII"},
	} {
		for _, value := range values {
			t.Run(key, func(t *testing.T) {
				f := newOAuthFixture(t)
				f.tokens[key] = value
				// Validate optional metadata even without refresh credentials.
				delete(f.tokens, "refresh_token")
				var progress bytes.Buffer
				auth, err := authorize(context.Background(), f.options(), &progress, testBrowser)
				if err == nil || auth != nil || !strings.Contains(err.Error(), key) {
					t.Fatalf("auth=%v error=%v", auth, err)
				}
				if strings.Contains(err.Error()+progress.String(), "secret-invalid") {
					t.Fatal("error or progress leaked token metadata")
				}
			})
		}
	}
}
