// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

func TestBuildFunctionConfigUsesInteractiveConnectionSelection(t *testing.T) {
	original := selectConnectionNameForApply
	selectConnectionNameForApply = func(context.Context) (string, error) {
		return "conn-picked", nil
	}
	defer func() {
		selectConnectionNameForApply = original
	}()

	cfg, filePath, packageURL, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		Name:          "fn-1",
		Jar:           "builtin://my-fn",
		UseConnection: true,
	}, false)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if cfg.Connection != "conn-picked" {
		t.Fatalf("cfg.Connection = %q, want %q", cfg.Connection, "conn-picked")
	}
	if filePath != "" {
		t.Fatalf("filePath = %q, want empty", filePath)
	}
	if packageURL != "builtin://my-fn" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://my-fn")
	}
}

func TestBuildFunctionConfigRejectsConflictingConnectionFlags(t *testing.T) {
	_, _, _, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		Name:          "fn-1",
		Jar:           "builtin://my-fn",
		Connection:    "conn-explicit",
		UseConnection: true,
	}, false)
	if err == nil {
		t.Fatal("buildFunctionConfig() expected error, got nil")
	}
}

func TestBuildFunctionConfigRequiresConnectionOnCreate(t *testing.T) {
	_, _, _, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		Name: "fn-1",
		Jar:  "builtin://word-count",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "--connection or --use-connection is required") {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
}

func TestFunctionUpdateOmitsConnectionFlags(t *testing.T) {
	cmd := (&functionsOptions{}).newUpdateCommand()
	if cmd.Flags().Lookup("connection") != nil || cmd.Flags().Lookup("use-connection") != nil {
		t.Fatal("function update exposes immutable connection flags")
	}
}

func TestBuildFunctionUpdateClearsConfigFileConnection(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "function.yaml")
	if err := os.WriteFile(configPath, []byte("name: fn-1\njar: builtin://word-count\nconnection: conn-old\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, _, _, err := buildFunctionConfig(context.Background(), &functionApplyOptions{FunctionConfigFile: configPath}, true)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if cfg.Connection != "" {
		t.Fatalf("cfg.Connection = %q, want empty", cfg.Connection)
	}
}

func TestLoadFunctionState(t *testing.T) {
	state, err := loadFunctionState(`{"numberValue":7}`, "", "counter")
	if err != nil {
		t.Fatalf("loadFunctionState() error = %v", err)
	}
	if state.Key != "counter" || state.NumberValue == nil || *state.NumberValue != 7 {
		t.Fatalf("state = %#v", state)
	}
	if _, err := loadFunctionState(`{"key":"other"}`, "", "counter"); err == nil {
		t.Fatal("loadFunctionState() expected key mismatch")
	}
	zeroState, err := loadFunctionState(`{"numberValue":0}`, "", "zero")
	if err != nil || zeroState.NumberValue == nil || *zeroState.NumberValue != 0 {
		t.Fatalf("zeroState = %#v, err = %v", zeroState, err)
	}
	payload, err := json.Marshal(zeroState)
	if err != nil || !strings.Contains(string(payload), `"numberValue":0`) {
		t.Fatalf("payload = %s, err = %v", payload, err)
	}
}

func TestFunctionsCommandsIncludeExtendedOperations(t *testing.T) {
	cmd := NewCmdFunctions(&Options{IOStreams: IOStreams{}})
	for _, path := range [][]string{{"stats"}, {"trigger"}, {"state", "get"}, {"state", "put"}} {
		if _, _, err := cmd.Find(path); err != nil {
			t.Fatalf("Find(%v) error = %v", path, err)
		}
	}
}

func TestBuildFunctionConfigLoadsConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "function.yaml")
	content := `tenant: public
namespace: default
name: fn-1
jar: builtin://word-count
connection: conn-file
snServiceAccount: svc-file
parallelism: 2
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, _, packageURL, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		FunctionConfigFile: configPath,
	}, false)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if cfg.Name != "fn-1" {
		t.Fatalf("cfg.Name = %q, want %q", cfg.Name, "fn-1")
	}
	if cfg.Connection != "conn-file" {
		t.Fatalf("cfg.Connection = %q, want %q", cfg.Connection, "conn-file")
	}
	if cfg.SNServiceAccount != "svc-file" {
		t.Fatalf("cfg.SNServiceAccount = %q, want %q", cfg.SNServiceAccount, "svc-file")
	}
	if packageURL != "builtin://word-count" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://word-count")
	}
}

func TestBuildFunctionConfigAppliesSNServiceAccountFlag(t *testing.T) {
	cfg, _, _, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		Name:             "fn-1",
		Jar:              "builtin://word-count",
		Connection:       "conn-1",
		SNServiceAccount: "svc-1",
	}, false)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if cfg.SNServiceAccount != "svc-1" {
		t.Fatalf("cfg.SNServiceAccount = %q, want %q", cfg.SNServiceAccount, "svc-1")
	}
}

func TestBuildFunctionConfigKeepsFalseBoolValuesFromConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "function.yaml")
	content := `tenant: public
