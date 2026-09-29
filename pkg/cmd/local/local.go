// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

//go:embed assets/*
var assets embed.FS

type stack struct {
	dir string
	out io.Writer
	err io.Writer
}

// NewCommand manages a single-machine, development-only Managed Agents stack.
func NewCommand(out, errOut io.Writer) *cobra.Command {
	defaultDir, err := os.UserConfigDir()
	if err != nil {
		defaultDir = "."
	}
	s := &stack{out: out, err: errOut}
	cmd := &cobra.Command{Use: "local", Short: "Manage a local Docker Compose stack"}
	cmd.PersistentFlags().StringVar(&s.dir, "data-dir", filepath.Join(defaultDir, "ork", "local"), "Local stack configuration and secrets directory")
	var withGateway bool
	start := &cobra.Command{
		Use: "start", Short: "Start the local stack", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return s.start(withGateway) },
	}
	start.Flags().BoolVar(&withGateway, "with-gateway", false, "Also start AI Gateway (requires access to its image)")
	stop := &cobra.Command{
		Use: "stop", Short: "Stop the local stack and keep its data", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return s.stop() },
	}
	status := &cobra.Command{
		Use: "status", Short: "Show local stack containers", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := os.Stat(filepath.Join(s.dir, "compose.yaml")); errors.Is(err, os.ErrNotExist) {
				fmt.Fprintln(s.out, "Local stack has not been initialized.")
				return nil
			} else if err != nil {
				return err
			}
			return s.compose(nil, "ps")
		},
	}
	cmd.AddCommand(start, stop, status)
	return cmd
}

func (s *stack) start(withGateway bool) error {
	if err := s.checkDocker(); err != nil {
		return err
	}
	if err := s.prepare(); err != nil {
		return err
	}
	composeEnv := []string(nil)
	if withGateway {
		composeEnv = []string{"ORCA_LOCAL_LLM_EGRESS=gateway"}
	}
	if err := s.compose(composeEnv, "config", "--quiet"); err != nil {
		return err
	}
	if withGateway {
		// Fail before starting other services when the optional private image
		// cannot be pulled by this Docker installation.
		if err := s.compose(nil, "pull", "ai-gateway"); err != nil {
			return err
		}
	}
	args := []string{"up", "-d", "--wait", "registry", "harness"}
	if withGateway {
		args = append(args, "ai-gateway")
	}
	if err := s.compose(composeEnv, args...); err != nil {
		return err
	}
	if err := s.bootstrap(); err != nil {
		return err
	}
	fmt.Fprintf(s.out, "Registry: http://127.0.0.1:%s\n", registryPort())
	fmt.Fprintf(s.out, "Workspace API key: %s\n", filepath.Join(s.dir, "secrets", "workspace-api-key"))
	fmt.Fprintln(s.out, "Local Harness uses the unisolated in-memory sandbox. Use this stack only with trusted code.")
	return nil
}

func (s *stack) stop() error {
	if _, err := os.Stat(filepath.Join(s.dir, "compose.yaml")); errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(s.out, "Local stack has not been initialized.")
		return nil
	} else if err != nil {
		return err
	}
	if err := s.checkDocker(); err != nil {
		return err
	}
	return s.compose(nil, "down", "--remove-orphans")
}

