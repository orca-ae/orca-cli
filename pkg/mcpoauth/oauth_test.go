// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package mcpoauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// All protocol assertions use HTTP boundaries and the browser seam, not the
// implementation's discovery, callback, or token helpers.
type oauthFixture struct {
	t               *testing.T
	server          *httptest.Server
	metadata        map[string]any
	tokens          map[string]any
	metadataPath    string
	issuerPath      string
	advertisePRM    bool
	resourceScopes  []string
	tokenStatus     int
	registration    map[string]any
	mu              sync.Mutex
	registered      []map[string]any
	authorizations  []url.Values
	exchanges       []url.Values
	exchangeHeaders []http.Header
}

func newOAuthFixture(t *testing.T) *oauthFixture {
	t.Helper()
	f := &oauthFixture{t: t, metadataPath: "/.well-known/oauth-authorization-server", advertisePRM: true, resourceScopes: []string{"read"},
		tokens:       map[string]any{"access_token": "access-secret", "refresh_token": "refresh-secret", "token_type": "Bearer", "expires_in": 3600},
		registration: map[string]any{"client_id": "dynamic-client", "token_endpoint_auth_method": "none"}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)
	f.metadata = map[string]any{
		"issuer":                                         f.server.URL,
		"authorization_endpoint":                         f.server.URL + "/authorize",
		"token_endpoint":                                 f.server.URL + "/token",
		"registration_endpoint":                          f.server.URL + "/register",
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"scopes_supported":                               []string{"read", "offline_access"},
		"authorization_response_iss_parameter_supported": true,
	}
	return f
}

func (f *oauthFixture) options() Options {
	return Options{ServerURL: f.server.URL + "/mcp", AllowHTTP: true, Timeout: 3 * time.Second}
}

