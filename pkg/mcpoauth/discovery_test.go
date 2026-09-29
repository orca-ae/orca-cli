// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package mcpoauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBearerChallenge(t *testing.T) {
	tests := []struct {
		name     string
		headers  []string
		metadata string
		scopes   []string
	}{
		{"quoted", []string{`Bearer resource_metadata="/metadata?x=a,b", scope="read write read"`}, "/metadata?x=a,b", []string{"read", "write"}},
		{"combined", []string{`Basic realm="ignore", Bearer scope = "read", resource_metadata = "/prm", Digest scope="wrong"`}, "/prm", []string{"read"}},
		{"multiple", []string{`Basic resource_metadata="https://evil.example", scope="wrong"`, `bEaReR resource_metadata=/prm, scope=read`, `Bearer scope="write"`}, "/prm", []string{"read"}},
		{"separate realms", []string{`Bearer realm="first", resource_metadata="/first", scope="read", Bearer realm="second", resource_metadata="/second", scope="admin"`}, "/first", []string{"read"}},
		{"escape", []string{`Bearer resource_metadata="\/prm", scope="read"`}, "/prm", []string{"read"}},
		{"none", []string{`Basic realm="private"`}, "", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			for _, s := range tt.headers {
				h.Add("WWW-Authenticate", s)
			}
			m, s := bearerChallenge(h)
			if m != tt.metadata || !reflect.DeepEqual(s, tt.scopes) {
				t.Fatalf("got %q %v", m, s)
			}
		})
	}
}

func TestResourceIdentity(t *testing.T) {
	for _, tt := range []struct {
		resource string
		valid    bool
	}{
		{"https://EXAMPLE.com:443", true}, {"https://example.com/a", true}, {"https://example.com/a/mcp/", true},
		{"https://example.com/ab", false}, {"https://example.com/a/mcp/child", false}, {"https://other.example/a", false},
		{"https://example.com:444/a", false}, {"http://example.com/a", false}, {"https://example.com/a?x=1", false},
		{"https://example.com/a/mcp/../../elsewhere", false}, {"https://example.com/a/mcp/%2e%2e/%2e%2e/elsewhere", false},
	} {
		t.Run(tt.resource, func(t *testing.T) {
			err := resourceIdentity("https://example.com/a/mcp", tt.resource)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}

func TestProtocolTransport(t *testing.T) {
	for _, tt := range []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"redirect", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "https://example.com/?code=secret-code")
			w.WriteHeader(302)
		}, "redirect"},
		{"oversized", func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, strings.Repeat("x", maxResponseBytes+1))
		}, "1 MiB"},
		{"error-redaction", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			_, _ = io.WriteString(w, `{"error":"secret-code","error_description":"secret-token"}`)
		}, "HTTP 400"},
		{"malformed", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "secret-token") }, "JSON object"},
		{"array", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "[]") }, "JSON object"},
		{"null", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "null") }, "JSON object"},
		{"trailing", func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{} {"access_token":"secret-token"}`)
		}, "JSON object"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()
			f := flow{opts: Options{AllowHTTP: true}, client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
			_, err := f.post(context.Background(), server.URL, "application/json", "{}", "test")
			if err == nil || !strings.Contains(err.Error(), tt.want) || strings.Contains(err.Error(), "secret-") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDiscoveryRequestDeadline(t *testing.T) {
	stop := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-stop:
		}
	}))
	defer server.Close()
	defer close(stop)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := Authorize(ctx, Options{ServerURL: server.URL, AllowHTTP: true, NoBrowser: true}, io.Discard)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
}

func TestProbeDoesNotWaitForSSEBody(t *testing.T) {
	stop := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-stop:
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	defer close(stop)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := Authorize(ctx, Options{ServerURL: server.URL, AllowHTTP: true, NoBrowser: true}, io.Discard)
	if err == nil || errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "could not discover") {
		t.Fatalf("got %v", err)
	}
}

func TestHTTPSCertificateVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("untrusted TLS should not reach handler") }))
	defer server.Close()
	_, err := Authorize(context.Background(), Options{ServerURL: server.URL, NoBrowser: true}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("got %v", err)
	}
}
