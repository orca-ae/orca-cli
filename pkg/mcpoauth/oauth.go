// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

// Package mcpoauth obtains MCP OAuth credentials without persisting local secrets.
package mcpoauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Options configures an authorization-code flow with public clients or dynamically
// registered Basic clients. Zero Timeout uses five minutes; empty CallbackAddress
// uses an ephemeral 127.0.0.1 port. ClientID skips dynamic registration for a public
// client. Issuer pins the expected identity while a sole advertised OAuth proxy
// remains the metadata discovery location. NoRefresh only suppresses refresh
// requests; a refresh token returned by the server is still included in the result.
type Options struct {
	ServerURL, Issuer, ClientID, CallbackAddress string
	Scopes                                       []string
	Timeout                                      time.Duration
	NoBrowser, NoRefresh, AllowHTTP              bool
}

func defaults(o Options) Options {
	if o.Timeout == 0 {
		o.Timeout = 5 * time.Minute
	}
	if o.CallbackAddress == "" {
		o.CallbackAddress = "127.0.0.1:0"
	}
	return o
}

// ValidateOptions validates configuration without network access. HTTP is only
// accepted for numeric loopback test servers, never DNS names such as localhost.
func ValidateOptions(o Options) error {
	o = defaults(o)
	if _, err := checkedURL(o.ServerURL, o.AllowHTTP); err != nil {
		return fmt.Errorf("MCP server: %w", err)
	}
	if o.Issuer != "" {
		if err := checkIssuer(o.Issuer, o.AllowHTTP); err != nil {
			return err
		}
	}
	if o.Timeout < 0 {
		return errors.New("OAuth timeout must be positive")
	}
	host, port, err := net.SplitHostPort(o.CallbackAddress)
	n, e := strconv.Atoi(port)
	if err != nil || e != nil || host != "127.0.0.1" || n < 0 || n > 65535 || port != strconv.Itoa(n) {
		return errors.New("callback address must be 127.0.0.1:<port> (0-65535)")
	}
	for _, s := range o.Scopes {
		for _, c := range s {
			if c < 32 && c != 9 && c != 10 && c != 13 || c == 127 {
				return errors.New("invalid OAuth scope")
			}
		}
	}
	return nil
}

func checkedURL(raw string, allowHTTP bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || strings.Contains(raw, "#") || u.Opaque != "" {
		return nil, errors.New("invalid OAuth URL (host required; userinfo and fragments forbidden)")
	}
	if u.Scheme != "https" {
		if u.Scheme != "http" || !allowHTTP {
			return nil, errors.New("OAuth URLs must use HTTPS")
		}
		if u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
			return nil, errors.New("HTTP is allowed only for numeric loopback test servers")
		}
	}
	if p := u.Port(); p != "" {
		n, e := strconv.Atoi(p)
		if e != nil || n < 1 || n > 65535 {
			return nil, errors.New("invalid OAuth URL port")
		}
	}
	return u, nil
}

func checkIssuer(s string, allow bool) error {
	u, e := checkedURL(s, allow)
	if e != nil {
		return fmt.Errorf("issuer: %w", e)
	}
	if u.RawQuery != "" || u.ForceQuery {
		return errors.New("issuer must not contain a query")
	}
	return nil
}

// Authorize returns the backend mcp_oauth auth object. The caller owns secure
// storage of this object. Progress includes a browser URL, but never tokens or
// the received authorization code. All network work shares the total timeout.
func Authorize(ctx context.Context, opts Options, progress io.Writer) (map[string]any, error) {
	return authorize(ctx, opts, progress, launchBrowser)
}

func launchBrowser(ctx context.Context, target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", target)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", target)
	}
	return cmd.Run()
}

type flow struct {
	opts   Options
	client *http.Client
}
type config struct {
	resource, issuer, authorization, token, registration string
	tokenAuthMethod                                      string
	scopes                                               []string
	requireIssuer                                        bool
}