func (f *oauthFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/mcp":
		if r.Method != http.MethodPost {
			f.t.Errorf("MCP probe method = %s", r.Method)
		}
		if f.advertisePRM {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata=%q, scope="read"`, f.server.URL+"/resource-metadata"))
		}
		w.WriteHeader(http.StatusUnauthorized)
	case "/resource-metadata", "/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource":
		json.NewEncoder(w).Encode(map[string]any{"resource": f.server.URL, "authorization_servers": []string{f.server.URL + f.issuerPath}, "scopes_supported": f.resourceScopes})
	case f.metadataPath:
		json.NewEncoder(w).Encode(f.metadata)
	case "/register":
		if r.Method != http.MethodPost {
			f.t.Errorf("registration method = %s", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			f.t.Errorf("decode registration: %v", err)
		}
		f.mu.Lock()
		f.registered = append(f.registered, payload)
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(f.registration)
	case "/authorize":
		q := r.URL.Query()
		f.mu.Lock()
		f.authorizations = append(f.authorizations, q)
		f.mu.Unlock()
		callback, err := url.Parse(q.Get("redirect_uri"))
		if err != nil {
			http.Error(w, "bad redirect", 400)
			return
		}
		callback.RawQuery = url.Values{"code": {"code-secret"}, "state": {q.Get("state")}, "iss": {f.server.URL + f.issuerPath}}.Encode()
		http.Redirect(w, r, callback.String(), http.StatusFound)
	case "/token":
		if r.Method != http.MethodPost {
			f.t.Errorf("token method = %s", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			f.t.Errorf("token content type = %q", r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			f.t.Errorf("parse token form: %v", err)
		}
		f.mu.Lock()
		f.exchanges = append(f.exchanges, r.PostForm)
		f.exchangeHeaders = append(f.exchangeHeaders, r.Header.Clone())
		f.mu.Unlock()
		if f.tokenStatus != 0 {
			w.WriteHeader(f.tokenStatus)
		}
		json.NewEncoder(w).Encode(f.tokens)
	default:
		http.NotFound(w, r)
	}
}

func testBrowser(ctx context.Context, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("browser HTTP %d", resp.StatusCode)
	}
	return err
}

func assertCallbackClosed(t *testing.T, redirect string) {
	t.Helper()
	u, err := url.Parse(redirect)
	if err != nil || u.Host == "" {
		t.Fatalf("invalid callback URL %q", redirect)
	}
	conn, err := net.DialTimeout("tcp", u.Host, 200*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Errorf("callback listener still open at %s", u.Host)
	}
}

func TestAuthorizeEndToEnd(t *testing.T) {
	for _, noRefresh := range []bool{false, true} {
		t.Run(fmt.Sprintf("no_refresh=%t", noRefresh), func(t *testing.T) {
			f := newOAuthFixture(t)
			opts := f.options()
			opts.NoRefresh = noRefresh
			opts.Scopes = []string{"read write", "read"}
			if noRefresh {
				delete(f.tokens, "refresh_token")
			}
			var progress bytes.Buffer
			started := time.Now()
			auth, err := authorize(context.Background(), opts, &progress, testBrowser)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"type": "mcp_oauth", "mcp_server_url": opts.ServerURL, "access_token": "access-secret"}
			expires, ok := auth["expires_at"].(string)
			parsedExpiry, parseErr := time.Parse(time.RFC3339Nano, expires)
			if !ok || parseErr != nil || parsedExpiry.Before(started.Add(time.Hour)) || parsedExpiry.After(time.Now().Add(time.Hour)) {
				t.Fatalf("invalid expires_at: %v", auth["expires_at"])
			}
			want["expires_at"] = expires
			if !noRefresh {
				want["refresh"] = map[string]any{"refresh_token": "refresh-secret", "token_endpoint": f.server.URL + "/token", "client_id": "dynamic-client", "token_endpoint_auth": map[string]any{"type": "none"}, "resource": f.server.URL, "scope": "read write offline_access"}
			}
			if !reflect.DeepEqual(auth, want) {
				t.Errorf("auth = %#v, want %#v", auth, want)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.registered) != 1 || len(f.authorizations) != 1 || len(f.exchanges) != 1 {
				t.Fatalf("requests: registration=%d authorization=%d exchange=%d", len(f.registered), len(f.authorizations), len(f.exchanges))
			}
			reg, q, form := f.registered[0], f.authorizations[0], f.exchanges[0]
			scope := "read write"
			grants := []any{"authorization_code"}
			if !noRefresh {
				scope += " offline_access"
				grants = append(grants, "refresh_token")
			}
			wantReg := map[string]any{"client_name": "orca-cli", "application_type": "native", "redirect_uris": []any{q.Get("redirect_uri")}, "grant_types": grants, "response_types": []any{"code"}, "token_endpoint_auth_method": "none", "scope": scope}
			if !reflect.DeepEqual(reg, wantReg) {
				t.Errorf("DCR payload = %#v, want %#v", reg, wantReg)
			}
			for key, want := range map[string]string{"response_type": "code", "client_id": "dynamic-client", "resource": f.server.URL, "scope": scope, "code_challenge_method": "S256"} {
				if q.Get(key) != want {
					t.Errorf("authorization %s = %q, want %q", key, q.Get(key), want)
				}
			}
			if q.Get("state") == "" {
				t.Error("empty state")
			}
			if q.Has("access_type") {
				t.Error("authorization requested provider-specific access_type")
			}
			redirect, err := url.Parse(q.Get("redirect_uri"))
			if err != nil {
				t.Fatal(err)
			}
			if redirect.Scheme != "http" || redirect.Hostname() != "127.0.0.1" || redirect.Port() == "0" || redirect.Port() == "" || redirect.Path != "/oauth/callback" {
				t.Errorf("callback = %s", redirect)
			}
			for key, want := range map[string]string{"grant_type": "authorization_code", "code": "code-secret", "client_id": "dynamic-client", "redirect_uri": redirect.String(), "resource": f.server.URL} {
				if form.Get(key) != want {
					t.Errorf("token %s = %q, want %q", key, form.Get(key), want)
				}
			}
			verifier := form.Get("code_verifier")
			if len(verifier) < 43 || len(verifier) > 128 {
				t.Errorf("PKCE verifier length = %d", len(verifier))
			}
			sum := sha256.Sum256([]byte(verifier))
			if q.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(sum[:]) {
				t.Error("PKCE challenge does not match verifier")
			}
			if form.Has("client_secret") || f.exchangeHeaders[0].Get("Authorization") != "" {
				t.Error("public client sent client authentication")
			}
			for _, secret := range []string{"access-secret", "refresh-secret", "code-secret", verifier} {
				if secret != "" && strings.Contains(progress.String(), secret) {
					t.Errorf("progress leaked %q", secret)
				}
			}
			assertCallbackClosed(t, redirect.String())
		})
	}
}

func TestAuthorizeDiscoveryAndPreRegisteredClient(t *testing.T) {
	for _, path := range []string{"/.well-known/oauth-authorization-server", "/.well-known/openid-configuration", "/.well-known/oauth-authorization-server/tenant", "/tenant/.well-known/openid-configuration"} {
		t.Run(path, func(t *testing.T) {
			f := newOAuthFixture(t)
			f.metadataPath = path
			f.advertisePRM = false
			if strings.Contains(path, "tenant") {
				f.issuerPath = "/tenant"
				f.metadata["issuer"] = f.server.URL + f.issuerPath
			}
			delete(f.metadata, "registration_endpoint")
			opts := f.options()
			opts.ClientID = "existing-client"
			auth, err := authorize(context.Background(), opts, io.Discard, testBrowser)
			if err != nil {
				t.Fatal(err)
			}
			if auth["access_token"] != "access-secret" {
				t.Errorf("auth = %#v", auth)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.registered) != 0 {
				t.Error("pre-registered client used DCR")
			}
			if len(f.exchanges) != 1 || f.exchanges[0].Get("client_id") != opts.ClientID {
				t.Errorf("token requests = %v", f.exchanges)
			}
		})
	}
}

func TestAuthorizeRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name, key string
		value     any
	}{
		{"missing issuer", "issuer", nil},
		{"issuer mismatch", "issuer", "https://other.example"},
		{"missing PKCE", "code_challenge_methods_supported", nil},
		{"plain PKCE only", "code_challenge_methods_supported", []string{"plain"}},
		{"unsupported authentication", "token_endpoint_auth_methods_supported", []string{"private_key_jwt"}},
		{"missing token endpoint", "token_endpoint", nil},
		{"missing registration endpoint", "registration_endpoint", nil},
		{"unsafe endpoint", "token_endpoint", "http://example.com/token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newOAuthFixture(t)
			if tt.value == nil {
				delete(f.metadata, tt.key)
			} else {
				f.metadata[tt.key] = tt.value
			}
			called := false
			auth, err := authorize(context.Background(), f.options(), io.Discard, func(context.Context, string) error { called = true; return nil })
			if err == nil || auth != nil {
				t.Fatalf("auth=%v error=%v, want failure", auth, err)
			}
			if called {
				t.Error("opened browser with invalid metadata")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.exchanges) != 0 || len(f.registered) != 0 {
				t.Error("invalid metadata reached registration or token exchange")
			}
		})
	}
}

func TestAuthorizeInvalidStateCanRetry(t *testing.T) {
	f := newOAuthFixture(t)
	browser := func(ctx context.Context, target string) error {
		u, err := url.Parse(target)
		if err != nil {
			return err
		}
		callback := u.Query().Get("redirect_uri")
		for _, suffix := range []string{"?state=wrong&code=attacker-code", "?code=attacker-code"} {
			resp, err := (&http.Client{Timeout: time.Second}).Get(callback + suffix)
			if err != nil {
				return err
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("invalid state HTTP = %d", resp.StatusCode)
			}
		}
		return testBrowser(ctx, target)
	}
	if _, err := authorize(context.Background(), f.options(), io.Discard, browser); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.exchanges) != 1 || f.exchanges[0].Get("code") != "code-secret" {
		t.Errorf("exchanges = %v", f.exchanges)
	}
}

func TestAuthorizeRejectsCallbackErrors(t *testing.T) {
	for _, mode := range []string{"missing issuer", "issuer mismatch", "missing code", "access denied"} {
		t.Run(mode, func(t *testing.T) {
			f := newOAuthFixture(t)
			browser := func(ctx context.Context, target string) error {
				u, err := url.Parse(target)
				if err != nil {
					return err
				}
				q := url.Values{"state": {u.Query().Get("state")}, "code": {"code-secret"}, "iss": {f.server.URL}}
				switch mode {
				case "missing issuer":
					q.Del("iss")
				case "issuer mismatch":
					q.Set("iss", f.server.URL+"/other")
				case "missing code":
					q.Del("code")
				case "access denied":
					q.Set("error", "access_denied")
					q.Set("error_description", "untrusted-secret-description")
				}
				// A validating receiver may return 400; either response must not
				// cause an authorization-code exchange.
				_ = testBrowser(ctx, u.Query().Get("redirect_uri")+"?"+q.Encode())
				return nil
			}
			var progress bytes.Buffer
			auth, err := authorize(context.Background(), f.options(), &progress, browser)
			if err == nil || auth != nil {
				t.Fatalf("auth=%v error=%v", auth, err)
			}
			if strings.Contains(err.Error()+progress.String(), "code-secret") || strings.Contains(err.Error()+progress.String(), "untrusted-secret-description") {
				t.Error("callback failure leaked untrusted secrets")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.exchanges) != 0 {
				t.Error("invalid callback reached token exchange")
			}
		})
	}
}

func TestAuthorizeTokenFailureRedaction(t *testing.T) {
	f := newOAuthFixture(t)
	f.tokenStatus = http.StatusBadRequest
	f.tokens = map[string]any{"error": "invalid_grant", "error_description": "code-secret refresh-secret access-secret", "access_token": "access-secret"}
	var progress bytes.Buffer
	auth, err := authorize(context.Background(), f.options(), &progress, testBrowser)
	if err == nil || auth != nil {
		t.Fatalf("auth=%v error=%v", auth, err)
	}
	for _, secret := range []string{"code-secret", "refresh-secret", "access-secret"} {
		if strings.Contains(err.Error()+progress.String(), secret) {
			t.Errorf("error/progress leaked %q", secret)
		}
	}
}

func TestAuthorizeRejectsInvalidRegistrationAndTokens(t *testing.T) {
	for _, stage := range []string{"registration", "token"} {
		for _, value := range []any{nil, "", 42} {
			t.Run(fmt.Sprintf("%s/%v", stage, value), func(t *testing.T) {
				f := newOAuthFixture(t)
				payload, key := f.registration, "client_id"
				if stage == "token" {
					payload, key = f.tokens, "access_token"
				}
				if value == nil {
					delete(payload, key)
				} else {
					payload[key] = value
				}
				auth, err := authorize(context.Background(), f.options(), io.Discard, testBrowser)
				if err == nil || auth != nil {
					t.Fatalf("auth=%v error=%v, want invalid %s response", auth, err, stage)
				}
				f.mu.Lock()
				defer f.mu.Unlock()
				if stage == "registration" && (len(f.authorizations) != 0 || len(f.exchanges) != 0) {
					t.Error("invalid registration reached authorization or token exchange")
				}
			})
		}
	}
	t.Run("secret authentication without secret", func(t *testing.T) {
		f := newOAuthFixture(t)
		f.registration["token_endpoint_auth_method"] = "client_secret_basic"
		if auth, err := authorize(context.Background(), f.options(), io.Discard, testBrowser); err == nil || auth != nil {
			t.Fatalf("auth=%v error=%v", auth, err)
		}
	})
}

func TestAuthorizeRefreshOutputUsesReturnedToken(t *testing.T) {
	for _, noRefresh := range []bool{false, true} {
		t.Run(fmt.Sprintf("no_refresh=%t", noRefresh), func(t *testing.T) {
			f := newOAuthFixture(t)
			opts := f.options()
			opts.NoRefresh = noRefresh
			// NoRefresh controls the request, not whether a returned refresh token
			// is preserved. Conversely, requesting refresh does not guarantee one.
			if !noRefresh {
				delete(f.tokens, "refresh_token")
			}
			opts.Timeout = 0 // Exercise the default rather than an immediate deadline.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var progress bytes.Buffer
			auth, err := authorize(ctx, opts, &progress, testBrowser)
			if err != nil {
				t.Fatal(err)
			}
			_, hasRefresh := auth["refresh"]
			if hasRefresh != noRefresh {
				t.Errorf("refresh present=%t, want %t", hasRefresh, noRefresh)
			}
			if !noRefresh && !strings.Contains(strings.ToLower(progress.String()), "refresh") {
				t.Error("missing warning for absent refresh token")
			}
		})
	}
}

func TestAuthorizeExplicitIssuerMustMatchResourceMetadata(t *testing.T) {
	f := newOAuthFixture(t)
	opts := f.options()
	opts.Issuer = f.server.URL + "/unadvertised"
	called := false
	auth, err := authorize(context.Background(), opts, io.Discard, func(context.Context, string) error { called = true; return nil })
	if err == nil || auth != nil {
		t.Fatalf("auth=%v error=%v", auth, err)
	}
	if called {
		t.Error("issuer mismatch reached browser")
	}
}

func TestAuthorizeTimeoutAndCancellationCloseCallback(t *testing.T) {
	for _, cancelContext := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancelContext), func(t *testing.T) {
			f := newOAuthFixture(t)
			opts := f.options()
			opts.Timeout = 150 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var redirect string
			start := time.Now()
			auth, err := authorize(ctx, opts, io.Discard, func(_ context.Context, target string) error {
				u, err := url.Parse(target)
				if err != nil {
					return err
				}
				redirect = u.Query().Get("redirect_uri")
				if cancelContext {
					cancel()
				}
				return nil
			})
			if err == nil || auth != nil {
				t.Fatalf("auth=%v error=%v", auth, err)
			}
			if time.Since(start) > 2*time.Second {
				t.Error("timeout/cancellation did not promptly finish")
			}
			if redirect == "" {
				t.Fatal("flow never reached browser")
			}
			assertCallbackClosed(t, redirect)
		})
	}
}

// Capture the manual URL without inspecting a buffer concurrently with writes.
type authorizationURLWriter struct {
	mu   sync.Mutex
	text string
	once sync.Once
	urls chan string
}

func (w *authorizationURLWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.text += string(p)
	for _, word := range strings.Fields(w.text) {
		u, err := url.Parse(word)
		if err == nil && u.Path == "/authorize" && u.Query().Get("state") != "" && u.Query().Get("redirect_uri") != "" {
			w.once.Do(func() { w.urls <- word })
		}
	}
	return len(p), nil
}

func TestAuthorizeManualBrowserAndLaunchFallback(t *testing.T) {
	for _, noBrowser := range []bool{false, true} {
		t.Run(fmt.Sprintf("no_browser=%t", noBrowser), func(t *testing.T) {
			f := newOAuthFixture(t)
			opts := f.options()
			opts.NoBrowser = noBrowser
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			w := &authorizationURLWriter{urls: make(chan string, 1)}
			done := make(chan error, 1)
			go func() {
				select {
				case target := <-w.urls:
					done <- testBrowser(ctx, target)
				case <-ctx.Done():
					done <- ctx.Err()
				}
			}()
			calls := 0
			auth, err := authorize(ctx, opts, w, func(context.Context, string) error { calls++; return errors.New("browser unavailable") })
			if err != nil {
				cancel()
				<-done
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if auth["access_token"] != "access-secret" {
				t.Errorf("auth = %v", auth)
			}
			wantCalls := 1
			if noBrowser {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Errorf("browser calls = %d, want %d", calls, wantCalls)
			}
		})
	}
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name  string
		opts  Options
		valid bool
	}{
		{"defaults", Options{ServerURL: "https://example.com/mcp"}, true},
		{"numeric local HTTP", Options{ServerURL: "http://127.0.0.1:1234/mcp", AllowHTTP: true}, true},
		{"IPv6 local HTTP", Options{ServerURL: "http://[::1]:1234/mcp", AllowHTTP: true}, true},
		{"empty", Options{}, false},
		{"HTTP requires opt-in", Options{ServerURL: "http://127.0.0.1/mcp"}, false},
		{"nonlocal HTTP", Options{ServerURL: "http://example.com/mcp", AllowHTTP: true}, false},
		{"localhost HTTP", Options{ServerURL: "http://localhost/mcp", AllowHTTP: true}, false},
		{"URL userinfo", Options{ServerURL: "https://user:secret@example.com/mcp"}, false},
		{"fragment", Options{ServerURL: "https://example.com/mcp#fragment"}, false},
		{"invalid port", Options{ServerURL: "https://example.com:bad/mcp"}, false},
		{"unsafe issuer", Options{ServerURL: "https://example.com/mcp", Issuer: "http://example.com"}, false},
		{"wildcard callback", Options{ServerURL: "https://example.com/mcp", CallbackAddress: "0.0.0.0:0"}, false},
		{"hostname callback", Options{ServerURL: "https://example.com/mcp", CallbackAddress: "localhost:0"}, false},
		{"callback port range", Options{ServerURL: "https://example.com/mcp", CallbackAddress: "127.0.0.1:65536"}, false},
		{"negative timeout", Options{ServerURL: "https://example.com/mcp", Timeout: -time.Second}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateOptions(tt.opts); (err == nil) != tt.valid {
				t.Errorf("ValidateOptions error = %v, valid = %t", err, tt.valid)
			}
		})
	}
}
