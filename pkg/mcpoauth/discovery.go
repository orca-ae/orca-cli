// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package mcpoauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func origin(u *url.URL) string { return u.Scheme + "://" + u.Host }
func unique(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		if s != "" && !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}
func scopes(in []string) []string {
	var out []string
	for _, s := range in {
		out = append(out, strings.Fields(s)...)
	}
	return unique(out)
}
func stringList(m map[string]any, key string) ([]string, error) {
	v, exists := m[key]
	if !exists {
		return nil, nil
	}
	a, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("OAuth metadata %s must be an array of strings", key)
	}
	out := make([]string, 0, len(a))
	for _, v := range a {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("OAuth metadata %s must be an array of strings", key)
		}
		out = append(out, s)
	}
	return out, nil
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

// Split HTTP auth challenges at commas outside quoted strings. Track the
// current scheme so parameters from Basic/Digest challenges cannot affect MCP.
// Select the first Bearer challenge as a unit: never combine its resource with
// scopes from a different realm or protected resource.
func bearerChallenge(h http.Header) (string, []string) {
	var metadata string
	var found []string
	for _, header := range h.Values("WWW-Authenticate") {
		var parts []string
		start := 0
		quoted, escaped := false, false
		for i, c := range header {
			if escaped {
				escaped = false
				continue
			}
			if quoted && c == 92 {
				escaped = true
				continue
			}
			if c == 34 {
				quoted = !quoted
			}
			if c == 44 && !quoted {
				parts = append(parts, header[start:i])
				start = i + 1
			}
		}
		parts = append(parts, header[start:])
		bearer := false
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			// A scheme is a token followed by whitespace and not an equals sign.
			i := strings.IndexAny(part, " \t")
			if i >= 0 && !strings.Contains(part[:i], "=") && !strings.HasPrefix(strings.TrimSpace(part[i:]), "=") {
				if bearer {
					return metadata, unique(found)
				}
				bearer = strings.EqualFold(part[:i], "Bearer")
				part = strings.TrimSpace(part[i:])
			} else if !strings.Contains(part, "=") {
				if bearer {
					return metadata, unique(found)
				}
				bearer = strings.EqualFold(part, "Bearer")
				continue
			}
			if !bearer {
				continue
			}
			key, value, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if strings.HasPrefix(value, "\"") {
				// HTTP quoted-pair permits escaped punctuation, not only Go escapes.
				if len(value) < 2 || value[len(value)-1] != 34 {
					continue
				}
				var b strings.Builder
				raw := value[1 : len(value)-1]
				for j := 0; j < len(raw); j++ {
					if raw[j] == 92 && j+1 < len(raw) {
						j++
					}
					b.WriteByte(raw[j])
				}
				value = b.String()
			}
			if strings.EqualFold(key, "resource_metadata") && metadata == "" {
				metadata = value
			}
			if strings.EqualFold(key, "scope") {
				found = append(found, strings.Fields(value)...)
			}
		}
		if bearer {
			return metadata, unique(found)
		}
	}
	return metadata, unique(found)
}

func resourceIdentity(server, resource string) error {
	s, _ := url.Parse(server)
	r, _ := url.Parse(resource)
	port := func(u *url.URL) string {
		if u.Port() != "" {
			return u.Port()
		}
		if u.Scheme == "https" {
			return "443"
		}
		return "80"
	}
	if s.Scheme != r.Scheme || !strings.EqualFold(s.Hostname(), r.Hostname()) || port(s) != port(r) {
		return errors.New("protected-resource metadata points to a different origin")
	}
	// Clean decoded paths before comparing to reject dot-segment aliases that
	// claim a parent resource but actually resolve outside its subtree.
	sp := path.Clean("/" + strings.TrimPrefix(s.Path, "/"))
	rp := path.Clean("/" + strings.TrimPrefix(r.Path, "/"))
	if r.RawQuery != "" || r.ForceQuery || !(rp == "/" || sp == rp || strings.HasPrefix(sp, rp+"/")) {
		return errors.New("protected-resource metadata does not identify the MCP endpoint")
	}
	return nil
}

