// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	registry "github.com/orca-ae/orca-sdk-go"
)

func TestNormalizeKafkaConnectorFile(t *testing.T) {
	cfg := &kafkaConnectorConfigFile{
		Name: "connector-1",
		Config: map[string]interface{}{
			"connector.class": "org.example.Connector",
			"tasks.max":       1,
		},
	}

	name, normalized, err := normalizeKafkaConnectorFile(cfg)
	if err != nil {
		t.Fatalf("normalizeKafkaConnectorFile() error = %v", err)
	}
	if name != "connector-1" {
		t.Fatalf("name = %q, want %q", name, "connector-1")
	}
	if normalized["tasks.max"] != "1" {
		t.Fatalf("tasks.max = %q, want %q", normalized["tasks.max"], "1")
	}
}

func TestNormalizeKafkaConnectorFileRejectsNonScalarConfig(t *testing.T) {
	_, _, err := normalizeKafkaConnectorFile(&kafkaConnectorConfigFile{
		Name: "connector-1",
		Config: map[string]interface{}{
			"connector.class": "org.example.Connector",
			"nested":          map[string]interface{}{"key": "value"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "must be a scalar") {
		t.Fatalf("normalizeKafkaConnectorFile() error = %v", err)
	}
}

func TestValidateKafkaConnectConnectorNameMatchesRuntime(t *testing.T) {
	if err := validateKafkaConnectConnectorName("connector.with.dots"); err != nil {
		t.Fatalf("validateKafkaConnectConnectorName() error = %v", err)
	}
	if err := validateKafkaConnectConnectorName("123456789012345678901234567890"); err == nil {
		t.Fatal("validateKafkaConnectConnectorName() expected length error")
	}
}

func TestPrepareKafkaConnectConfigForApplyUsesInteractiveSelections(t *testing.T) {
	original := selectConnectionNameForApply
	callCount := 0
	selectConnectionNameForApply = func(context.Context) (string, error) {
		callCount++
		if callCount == 1 {
			return "kafka-conn", nil
		}
		return "pulsar-conn", nil
	}
	defer func() {
		selectConnectionNameForApply = original
	}()

	resolved, err := resolveKafkaConnectConnectionFlags(context.Background(), kafkaConnectConnectionFlags{
		UseConnection: true,
	})
	if err != nil {
		t.Fatalf("resolveKafkaConnectConnectionFlags() error = %v", err)
	}

	result, err := prepareKafkaConnectConfigForApply(
		map[string]string{
			"connector.class":                     "org.example.Connector",
			"tasks.max":                           "1",
			kafkaConnectConfigKeyPulsarPackageURL: "function://public/default/pkg@v1",
		},
		nil,
		false,
		resolved,
	)
	if err != nil {
		t.Fatalf("prepareKafkaConnectConfigForApply() error = %v", err)
	}
	if result[kafkaConnectConfigKeyConnection] != "kafka-conn" {
		t.Fatalf("%s = %q, want %q", kafkaConnectConfigKeyConnection, result[kafkaConnectConfigKeyConnection], "kafka-conn")
	}
}

func TestPrepareKafkaConnectConfigForApplyRejectsConnectionChange(t *testing.T) {
	_, err := prepareKafkaConnectConfigForApply(
		map[string]string{
			"connector.class":               "org.example.Connector",
			"tasks.max":                     "1",
			kafkaConnectConfigKeyConnection: "new-conn",
		},
		map[string]string{
			kafkaConnectConfigKeyConnection: "current-conn",
		},
		true,
		kafkaConnectConnectionFlags{},
	)
	if err == nil {
		t.Fatal("prepareKafkaConnectConfigForApply() expected error, got nil")
	}
}

func TestKafkaConnectConfigPatchCommandIsExplicitlyUnsupported(t *testing.T) {
	cmd := (&kafkaConnectOptions{}).newPatchConnectorCmd()
	cmd.SetArgs([]string{"connector-1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not implemented by current Java runtime") {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestLoadKafkaConnectorConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "connector.yaml")
	content := `name: connector-1
initial_state: RUNNING
config:
  connector.class: org.example.Connector
  tasks.max: "1"
  sn.connection: conn-1
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := loadKafkaConnectorConfigFile(configPath)
	if err != nil {
		t.Fatalf("loadKafkaConnectorConfigFile() error = %v", err)
	}
	if cfg.Name != "connector-1" {
		t.Fatalf("cfg.Name = %q, want %q", cfg.Name, "connector-1")
	}
	if cfg.InitialState != "RUNNING" {
		t.Fatalf("cfg.InitialState = %q, want %q", cfg.InitialState, "RUNNING")
	}
}

func TestWorkspaceRootRegistersKafkaConnect(t *testing.T) {
	streams := IOStreams{}
	root := NewGroupCommand(&Options{IOStreams: streams})
	cmd, _, err := root.Find([]string{"kafka-connect"})
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if cmd == nil || cmd.Name() != "kafka-connect" {
		t.Fatalf("expected kafka-connect command to be registered")
	}

	registered := 0
	for _, child := range root.Commands() {
		if child.Name() == "kafka-connect" {
			registered++
		}
	}
	if registered != 1 {
		t.Fatalf("expected kafka-connect to be registered once, got %d", registered)
	}
}

func TestKafkaConnectCommandsCoverDocumentedOperations(t *testing.T) {
	cmd := NewCmdKafkaConnect(&Options{IOStreams: IOStreams{}})
	for _, path := range [][]string{
		{"health"},
		{"available-connectors"},
		{"config-definition"},
		{"get", "config"},
		{"get", "status"},
		{"get", "tasks"},
		{"get", "task-status"},
		{"get", "tasks-config"},
		{"get", "topics"},
		{"get", "plugin-catalog"},
		{"reset", "topics"},
	} {
		if _, _, err := cmd.Find(path); err != nil {
			t.Fatalf("Find(%v) error = %v", path, err)
		}
	}
}

func TestKafkaConnectPatchOffsetsRejectsPartialFlagSetsBeforeRuntimeResolution(t *testing.T) {
	cmd := (&kafkaConnectOptions{}).newPatchOffsetsCmd()
	cmd.SetArgs([]string{"connector-1", "--kafka-topic", "orders"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "must be specified together") {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestRenderJSONOrTextSupportsArrayPayloads(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := renderJSONOrText(&buf, "text", []map[string]interface{}{
		{
			"name": "quickstart",
			"type": "STRING",
		},
	})
	if err != nil {
		t.Fatalf("renderJSONOrText() error = %v", err)
	}
	if !strings.Contains(buf.String(), "\"quickstart\"") {
		t.Fatalf("rendered output = %q, want to contain %q", buf.String(), "\"quickstart\"")
	}
}

func TestRenderKafkaConnectOffsetsText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := renderKafkaConnectOffsets(&buf, "text", "my-connector", &registry.ConnectorOffsets{
		Offsets: []map[string]interface{}{
			{
				"partition": map[string]interface{}{
					"kafka_topic":     "topic-a",
					"kafka_partition": 1,
				},
				"offset": map[string]interface{}{
					"kafka_offset": 12,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("renderKafkaConnectOffsets() error = %v", err)
	}

	rendered := buf.String()
	if !strings.Contains(rendered, "CONNECTOR OFFSETS: my-connector") {
		t.Fatalf("rendered output = %q, want connector header", rendered)
	}
	if !strings.Contains(rendered, "\"kafka_topic\": \"topic-a\"") {
		t.Fatalf("rendered output = %q, want topic payload", rendered)
	}
	if !strings.Contains(rendered, "\"kafka_offset\": 12") {
		t.Fatalf("rendered output = %q, want offset payload", rendered)
	}
}

func TestRenderKafkaConnectOffsetsTextEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := renderKafkaConnectOffsets(&buf, "text", "my-connector", &registry.ConnectorOffsets{})
	if err != nil {
		t.Fatalf("renderKafkaConnectOffsets() error = %v", err)
	}
	if !strings.Contains(buf.String(), "No offsets available") {
		t.Fatalf("rendered output = %q, want empty offsets message", buf.String())
	}
}
