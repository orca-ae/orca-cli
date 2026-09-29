// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestPreparePreservesCredentials(t *testing.T) {
	s := &stack{dir: filepath.Join(t.TempDir(), "local"), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	if err := s.prepare(); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(s.dir, "secrets", "session-jwt-private.pem")
	publicPath := filepath.Join(s.dir, "secrets", "session-jwt-public.pem")
	private, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	public, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	privateBlock, _ := pem.Decode(private)
	publicBlock, _ := pem.Decode(public)
	if privateBlock == nil || publicBlock == nil {
		t.Fatal("JWT keypair is not PEM")
	}
	privateKey, err := x509.ParsePKCS8PrivateKey(privateBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := x509.ParsePKIXPublicKey(publicBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if privateKey.(*rsa.PrivateKey).PublicKey.N.Cmp(publicKey.(*rsa.PublicKey).N) != 0 {
		t.Fatal("JWT keys do not match")
	}
	tokenPath := filepath.Join(s.dir, "secrets", "internal-token")
	token, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.prepare(); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(privatePath)
	if !bytes.Equal(private, again) {
		t.Fatal("repeated prepare rotated JWT private key")
	}
	again, _ = os.ReadFile(tokenPath)
	if !bytes.Equal(token, again) {
		t.Fatal("repeated prepare rotated internal token")
	}
	info, err := os.Stat(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("data directory mode = %o", info.Mode().Perm())
	}
}

func TestEmbeddedComposeIsValid(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker CLI is unavailable")
	}
	s := &stack{dir: filepath.Join(t.TempDir(), "local"), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	if err := s.prepare(); err != nil {
		t.Fatal(err)
	}
	output, err := s.composeOutput(nil, "config")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"registry:", "harness:", "postgres:", "rustfs:", "TRANSCRIPT_STORE_BACKEND: postgres"} {
		if !strings.Contains(string(output), required) {
			t.Fatalf("Compose config missing %q", required)
		}
	}
}

// Compose v5 writes "Container ... Creating" progress to stderr even for
// `compose run`; the bootstrap parses the one-off container's stdout as JSON.
func TestComposeOutputKeepsProgressOutOfTheResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake Docker CLI is a shell script")
	}
	bin := t.TempDir()
	fake := `#!/bin/sh
echo ' Container ork-local-test-registry-run-1 Creating' >&2
echo ' Container ork-local-test-registry-run-1 Created' >&2
if [ "$FAKE_DOCKER_FAIL" = 1 ]; then
  echo 'no such service: registry' >&2
  exit 1
fi
echo '{"workspace_id":"wrkspc_test"}'
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(fake), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	s := &stack{dir: t.TempDir()}

	output, err := s.composeOutput(nil, "run", "--rm", "registry")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(output)); got != `{"workspace_id":"wrkspc_test"}` {
		t.Fatalf("output = %q, want only the command's stdout", got)
	}

	t.Setenv("FAKE_DOCKER_FAIL", "1")
	if _, err := s.composeOutput(nil, "run", "--rm", "registry"); err == nil || !strings.Contains(err.Error(), "no such service: registry") {
		t.Fatalf("error = %v, want it to carry the command's stderr", err)
	}
}

func TestGatewayConfigIsYAML(t *testing.T) {
	data, err := assets.ReadFile("assets/gateway.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"server", "identity", "vaults", "destinations", "routes", "plugins"} {
		if _, ok := config[required]; !ok {
			t.Fatalf("gateway config missing %q", required)
		}
	}
}

func TestDataDirectoriesUseDifferentComposeProjects(t *testing.T) {
	first := (&stack{dir: filepath.Join(t.TempDir(), "first")}).projectName()
	second := (&stack{dir: filepath.Join(t.TempDir(), "second")}).projectName()
	if first == second {
		t.Fatalf("different local stacks share project %q", first)
	}
}
