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

func TestPublicClientAuthenticationMustBeExplicit(t *testing.T) {
	for _, stage := range []string{"metadata missing", "metadata empty", "DCR missing", "DCR null"} {
		t.Run(stage, func(t *testing.T) {
			f := newOAuthFixture(t)
			switch stage {
			case "metadata missing":
				delete(f.metadata, "token_endpoint_auth_methods_supported")
			case "metadata empty":
				f.metadata["token_endpoint_auth_methods_supported"] = []string{}
			case "DCR missing":
				delete(f.registration, "token_endpoint_auth_method")
			case "DCR null":
				f.registration["token_endpoint_auth_method"] = nil
			}
			_, err := authorize(context.Background(), f.options(), io.Discard, func(context.Context, string) error {
				t.Error("browser opened for a client without explicit public authentication")
				return nil
			})
			if err == nil {
				t.Fatal("expected public client authentication error")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.authorizations) != 0 || len(f.exchanges) != 0 {
				t.Fatal("non-public client reached authorization or token exchange")
			}
		})
	}
}
