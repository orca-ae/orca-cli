// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

import (
	"os"
	"strings"
	"testing"
)

func TestManagedAgentsAPIKeyIncludesTriggerScopes(t *testing.T) {
	data, err := os.ReadFile("seed-managed-agents-api-key.sh")
	if err != nil {
		t.Fatalf("read API key seed script: %v", err)
	}

	for _, scope := range []string{
		"workspace.agentTriggers.create",
		"workspace.agentTriggers.alter",
		"workspace.agentTriggers.describe",
		"workspace.agentTriggers.delete",
	} {
		if !strings.Contains(string(data), "'"+scope+"'") {
			t.Errorf("API key seed script is missing %q", scope)
		}
	}
}
