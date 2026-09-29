// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	util "github.com/apache/pulsar-client-go/pulsaradmin/pkg/utils"
	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

func TestBuildSinkConfigUsesInteractiveConnectionSelection(t *testing.T) {
	original := selectConnectionNameForApply
	selectConnectionNameForApply = func(context.Context) (string, error) {
		return "conn-sink", nil
	}
	defer func() {
		selectConnectionNameForApply = original
	}()

	cfg, filePath, packageURL, err := buildSinkConfig(context.Background(), &sinkApplyOptions{
		Name:             "sink-1",
		SinkType:         "jdbc",
		UseConnection:    true,
		LogTopic:         "persistent://public/default/sink-logs",
		SNServiceAccount: "svc-sink",
	}, false)
	if err != nil {
		t.Fatalf("buildSinkConfig() error = %v", err)
	}
	if cfg.Connection != "conn-sink" {
		t.Fatalf("cfg.Connection = %q, want %q", cfg.Connection, "conn-sink")
	}
	if cfg.SNServiceAccount != "svc-sink" {
		t.Fatalf("cfg.SNServiceAccount = %q, want %q", cfg.SNServiceAccount, "svc-sink")
	}
	if cfg.LogTopic != "persistent://public/default/sink-logs" {
		t.Fatalf("cfg.LogTopic = %q, want sink log topic", cfg.LogTopic)
	}
	if filePath != "" {
		t.Fatalf("filePath = %q, want empty", filePath)
	}
	if packageURL != "builtin://jdbc" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://jdbc")
	}
}

type workspaceSinksClientMock struct {
	getFn    func(ctx context.Context, name string) (*registry.RegistrySinkConfig, error)
	updateFn func(ctx context.Context, name string, cfg registry.RegistrySinkConfig, filePath, packageURL string, updateOptions *registry.UpdateOptionsImpl) error
}

func (m *workspaceSinksClientMock) List(context.Context) ([]string, error) {
	return nil, nil
}

func (m *workspaceSinksClientMock) Get(ctx context.Context, name string) (*registry.RegistrySinkConfig, error) {
	if m.getFn != nil {
		return m.getFn(ctx, name)
	}
	return nil, nil
}

func (m *workspaceSinksClientMock) Create(context.Context, registry.RegistrySinkConfig, string, string) error {
	return nil
}

func (m *workspaceSinksClientMock) Update(ctx context.Context, name string, cfg registry.RegistrySinkConfig, filePath, packageURL string, updateOptions *registry.UpdateOptionsImpl) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, name, cfg, filePath, packageURL, updateOptions)
	}
	return nil
}

func (m *workspaceSinksClientMock) Delete(context.Context, string) error {
	return nil
}

func (m *workspaceSinksClientMock) Start(context.Context, string) error {
	return nil
}

func (m *workspaceSinksClientMock) Stop(context.Context, string) error {
	return nil
}

func (m *workspaceSinksClientMock) Restart(context.Context, string) error {
	return nil
}

func (m *workspaceSinksClientMock) StartInstance(context.Context, string, string) error {
	return nil
}

func (m *workspaceSinksClientMock) StopInstance(context.Context, string, string) error {
	return nil
}

func (m *workspaceSinksClientMock) RestartInstance(context.Context, string, string) error {
	return nil
}

func (m *workspaceSinksClientMock) Status(context.Context, string) (*registry.SinkStatus, error) {
	return nil, nil
}

func (m *workspaceSinksClientMock) InstanceStatus(context.Context, string, string) (*registry.SinkInstanceStatusData, error) {
	return nil, nil
}

func TestBuildSinkConfigRejectsConflictingConnectionFlags(t *testing.T) {
	_, _, _, err := buildSinkConfig(context.Background(), &sinkApplyOptions{
		Name:          "sink-1",
		SinkType:      "jdbc",
		Connection:    "conn-explicit",
		UseConnection: true,
	}, false)
	if err == nil {
		t.Fatal("buildSinkConfig() expected error, got nil")
	}
}

func TestBuildSinkConfigRequiresConnectionOnCreate(t *testing.T) {
	_, _, _, err := buildSinkConfig(context.Background(), &sinkApplyOptions{Name: "sink-1", SinkType: "jdbc"}, false)
	if err == nil || !strings.Contains(err.Error(), "--connection or --use-connection is required") {
		t.Fatalf("buildSinkConfig() error = %v", err)
	}
}

