// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestCoreResourceCleanup(t *testing.T) {
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("E2E cleanup test requires %s: %v", tool, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "ork")
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/ork").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	for _, tc := range []struct {
		name     string
		response string
		wantOK   bool
	}{
		{"archived", `{"id":"session-1","archived_at":"2026-09-21T00:00:00Z"}`, true},
		{"wrong session", `{"id":"session-2","archived_at":"2026-09-21T00:00:00Z"}`, false},
		{"missing archive timestamp", `{"id":"session-1"}`, false},
		{"empty archive timestamp", `{"id":"session-1","archived_at":""}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.Method+" "+r.URL.Path)
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "POST /v1/sessions/session-1/archive":
					fmt.Fprint(w, tc.response)
				case "POST /v1/agents/agent-1/archive", "POST /v1/environments/environment-1/archive", "DELETE /v1/files/file-1":
					fmt.Fprint(w, `{}`)
				default:
					http.Error(w, "unexpected cleanup request", http.StatusBadRequest)
				}
			}))
			defer server.Close()

			resources := t.TempDir()
			for _, resource := range []string{"session", "agent", "environment", "file"} {
				body := fmt.Sprintf(`{"id":%q}`, resource+"-1")
				if err := os.WriteFile(filepath.Join(resources, resource+".json"), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// Execute the actual cleanup function with the built CLI, including its
			// flags and jq pipeline, without deploying the rest of the E2E stack.
			cmd := exec.CommandContext(ctx, "bash", "-c", `
source ./lib.sh
eval "$(sed -n '/^core_resource_cleanup() {/,/^}/p' core.sh)"
core_resource_cleanup
`)
			cmd.Env = append(os.Environ(), "ORCA_BIN="+binary, "ORCA_REGISTRY_URL="+server.URL,
				"ORCA_ACCESS_TOKEN=", "ORCA_API_KEY=e2e-key", "temp_dir="+resources)
			output, err := cmd.CombinedOutput()
			if (err == nil) != tc.wantOK {
				t.Fatalf("cleanup error = %v, want success %t\n%s", err, tc.wantOK, output)
			}
			want := []string{"POST /v1/sessions/session-1/archive"}
			if tc.wantOK {
				want = append(want, "POST /v1/agents/agent-1/archive", "POST /v1/environments/environment-1/archive", "DELETE /v1/files/file-1")
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(requests, want) {
				t.Errorf("cleanup requests = %v, want %v", requests, want)
			}
			_, statErr := os.Stat(filepath.Join(resources, "session.json"))
			if tc.wantOK && !os.IsNotExist(statErr) {
				t.Error("successful cleanup retained the session fixture")
			}
			if !tc.wantOK && statErr != nil {
				t.Errorf("failed cleanup lost the session fixture needed for retry: %v", statErr)
			}
		})
	}
}