namespace: default
name: fn-1
jar: builtin://word-count
connection: conn-1
forwardSourceMessageProperty: false
cleanupSubscription: false
autoAck: false
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, _, packageURL, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		FunctionConfigFile: configPath,
	}, false)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if cfg.ForwardSourceMessageProperty {
		t.Fatal("cfg.ForwardSourceMessageProperty = true, want false")
	}
	if cfg.CleanupSubscription {
		t.Fatal("cfg.CleanupSubscription = true, want false")
	}
	if cfg.AutoAck {
		t.Fatal("cfg.AutoAck = true, want false")
	}
	if packageURL != "builtin://word-count" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://word-count")
	}
}

func TestBuildFunctionConfigAppliesExplicitBoolFlagOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "function.yaml")
	content := `tenant: public
namespace: default
name: fn-1
jar: builtin://word-count
forwardSourceMessageProperty: false
cleanupSubscription: false
autoAck: false
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	opts := newFunctionApplyOptions()

	cmd := &cobra.Command{Use: "create"}
	addFunctionApplyFlags(cmd, opts, true)
	if err := cmd.Flags().Set("connection", "conn-1"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := cmd.Flags().Set("function-config-file", configPath); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := cmd.Flags().Set("forward-source-message-property", "true"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := cmd.Flags().Set("cleanup-subscription", "true"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := cmd.Flags().Set("auto-ack", "true"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	opts.ForwardSourceMessagePropertyChanged = cmd.Flags().Changed("forward-source-message-property")
	opts.CleanupSubscriptionChanged = cmd.Flags().Changed("cleanup-subscription")
	opts.AutoAckChanged = cmd.Flags().Changed("auto-ack")

	cfg, _, packageURL, err := buildFunctionConfig(context.Background(), opts, false)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if !cfg.ForwardSourceMessageProperty {
		t.Fatal("cfg.ForwardSourceMessageProperty = false, want true")
	}
	if !cfg.CleanupSubscription {
		t.Fatal("cfg.CleanupSubscription = false, want true")
	}
	if !cfg.AutoAck {
		t.Fatal("cfg.AutoAck = false, want true")
	}
	if packageURL != "builtin://word-count" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://word-count")
	}
}

func TestBuildFunctionConfigUsesDefaultTrueBoolValuesWithoutConfigFile(t *testing.T) {
	opts := newFunctionApplyOptions()
	opts.Name = "fn-1"
	opts.Jar = "builtin://word-count"
	opts.Connection = "conn-1"

	cfg, _, packageURL, err := buildFunctionConfig(context.Background(), opts, false)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if !cfg.ForwardSourceMessageProperty {
		t.Fatal("cfg.ForwardSourceMessageProperty = false, want true")
	}
	if !cfg.CleanupSubscription {
		t.Fatal("cfg.CleanupSubscription = false, want true")
	}
	if !cfg.AutoAck {
		t.Fatal("cfg.AutoAck = false, want true")
	}
	if packageURL != "builtin://word-count" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://word-count")
	}
}

func TestBuildFunctionConfigAutoAckFlagCanDisableConfigFileValue(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "function.yaml")
	content := `tenant: public
namespace: default
name: fn-1
jar: builtin://word-count
connection: conn-1
autoAck: true
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	opts := newFunctionApplyOptions()
	opts.Connection = "conn-1"

	cmd := &cobra.Command{Use: "create"}
	addFunctionApplyFlags(cmd, opts, true)
	if err := cmd.Flags().Set("function-config-file", configPath); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := cmd.Flags().Set("auto-ack", "false"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	opts.AutoAckChanged = cmd.Flags().Changed("auto-ack")

	cfg, _, _, err := buildFunctionConfig(context.Background(), opts, false)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if cfg.AutoAck {
		t.Fatal("cfg.AutoAck = true, want false")
	}
}

func TestBuildFunctionConfigAppliesWindowFlags(t *testing.T) {
	cfg, _, packageURL, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		Name:                      "fn-1",
		Jar:                       "builtin://word-count",
		Connection:                "conn-1",
		WindowLengthCount:         10,
		WindowLengthDurationMs:    1000,
		SlidingIntervalCount:      3,
		SlidingIntervalDurationMs: 2000,
	}, false)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if cfg.WindowConfig == nil {
		t.Fatal("cfg.WindowConfig = nil, want non-nil")
	}
	if cfg.WindowConfig.WindowLengthCount == nil || *cfg.WindowConfig.WindowLengthCount != 10 {
		t.Fatalf("WindowLengthCount = %v, want 10", cfg.WindowConfig.WindowLengthCount)
	}
	if cfg.WindowConfig.WindowLengthDurationMs == nil || *cfg.WindowConfig.WindowLengthDurationMs != 1000 {
		t.Fatalf("WindowLengthDurationMs = %v, want 1000", cfg.WindowConfig.WindowLengthDurationMs)
	}
	if cfg.WindowConfig.SlidingIntervalCount == nil || *cfg.WindowConfig.SlidingIntervalCount != 3 {
		t.Fatalf("SlidingIntervalCount = %v, want 3", cfg.WindowConfig.SlidingIntervalCount)
	}
	if cfg.WindowConfig.SlidingIntervalDurationMs == nil || *cfg.WindowConfig.SlidingIntervalDurationMs != 2000 {
		t.Fatalf("SlidingIntervalDurationMs = %v, want 2000", cfg.WindowConfig.SlidingIntervalDurationMs)
	}
	if packageURL != "builtin://word-count" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://word-count")
	}
}

