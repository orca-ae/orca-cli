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

	registry "github.com/orca-ae/orca-sdk-go"
)

type workspaceSourcesClientMock struct {
	updateFn func(ctx context.Context, name string, cfg registry.RegistrySourceConfig, filePath, packageURL string, updateOptions *registry.UpdateOptionsImpl) error
}

type workspaceCatalogClientMock struct {
	listSourcesFn               func(ctx context.Context) ([]registry.ConnectorDefinition, error)
	getSourceConfigDefinitionFn func(ctx context.Context, name string) ([]registry.ConfigFieldDefinition, error)
	listSinksFn                 func(ctx context.Context) ([]registry.ConnectorDefinition, error)
	getSinkConfigDefinitionFn   func(ctx context.Context, name string) ([]registry.ConfigFieldDefinition, error)
	listKafkaConnectorsFn       func(ctx context.Context) ([]registry.ConnectorDefinition, error)
	getKafkaConfigDefinitionFn  func(ctx context.Context, name string) ([]registry.ConfigFieldDefinition, error)
}

func (m *workspaceCatalogClientMock) ListSources(ctx context.Context) ([]registry.ConnectorDefinition, error) {
	if m.listSourcesFn != nil {
		return m.listSourcesFn(ctx)
	}
	return nil, nil
}

func (m *workspaceCatalogClientMock) GetSourceConfigDefinition(ctx context.Context, name string) ([]registry.ConfigFieldDefinition, error) {
	if m.getSourceConfigDefinitionFn != nil {
		return m.getSourceConfigDefinitionFn(ctx, name)
	}
	return nil, nil
}

func (m *workspaceCatalogClientMock) ListSinks(ctx context.Context) ([]registry.ConnectorDefinition, error) {
	if m.listSinksFn != nil {
		return m.listSinksFn(ctx)
	}
	return nil, nil
}

func (m *workspaceCatalogClientMock) GetSinkConfigDefinition(ctx context.Context, name string) ([]registry.ConfigFieldDefinition, error) {
	if m.getSinkConfigDefinitionFn != nil {
		return m.getSinkConfigDefinitionFn(ctx, name)
	}
	return nil, nil
}

func (m *workspaceCatalogClientMock) ListKafkaConnectors(ctx context.Context) ([]registry.ConnectorDefinition, error) {
	if m.listKafkaConnectorsFn != nil {
		return m.listKafkaConnectorsFn(ctx)
	}
	return nil, nil
}

func (m *workspaceCatalogClientMock) GetKafkaConfigDefinition(ctx context.Context, name string) ([]registry.ConfigFieldDefinition, error) {
	if m.getKafkaConfigDefinitionFn != nil {
		return m.getKafkaConfigDefinitionFn(ctx, name)
	}
	return nil, nil
}

func (m *workspaceSourcesClientMock) List(context.Context) ([]string, error) {
	return nil, nil
}

func (m *workspaceSourcesClientMock) Get(context.Context, string) (*registry.RegistrySourceConfig, error) {
	return nil, nil
}

func (m *workspaceSourcesClientMock) Create(context.Context, registry.RegistrySourceConfig, string, string) error {
	return nil
}

func (m *workspaceSourcesClientMock) Update(ctx context.Context, name string, cfg registry.RegistrySourceConfig, filePath, packageURL string, updateOptions *registry.UpdateOptionsImpl) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, name, cfg, filePath, packageURL, updateOptions)
	}
	return nil
}

func (m *workspaceSourcesClientMock) Delete(context.Context, string) error {
	return nil
}

func (m *workspaceSourcesClientMock) Start(context.Context, string) error {
	return nil
}

func (m *workspaceSourcesClientMock) Stop(context.Context, string) error {
	return nil
}

func (m *workspaceSourcesClientMock) Restart(context.Context, string) error {
	return nil
}

func (m *workspaceSourcesClientMock) StartInstance(context.Context, string, string) error {
	return nil
}

func (m *workspaceSourcesClientMock) StopInstance(context.Context, string, string) error {
	return nil
}

func (m *workspaceSourcesClientMock) RestartInstance(context.Context, string, string) error {
	return nil
}

func (m *workspaceSourcesClientMock) Status(context.Context, string) (*registry.SourceStatus, error) {
	return nil, nil
}

func (m *workspaceSourcesClientMock) InstanceStatus(context.Context, string, string) (*registry.SourceInstanceStatusData, error) {
	return nil, nil
}

