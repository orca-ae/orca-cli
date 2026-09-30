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

func TestTokenTypeMustExplicitlyBeBearer(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value any
		valid bool
	}{
		{"missing", nil, false}, {"null", nil, false}, {"empty", "", false},
		{"number", 42, false}, {"DPoP", "DPoP", false}, {"lowercase", "bearer", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newOAuthFixture(t)
			f.tokens["token_type"] = tt.value
			if tt.name == "missing" {
				delete(f.tokens, "token_type")
			}
			var progress bytes.Buffer
			_, err := authorize(context.Background(), f.options(), &progress, testBrowser)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if strings.Contains(progress.String(), "access-secret") || err != nil && strings.Contains(err.Error(), "access-secret") {
				t.Fatal("token leaked in diagnostics")
			}
		})
	}
}

func TestAuthorizeRejectsInvalidClientAuthentication(t *testing.T) {
	for _, stage := range []string{"metadata empty", "metadata null", "DCR missing method and secret", "DCR null", "DCR unsupported", "DCR missing secret", "DCR invalid secret"} {
		t.Run(stage, func(t *testing.T) {
			f := newOAuthFixture(t)
			switch stage {
			case "metadata empty":
				f.metadata["token_endpoint_auth_methods_supported"] = []string{}
			case "metadata null":
				f.metadata["token_endpoint_auth_methods_supported"] = nil
			case "DCR missing method and secret":
				delete(f.registration, "token_endpoint_auth_method")
			case "DCR null":
				f.registration["token_endpoint_auth_method"] = nil
			case "DCR unsupported":
				f.registration["token_endpoint_auth_method"] = "private_key_jwt"
			case "DCR missing secret":
				f.registration["token_endpoint_auth_method"] = "client_secret_basic"
			case "DCR invalid secret":
				f.registration["token_endpoint_auth_method"] = "client_secret_post"
				f.registration["client_secret"] = 42
			}
			_, err := authorize(context.Background(), f.options(), io.Discard, func(context.Context, string) error {
				t.Error("browser opened with invalid client authentication")
				return nil
			})
			if err == nil {
				t.Fatal("expected client authentication error")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.authorizations) != 0 || len(f.exchanges) != 0 {
				t.Fatal("invalid client reached authorization or token exchange")
			}
		})
	}
}

func TestPreRegisteredClientRequiresPublicAuthentication(t *testing.T) {
	for _, methods := range [][]string{nil, {"client_secret_basic"}, {"client_secret_post"}} {
		f := newOAuthFixture(t)
		if methods == nil {
			delete(f.metadata, "token_endpoint_auth_methods_supported")
		} else {
			f.metadata["token_endpoint_auth_methods_supported"] = methods
		}
		opts := f.options()
		opts.ClientID = "public-client"
		_, err := authorize(context.Background(), opts, io.Discard, func(context.Context, string) error {
			t.Error("pre-registered client without a secret reached browser")
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "pre-registered public") {
			t.Fatalf("expected pre-registered public client error, got %v", err)
		}
		f.mu.Lock()
		if len(f.registered) != 0 || len(f.exchanges) != 0 {
			t.Error("pre-registered client reached DCR or token exchange")
		}
		f.mu.Unlock()
	}
}