func TestSinkUpdateOmitsConnectionFlags(t *testing.T) {
	cmd := (&sinksOptions{}).newUpdateCommand()
	if cmd.Flags().Lookup("connection") != nil || cmd.Flags().Lookup("use-connection") != nil {
		t.Fatal("sink update exposes immutable connection flags")
	}
}

func TestBuildSinkConfigLoadsRegistryFieldsFromFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "sink.yaml")
	if err := os.WriteFile(configPath, []byte(`name: sink-1
archive: builtin://jdbc
connection: conn-sink
logTopic: persistent://public/default/sink-logs
snServiceAccount: svc-sink
`), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, _, _, err := buildSinkConfig(context.Background(), &sinkApplyOptions{SinkConfigFile: configPath}, false)
	if err != nil {
		t.Fatalf("buildSinkConfig() error = %v", err)
	}
	if cfg.LogTopic != "persistent://public/default/sink-logs" || cfg.SNServiceAccount != "svc-sink" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestAddSinkApplyFlagsExposesParallelismFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	opts := &sinkApplyOptions{}

	addSinkApplyFlags(cmd, opts, true)

	if cmd.Flags().Lookup("parallelism") == nil {
		t.Fatal("expected --parallelism flag to be registered")
	}
}

func TestBuildSinkConfigUsesParallelismFromFlags(t *testing.T) {
	cfg, filePath, packageURL, err := buildSinkConfig(context.Background(), &sinkApplyOptions{
		Name:        "sink-1",
		SinkType:    "jdbc",
		Connection:  "conn-sink",
		Parallelism: 3,
	}, false)
	if err != nil {
		t.Fatalf("buildSinkConfig() error = %v", err)
	}
	if cfg.Parallelism != 3 {
		t.Fatalf("cfg.Parallelism = %d, want %d", cfg.Parallelism, 3)
	}
	if filePath != "" {
		t.Fatalf("filePath = %q, want empty", filePath)
	}
	if packageURL != "builtin://jdbc" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://jdbc")
	}
}

func TestBuildSinkUpdateOptions(t *testing.T) {
	if updateOptions := buildSinkUpdateOptions(false, true); updateOptions != nil {
		t.Fatalf("buildSinkUpdateOptions() = %#v, want nil", updateOptions)
	}

	updateOptions := buildSinkUpdateOptions(true, true)
	if updateOptions == nil {
		t.Fatal("buildSinkUpdateOptions() = nil, want non-nil")
	}
	if !updateOptions.UpdateAuthData {
		t.Fatal("updateOptions.UpdateAuthData = false, want true")
	}
}

func TestSinkCommandExposesCatalogCommands(t *testing.T) {
	o := &sinksOptions{}
	cmd := NewCmdSinks(&Options{IOStreams: o.ioStreams})

	if _, _, err := cmd.Find([]string{"available-sinks"}); err != nil {
		t.Fatalf("available-sinks command not found: %v", err)
	}
	if _, _, err := cmd.Find([]string{"config-definition"}); err != nil {
		t.Fatalf("config-definition command not found: %v", err)
	}
}

func TestSinkAvailableSinksCommandForwardsAndRendersText(t *testing.T) {
	original := newWorkspaceCatalogClient
	defer func() {
		newWorkspaceCatalogClient = original
	}()

	called := false
	newWorkspaceCatalogClient = func() (workspaceCatalogClient, error) {
		return &workspaceCatalogClientMock{
			listSinksFn: func(context.Context) ([]registry.ConnectorDefinition, error) {
				called = true
				return []registry.ConnectorDefinition{{
					Name:            "jdbc",
					Description:     "JDBC sink",
					SinkClass:       "org.example.JdbcSink",
					SinkConfigClass: "org.example.JdbcSinkConfig",
				}}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	o := &sinksOptions{ioStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}}
	cmd := o.newAvailableSinksCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("ListSinks() was not called")
	}
	output := stdout.String()
	for _, expected := range []string{"jdbc", "JDBC sink", "org.example.JdbcSink", "org.example.JdbcSinkConfig"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output missing %q: %s", expected, output)
		}
	}
}

