// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

// This opt-in test needs a Docker daemon and the Compose PostgreSQL image.
// It holds the real entrypoint's socket-only init server open, then runs the
// embedded healthcheck before allowing startup to finish on a fresh volume.
func TestPostgresReadinessDuringInitialization(t *testing.T) {
	if os.Getenv("ORCA_LOCAL_DOCKER_TEST") != "1" {
		t.Skip("set ORCA_LOCAL_DOCKER_TEST=1 to run Docker integration tests")
	}
	s := &stack{dir: t.TempDir(), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	if err := s.prepare(); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) ([]byte, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		base := []string{"compose", "--project-name", s.projectName(), "--file", filepath.Join(s.dir, "compose.yaml")}
		return exec.CommandContext(ctx, "docker", append(base, args...)...).CombinedOutput()
	}
	mustRun := func(args ...string) []byte {
		t.Helper()
		output, err := run(args...)
		if err != nil {
			t.Fatalf("compose %v: %v\n%s", args, err, output)
		}
		return output
	}
	t.Cleanup(func() {
		if output, err := run("down", "--volumes", "--remove-orphans"); err != nil {
			t.Errorf("cleanup: %v\n%s", err, output)
		}
	})

	data, err := assets.ReadFile("assets/compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Services map[string]struct {
			Healthcheck struct {
				Test []string
			}
		}
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	probe := config.Services["postgres"].Healthcheck.Test
	if len(probe) < 2 || probe[0] != "CMD" {
		t.Fatalf("expected an exec-form healthcheck, got %v", probe)
	}

	// Keep initialization paused without relying on machine speed or sleeps.
	initPath := filepath.Join(s.dir, "postgres-init.sql")
	initSQL, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatal(err)
	}
	initSQL = append(initSQL, []byte("\n\\! touch /tmp/init-waiting\n\\! while [ ! -f /tmp/init-release ]; do sleep 0.1; done\n")...)
	if err := os.WriteFile(initPath, initSQL, 0600); err != nil {
		t.Fatal(err)
	}
	mustRun("up", "-d", "postgres")
	mustRun("exec", "-T", "postgres", "sh", "-c", "for i in $(seq 1 300); do [ ! -f /tmp/init-waiting ] || exit 0; sleep 0.1; done; exit 1")
	// Confirm the temporary server is live, not simply an unstarted database.
	mustRun("exec", "-T", "postgres", "pg_isready", "-U", "orca")
	probeArgs := append([]string{"exec", "-T", "postgres"}, probe[1:]...)
	if output, err := run(probeArgs...); err == nil {
		t.Fatalf("healthcheck accepted the temporary socket-only init server: %s", output)
	} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 2 {
		t.Fatalf("expected pg_isready no-response exit 2, got %v: %s", err, output)
	}
	mustRun("exec", "-T", "postgres", "touch", "/tmp/init-release")
	mustRun("up", "-d", "--wait", "--wait-timeout", "60", "postgres")
	mustRun(probeArgs...)
	for _, database := range []string{"registry", "transcriptstore", "filestore", "memorystore"} {
		output := mustRun("exec", "-T", "postgres", "psql", "-h", "127.0.0.1", "-U", "orca", "-d", database, "-Atc", "SELECT 1")
		if strings.TrimSpace(string(output)) != "1" {
			t.Fatalf("database %s not queryable over TCP: %s", database, output)
		}
	}
}
