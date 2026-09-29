// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

type workspaceConnectionsClientMock struct {
	listFn     func(ctx context.Context) ([]registry.ConnectionConfig, error)
	getFn      func(ctx context.Context, name string) (*registry.ConnectionConfig, error)
	createFn   func(ctx context.Context, cfg registry.ConnectionConfig) error
	validateFn func(ctx context.Context, cfg registry.ConnectionConfig) error
	updateFn   func(ctx context.Context, name string, cfg registry.ConnectionConfig) error
	deleteFn   func(ctx context.Context, name string) error
	testFn     func(ctx context.Context, name string) (*registry.ConnectionHealthStatus, error)
}

func (m *workspaceConnectionsClientMock) Validate(ctx context.Context, cfg registry.ConnectionConfig) error {
	if m.validateFn != nil {
		return m.validateFn(ctx, cfg)
	}
	return nil
}

func (m *workspaceConnectionsClientMock) List(ctx context.Context) ([]registry.ConnectionConfig, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return nil, nil
}

func (m *workspaceConnectionsClientMock) Get(ctx context.Context, name string) (*registry.ConnectionConfig, error) {
	if m.getFn != nil {
		return m.getFn(ctx, name)
	}
	return nil, nil
}

func (m *workspaceConnectionsClientMock) Create(ctx context.Context, cfg registry.ConnectionConfig) error {
	if m.createFn != nil {
		return m.createFn(ctx, cfg)
	}
	return nil
}

func (m *workspaceConnectionsClientMock) Update(ctx context.Context, name string, cfg registry.ConnectionConfig) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, name, cfg)
	}
	return nil
}

func (m *workspaceConnectionsClientMock) Delete(ctx context.Context, name string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, name)
	}
	return nil
}

func (m *workspaceConnectionsClientMock) Test(ctx context.Context, name string) (*registry.ConnectionHealthStatus, error) {
	if m.testFn != nil {
		return m.testFn(ctx, name)
	}
	return nil, nil
}

func TestBuildCreatePayloadKafka(t *testing.T) {
	t.Parallel()

	options, cmd := newTestConnectionMutationCommand(connectionMutationModeCreate)
	if err := cmd.ParseFlags([]string{
		"--name", "conn-1",
		"--type", "kafka",
		"--kafka-bootstrap-servers", "broker:9092",
		"--kafka-auth-type", "oauth2",
		"--kafka-oauth2-secret-name", "auth-secret",
		"--kafka-oauth2-secret-key", "credentials.json",
	}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	payload, err := options.buildCreatePayload(cmd)
	if err != nil {
		t.Fatalf("buildCreatePayload() error = %v", err)
	}

	if payload.Name != "conn-1" {
		t.Fatalf("payload.Name = %q, want %q", payload.Name, "conn-1")
	}
	if payload.Spec.Type != registry.ConnectionTypeKafka {
		t.Fatalf("payload.Spec.Type = %q, want %q", payload.Spec.Type, registry.ConnectionTypeKafka)
	}
	if payload.Spec.Kafka == nil || payload.Spec.Kafka.BootstrapServers != "broker:9092" {
		t.Fatalf("payload.Spec.Kafka.BootstrapServers = %#v, want %q", payload.Spec.Kafka, "broker:9092")
	}

	auth, ok := payload.Spec.Kafka.Authentication["oauth2Config"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload.Spec.Kafka.Authentication[oauth2Config] = %#v", payload.Spec.Kafka.Authentication["oauth2Config"])
	}
	if auth["keySecretName"] != "auth-secret" {
		t.Fatalf("oauth2Config.keySecretName = %#v, want %q", auth["keySecretName"], "auth-secret")
	}
	if auth["keySecretKey"] != "credentials.json" {
		t.Fatalf("oauth2Config.keySecretKey = %#v, want %q", auth["keySecretKey"], "credentials.json")
	}
}

func TestBuildCreatePayloadRejectsInvalidOtherProperty(t *testing.T) {
	t.Parallel()

	options, cmd := newTestConnectionMutationCommand(connectionMutationModeCreate)
	if err := cmd.ParseFlags([]string{
		"--name", "conn-1",
		"--type", "other",
		"--other-endpoint", "https://example.com",
		"--other-property", "invalid",
	}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	_, err := options.buildCreatePayload(cmd)
	if err == nil {
		t.Fatal("buildCreatePayload() error = nil, want non-nil")
	}
}

func TestCreateCommandValidatesPayloadBeforeInitializingClient(t *testing.T) {
	original := newWorkspaceConnectionsClient
	newWorkspaceConnectionsClient = func() (workspaceConnectionsClient, error) {
		t.Fatal("newWorkspaceConnectionsClient() should not be called when payload validation fails")
		return &workspaceConnectionsClientMock{}, nil
	}
	defer func() {
		newWorkspaceConnectionsClient = original
	}()

	ioStreams := IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard}
	options := &connectionsOptions{ioStreams: ioStreams}

	cmd := options.newCreateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"--name", "conn-1",
		"--type", "other",
		"--other-endpoint", "https://example.com",
		"--other-property", "invalid",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--other-property entries must use key=value format") {
		t.Fatalf("Execute() error = %v, want other-property validation error", err)
	}
}

