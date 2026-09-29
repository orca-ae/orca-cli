// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"strings"
)

const (
	managedAgentEventUserMessage      = "user.message"
	managedAgentEventDefineOutcome    = "user.define_outcome"
	managedAgentEventToolConfirmation = "user.tool_confirmation"
)

func buildSessionMessageEventPayload(text string) (map[string]interface{}, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("--text is required")
	}
	return buildManagedAgentTypedEventsPayload([]map[string]interface{}{{
		"type": managedAgentEventUserMessage,
		"content": []map[string]interface{}{{
			"type": "text",
			"text": text,
		}},
	}}), nil
}

func buildSessionOutcomeEventPayload(description, rubric string, maxIterations int) (map[string]interface{}, error) {
	description = strings.TrimSpace(description)
	rubric = strings.TrimSpace(rubric)
	if description == "" {
		return nil, fmt.Errorf("--description is required")
	}
	if rubric == "" {
		return nil, fmt.Errorf("--rubric is required")
	}
	if maxIterations < 1 || maxIterations > 20 {
		return nil, fmt.Errorf("--max-iterations must be between 1 and 20")
	}
	return buildManagedAgentTypedEventsPayload([]map[string]interface{}{{
		"type":           managedAgentEventDefineOutcome,
		"description":    description,
		"rubric":         rubric,
		"max_iterations": maxIterations,
	}}), nil
}

func buildSessionToolConfirmationEventPayload(toolUseID, decision, denyMessage string) (map[string]interface{}, error) {
	toolUseID = strings.TrimSpace(toolUseID)
	decision = strings.TrimSpace(strings.ToLower(decision))
	if toolUseID == "" {
		return nil, fmt.Errorf("--tool-use-id is required")
	}
	if decision != "allow" && decision != "deny" {
		return nil, fmt.Errorf("--decision must be allow or deny")
	}
	event := map[string]interface{}{
		"type":        managedAgentEventToolConfirmation,
		"tool_use_id": toolUseID,
		"result":      decision,
	}
	if strings.TrimSpace(denyMessage) != "" {
		event["deny_message"] = strings.TrimSpace(denyMessage)
	}
	return buildManagedAgentTypedEventsPayload([]map[string]interface{}{event}), nil
}

func buildManagedAgentTypedEventsPayload(events []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"events": events}
}