func authorize(ctx context.Context, opts Options, progress io.Writer, openBrowser func(context.Context, string) error) (map[string]any, error) {
	if err := ValidateOptions(opts); err != nil {
		return nil, err
	}
	opts = defaults(opts)
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if progress == nil {
		progress = io.Discard
	}
	// Own the transport: process-global DefaultTransport overrides must not
	// disable TLS verification, introduce redirects, or panic via type assertion.
	transport := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            (&net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           10,
		IdleConnTimeout:        30 * time.Second,
		TLSHandshakeTimeout:    10 * time.Second,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: 64 * 1024,
	}
	defer transport.CloseIdleConnections()
	f := flow{opts, &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	fmt.Fprintln(progress, "Discovering OAuth metadata")
	cfg, err := f.discover(ctx)
	if err != nil {
		return nil, err
	}
	state, err := randomString()
	if err != nil {
		return nil, err
	}
	verifier, err := randomString()
	if err != nil {
		return nil, err
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", opts.CallbackAddress)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("could not bind OAuth loopback callback")
	}
	redirect := "http://" + ln.Addr().String() + "/oauth/callback"
	results := make(chan url.Values, 1)
	var once sync.Once
	srv := &http.Server{ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 5 * time.Second, MaxHeaderBytes: 16 * 1024, ErrorLog: log.New(io.Discard, "", 0)}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.URL.Path != "/oauth/callback" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Host != ln.Addr().String() || len(r.URL.RawQuery) > 16384 {
			http.Error(w, "Invalid callback", 400)
			return
		}
		q, e := url.ParseQuery(r.URL.RawQuery)
		valid := e == nil
		for _, key := range []string{"state", "code", "iss", "error", "error_description"} {
			if len(q[key]) > 1 {
				valid = false
			}
		}
		if !valid || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "Invalid OAuth state or callback", 400)
			return
		}
		accepted := false
		once.Do(func() { results <- q; accepted = true })
		if !accepted {
			http.Error(w, "Callback already received", 409)
			return
		}
		fmt.Fprintln(w, "OAuth authorization received. Return to the terminal.")
	})
	served := make(chan struct{})
	go func() { defer close(served); _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close(); _ = ln.Close(); <-served }()
	client := oauthClient{id: opts.ClientID, method: cfg.tokenAuthMethod}
	if client.id == "" {
		fmt.Fprintln(progress, "Registering OAuth client")
		client, err = f.register(ctx, cfg, redirect)
		if err != nil {
			return nil, err
		}
	}
	digest := sha256.Sum256([]byte(verifier))
	u, _ := url.Parse(cfg.authorization)
	q := u.Query()
	for k, v := range map[string]string{"response_type": "code", "client_id": client.id, "redirect_uri": redirect, "state": state, "code_challenge": base64.RawURLEncoding.EncodeToString(digest[:]), "code_challenge_method": "S256", "resource": cfg.resource} {
		q.Set(k, v)
	}
	q.Del("scope")
	q.Del("access_type")
	if len(cfg.scopes) > 0 {
		q.Set("scope", strings.Join(cfg.scopes, " "))
	}
	u.RawQuery = q.Encode()
	fmt.Fprintf(progress, "Open this URL to authorize:\n%s\n", u.String())
	if !opts.NoBrowser {
		// A broken desktop helper must not consume the whole authorization window.
		browserCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		err = openBrowser(browserCtx, u.String())
		stop()
		if err != nil {
			fmt.Fprintln(progress, "Browser was not opened automatically; use the URL above.")
		}
	}
	var callback url.Values
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case callback = <-results:
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if callback.Get("error") != "" {
		return nil, errors.New("OAuth authorization denied by server")
	}
	iss := callback.Get("iss")
	if callback.Has("iss") && iss != cfg.issuer {
		return nil, errors.New("OAuth callback issuer mismatch")
	}
	if cfg.requireIssuer && iss == "" {
		return nil, errors.New("OAuth callback is missing required issuer parameter")
	}
	code := callback.Get("code")
	if code == "" {
		return nil, errors.New("OAuth callback is missing authorization code")
	}
	fmt.Fprintln(progress, "Exchanging authorization code for tokens")
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirect}, "code_verifier": {verifier}, "resource": {cfg.resource}}
	tokens, err := f.exchange(ctx, cfg.token, form, client)
	if err != nil {
		return nil, err
	}
	access, ok := tokens["access_token"].(string)
	if !ok || access == "" {
		return nil, errors.New("token response is missing access_token")
	}
	tokenType, ok := tokens["token_type"].(string)
	if !ok || !strings.EqualFold(tokenType, "Bearer") {
		return nil, errors.New("token response must identify a Bearer token_type")
	}
	auth := map[string]any{"type": "mcp_oauth", "mcp_server_url": opts.ServerURL, "access_token": access}
	if value, exists := tokens["expires_in"]; exists {
		expires, err := tokenExpiry(value, time.Now())
		if err != nil {
			return nil, err
		}
		auth["expires_at"] = expires
	}
	refreshScope := strings.Join(cfg.scopes, " ")
	if value, exists := tokens["scope"]; exists {
		granted, ok := value.(string)
		if !ok || !validGrantedScope(granted) {
			return nil, errors.New("token response contains invalid scope")
		}
		// An explicitly empty granted scope must not restore requested scopes.
		refreshScope = granted
	}
	if refresh, ok := tokens["refresh_token"].(string); ok && refresh != "" {
		settings := map[string]any{"refresh_token": refresh, "token_endpoint": cfg.token, "client_id": client.id, "token_endpoint_auth": client.refreshAuth(), "resource": cfg.resource}
		if refreshScope != "" {
			settings["scope"] = refreshScope
		}
		auth["refresh"] = settings
	} else {
		fmt.Fprintln(progress, "Warning: no refresh token returned; reauthorization will be required.")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return auth, nil
}