func TestBuildSourceConfigUsesInteractiveConnectionSelection(t *testing.T) {
	original := selectConnectionNameForApply
	selectConnectionNameForApply = func(context.Context) (string, error) {
		return "conn-source", nil
	}
	defer func() {
		selectConnectionNameForApply = original
	}()

	cfg, filePath, packageURL, err := buildSourceConfig(context.Background(), &sourceApplyOptions{
		Name:             "src-1",
		SourceType:       "kafka",
		UseConnection:    true,
		LogTopic:         "persistent://public/default/source-logs",
		SNServiceAccount: "svc-source",
	}, false)
	if err != nil {
		t.Fatalf("buildSourceConfig() error = %v", err)
	}
	if cfg.Connection != "conn-source" {
		t.Fatalf("cfg.Connection = %q, want %q", cfg.Connection, "conn-source")
	}
	if cfg.SNServiceAccount != "svc-source" {
		t.Fatalf("cfg.SNServiceAccount = %q, want %q", cfg.SNServiceAccount, "svc-source")
	}
	if cfg.LogTopic != "persistent://public/default/source-logs" {
		t.Fatalf("cfg.LogTopic = %q, want source log topic", cfg.LogTopic)
	}
	if filePath != "" {
		t.Fatalf("filePath = %q, want empty", filePath)
	}
	if packageURL != "builtin://kafka" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://kafka")
	}
}

func TestBuildSourceConfigRejectsConflictingConnectionFlags(t *testing.T) {
	_, _, _, err := buildSourceConfig(context.Background(), &sourceApplyOptions{
		Name:          "src-1",
		SourceType:    "kafka",
		Connection:    "conn-explicit",
		UseConnection: true,
	}, false)
	if err == nil {
		t.Fatal("buildSourceConfig() expected error, got nil")
	}
}

func TestBuildSourceConfigRequiresConnectionOnCreate(t *testing.T) {
	_, _, _, err := buildSourceConfig(context.Background(), &sourceApplyOptions{Name: "src-1", SourceType: "kafka"}, false)
	if err == nil || !strings.Contains(err.Error(), "--connection or --use-connection is required") {
		t.Fatalf("buildSourceConfig() error = %v", err)
	}
}

func TestSourceUpdateOmitsConnectionFlags(t *testing.T) {
	cmd := (&sourcesOptions{}).newUpdateCommand()
	if cmd.Flags().Lookup("connection") != nil || cmd.Flags().Lookup("use-connection") != nil {
		t.Fatal("source update exposes immutable connection flags")
	}
}

func TestBuildSourceConfigSetsSourceType(t *testing.T) {
	cfg, _, _, err := buildSourceConfig(context.Background(), &sourceApplyOptions{
		Name:       "src-1",
		SourceType: "kafka",
		Connection: "conn-1",
	}, false)
	if err != nil {
		t.Fatalf("buildSourceConfig() error = %v", err)
	}
	if cfg.SourceType != "kafka" {
		t.Fatalf("cfg.SourceType = %q", cfg.SourceType)
	}
}

