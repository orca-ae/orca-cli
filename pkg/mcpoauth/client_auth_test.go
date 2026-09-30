// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package mcpoauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestAuthorizeClientAuthentication(t *testing.T) {
	const clientID = "client:+ name"
	const secret = "secret:/+ päss"
	for _, tt := range []struct {
		name                           string
		supported                      []string
		requested, returned, effective string
	}{
		{"prefer public", []string{"client_secret_basic", "client_secret_post", "none"}, "none", "none", "none"},
		{"post", []string{"client_secret_basic", "client_secret_post"}, "client_secret_post", "client_secret_post", "client_secret_post"},
		{"basic", []string{"client_secret_basic"}, "client_secret_basic", "client_secret_basic", "client_secret_basic"},
		{"metadata default", nil, "client_secret_basic", "client_secret_basic", "client_secret_basic"},
		{"DCR default", []string{"none"}, "none", "", "client_secret_basic"},
		{"DCR changes method", []string{"none", "client_secret_post"}, "none", "client_secret_post", "client_secret_post"},
		{"DCR public ignores secret", []string{"client_secret_basic"}, "client_secret_basic", "none", "none"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newOAuthFixture(t)
			if tt.supported == nil {
				delete(f.metadata, "token_endpoint_auth_methods_supported")
			} else {
				f.metadata["token_endpoint_auth_methods_supported"] = tt.supported
			}
			f.registration["client_id"] = clientID
			f.registration["client_secret"] = secret
			if tt.returned == "" {
				delete(f.registration, "token_endpoint_auth_method")
			} else {
				f.registration["token_endpoint_auth_method"] = tt.returned
			}
			var progress bytes.Buffer
			auth, err := authorize(context.Background(), f.options(), &progress, testBrowser)
			if err != nil {
				t.Fatal(err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.registered) != 1 || len(f.authorizations) != 1 || len(f.exchanges) != 1 {
				t.Fatal("expected registration, authorization and token exchange")
			}
			if got := f.registered[0]["token_endpoint_auth_method"]; got != tt.requested {
				t.Errorf("requested method = %v, want %s", got, tt.requested)
			}
			if f.registered[0]["application_type"] != "native" {
				t.Error("CLI must register as a native application")
			}
			form, header, authorization := f.exchanges[0], f.exchangeHeaders[0], f.authorizations[0]
			switch tt.effective {
			case "client_secret_basic":
				req := &http.Request{Header: header}
				user, password, ok := req.BasicAuth()
				if !ok || user != "client%3A%2B+name" || password != "secret%3A%2F%2B+p%C3%A4ss" || form.Has("client_secret") {
					t.Error("Basic authentication must form-encode credentials and never put the secret in the body")
				}
			case "client_secret_post":
				if form.Get("client_secret") != secret || header.Get("Authorization") != "" {
					t.Error("Post authentication must send the secret only in the form")
				}
			case "none":
				if form.Has("client_secret") || header.Get("Authorization") != "" {
					t.Error("public client must ignore incidental DCR secrets")
				}
			}
			digest := sha256.Sum256([]byte(form.Get("code_verifier")))
			if authorization.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(digest[:]) || form.Get("resource") != f.server.URL {
				t.Error("client authentication lost PKCE or resource binding")
			}
			if authorization.Has("client_secret") {
				t.Error("authorization URL contains client secret")
			}
			refresh := auth["refresh"].(map[string]any)
			want := map[string]any{"type": tt.effective}
			if tt.effective != "none" {
				want["client_secret"] = secret
			}
			if !reflect.DeepEqual(refresh["token_endpoint_auth"], want) || refresh["client_id"] != clientID || refresh["resource"] != f.server.URL {
				t.Error("refresh settings did not preserve the actual DCR authentication method")
			}
			for _, value := range []string{secret, "secret%3A%2F%2B+p%C3%A4ss", "access-secret", "refresh-secret", "code-secret", form.Get("code_verifier"), header.Get("Authorization")} {
				if value != "" && strings.Contains(progress.String(), value) {
					t.Error("progress leaked OAuth credentials")
				}
			}
		})
	}
}