func TestSinkConfigDefinitionCommandForwardsAndRendersText(t *testing.T) {
	original := newWorkspaceCatalogClient
	defer func() {
		newWorkspaceCatalogClient = original
	}()

	var gotName string
	newWorkspaceCatalogClient = func() (workspaceCatalogClient, error) {
		return &workspaceCatalogClientMock{
			getSinkConfigDefinitionFn: func(_ context.Context, name string) ([]registry.ConfigFieldDefinition, error) {
				gotName = name
				return []registry.ConfigFieldDefinition{{
					FieldName: "jdbcUrl",
					TypeName:  "java.lang.String",
					Attributes: map[string]string{
						"required": "true",
					},
				}}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	o := &sinksOptions{ioStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}}
	cmd := o.newConfigDefinitionCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"jdbc"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotName != "jdbc" {
		t.Fatalf("config definition name = %q, want %q", gotName, "jdbc")
	}
	output := stdout.String()
	for _, expected := range []string{"jdbcUrl", "java.lang.String", "required=true"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output missing %q: %s", expected, output)
		}
	}
}

func TestSinkUpdateCommandExposesUpdateAuthDataAndDeprecatedFlags(t *testing.T) {
	o := &sinksOptions{}
	cmd := o.newUpdateCommand()

	if cmd.Flags().Lookup("update-auth-data") == nil {
		t.Fatal("expected --update-auth-data flag to be registered")
	}

	for _, flagName := range []string{"auto-ack", "processing-guarantees", "retain-ordering"} {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Fatalf("expected %q flag to be registered", flagName)
		}
		if flag.Deprecated == "" {
			t.Fatalf("expected %q flag to be marked deprecated", flagName)
		}
	}
}

func TestSinkUpdateCommandPreservesImmutableFieldsAndForwardsUpdateOptions(t *testing.T) {
	original := newWorkspaceSinksClient
	defer func() {
		newWorkspaceSinksClient = original
	}()

	var gotName string
	var gotCfg registry.RegistrySinkConfig
	var gotUpdateOptions *registry.UpdateOptionsImpl

	newWorkspaceSinksClient = func() (workspaceSinksClient, error) {
		return &workspaceSinksClientMock{
			getFn: func(_ context.Context, name string) (*registry.RegistrySinkConfig, error) {
				return &registry.RegistrySinkConfig{
					SinkConfig: util.SinkConfig{
						Name:                 name,
						AutoAck:              false,
						ProcessingGuarantees: "ATLEAST_ONCE",
						RetainOrdering:       false,
					},
				}, nil
			},
			updateFn: func(_ context.Context, name string, cfg registry.RegistrySinkConfig, filePath, packageURL string, updateOptions *registry.UpdateOptionsImpl) error {
				gotName = name
				gotCfg = cfg
				gotUpdateOptions = updateOptions
				if filePath != "" {
					t.Fatalf("filePath = %q, want empty", filePath)
				}
				if packageURL != "" {
					t.Fatalf("packageURL = %q, want empty", packageURL)
				}
				return nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	o := &sinksOptions{
		ioStreams: IOStreams{
			In:     nil,
			Out:    &stdout,
			ErrOut: io.Discard,
		},
	}

	cmd := o.newUpdateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"sink-1",
		"--auto-ack",
		"--processing-guarantees", "EFFECTIVELY_ONCE",
		"--retain-ordering",
		"--update-auth-data",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotName != "sink-1" {
		t.Fatalf("update name = %q, want %q", gotName, "sink-1")
	}
	if gotCfg.AutoAck {
		t.Fatal("gotCfg.AutoAck = true, want false from current config")
	}
	if gotCfg.ProcessingGuarantees != "ATLEAST_ONCE" {
		t.Fatalf("gotCfg.ProcessingGuarantees = %q, want %q", gotCfg.ProcessingGuarantees, "ATLEAST_ONCE")
	}
	if gotCfg.RetainOrdering {
		t.Fatal("gotCfg.RetainOrdering = true, want false from current config")
	}
	if gotUpdateOptions == nil {
		t.Fatal("gotUpdateOptions = nil, want non-nil")
	}
	if !gotUpdateOptions.UpdateAuthData {
		t.Fatal("gotUpdateOptions.UpdateAuthData = false, want true")
	}
}