// RFC 6749 expires_in is a nonnegative lifetime in whole seconds. Bound before
// converting to nanoseconds; zero is a valid immediately expired credential.
func tokenExpiry(value any, now time.Time) (string, error) {
	seconds, ok := value.(float64) // JSON numbers decoded by object.
	const maxSeconds = int64(math.MaxInt64) / int64(time.Second)
	if !ok || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 || math.Trunc(seconds) != seconds || seconds > float64(maxSeconds) {
		return "", errors.New("token response contains invalid expires_in")
	}
	return now.UTC().Add(time.Duration(int64(seconds)) * time.Second).Format(time.RFC3339Nano), nil
}

// scope-token is printable ASCII excluding quote and backslash (RFC 6749).
// Empty scope is accepted and omitted from refresh settings.
func validGrantedScope(scope string) bool {
	if scope == "" {
		return true
	}
	for _, token := range strings.Split(scope, " ") {
		if token == "" {
			return false
		}
		for _, c := range token {
			if c < 0x21 || c > 0x7e || c == 0x22 || c == 0x5c {
				return false
			}
		}
	}
	return true
}

func randomString() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", errors.New("could not generate OAuth randomness")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

const maxResponseBytes = 1024 * 1024

func (f *flow) request(ctx context.Context, target, method, contentType, body string, headersOnly bool) (int, http.Header, []byte, error) {
	return f.requestAuthenticated(ctx, target, method, contentType, body, headersOnly, "")
}