func TestUpdateCommandValidatesCommonFlagsBeforeInitializingClient(t *testing.T) {
	original := newWorkspaceConnectionsClient
	newWorkspaceConnectionsClient = func() (workspaceConnectionsClient, error) {
		t.Fatal("newWorkspaceConnectionsClient() should not be called when common validation fails")
		return &workspaceConnectionsClientMock{}, nil
	}
	defer func() {
		newWorkspaceConnectionsClient = original
	}()

	ioStreams := IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard}
	options := &connectionsOptions{ioStreams: ioStreams}

	cmd := options.newUpdateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"--name", "conn-1",
		"--output", "invalid",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--output must be one of: text, json, yaml") {
		t.Fatalf("Execute() error = %v, want output validation error", err)
	}
}

func TestBuildUpdatedPayloadPatchesExistingPulsar(t *testing.T) {
	t.Parallel()

	options, cmd := newTestConnectionMutationCommand(connectionMutationModeUpdate)
	if err := cmd.ParseFlags([]string{
		"--name", "conn-1",
		"--pulsar-admin-url", "http://new-admin:8080",
	}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	existing := registry.ConnectionConfig{
		Name: "conn-1",
		Spec: registry.ConnectionSpec{
			Type: registry.ConnectionTypePulsar,
			Pulsar: &registry.PulsarConnectionConfig{
				ServiceURL: "pulsar://broker:6650",
				AdminURL:   "http://old-admin:8080",
				Authentication: &registry.PulsarAuthConfig{
					Token: &registry.SecretKeyRef{Name: "token-secret", Key: "token"},
				},
			},
		},
	}

	payload, err := options.buildUpdatedPayload(cmd, existing)
	if err != nil {
		t.Fatalf("buildUpdatedPayload() error = %v", err)
	}

	if payload.Spec.Pulsar == nil {
		t.Fatal("payload.Spec.Pulsar = nil, want non-nil")
	}
	if payload.Spec.Pulsar.ServiceURL != "pulsar://broker:6650" {
		t.Fatalf("payload.Spec.Pulsar.ServiceURL = %q, want %q", payload.Spec.Pulsar.ServiceURL, "pulsar://broker:6650")
	}
	if payload.Spec.Pulsar.AdminURL != "http://new-admin:8080" {
		t.Fatalf("payload.Spec.Pulsar.AdminURL = %q, want %q", payload.Spec.Pulsar.AdminURL, "http://new-admin:8080")
	}
	if payload.Spec.Pulsar.Authentication == nil || payload.Spec.Pulsar.Authentication.Token == nil {
		t.Fatalf("payload.Spec.Pulsar.Authentication = %#v, want token auth", payload.Spec.Pulsar.Authentication)
	}
}

func TestBuildUpdatedPayloadPatchesExistingPulsarFromUppercaseTransportType(t *testing.T) {
	t.Parallel()

	options, cmd := newTestConnectionMutationCommand(connectionMutationModeUpdate)
	if err := cmd.ParseFlags([]string{
		"--name", "conn-1",
		"--pulsar-admin-url", "http://new-admin:8080",
	}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	var existing registry.ConnectionConfig
	if err := json.Unmarshal([]byte(`{
		"name": "conn-1",
		"spec": {
			"type": "PULSAR",
			"pulsar": {
				"serviceUrl": "pulsar://broker:6650",
				"adminUrl": "http://old-admin:8080"
			}
		}
	}`), &existing); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	payload, err := options.buildUpdatedPayload(cmd, existing)
	if err != nil {
		t.Fatalf("buildUpdatedPayload() error = %v", err)
	}

	if payload.Spec.Type != registry.ConnectionTypePulsar {
		t.Fatalf("payload.Spec.Type = %q, want %q", payload.Spec.Type, registry.ConnectionTypePulsar)
	}
	if payload.Spec.Pulsar == nil {
		t.Fatal("payload.Spec.Pulsar = nil, want non-nil")
	}
	if payload.Spec.Pulsar.ServiceURL != "pulsar://broker:6650" {
		t.Fatalf("payload.Spec.Pulsar.ServiceURL = %q, want %q", payload.Spec.Pulsar.ServiceURL, "pulsar://broker:6650")
	}
	if payload.Spec.Pulsar.AdminURL != "http://new-admin:8080" {
		t.Fatalf("payload.Spec.Pulsar.AdminURL = %q, want %q", payload.Spec.Pulsar.AdminURL, "http://new-admin:8080")
	}
}

func TestBuildUpdatedPayloadTypeChangeResetsPreviousType(t *testing.T) {
	t.Parallel()

	options, cmd := newTestConnectionMutationCommand(connectionMutationModeUpdate)
	if err := cmd.ParseFlags([]string{
		"--name", "conn-1",
		"--type", "kafka",
		"--kafka-bootstrap-servers", "broker:9092",
	}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	existing := registry.ConnectionConfig{
		Name: "conn-1",
		Spec: registry.ConnectionSpec{
			Type: registry.ConnectionTypePulsar,
			Pulsar: &registry.PulsarConnectionConfig{
				ServiceURL: "pulsar://broker:6650",
			},
		},
	}

	payload, err := options.buildUpdatedPayload(cmd, existing)
	if err != nil {
		t.Fatalf("buildUpdatedPayload() error = %v", err)
	}

	if payload.Spec.Type != registry.ConnectionTypeKafka {
		t.Fatalf("payload.Spec.Type = %q, want %q", payload.Spec.Type, registry.ConnectionTypeKafka)
	}
	if payload.Spec.Pulsar != nil {
		t.Fatalf("payload.Spec.Pulsar = %#v, want nil", payload.Spec.Pulsar)
	}
	if payload.Spec.Kafka == nil || payload.Spec.Kafka.BootstrapServers != "broker:9092" {
		t.Fatalf("payload.Spec.Kafka = %#v, want bootstrapServers broker:9092", payload.Spec.Kafka)
	}
}

func TestBuildUpdatedPayloadClearsKafkaAuthAndTLS(t *testing.T) {
	t.Parallel()

	options, cmd := newTestConnectionMutationCommand(connectionMutationModeUpdate)
	if err := cmd.ParseFlags([]string{
		"--name", "conn-1",
		"--kafka-auth-type", "none",
		"--kafka-tls-enabled=false",
	}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	existing := registry.ConnectionConfig{
		Name: "conn-1",
		Spec: registry.ConnectionSpec{
			Type: registry.ConnectionTypeKafka,
			Kafka: &registry.KafkaConnectionConfig{
				BootstrapServers: "broker:9092",
				Authentication: map[string]interface{}{
					"genericAuth": map[string]interface{}{
						"clientAuthenticationPlugin":     "plugin",
						"clientAuthenticationParameters": "params",
					},
				},
				TLS: map[string]interface{}{
					"enabled": true,
				},
			},
		},
	}

	payload, err := options.buildUpdatedPayload(cmd, existing)
	if err != nil {
		t.Fatalf("buildUpdatedPayload() error = %v", err)
	}

	if payload.Spec.Kafka == nil {
		t.Fatal("payload.Spec.Kafka = nil, want non-nil")
	}
	if payload.Spec.Kafka.Authentication != nil {
		t.Fatalf("payload.Spec.Kafka.Authentication = %#v, want nil", payload.Spec.Kafka.Authentication)
	}
	if payload.Spec.Kafka.TLS != nil {
		t.Fatalf("payload.Spec.Kafka.TLS = %#v, want nil", payload.Spec.Kafka.TLS)
	}
	if payload.Spec.Kafka.BootstrapServers != "broker:9092" {
		t.Fatalf("payload.Spec.Kafka.BootstrapServers = %q, want %q", payload.Spec.Kafka.BootstrapServers, "broker:9092")
	}
}

func TestBuildUpdatedPayloadRejectsKafkaTLSSubFlagsWhenTLSDisabled(t *testing.T) {
	t.Parallel()

	options, cmd := newTestConnectionMutationCommand(connectionMutationModeUpdate)
	if err := cmd.ParseFlags([]string{
		"--name", "conn-1",
		"--kafka-tls-enabled=false",
		"--kafka-tls-trust-secret-name", "trust-secret",
	}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	existing := registry.ConnectionConfig{
		Name: "conn-1",
		Spec: registry.ConnectionSpec{
			Type: registry.ConnectionTypeKafka,
			Kafka: &registry.KafkaConnectionConfig{
				BootstrapServers: "broker:9092",
			},
		},
	}

	_, err := options.buildUpdatedPayload(cmd, existing)
	if err == nil {
		t.Fatal("buildUpdatedPayload() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "Kafka TLS sub-flags cannot be used with --kafka-tls-enabled=false") {
		t.Fatalf("buildUpdatedPayload() error = %v, want Kafka TLS disabled validation error", err)
	}
}

func TestBuildUpdatedPayloadMergesOtherProperties(t *testing.T) {
	t.Parallel()

	options, cmd := newTestConnectionMutationCommand(connectionMutationModeUpdate)
	if err := cmd.ParseFlags([]string{
		"--name", "conn-1",
		"--other-property", "region=eu-west-1",
		"--other-property", "team=platform",
	}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	existing := registry.ConnectionConfig{
		Name: "conn-1",
		Spec: registry.ConnectionSpec{
			Type: registry.ConnectionTypeOther,
			Other: &registry.OtherConnectionConfig{
				Endpoint: "https://example.com",
				Properties: map[string]string{
					"env":    "prod",
					"region": "us-east-1",
				},
			},
		},
	}

	payload, err := options.buildUpdatedPayload(cmd, existing)
	if err != nil {
		t.Fatalf("buildUpdatedPayload() error = %v", err)
	}

	if payload.Spec.Other == nil {
		t.Fatal("payload.Spec.Other = nil, want non-nil")
	}
	if payload.Spec.Other.Endpoint != "https://example.com" {
		t.Fatalf("payload.Spec.Other.Endpoint = %q, want %q", payload.Spec.Other.Endpoint, "https://example.com")
	}
	expected := map[string]string{
		"env":    "prod",
		"region": "eu-west-1",
		"team":   "platform",
	}
	if len(payload.Spec.Other.Properties) != len(expected) {
		t.Fatalf("payload.Spec.Other.Properties len = %d, want %d", len(payload.Spec.Other.Properties), len(expected))
	}
	for key, want := range expected {
		if got := payload.Spec.Other.Properties[key]; got != want {
			t.Fatalf("payload.Spec.Other.Properties[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestCreateAndUpdateCommandsDoNotExposeFileFlag(t *testing.T) {
	t.Parallel()

	ioStreams := IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard}
	options := &connectionsOptions{ioStreams: ioStreams}

	createCmd := options.newCreateCommand()
	updateCmd := options.newUpdateCommand()

	if createCmd.Flags().Lookup("file") != nil {
		t.Fatal("create command still exposes --file")
	}
	if updateCmd.Flags().Lookup("file") != nil {
		t.Fatal("update command still exposes --file")
	}
	if createCmd.Flags().Lookup("name") == nil || createCmd.Flags().Lookup("type") == nil {
		t.Fatal("create command is missing --name or --type")
	}
	if updateCmd.Flags().Lookup("name") == nil {
		t.Fatal("update command is missing --name")
	}
}

func TestRenderConnectionsListText(t *testing.T) {
	t.Parallel()

	var builder strings.Builder
	err := renderConnectionsList(&builder, "text", []registry.ConnectionConfig{
		{
			Name: "conn-1",
			Spec: registry.ConnectionSpec{Type: registry.ConnectionTypePulsar},
			Status: &registry.ConnectionStatus{
				Phase: registry.ConnectionPhaseHealthy,
			},
		},
	})
	if err != nil {
		t.Fatalf("renderConnectionsList() error = %v", err)
	}

	rendered := builder.String()
	for _, expected := range []string{"conn-1", "pulsar", "Healthy"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered output missing %q: %s", expected, rendered)
		}
	}
}

func TestRenderConnectionJSONUsesTransportShape(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	err := renderConnection(&output, "json", registry.ConnectionConfig{
		ClusterRef: "cluster-1",
		Internal:   true,
		Name:       "conn-1",
		Spec:       registry.ConnectionSpec{Type: registry.ConnectionTypeKafka},
	})
	if err != nil {
		t.Fatalf("renderConnection() error = %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(output.String()), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["name"] != "conn-1" || decoded["clusterRef"] != "cluster-1" || decoded["internal"] != true {
		t.Fatalf("decoded = %#v", decoded)
	}
	if _, exists := decoded["apiVersion"]; exists {
		t.Fatalf("unexpected CR-shaped output: %#v", decoded)
	}
}

func TestConnectionsValidateCommandForwardsPayload(t *testing.T) {
	original := newWorkspaceConnectionsClient
	defer func() { newWorkspaceConnectionsClient = original }()

	var got registry.ConnectionConfig
	newWorkspaceConnectionsClient = func() (workspaceConnectionsClient, error) {
		return &workspaceConnectionsClientMock{
			validateFn: func(_ context.Context, cfg registry.ConnectionConfig) error {
				got = cfg
				return nil
			},
		}, nil
	}

	var output strings.Builder
	o := &connectionsOptions{ioStreams: IOStreams{Out: &output, ErrOut: io.Discard}}
	cmd := o.newValidateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--name", "conn-1", "--type", "other", "--other-endpoint", "https://example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Name != "conn-1" || got.Spec.Type != registry.ConnectionTypeOther || got.Spec.Other == nil || got.Spec.Other.Endpoint != "https://example.com" {
		t.Fatalf("got = %#v", got)
	}
}

func newTestConnectionMutationCommand(mode connectionMutationMode) (*connectionMutationOptions, *cobra.Command) {
	options := newConnectionMutationOptions(mode)
	cmd := &cobra.Command{Use: "test"}
	options.addFlags(cmd)
	return options, cmd
}
