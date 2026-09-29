// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package commands_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/orca-ae/orca-cli/pkg/cmd/root"
	"github.com/orca-ae/orca-cli/pkg/cmd/workspace"
	"github.com/spf13/cobra"
)

const unsupportedKafkaPatch = "Kafka Connect config PATCH is declared by OpenAPI but not implemented"

type commandCase struct {
	path    string
	args    []string
	wantErr string
}

func ok(path string, args ...string) commandCase {
	return commandCase{path: path, args: args}
}

func fails(path, wantErr string, args ...string) commandCase {
	return commandCase{path: path, args: args, wantErr: wantErr}
}

func TestEveryLeafCommandE2E(t *testing.T) {
	cases := allLeafCommandCases()
	assertExactLeafCoverage(t, cases)

	for _, tc := range cases {
		t.Run(strings.TrimPrefix(tc.path, "ork "), func(t *testing.T) {
			fixture := newCommandFixture(t)
			server := httptest.NewServer(fixture)
			defer server.Close()

			args := []string{"--registry-url", server.URL, "--access-token", "e2e-token"}
			args = append(args, strings.Fields(strings.TrimPrefix(tc.path, "ork "))...)
			args = append(args, expandFixtureArgs(tc.args, fixture)...)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd := root.NewCommand(workspace.IOStreams{
				In:     strings.NewReader("y\n"),
				Out:    &stdout,
				ErrOut: &stderr,
			})
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(args)
			err := cmd.Execute()
			if tc.wantErr == "" && err != nil {
				t.Fatalf("Execute() error = %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("Execute() error = %v, want error containing %q", err, tc.wantErr)
			}
			if fixture.requestCount() == 0 {
				t.Fatal("command made no HTTP request")
			}
			if authErr := fixture.authenticationError(); authErr != "" {
				t.Fatal(authErr)
			}
		})
	}
}

func allLeafCommandCases() []commandCase {
	return []commandCase{
		ok("ork agent archive", "agent-1"),
		ok("ork agent create", "--name", "agent", "--model-json", `{"provider":"anthropic","id":"model"}`),
		ok("ork agent environments archive", "environment-1"),
		ok("ork agent environments create", "--name", "environment"),
		ok("ork agent environments delete", "environment-1"),
		ok("ork agent environments get", "environment-1"),
		ok("ork agent environments list"),
		ok("ork agent environments update", "environment-1", "--description", "updated"),
		ok("ork agent files content", "file-1", "--output-file", "{output}"),
		ok("ork agent files create", "--file", "{input}"),
		ok("ork agent files delete", "file-1"),
		ok("ork agent files get", "file-1"),
		ok("ork agent files list"),
		ok("ork agent get", "agent-1"),
		ok("ork agent list"),
		ok("ork agent memory-stores archive", "memory-store-1"),
		ok("ork agent memory-stores create", "--name", "memory-store"),
		ok("ork agent memory-stores delete", "memory-store-1"),
		ok("ork agent memory-stores get", "memory-store-1"),
		ok("ork agent memory-stores list"),
		ok("ork agent memory-stores memories create", "--memory-store", "memory-store-1", "--path", "/notes", "--content", "memory"),
		ok("ork agent memory-stores memories delete", "memory-1", "--memory-store", "memory-store-1"),
		ok("ork agent memory-stores memories get", "memory-1", "--memory-store", "memory-store-1"),
		ok("ork agent memory-stores memories list", "--memory-store", "memory-store-1"),
		ok("ork agent memory-stores memories update", "memory-1", "--memory-store", "memory-store-1", "--content", "updated"),
		ok("ork agent memory-stores update", "memory-store-1", "--description", "updated"),
		ok("ork agent memory-versions get", "version-1", "--memory-store", "memory-store-1"),
		ok("ork agent memory-versions list", "--memory-store", "memory-store-1"),
		ok("ork agent memory-versions redact", "version-1", "--memory-store", "memory-store-1"),
		ok("ork agent providers get", "managed-agents"),
		ok("ork agent providers list"),
		ok("ork agent sessions archive", "session-1"),
		ok("ork agent sessions create", "--environment-id", "environment-1", "--agent", "agent-1"),
		ok("ork agent sessions delete", "session-1"),
		ok("ork agent sessions events list", "--session", "session-1"),
		ok("ork agent sessions events send message", "--session", "session-1", "--text", "hello"),
		ok("ork agent sessions events send outcome", "--session", "session-1", "--description", "done", "--rubric", "correct"),
		ok("ork agent sessions events send tool-confirmation", "--session", "session-1", "--tool-use-id", "tool-1", "--decision", "allow"),
		ok("ork agent sessions events stream", "--session", "session-1", "--timeout", "1s"),
		ok("ork agent sessions files content", "file-1", "--session", "session-1", "--output-file", "{output}"),
		ok("ork agent sessions files delete", "file-1", "--session", "session-1"),
		ok("ork agent sessions files get", "file-1", "--session", "session-1"),
		ok("ork agent sessions files list", "--session", "session-1"),
		ok("ork agent sessions get", "session-1"),
		ok("ork agent sessions list"),
		ok("ork agent sessions outcome", "session-1"),
		ok("ork agent sessions resources add", "--session", "session-1", "--type", "file", "--file-id", "file-1"),
		ok("ork agent sessions resources delete", "resource-1", "--session", "session-1"),
		ok("ork agent sessions resources get", "resource-1", "--session", "session-1"),
		ok("ork agent sessions resources list", "--session", "session-1"),
		ok("ork agent sessions resources update", "resource-1", "--session", "session-1", "--authorization-token", "token"),
		ok("ork agent sessions threads archive", "thread-1", "--session", "session-1"),
		ok("ork agent sessions threads events list", "--session", "session-1", "--thread", "thread-1"),
		ok("ork agent sessions threads events stream", "--session", "session-1", "--thread", "thread-1", "--timeout", "1s"),
		ok("ork agent sessions threads get", "thread-1", "--session", "session-1"),
		ok("ork agent sessions threads list", "--session", "session-1"),
		ok("ork agent sessions update", "session-1", "--title", "updated"),
		ok("ork agent skills create", "--file", "{skill}"),
		ok("ork agent skills delete", "skill-1"),
		ok("ork agent skills get", "skill-1"),
		ok("ork agent skills list"),
		ok("ork agent skills versions content", "skill-1", "1", "--output-file", "{output}"),
		ok("ork agent skills versions create", "skill-1", "--file", "{skill}"),
		ok("ork agent skills versions delete", "skill-1", "1"),
		ok("ork agent skills versions get", "skill-1", "1"),
		ok("ork agent skills versions list", "skill-1"),
		ok("ork agent triggers create", "--name", "trigger", "--agent", "agent-1", "--environment-id", "environment-1", "--source-type", "cron", "--session-mode", "SESSION_PER_EVENT", "--schedule", "* * * * *", "--payload", `{}`),
		ok("ork agent triggers delete", "trigger-1"),
		ok("ork agent triggers get", "trigger-1"),
		ok("ork agent triggers list"),
		ok("ork agent triggers pause", "trigger-1"),
		ok("ork agent triggers sessions", "trigger-1"),
		ok("ork agent triggers unpause", "trigger-1"),
		ok("ork agent triggers update", "trigger-1", "--name", "updated"),
		ok("ork agent update", "agent-1", "--description", "updated"),
		ok("ork agent vaults archive", "vault-1"),
		ok("ork agent vaults create", "--display-name", "vault"),
		ok("ork agent vaults credentials archive", "credential-1", "--vault", "vault-1"),
		ok("ork agent vaults credentials create", "--vault", "vault-1", "--auth-json", `{"type":"static_bearer","mcp_server_url":"https://example.com/mcp","token":"token"}`),
		ok("ork agent vaults credentials delete", "credential-1", "--vault", "vault-1"),
		ok("ork agent vaults credentials get", "credential-1", "--vault", "vault-1"),
		ok("ork agent vaults credentials list", "--vault", "vault-1"),
		ok("ork agent vaults credentials update", "credential-1", "--vault", "vault-1", "--display-name", "updated"),
		ok("ork agent vaults credentials validate", "credential-1", "--vault", "vault-1"),
		ok("ork agent vaults delete", "vault-1"),
		ok("ork agent vaults get", "vault-1"),
		ok("ork agent vaults list"),
		ok("ork agent vaults update", "vault-1", "--display-name", "updated"),
		ok("ork agent versions", "agent-1"),
		ok("ork api-groups"),
		ok("ork api-resources"),
		ok("ork api-versions"),
		ok("ork connections create", "--name", "connection", "--type", "kafka", "--kafka-bootstrap-servers", "broker:9092"),
		ok("ork connections delete", "connection"),
		ok("ork connections get", "connection"),
		ok("ork connections list"),
		ok("ork connections test", "connection"),
		ok("ork connections update", "--name", "connection", "--kafka-bootstrap-servers", "broker-2:9092"),
		ok("ork connections validate", "--name", "connection", "--type", "kafka", "--kafka-bootstrap-servers", "broker:9092"),
		ok("ork functions create", "--name", "function", "--function-type", "exclamation", "--connection", "connection"),
		ok("ork functions delete", "function"),
		ok("ork functions get", "function"),
		ok("ork functions list"),
		ok("ork functions restart", "function"),
		ok("ork functions start", "function"),
		ok("ork functions state get", "function", "key"),
		ok("ork functions state put", "function", "key", "--state-json", `{"value":"value"}`),
		ok("ork functions stats", "function"),
		ok("ork functions status", "function"),
		ok("ork functions stop", "function"),
		ok("ork functions trigger", "function", "--data", "hello"),
		ok("ork functions update", "function", "--function-type", "exclamation"),
		ok("ork health live"),
		ok("ork health ready"),
		ok("ork healthz"),
		ok("ork guardrails archive", "guardrail-1"),
		ok("ork guardrails create", "--config-json", `{"name":"guardrail","rule":{"kind":"expression","expression":"true","on_false":"deny"}}`),
		ok("ork guardrails delete", "guardrail-1"),
		ok("ork guardrails get", "guardrail-1"),
		ok("ork guardrails list"),
		ok("ork guardrails list-types"),
		ok("ork guardrails update", "guardrail-1", "--config-json", `{"enabled":false}`),
		ok("ork kafka-connect apply", "--config-file", "{connector-config}", "--connection", "connection"),
		ok("ork kafka-connect available-connectors"),
		ok("ork kafka-connect config-definition", "connector-type"),
		ok("ork kafka-connect delete connector", "connector"),
		ok("ork kafka-connect delete offsets", "connector", "--force"),
		ok("ork kafka-connect describe connector", "connector"),
		ok("ork kafka-connect describe plugin", "org.example.Connector"),
		ok("ork kafka-connect get config", "connector"),
		ok("ork kafka-connect get connectors"),
		ok("ork kafka-connect get offsets", "connector"),
		ok("ork kafka-connect get plugin-catalog"),
		ok("ork kafka-connect get plugins"),
		ok("ork kafka-connect get status", "connector"),
		ok("ork kafka-connect get task-status", "connector", "0"),
		ok("ork kafka-connect get tasks", "connector"),
		ok("ork kafka-connect get tasks-config", "connector"),
		ok("ork kafka-connect get topics", "connector"),
		ok("ork kafka-connect health"),
		ok("ork kafka-connect info"),
		fails("ork kafka-connect patch connector", unsupportedKafkaPatch, "connector"),
		ok("ork kafka-connect patch offsets", "connector", "--kafka-topic", "topic", "--kafka-partition", "0", "--kafka-offset", "1", "--force"),
		ok("ork kafka-connect pause", "connector"),
		ok("ork kafka-connect reset topics", "connector"),
		ok("ork kafka-connect restart connector", "connector"),
		ok("ork kafka-connect restart task", "connector", "0"),
		ok("ork kafka-connect resume", "connector"),
		ok("ork kafka-connect stop", "connector"),
		ok("ork model-prices get", "model-1", "--provider", "provider-a"),
		ok("ork model-prices list"),
		ok("ork packages delete", "function://package@v1"),
		ok("ork packages download", "function://package@v1", "--path", "{output}"),
		ok("ork packages get-metadata", "function://package@v1"),
		ok("ork packages list", "--type", "function"),
		ok("ork packages list-versions", "function://package"),
		ok("ork packages update-metadata", "function://package@v1", "--description", "updated"),
		ok("ork packages upload", "function://package@v1", "--path", "{input}"),
		ok("ork readyz"),
		ok("ork sinks available-sinks"),
		ok("ork sinks config-definition", "blackhole"),
		ok("ork sinks create", "--name", "sink", "--sink-type", "blackhole", "--inputs", "topic", "--connection", "connection"),
		ok("ork sinks delete", "sink"),
		ok("ork sinks get", "sink"),
		ok("ork sinks list"),
		ok("ork sinks restart", "sink"),
		ok("ork sinks start", "sink"),
		ok("ork sinks status", "sink"),
		ok("ork sinks stop", "sink"),
		ok("ork sinks update", "sink", "--sink-type", "blackhole", "--inputs", "topic"),
		ok("ork sources available-sources"),
		ok("ork sources config-definition", "data-generator"),
		ok("ork sources create", "--name", "source", "--source-type", "data-generator", "--connection", "connection"),
		ok("ork sources delete", "source"),
		ok("ork sources get", "source"),
		ok("ork sources list"),
		ok("ork sources restart", "source"),
		ok("ork sources start", "source"),
		ok("ork sources status", "source"),
		ok("ork sources stop", "source"),
		ok("ork sources update", "source", "--source-type", "data-generator"),
	}
}

func assertExactLeafCoverage(t *testing.T, cases []commandCase) {
	t.Helper()
	want := make([]string, 0, len(cases))
	seen := make(map[string]struct{}, len(cases))
	for _, tc := range cases {
		if _, exists := seen[tc.path]; exists {
			t.Fatalf("duplicate E2E case for %q", tc.path)
		}
		seen[tc.path] = struct{}{}
		want = append(want, tc.path)
	}
	sort.Strings(want)

	var got []string
	collectLeaves(root.NewCommand(workspace.IOStreams{}), &got)
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("leaf command E2E inventory drifted\n\nCobra leaves:\n%s\n\nE2E cases:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func collectLeaves(cmd *cobra.Command, leaves *[]string) {
	visibleChildren := make([]*cobra.Command, 0)
	for _, child := range cmd.Commands() {
		// The local subtree operates Docker, not the Registry HTTP API. Its
		// lifecycle is verified by the local command tests instead.
		if !child.Hidden && child.Name() != "local" {
			visibleChildren = append(visibleChildren, child)
		}
	}
	if len(visibleChildren) == 0 {
		*leaves = append(*leaves, cmd.CommandPath())
		return
	}
	for _, child := range visibleChildren {
		collectLeaves(child, leaves)
	}
}

type commandFixture struct {
	input          string
	skill          string
	output         string
	connector      string
	mu             sync.Mutex
	requests       []string
	authentication []string
}

func newCommandFixture(t *testing.T) *commandFixture {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("fixture-content"), 0600); err != nil {
		t.Fatal(err)
	}
	connector := filepath.Join(dir, "connector.json")
	connectorConfig := `{"name":"connector","config":{"connector.class":"org.example.Connector","tasks.max":"1"}}`
	if err := os.WriteFile(connector, []byte(connectorConfig), 0600); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skill, []byte("# E2E skill\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return &commandFixture{
		input:     input,
		skill:     skill,
		output:    filepath.Join(dir, "output.bin"),
		connector: connector,
	}
}

func expandFixtureArgs(args []string, fixture *commandFixture) []string {
	expanded := make([]string, len(args))
	for i, arg := range args {
		switch arg {
		case "{input}":
			expanded[i] = fixture.input
		case "{output}":
			expanded[i] = fixture.output
		case "{skill}":
			expanded[i] = fixture.skill
		case "{connector-config}":
			expanded[i] = fixture.connector
		default:
			expanded[i] = arg
		}
	}
	return expanded
}

func (f *commandFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	unauthenticatedProbe := r.URL.Path == "/healthz" || r.URL.Path == "/readyz"
	if unauthenticatedProbe && r.Header.Get("Authorization") != "" {
		f.authentication = append(f.authentication, fmt.Sprintf("%s %s unexpectedly used Authorization %q", r.Method, r.URL.Path, r.Header.Get("Authorization")))
	}
	if !unauthenticatedProbe && r.Header.Get("Authorization") != "Bearer e2e-token" {
		f.authentication = append(f.authentication, fmt.Sprintf("%s %s used Authorization %q", r.Method, r.URL.Path, r.Header.Get("Authorization")))
	}
	f.mu.Unlock()

	switch {
	case r.URL.Path == "/apis":
		writeJSON(w, `{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io","versions":[{"group_version":"cloud.sn.io/v1","version":"v1"}],"preferred_version":{"group_version":"cloud.sn.io/v1","version":"v1"}},{"name":"policy.runorca.ai","versions":[{"group_version":"policy.runorca.ai/v1","version":"v1"}],"preferred_version":{"group_version":"policy.runorca.ai/v1","version":"v1"}},{"name":"pricing.runorca.ai","versions":[{"group_version":"pricing.runorca.ai/v1","version":"v1"}],"preferred_version":{"group_version":"pricing.runorca.ai/v1","version":"v1"}}]}`)
	case r.URL.Path == "/apis/cloud.sn.io/v1/":
		writeJSON(w, `{"kind":"APIResourceList","group_version":"cloud.sn.io/v1","resources":[]}`)
	case r.URL.Path == "/api":
		writeJSON(w, `{"kind":"APIVersions","versions":["v1"],"preferred_version":"v1"}`)
	case r.URL.Path == "/healthz" || r.URL.Path == "/readyz":
		writeJSON(w, `{"status":"ok","service":"fixture"}`)
	case r.URL.Path == "/apis/cloud.sn.io/v1/health" ||
		r.URL.Path == "/apis/cloud.sn.io/v1/health/live" ||
		r.URL.Path == "/apis/cloud.sn.io/v1/health/ready":
		writeJSON(w, `true`)
	case strings.HasSuffix(r.URL.Path, "/events/stream") || strings.HasSuffix(r.URL.Path, "/stream"):
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "id: event-1\nevent: message\ndata: {}\n\n")
	case strings.HasSuffix(r.URL.Path, "/content"):
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = fmt.Fprint(w, "fixture-content")
	case r.Method == http.MethodGet && r.URL.Path == "/apis/cloud.sn.io/v1/packages/function/package/v1":
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = fmt.Fprint(w, "fixture-content")
	case r.URL.Path == "/apis/cloud.sn.io/v1/functions/function:trigger":
		writeJSON(w, `"triggered"`)
	case r.Method == http.MethodGet && r.URL.Path == "/apis/cloud.sn.io/v1/connections":
		writeJSON(w, `[]`)
	case r.Method == http.MethodGet && r.URL.Path == "/apis/cloud.sn.io/v1/connections/connection":
		writeJSON(w, `{"name":"connection","spec":{"type":"kafka","kafka":{"bootstrapServers":"broker:9092"}}}`)
	case r.URL.Path == "/apis/cloud.sn.io/v1/connections/connection:test":
		writeJSON(w, `{"name":"connection","healthy":true,"phase":"Ready"}`)
	case r.Method == http.MethodGet && (r.URL.Path == "/apis/cloud.sn.io/v1/functions" ||
		r.URL.Path == "/apis/cloud.sn.io/v1/connectors/sources" ||
		r.URL.Path == "/apis/cloud.sn.io/v1/connectors/sinks" ||
		r.URL.Path == "/apis/cloud.sn.io/v1/connectors/kafka/connectors"):
		writeJSON(w, `[]`)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/connectors/kafka/connectors/connector/config"):
		writeJSON(w, `{"connector.class":"org.example.Connector","sn.connection":"connection","tasks.max":"1"}`)
	case strings.HasPrefix(r.URL.Path, "/apis/cloud.sn.io/v1/catalog/"):
		writeJSON(w, `[]`)
	case r.Method == http.MethodGet && (r.URL.Path == "/apis/cloud.sn.io/v1/connectors/kafka/connector-plugins" ||
		r.URL.Path == "/apis/cloud.sn.io/v1/connectors/kafka/connector-plugins/catalog" ||
		strings.HasSuffix(r.URL.Path, "/connector-plugins/org.example.Connector/config") ||
		strings.HasSuffix(r.URL.Path, "/tasks")):
		writeJSON(w, `[]`)
	case r.Method == http.MethodGet && r.URL.Path == "/apis/cloud.sn.io/v1/agents/providers":
		writeJSON(w, `[]`)
	case r.Method == http.MethodGet && r.URL.Path == "/apis/cloud.sn.io/v1/packages/function":
		writeJSON(w, `[]`)
	case r.Method == http.MethodGet && r.URL.Path == "/apis/cloud.sn.io/v1/packages/function/package":
		writeJSON(w, `[]`)
	case strings.HasSuffix(r.URL.Path, "/metadata"):
		writeJSON(w, `{"description":"fixture","contact":"e2e@example.com","properties":{}}`)
	default:
		writeJSON(w, `{}`)
	}
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, body)
}

func (f *commandFixture) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *commandFixture) authenticationError() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.authentication, "; ")
}
