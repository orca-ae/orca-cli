// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestManagedAgentsWorkflowTrustBoundary(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/e2e-managed-agents.yml")
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		On   map[string]interface{} `json:"on"`
		Jobs map[string]struct {
			If     string `json:"if"`
			RunsOn string `json:"runs-on"`
			Needs  string `json:"needs"`
			Steps  []struct {
				Uses string                 `json:"uses"`
				With map[string]interface{} `json:"with"`
				Run  string                 `json:"run"`
			} `json:"steps"`
		} `json:"jobs"`
	}
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	if len(w.On) != 3 {
		t.Fatalf("unexpected triggers: %v", w.On)
	}
	for _, event := range []string{"push", "schedule", "workflow_dispatch"} {
		if _, ok := w.On[event]; !ok {
			t.Fatalf("missing trusted event %s", event)
		}
	}
	gate := w.Jobs["secret-gate"]
	wantGate := "github.ref == 'refs/heads/main' && (github.event_name == 'push' || github.event_name == 'schedule' || github.event_name == 'workflow_dispatch')"
	if strings.Join(strings.Fields(gate.If), " ") != wantGate {
		t.Fatalf("unsafe gate: %s", gate.If)
	}
	e2e := w.Jobs["e2e"]
	if e2e.Needs != "secret-gate" || e2e.If != "needs.secret-gate.outputs.enabled == 'true'" {
		t.Fatal("E2E must depend on trust gate")
	}
	if e2e.RunsOn != "ubuntu-latest" {
		t.Fatal("credential job must use ephemeral hosted runner")
	}
	first := e2e.Steps[0]
	if first.With["ref"] != "${{ github.sha }}" || first.With["persist-credentials"] != false {
		t.Fatal("checkout must pin triggering SHA without persisted credential")
	}
	for _, step := range e2e.Steps {
		if strings.Contains(step.Uses, "actions/setup-go@") && step.With["cache"] != false {
			t.Fatal("trusted E2E must not restore PR caches")
		}
		if strings.Contains(step.Run, "create secret docker-registry") {
			t.Fatal("registry credentials must not enter test cluster")
		}
	}
	if strings.Contains(string(data), "MANAGED_AGENTS_IMAGE_PULL_SECRET") {
		t.Fatal("no image pull secret may be passed to test cluster")
	}
	values, err := os.ReadFile("managed-agents-values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Images map[string]struct {
			PullPolicy string `json:"pullPolicy"`
		} `json:"images"`
	}
	if err := yaml.Unmarshal(values, &v); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"registry", "harness", "aiGateway"} {
		if v.Images[name].PullPolicy != "Never" {
			t.Fatalf("%s must use preloaded image", name)
		}
	}
}

func TestLoadBackendImagesCleansCredentials(t *testing.T) {
	for _, fail := range []string{"none", "login", "pull", "kind"} {
		t.Run(fail, func(t *testing.T) {
			dir := t.TempDir()
			mock := `#!/usr/bin/env bash
set -euo pipefail
command=${0##*/}
echo "$command $*" >> "$TEST_LOG"
if [[ "$command" == gh ]]; then echo test-bot; exit; fi
if [[ "$command" == kind ]]; then
  [[ -z "${GH_TOKEN:-}" && -z "${DOCKER_CONFIG:-}" ]]
  [[ ! -e "$(cat "$TEST_CONFIG")" ]]
  [[ "$FAIL" != kind ]]
  exit
fi
[[ -n "$DOCKER_CONFIG" && -d "$DOCKER_CONFIG" ]]
echo "$DOCKER_CONFIG" > "$TEST_CONFIG"
if [[ "$1" == login ]]; then
  [[ "$(cat)" == synthetic-token ]]
  echo synthetic-token > "$DOCKER_CONFIG/config.json"
else
  [[ -z "${GH_TOKEN:-}" ]]
fi
[[ "$FAIL" != "$1" ]]
`
			for _, name := range []string{"gh", "docker", "kind"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(mock), 0700); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", "load-backend-images.sh")
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "GH_TOKEN=synthetic-token", "KIND_CLUSTER_NAME=test", "REGISTRY_IMAGE=registry:pin", "REGISTRY_RESOLVED_IMAGE=registry@sha256:abc", "HARNESS_IMAGE=harness:pin", "HARNESS_RESOLVED_IMAGE=harness@sha256:def", "GATEWAY_IMAGE=ghcr.io/test/gateway:pin", "TEST_LOG="+dir+"/log", "TEST_CONFIG="+dir+"/config-path", "FAIL="+fail)
			out, err := cmd.CombinedOutput()
			if (err == nil) != (fail == "none") {
				t.Fatalf("unexpected result: %v %s", err, out)
			}
			config, err := os.ReadFile(filepath.Join(dir, "config-path"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(strings.TrimSpace(string(config))); !os.IsNotExist(err) {
				t.Fatal("Docker auth directory was not removed")
			}
			log, err := os.ReadFile(filepath.Join(dir, "log"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(log), "synthetic-token") || strings.Contains(string(out), "synthetic-token") {
				t.Fatal("secret exposed in arguments/output")
			}
			if fail == "none" {
				for _, line := range []string{"docker pull --platform linux/amd64 registry@sha256:abc", "docker tag registry@sha256:abc registry:pin", "docker pull --platform linux/amd64 harness@sha256:def", "docker tag harness@sha256:def harness:pin", "kind load docker-image --name test registry:pin harness:pin ghcr.io/test/gateway:pin"} {
					if !strings.Contains(string(log), line) {
						t.Errorf("missing %s", line)
					}
				}
			}
		})
	}
}