func (f *flow) requestAuthenticated(ctx context.Context, target, method, contentType, body string, headersOnly bool, authorization string) (int, http.Header, []byte, error) {
	if _, e := checkedURL(target, f.opts.AllowHTTP); e != nil {
		return 0, nil, nil, e
	}
	req, e := http.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	if e != nil {
		return 0, nil, nil, errors.New("invalid OAuth request")
	}
	req.Header.Set("Accept", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if headersOnly {
		req.Header.Set("Accept", "application/json, text/event-stream")
	}
	res, e := f.client.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return 0, nil, nil, ctx.Err()
		}
		return 0, nil, nil, errors.New("OAuth HTTP request failed")
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		return 0, nil, nil, errors.New("OAuth protocol redirects are not allowed")
	}
	// Successful MCP probes can be unbounded SSE streams. Only the challenge
	// headers are needed; close immediately rather than wait for an event stream.
	if headersOnly {
		return res.StatusCode, res.Header, nil, nil
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if e != nil {
		if ctx.Err() != nil {
			return 0, nil, nil, ctx.Err()
		}
		return 0, nil, nil, errors.New("could not read OAuth response")
	}
	if len(data) > maxResponseBytes {
		return 0, nil, nil, errors.New("OAuth response exceeds 1 MiB")
	}
	return res.StatusCode, res.Header, data, nil
}

func object(data []byte) (map[string]any, error) {
	var m map[string]any
	if e := json.Unmarshal(data, &m); e != nil || m == nil {
		return nil, errors.New("OAuth response must be a JSON object")
	}
	return m, nil
}
func (f *flow) post(ctx context.Context, target, typ, body, stage string) (map[string]any, error) {
	status, _, data, e := f.request(ctx, target, "POST", typ, body, false)
	if e != nil {
		return nil, fmt.Errorf("%s: %w", stage, e)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%s failed with HTTP %d", stage, status)
	}
	return object(data)
}

// oauthClient keeps token endpoint credentials out of URLs and progress output.
type oauthClient struct {
	id, secret, method string
}

func (c oauthClient) refreshAuth() map[string]any {
	auth := map[string]any{"type": c.method}
	if c.method == "client_secret_basic" {
		auth["client_secret"] = c.secret
	}
	return auth
}

func (f *flow) exchange(ctx context.Context, target string, form url.Values, client oauthClient) (map[string]any, error) {
	authorization := ""
	if client.method == "client_secret_basic" {
		// RFC 6749 section 2.3.1 encodes each value before constructing Basic.
		pair := url.QueryEscape(client.id) + ":" + url.QueryEscape(client.secret)
		authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(pair))
	} else {
		form.Set("client_id", client.id)
	}
	status, _, data, err := f.requestAuthenticated(ctx, target, "POST", "application/x-www-form-urlencoded", form.Encode(), false, authorization)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("token exchange failed with HTTP %d", status)
	}
	return object(data)
}

func (f *flow) register(ctx context.Context, c config, redirect string) (oauthClient, error) {
	var client oauthClient
	grants := []string{"authorization_code"}
	if !f.opts.NoRefresh {
		grants = append(grants, "refresh_token")
	}
	payload := map[string]any{"client_name": "orca-cli", "redirect_uris": []string{redirect}, "grant_types": grants, "response_types": []string{"code"}, "token_endpoint_auth_method": c.tokenAuthMethod}
	if c.tokenAuthMethod == "none" {
		payload["application_type"] = "native"
	}
	if len(c.scopes) > 0 {
		payload["scope"] = strings.Join(c.scopes, " ")
	}
	b, _ := json.Marshal(payload)
	m, err := f.post(ctx, c.registration, "application/json", string(b), "dynamic client registration")
	if err != nil {
		return client, err
	}
	id, ok := m["client_id"].(string)
	if !ok || id == "" {
		return client, errors.New("dynamic client registration response is missing client_id")
	}
	// RFC 7591 defaults an omitted method to client_secret_basic, not none.
	method := "client_secret_basic"
	if value, exists := m["token_endpoint_auth_method"]; exists {
		var ok bool
		method, ok = value.(string)
		if !ok {
			return client, errors.New("invalid dynamic client registration authentication method")
		}
	}
	if method != c.tokenAuthMethod {
		return client, errors.New("dynamic client registration returned an unexpected authentication method")
	}
	client = oauthClient{id: id, method: method}
	if method == "client_secret_basic" {
		client.secret, ok = m["client_secret"].(string)
		if !ok || client.secret == "" {
			return oauthClient{}, errors.New("dynamic client registration response is missing client_secret")
		}
	}
	return client, nil
}