func TestBuildSourceConfigLoadsRegistryFieldsFromFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "source.yaml")
	if err := os.WriteFile(configPath, []byte(`name: src-1
archive: builtin://kafka
connection: conn-source
logTopic: persistent://public/default/source-logs
snServiceAccount: svc-source
`), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, _, _, err := buildSourceConfig(context.Background(), &sourceApplyOptions{SourceConfigFile: configPath}, false)
	if err != nil {
		t.Fatalf("buildSourceConfig() error = %v", err)
	}
	if cfg.LogTopic != "persistent://public/default/source-logs" || cfg.SNServiceAccount != "svc-source" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestBuildSourceUpdateOptions(t *testing.T) {
	if updateOptions := buildSourceUpdateOptions(false, true); updateOptions != nil {
		t.Fatalf("buildSourceUpdateOptions() = %#v, want nil", updateOptions)
	}

	updateOptions := buildSourceUpdateOptions(true, true)
	if updateOptions == nil {
		t.Fatal("buildSourceUpdateOptions() = nil, want non-nil")
	}
	if !updateOptions.UpdateAuthData {
		t.Fatal("updateOptions.UpdateAuthData = false, want true")
	}

	updateOptions = buildSourceUpdateOptions(true, false)
	if updateOptions == nil {
		t.Fatal("buildSourceUpdateOptions() = nil, want non-nil")
	}
	if updateOptions.UpdateAuthData {
		t.Fatal("updateOptions.UpdateAuthData = true, want false")
	}
}

func TestSourceCommandExposesCatalogCommands(t *testing.T) {
	o := &sourcesOptions{}
	cmd := NewCmdSources(&Options{IOStreams: o.ioStreams})

	if _, _, err := cmd.Find([]string{"available-sources"}); err != nil {
		t.Fatalf("available-sources command not found: %v", err)
	}
	if _, _, err := cmd.Find([]string{"config-definition"}); err != nil {
		t.Fatalf("config-definition command not found: %v", err)
	}
}

func TestSourceAvailableSourcesCommandForwardsAndRendersText(t *testing.T) {
	original := newWorkspaceCatalogClient
	defer func() {
		newWorkspaceCatalogClient = original
	}()

	called := false
	newWorkspaceCatalogClient = func() (workspaceCatalogClient, error) {
		return &workspaceCatalogClientMock{
			listSourcesFn: func(context.Context) ([]registry.ConnectorDefinition, error) {
				called = true
				return []registry.ConnectorDefinition{{
					Name:              "kafka",
					Description:       "Kafka source",
					SourceClass:       "org.example.KafkaSource",
					SourceConfigClass: "org.example.KafkaSourceConfig",
				}}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	o := &sourcesOptions{ioStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}}
	cmd := o.newAvailableSourcesCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("ListSources() was not called")
	}
	output := stdout.String()
	for _, expected := range []string{"kafka", "Kafka source", "org.example.KafkaSource", "org.example.KafkaSourceConfig"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output missing %q: %s", expected, output)
		}
	}
}

func TestSourceConfigDefinitionCommandForwardsAndRendersText(t *testing.T) {
	original := newWorkspaceCatalogClient
	defer func() {
		newWorkspaceCatalogClient = original
	}()

	var gotName string
	newWorkspaceCatalogClient = func() (workspaceCatalogClient, error) {
		return &workspaceCatalogClientMock{
			getSourceConfigDefinitionFn: func(_ context.Context, name string) ([]registry.ConfigFieldDefinition, error) {
				gotName = name
				return []registry.ConfigFieldDefinition{{
					FieldName: "bootstrapServers",
					TypeName:  "java.lang.String",
					Attributes: map[string]string{
						"required": "true",
					},
				}}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	o := &sourcesOptions{ioStreams: IOStreams{Out: &stdout, ErrOut: io.Discard}}
	cmd := o.newConfigDefinitionCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"kafka"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotName != "kafka" {
		t.Fatalf("config definition name = %q, want %q", gotName, "kafka")
	}
	output := stdout.String()
	for _, expected := range []string{"bootstrapServers", "java.lang.String", "required=true"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output missing %q: %s", expected, output)
		}
	}
}

func TestSourceUpdateCommandExposesUpdateAuthData(t *testing.T) {
	o := &sourcesOptions{}
	cmd := o.newUpdateCommand()

	if cmd.Flags().Lookup("update-auth-data") == nil {
		t.Fatal("expected --update-auth-data flag to be registered")
	}
}

func TestSourceUpdateCommandForwardsUpdateOptions(t *testing.T) {
	original := newWorkspaceSourcesClient
	defer func() {
		newWorkspaceSourcesClient = original
	}()

	var gotName string
	var gotUpdateOptions *registry.UpdateOptionsImpl

	newWorkspaceSourcesClient = func() (workspaceSourcesClient, error) {
		return &workspaceSourcesClientMock{
			updateFn: func(_ context.Context, name string, cfg registry.RegistrySourceConfig, filePath, packageURL string, updateOptions *registry.UpdateOptionsImpl) error {
				gotName = name
				gotUpdateOptions = updateOptions
				if cfg.Name != "src-1" {
					t.Fatalf("cfg.Name = %q, want %q", cfg.Name, "src-1")
				}
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
	o := &sourcesOptions{
		ioStreams: IOStreams{
			Out:    &stdout,
			ErrOut: io.Discard,
		},
	}

	cmd := o.newUpdateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"src-1", "--update-auth-data"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotName != "src-1" {
		t.Fatalf("update name = %q, want %q", gotName, "src-1")
	}
	if gotUpdateOptions == nil {
		t.Fatal("gotUpdateOptions = nil, want non-nil")
	}
	if !gotUpdateOptions.UpdateAuthData {
		t.Fatal("gotUpdateOptions.UpdateAuthData = false, want true")
	}
}
