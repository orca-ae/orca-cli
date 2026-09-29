// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPolicyAndPricingCommandsGateAndCallExpectedEndpoint(t *testing.T) {
	testCases := []struct {
		name       string
		group      string
		newCommand func(*Options) *cobra.Command
		args       []string
		method     string
		requestURI string
		body       map[string]interface{}
	}{
		{
			name:       "create guardrail",
			group:      policyExtensionGroup,
			newCommand: NewCmdGuardrails,
			args: []string{
				"create",
				"--config-json",
				`{"name":"protect production","rule":{"kind":"expression","expression":"true","on_false":"deny"}}`,
			},
			method:     http.MethodPost,
			requestURI: guardrailsPath,
			body: map[string]interface{}{
				"name": "protect production",
				"rule": map[string]interface{}{
					"kind": "expression", "expression": "true", "on_false": "deny",
				},
			},
		},
		{
			name:       "list guardrails",
			group:      policyExtensionGroup,
			newCommand: NewCmdGuardrails,
			args:       []string{"list", "--limit", "25", "--page", "next", "--include-archived"},
			method:     http.MethodGet,
			requestURI: guardrailsPath + "?include_archived=true&limit=25&page=next",
		},
		{
			name:       "get guardrail",
			group:      policyExtensionGroup,
			newCommand: NewCmdGuardrails,
			args:       []string{"get", "guardrail/one"},
			method:     http.MethodGet,
			requestURI: guardrailsPath + "/guardrail%2Fone",
		},
		{
			name:       "update guardrail",
			group:      policyExtensionGroup,
			newCommand: NewCmdGuardrails,
			args:       []string{"update", "guardrail-1", "--config-json", `{"description":null,"metadata":{"owner":null}}`},
			method:     http.MethodPost,
			requestURI: guardrailsPath + "/guardrail-1",
			body: map[string]interface{}{
				"description": nil,
				"metadata":    map[string]interface{}{"owner": nil},
			},
		},
		{
			name:       "archive guardrail",
			group:      policyExtensionGroup,
			newCommand: NewCmdGuardrails,
			args:       []string{"archive", "guardrail-1"},
			method:     http.MethodPost,
			requestURI: guardrailsPath + "/guardrail-1/archive",
		},
		{
			name:       "delete guardrail",
			group:      policyExtensionGroup,
			newCommand: NewCmdGuardrails,
			args:       []string{"delete", "guardrail-1"},
			method:     http.MethodDelete,
			requestURI: guardrailsPath + "/guardrail-1",
		},
		{
			name:       "list guardrail types",
			group:      policyExtensionGroup,
			newCommand: NewCmdGuardrails,
			args:       []string{"list-types"},
			method:     http.MethodGet,
			requestURI: guardrailTypesPath,
		},
		{
			name:       "list model prices",
			group:      pricingExtensionGroup,
			newCommand: NewCmdModelPrices,
			args:       []string{"list", "--limit", "10", "--page", "next"},
			method:     http.MethodGet,
			requestURI: modelPricesPath + "?limit=10&page=next",
		},
		{
			name:       "get model price",
			group:      pricingExtensionGroup,
			newCommand: NewCmdModelPrices,
			args:       []string{"get", "model/alpha", "--provider", "provider-a"},
			method:     http.MethodGet,
			requestURI: modelPricesPath + "/model%2Falpha?provider=provider-a",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
					t.Errorf("Authorization = %q, want Bearer test-token", got)
				}
				requests = append(requests, r.Method+" "+r.URL.RequestURI())
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/apis" {
					_, _ = fmt.Fprintf(w, `{"kind":"APIGroupList","groups":[{"name":%q}]}`, testCase.group)
					return
				}
				if got := r.Method; got != testCase.method {
					t.Errorf("method = %q, want %q", got, testCase.method)
				}
				if got := r.URL.RequestURI(); got != testCase.requestURI {
					t.Errorf("request URI = %q, want %q", got, testCase.requestURI)
				}
				if testCase.body != nil {
					var body map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode request body: %v", err)
					} else if !reflect.DeepEqual(body, testCase.body) {
						t.Errorf("body = %#v, want %#v", body, testCase.body)
					}
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			cmd := testCase.newCommand(&Options{
				IOStreams:   IOStreams{Out: io.Discard, ErrOut: io.Discard},
				RegistryURL: server.URL,
				AccessToken: "test-token",
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(testCase.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			wantRequests := []string{"GET /apis", testCase.method + " " + testCase.requestURI}
			if !reflect.DeepEqual(requests, wantRequests) {
				t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
			}
		})
	}
}

func TestPolicyAndPricingCommandsRejectUnavailableExtension(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		group      string
		newCommand func(*Options) *cobra.Command
		args       []string
	}{
		{name: "policy", group: policyExtensionGroup, newCommand: NewCmdGuardrails, args: []string{"list"}},
		{name: "pricing", group: pricingExtensionGroup, newCommand: NewCmdModelPrices, args: []string{"list"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
			}))
			defer server.Close()

			cmd := testCase.newCommand(&Options{
				IOStreams:   IOStreams{Out: io.Discard, ErrOut: io.Discard},
				RegistryURL: server.URL,
				AccessToken: "test-token",
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(testCase.args)

			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), `the "`+testCase.group+`" extension group is not available`) {
				t.Fatalf("Execute() error = %v, want unavailable %s error", err, testCase.group)
			}
			if requests != 1 {
				t.Fatalf("requests = %d, want only the discovery request", requests)
			}
		})
	}
}

func TestGuardrailPayloadReadsJSONFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guardrail.json")
	content := []byte(`{"name":"from file","rule":{"kind":"builtin","builtin":"block_tools"}}`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	payload, err := buildExtensionPayload(extensionPayloadOptions{configFile: path})
	if err != nil {
		t.Fatalf("buildExtensionPayload() error = %v", err)
	}
	if got := payload["name"]; got != "from file" {
		t.Fatalf("payload name = %#v, want from file", got)
	}
}

func TestGuardrailAndModelPriceJSONOutputPreservesServerFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/apis" {
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"pricing.runorca.ai"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"type":"model_price","model_id":"model-a","future_field":"preserved"}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	cmd := NewCmdModelPrices(&Options{
		IOStreams:   IOStreams{Out: &stdout, ErrOut: io.Discard},
		RegistryURL: server.URL,
		AccessToken: "test-token",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"get", "model-a", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"future_field": "preserved"`) {
		t.Fatalf("stdout did not preserve future field: %s", stdout.String())
	}
}