func (s *stack) checkDocker() error {
	cmd := exec.Command("docker", "compose", "version")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Docker Compose is required: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *stack) compose(extraEnv []string, args ...string) error {
	cmdArgs := append([]string{"compose", "--project-name", s.projectName(), "--file", filepath.Join(s.dir, "compose.yaml")}, args...)
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Dir = s.dir
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = s.out
	cmd.Stderr = s.err
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// composeOutput returns only what the command writes to stdout. Compose reports
// its own progress, such as "Container ... Creating", on stderr, so stderr is
// kept out of the result and only used to explain a failure.
func (s *stack) composeOutput(extraEnv []string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"compose", "--project-name", s.projectName(), "--file", filepath.Join(s.dir, "compose.yaml")}, args...)
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Dir = s.dir
	cmd.Env = append(os.Environ(), extraEnv...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker compose %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

func (s *stack) projectName() string {
	absolute, err := filepath.Abs(s.dir)
	if err != nil {
		absolute = s.dir
	}
	sum := sha256.Sum256([]byte(filepath.Clean(absolute)))
	return fmt.Sprintf("ork-local-%x", sum[:4])
}

func (s *stack) prepare() error {
	if err := os.MkdirAll(filepath.Join(s.dir, "secrets"), 0700); err != nil {
		return err
	}
	if err := os.Chmod(s.dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(s.dir, "secrets"), 0700); err != nil {
		return err
	}
	for _, name := range []string{"compose.yaml", "postgres-init.sql", "rustfs-bootstrap.sh", "gateway.yaml"} {
		data, err := assets.ReadFile("assets/" + name)
		if err != nil {
			return err
		}
		mode := os.FileMode(0600)
		if name == "rustfs-bootstrap.sh" || name == "gateway.yaml" {
			mode = 0644
		}
		if err := os.WriteFile(filepath.Join(s.dir, name), data, mode); err != nil {
			return err
		}
	}
	if err := s.prepareJWT(); err != nil {
		return err
	}
	token, err := randomString(32)
	if err != nil {
		return err
	}
	_, err = writeSecretOnce(filepath.Join(s.dir, "secrets", "internal-token"), token, 0644)
	return err
}

func (s *stack) prepareJWT() error {
	privatePath := filepath.Join(s.dir, "secrets", "session-jwt-private.pem")
	publicPath := filepath.Join(s.dir, "secrets", "session-jwt-public.pem")
	_, privateErr := os.Stat(privatePath)
	_, publicErr := os.Stat(publicPath)
	if privateErr == nil && publicErr == nil {
		return nil
	}
	if !errors.Is(privateErr, os.ErrNotExist) || !errors.Is(publicErr, os.ErrNotExist) {
		return fmt.Errorf("incomplete JWT keypair in %s; restore both files", filepath.Dir(privatePath))
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	private := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	public := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	if _, err := writeSecretOnce(privatePath, private, 0644); err != nil {
		return err
	}
	_, err = writeSecretOnce(publicPath, public, 0644)
	return err
}

func randomString(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return []byte(base64.RawURLEncoding.EncodeToString(b)), nil
}

func writeSecretOnce(path string, data []byte, mode os.FileMode) (bool, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return false, err
	}
	return true, nil
}

func registryPort() string {
	if port := os.Getenv("ORCA_LOCAL_REGISTRY_PORT"); port != "" {
		return port
	}
	return "8080"
}

func adminPort() string {
	if port := os.Getenv("ORCA_LOCAL_ADMIN_PORT"); port != "" {
		return port
	}
	return "18082"
}

func (s *stack) bootstrap() error {
	secretDir := filepath.Join(s.dir, "secrets")
	keyPath := filepath.Join(secretDir, "workspace-api-key")
	if _, err := os.Stat(keyPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	adminPath := filepath.Join(secretDir, "admin-api-key")
	platformPath := filepath.Join(secretDir, "platform-api-key")
	workspaceIDPath := filepath.Join(secretDir, "workspace-id")
	adminSuffix, err := randomString(36)
	if err != nil {
		return err
	}
	admin, err := readOrCreate(adminPath, append([]byte("orca_admin_"), adminSuffix...))
	if err != nil {
		return err
	}
	platformSuffix, err := randomString(36)
	if err != nil {
		return err
	}
	platform, err := readOrCreate(platformPath, append([]byte("orca_platform_"), platformSuffix...))
	if err != nil {
		return err
	}
	workspaceID, err := os.ReadFile(workspaceIDPath)
	if errors.Is(err, os.ErrNotExist) {
		output, runErr := s.composeOutput([]string{"ORCA_BOOTSTRAP_ADMIN_API_KEY=" + string(admin), "ORCA_BOOTSTRAP_PLATFORM_API_KEY=" + string(platform)},
			"run", "--rm", "--no-deps", "-e", "ORCA_BOOTSTRAP_ADMIN_API_KEY", "-e", "ORCA_BOOTSTRAP_PLATFORM_API_KEY", "registry", "node", "dist/bootstrap-admin.js")
		if runErr != nil {
			return runErr
		}
		var result struct {
			WorkspaceID string `json:"workspace_id"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(output), &result); err != nil {
			return fmt.Errorf("parse Registry bootstrap output: %w: %s", err, output)
		}
		if result.WorkspaceID == "" {
			return fmt.Errorf("Registry bootstrap did not return a workspace ID")
		}
		workspaceID = []byte(result.WorkspaceID)
		if _, err := writeSecretOnce(workspaceIDPath, workspaceID, 0600); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	requestBody := strings.NewReader(`{"name":"ork local"}`)
	url := fmt.Sprintf("http://127.0.0.1:%s/v1/organizations/workspaces/%s/api_keys", adminPort(), string(workspaceID))
	client := &http.Client{Timeout: 3 * time.Second}
	if err := waitForAdmin(client, adminPort()); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, requestBody)
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", string(admin))
	client.Timeout = 15 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("create local workspace API key: HTTP %d: %s", resp.StatusCode, body)
	}
	var keyResult struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&keyResult); err != nil {
		return err
	}
	if keyResult.Key == "" {
		return errors.New("Registry returned an empty workspace API key")
	}
	_, err = writeSecretOnce(keyPath, []byte(keyResult.Key), 0600)
	return err
}

func waitForAdmin(client *http.Client, port string) error {
	url := "http://127.0.0.1:" + port + "/healthz"
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := client.Get(url)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("Registry admin listener did not become ready at %s", url)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func readOrCreate(path string, candidate []byte) ([]byte, error) {
	if _, err := writeSecretOnce(path, candidate, 0600); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