// Only 404/405 mean an optional discovery document is absent. Transport errors,
// redirects, malformed documents and issuer mismatches are never downgraded.
func (f *flow) metadata(ctx context.Context, candidates []string) (map[string]any, error) {
	for _, candidate := range unique(candidates) {
		status, _, b, e := f.request(ctx, candidate, "GET", "", "", false)
		if e != nil {
			return nil, e
		}
		if status == 404 || status == 405 {
			continue
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("OAuth discovery failed with HTTP %d", status)
		}
		return object(b)
	}
	return nil, nil
}

func (f *flow) discover(ctx context.Context) (config, error) {
	var c config
	const probe = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"orca-cli-oauth","version":"1"}}}`
	_, h, _, e := f.request(ctx, f.opts.ServerURL, "POST", "application/json", probe, true)
	if e != nil {
		return c, e
	}
	advertised, challengeScopes := bearerChallenge(h)
	server, _ := url.Parse(f.opts.ServerURL)
	candidates := []string{}
	if advertised != "" {
		a, e := url.Parse(advertised)
		if e != nil {
			return c, errors.New("invalid protected-resource metadata URL")
		}
		candidates = append(candidates, server.ResolveReference(a).String())
	}
	serverPath := strings.TrimRight(server.EscapedPath(), "/")
	if serverPath != "" {
		candidates = append(candidates, origin(server)+"/.well-known/oauth-protected-resource"+serverPath)
	}
	candidates = append(candidates, origin(server)+"/.well-known/oauth-protected-resource")
	prm, e := f.metadata(ctx, candidates)
	if e != nil {
		return c, e
	}
	c.resource = f.opts.ServerURL
	if prm != nil {
		resource, ok := prm["resource"].(string)
		if !ok || resource == "" {
			return c, errors.New("protected-resource metadata is missing resource")
		}
		c.resource = resource
	}
	if _, e = checkedURL(c.resource, f.opts.AllowHTTP); e != nil {
		return c, e
	}
	if e = resourceIdentity(f.opts.ServerURL, c.resource); e != nil {
		return c, e
	}
	servers, e := stringList(prm, "authorization_servers")
	if e != nil {
		return c, e
	}
	servers = unique(servers)
	for _, issuer := range servers {
		if e = checkIssuer(issuer, f.opts.AllowHTTP); e != nil {
			return c, e
		}
	}
	// A sole advertised server also identifies the discovery location. A
	// trusted explicit issuer can pin its identity when an OAuth proxy hosts
	// metadata at a different URL. Never accept a mismatch automatically.
	discoveryIssuer := origin(server)
	if len(servers) == 1 {
		discoveryIssuer = servers[0]
	}
	if len(servers) > 1 {
		if f.opts.Issuer == "" {
			return c, errors.New("multiple authorization servers; specify an issuer")
		}
		if !contains(servers, f.opts.Issuer) {
			return c, errors.New("issuer must select a protected-resource authorization server")
		}
		discoveryIssuer = f.opts.Issuer
	}
	c.issuer = discoveryIssuer
	if f.opts.Issuer != "" {
		c.issuer = f.opts.Issuer
		if len(servers) == 0 {
			discoveryIssuer = f.opts.Issuer
		}
	}
	issuer, _ := url.Parse(discoveryIssuer)
	proxyOrigin := ""
	if len(servers) == 1 && f.opts.Issuer != "" && f.opts.Issuer != discoveryIssuer {
		proxyOrigin = origin(issuer)
	}
	issuerPath := strings.TrimRight(issuer.EscapedPath(), "/")
	candidates = []string{origin(issuer) + "/.well-known/oauth-authorization-server" + issuerPath, origin(issuer) + "/.well-known/openid-configuration" + issuerPath}
	if issuerPath != "" {
		candidates = append(candidates, strings.TrimRight(discoveryIssuer, "/")+"/.well-known/oauth-authorization-server", strings.TrimRight(discoveryIssuer, "/")+"/.well-known/openid-configuration")
	}
	m, e := f.metadata(ctx, candidates)
	if e != nil {
		return c, e
	}
	if m == nil {
		return c, errors.New("could not discover authorization-server metadata")
	}
	if actual, ok := m["issuer"].(string); !ok || actual != c.issuer {
		if f.opts.Issuer == "" && len(servers) == 1 {
			return c, errors.New("authorization-server metadata issuer mismatch; use --oauth-issuer to pin a trusted proxy issuer")
		}
		return c, errors.New("authorization-server metadata issuer mismatch")
	}
	methods, e := stringList(m, "code_challenge_methods_supported")
	if e != nil {
		return c, e
	}
	if !contains(methods, "S256") {
		return c, errors.New("authorization server does not advertise PKCE S256")
	}
	methods, e = stringList(m, "token_endpoint_auth_methods_supported")
	if e != nil {
		return c, e
	}
	// RFC 8414 defaults omitted methods to client_secret_basic.
	if _, exists := m["token_endpoint_auth_methods_supported"]; !exists {
		methods = []string{"client_secret_basic"}
	}
	if f.opts.ClientSecret != "" {
		if !contains(methods, "client_secret_basic") {
			return c, errors.New("authorization server does not support client_secret_basic")
		}
		c.tokenAuthMethod = "client_secret_basic"
	} else if contains(methods, "none") {
		c.tokenAuthMethod = "none"
	} else if contains(methods, "client_secret_basic") {
		if f.opts.ClientID != "" {
			return c, errors.New("pre-registered OAuth client requires a client secret")
		}
		c.tokenAuthMethod = "client_secret_basic"
	} else {
		return c, errors.New("authorization server advertises no supported OAuth client authentication")
	}
	endpoints := []struct {
		key string
		dst *string
	}{{"authorization_endpoint", &c.authorization}, {"token_endpoint", &c.token}}
	if f.opts.ClientID == "" {
		endpoints = append(endpoints, struct {
			key string
			dst *string
		}{"registration_endpoint", &c.registration})
	}
	for _, endpoint := range endpoints {
		s, ok := m[endpoint.key].(string)
		if !ok || s == "" {
			return c, fmt.Errorf("authorization-server metadata is missing %s", endpoint.key)
		}
		u, e := checkedURL(s, f.opts.AllowHTTP)
		if e != nil {
			return c, e
		}
		if proxyOrigin != "" && origin(u) != proxyOrigin {
			return c, errors.New("pinned OAuth proxy endpoint points to a different origin")
		}
		// Server-supplied query parameters must not smuggle credentials into
		// progress output or override security-critical authorization parameters.
		q, e := url.ParseQuery(u.RawQuery)
		if e != nil {
			return c, errors.New("invalid OAuth endpoint query")
		}
		if endpoint.key == "authorization_endpoint" {
			for _, key := range []string{"code", "access_token", "refresh_token", "id_token", "client_secret", "state", "code_verifier", "code_challenge", "code_challenge_method", "client_id", "redirect_uri", "resource", "response_type", "scope", "access_type"} {
				if _, exists := q[key]; exists {
					return c, errors.New("authorization endpoint contains reserved OAuth parameters")
				}
			}
		}
		*endpoint.dst = s
	}
	if v, exists := m["authorization_response_iss_parameter_supported"]; exists {
		b, ok := v.(bool)
		if !ok {
			return c, errors.New("invalid authorization response issuer metadata")
		}
		c.requireIssuer = b
	}
	c.scopes = scopes(f.opts.Scopes)
	if len(c.scopes) == 0 {
		c.scopes = challengeScopes
	}
	prmScopes, e := stringList(prm, "scopes_supported")
	if e != nil {
		return c, e
	}
	if len(c.scopes) == 0 {
		c.scopes = scopes(prmScopes)
	}
	serverScopes, e := stringList(m, "scopes_supported")
	if e != nil {
		return c, e
	}
	if !f.opts.NoRefresh && contains(serverScopes, "offline_access") {
		c.scopes = append(c.scopes, "offline_access")
	}
	c.scopes = unique(c.scopes)
	return c, nil
}