func TestBuildFunctionUpdateOptions(t *testing.T) {
	if updateOptions := buildFunctionUpdateOptions(false, true); updateOptions != nil {
		t.Fatalf("buildFunctionUpdateOptions() = %#v, want nil", updateOptions)
	}

	updateOptions := buildFunctionUpdateOptions(true, true)
	if updateOptions == nil {
		t.Fatal("buildFunctionUpdateOptions() = nil, want non-nil")
	}
	if !updateOptions.UpdateAuthData {
		t.Fatal("updateOptions.UpdateAuthData = false, want true")
	}

	updateOptions = buildFunctionUpdateOptions(true, false)
	if updateOptions == nil {
		t.Fatal("buildFunctionUpdateOptions() = nil, want non-nil")
	}
	if updateOptions.UpdateAuthData {
		t.Fatal("updateOptions.UpdateAuthData = true, want false")
	}
}

func TestBuildFunctionConfigRejectsConflictingPackageFlags(t *testing.T) {
	testCases := []struct {
		name string
		opts functionApplyOptions
	}{
		{
			name: "function type and jar",
			opts: functionApplyOptions{
				Name:         "fn-1",
				FunctionType: "word-count",
				Jar:          "file:///tmp/function.jar",
			},
		},
		{
			name: "function type and py",
			opts: functionApplyOptions{
				Name:         "fn-1",
				FunctionType: "word-count",
				Py:           "file:///tmp/function.py",
			},
		},
		{
			name: "jar and go",
			opts: functionApplyOptions{
				Name: "fn-1",
				Jar:  "file:///tmp/function.jar",
				Go:   "file:///tmp/function-go",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := buildFunctionConfig(context.Background(), &tc.opts, false)
			if err == nil {
				t.Fatal("buildFunctionConfig() expected error, got nil")
			}
			if !strings.Contains(err.Error(), "mutually exclusive") {
				t.Fatalf("buildFunctionConfig() error = %v, want mutually exclusive message", err)
			}
		})
	}
}

func TestBuildFunctionConfigRejectsConflictingPackageSelectorFromConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "function.yaml")
	content := `tenant: public
namespace: default
name: fn-1
jar: builtin://word-count
connection: conn-1
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, _, _, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		FunctionConfigFile: configPath,
		FunctionType:       "word-count",
	}, false)
	if err == nil {
		t.Fatal("buildFunctionConfig() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("buildFunctionConfig() error = %v, want mutually exclusive message", err)
	}
}

func TestBuildFunctionConfigRejectsConfigFileWithMultiplePackageFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "function.yaml")
	content := `tenant: public
namespace: default
name: fn-1
jar: builtin://word-count
py: file:///tmp/function.py
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, _, _, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		FunctionConfigFile: configPath,
	}, false)
	if err == nil {
		t.Fatal("buildFunctionConfig() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("buildFunctionConfig() error = %v, want mutually exclusive message", err)
	}
}

func TestBuildFunctionConfigAllowsJarFlagToOverrideConfigFileJar(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "function.yaml")
	content := `tenant: public
namespace: default
name: fn-1
jar: builtin://word-count
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, _, packageURL, err := buildFunctionConfig(context.Background(), &functionApplyOptions{
		FunctionConfigFile: configPath,
		Jar:                "builtin://override-word-count",
		Connection:         "conn-1",
	}, false)
	if err != nil {
		t.Fatalf("buildFunctionConfig() error = %v", err)
	}
	if cfg.Jar == nil || *cfg.Jar != "builtin://override-word-count" {
		t.Fatalf("cfg.Jar = %v, want builtin://override-word-count", cfg.Jar)
	}
	if packageURL != "builtin://override-word-count" {
		t.Fatalf("packageURL = %q, want %q", packageURL, "builtin://override-word-count")
	}
}

func TestValidateFunctionPackageFieldsRejectsMultiplePackageFields(t *testing.T) {
	jar := "builtin://word-count"
	py := "file:///tmp/function.py"
	cfg := registry.RegistryFunctionConfig{}
	cfg.Jar = &jar
	cfg.Py = &py

	err := validateFunctionPackageFields(cfg)
	if err == nil {
		t.Fatal("validateFunctionPackageFields() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple package fields") {
		t.Fatalf("validateFunctionPackageFields() error = %v, want multiple package fields message", err)
	}
}
