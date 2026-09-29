#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

e2e::require_env ORCA_BIN
e2e::require_env ORCA_REGISTRY_URL
e2e::require_env ORCA_API_KEY
e2e::require_command jq

ORCA_E2E_EXPECT_EXECUTION=${ORCA_E2E_EXPECT_EXECUTION:-false}
run_suffix=${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-0}-$$
resource_prefix="cli-e2e-${run_suffix}"
temp_dir=$(mktemp -d)

cleanup() {
  local resource_id

  set +e
  if [[ -s "${temp_dir}/guardrail.json" ]]; then
    resource_id=$(e2e::json_id "${temp_dir}/guardrail.json" 2>/dev/null)
    "${ORCA_BIN}" guardrails archive "${resource_id}" >/dev/null 2>&1
    "${ORCA_BIN}" guardrails delete "${resource_id}" >/dev/null 2>&1
  fi
  if [[ -s "${temp_dir}/trigger.json" ]]; then
    resource_id=$(e2e::json_id "${temp_dir}/trigger.json" 2>/dev/null)
    "${ORCA_BIN}" agent triggers delete "${resource_id}" >/dev/null 2>&1
  fi
  if [[ -s "${temp_dir}/session.json" ]]; then
    resource_id=$(e2e::json_id "${temp_dir}/session.json" 2>/dev/null)
    "${ORCA_BIN}" agent sessions archive "${resource_id}" >/dev/null 2>&1
  fi
  if [[ -s "${temp_dir}/agent.json" ]]; then
    resource_id=$(e2e::json_id "${temp_dir}/agent.json" 2>/dev/null)
    "${ORCA_BIN}" agent archive "${resource_id}" >/dev/null 2>&1
  fi
  if [[ -s "${temp_dir}/environment.json" ]]; then
    resource_id=$(e2e::json_id "${temp_dir}/environment.json" 2>/dev/null)
    "${ORCA_BIN}" agent environments archive "${resource_id}" >/dev/null 2>&1
  fi
  if [[ -s "${temp_dir}/file.json" ]]; then
    resource_id=$(e2e::json_id "${temp_dir}/file.json" 2>/dev/null)
    "${ORCA_BIN}" agent files delete "${resource_id}" >/dev/null 2>&1
  fi
  rm -rf "${temp_dir}"
}
trap cleanup EXIT

core_discovery_and_readiness() {
  "${ORCA_BIN}" healthz --output json | jq --exit-status . >/dev/null
  "${ORCA_BIN}" readyz --output json | jq --exit-status . >/dev/null
  "${ORCA_BIN}" api-versions --output json | jq --exit-status . >/dev/null
  "${ORCA_BIN}" api-groups --output json >"${temp_dir}/api-groups.json"

  jq --exit-status '
    any(.groups[]?; .name == "policy.runorca.ai") and
    any(.groups[]?; .name == "pricing.runorca.ai") and
    (all(.groups[]?; .name != "cloud.sn.io"))
  ' "${temp_dir}/api-groups.json" >/dev/null
  if "${ORCA_BIN}" api-resources --output json >"${temp_dir}/unexpected-cloud.json" 2>"${temp_dir}/cloud-error.log"; then
    echo "cloud api-resources unexpectedly succeeded against a non-cloud deployment" >&2
    exit 1
  fi
  grep --fixed-strings --quiet 'the "cloud.sn.io" extension group is not available on this deployment' \
    "${temp_dir}/cloud-error.log"
  "${ORCA_BIN}" api-resources --group policy.runorca.ai --output json |
    jq --exit-status '
      .group_version == "policy.runorca.ai/v1" and
      any(.resources[]?; .name == "guardrails")
    ' >/dev/null
  "${ORCA_BIN}" api-resources --group pricing.runorca.ai --output json |
    jq --exit-status '
      .group_version == "pricing.runorca.ai/v1" and
      any(.resources[]?; .name == "modelprices")
    ' >/dev/null
}

policy_pricing_extension_lifecycle() {
  local guardrail_id
  local guardrail_payload
  local model_id
  local provider

  "${ORCA_BIN}" guardrails list-types --output json |
    jq --exit-status 'any(.data[]?; .name == "block_tools")' >/dev/null

  guardrail_payload=$(jq --compact-output --null-input \
    --arg name "${resource_prefix}-guardrail" \
    '{name:$name, description:"created by orca-cli e2e", phases:["request"], scope:"explicit", rule:{kind:"expression", expression:"true", on_false:"deny"}, metadata:{suite:"orca-cli-e2e"}}')
  "${ORCA_BIN}" guardrails create \
    --config-json "${guardrail_payload}" \
    --output json >"${temp_dir}/guardrail.json"
  guardrail_id=$(e2e::json_id "${temp_dir}/guardrail.json")

  "${ORCA_BIN}" guardrails update "${guardrail_id}" \
    --config-json '{"description":"updated by orca-cli e2e"}' \
    --output json |
    jq --exit-status '.description == "updated by orca-cli e2e"' >/dev/null
  "${ORCA_BIN}" guardrails get "${guardrail_id}" --output json |
    jq --exit-status --arg id "${guardrail_id}" '.id == $id' >/dev/null
  "${ORCA_BIN}" guardrails list --limit 100 --output json |
    jq --exit-status --arg id "${guardrail_id}" 'any(.data[]?; .id == $id)' >/dev/null

  "${ORCA_BIN}" model-prices list --limit 10 --output json >"${temp_dir}/model-prices.json"
  jq --exit-status '.data | length > 0' "${temp_dir}/model-prices.json" >/dev/null
  model_id=$(jq --exit-status --raw-output '.data[0].model_id | strings | select(length > 0)' "${temp_dir}/model-prices.json")
  provider=$(jq --exit-status --raw-output '.data[0].provider | strings | select(length > 0)' "${temp_dir}/model-prices.json")
  "${ORCA_BIN}" model-prices get "${model_id}" --provider "${provider}" --output json |
    jq --exit-status --arg model_id "${model_id}" --arg provider "${provider}" \
      '.model_id == $model_id and .provider == $provider' >/dev/null

  "${ORCA_BIN}" guardrails archive "${guardrail_id}" --output json |
    jq --exit-status --arg id "${guardrail_id}" '.id == $id and .archived_at != null' >/dev/null
  "${ORCA_BIN}" guardrails delete "${guardrail_id}" --output json |
    jq --exit-status --arg id "${guardrail_id}" '.id == $id and .type == "guardrail_deleted"' >/dev/null
  rm -f "${temp_dir}/guardrail.json"
}

core_environment_lifecycle() {
  local environment_id

  "${ORCA_BIN}" agent environments create \
    --name "${resource_prefix}-environment" \
    --description "created by orca-cli e2e" \
    --config-type cloud \
    --networking-type unrestricted \
    --output json >"${temp_dir}/environment.json"
  environment_id=$(e2e::json_id "${temp_dir}/environment.json")

  "${ORCA_BIN}" agent environments update "${environment_id}" \
    --description "updated by orca-cli e2e" \
    --output json | jq --exit-status '.description == "updated by orca-cli e2e"' >/dev/null
  "${ORCA_BIN}" agent environments get "${environment_id}" --output json |
    jq --exit-status --arg id "${environment_id}" '.id == $id' >/dev/null
}

core_agent_lifecycle() {
  local agent_id

  "${ORCA_BIN}" agent create \
    --name "${resource_prefix}-agent" \
    --model-json '{"provider":"anthropic","id":"claude-sonnet-4-5-20250929"}' \
    --system "Return concise answers." \
    --metadata "suite=orca-cli-e2e" \
    --output json >"${temp_dir}/agent.json"
  agent_id=$(e2e::json_id "${temp_dir}/agent.json")

  "${ORCA_BIN}" agent update "${agent_id}" \
    --description "updated by orca-cli e2e" \
    --output json | jq --exit-status '.description == "updated by orca-cli e2e"' >/dev/null
  "${ORCA_BIN}" agent get "${agent_id}" --output json |
    jq --exit-status --arg id "${agent_id}" '.id == $id' >/dev/null
}

core_trigger_lifecycle() {
  local agent_id
  local agent_version
  local environment_id
  local trigger_id

  agent_id=$(e2e::json_id "${temp_dir}/agent.json")
  agent_version=$(jq --exit-status --raw-output '.version | numbers | select(. > 0)' "${temp_dir}/agent.json")
  environment_id=$(e2e::json_id "${temp_dir}/environment.json")

  "${ORCA_BIN}" agent triggers create \
    --name "${resource_prefix}-trigger" \
    --agent "${agent_id}" \
    --agent-version "${agent_version}" \
    --source-type cron \
    --session-mode SESSION_PER_EVENT \
    --schedule "0 0 1 1 *" \
    --timezone Etc/UTC \
    --payload "orca-cli trigger e2e ${run_suffix}" \
    --environment-id "${environment_id}" \
    --title-template "${resource_prefix}-trigger-session" \
    --metadata "phase=created" \
    --replicas 1 \
    --paused \
    --output json >"${temp_dir}/trigger.json"
  trigger_id=$(e2e::json_id "${temp_dir}/trigger.json")

  jq --exit-status \
    --arg id "${trigger_id}" \
    --arg agent_id "${agent_id}" \
    '.id == $id and .agent.id == $agent_id and .source.type == "cron" and
      .session_mode == "SESSION_PER_EVENT" and .status == "paused"' \
    "${temp_dir}/trigger.json" >/dev/null

  "${ORCA_BIN}" agent triggers list --agent "${agent_id}" --limit 10 --output json |
    jq --exit-status --arg id "${trigger_id}" 'any(.data[]?; .id == $id)' >/dev/null
  "${ORCA_BIN}" agent triggers get "${trigger_id}" --output json |
    jq --exit-status --arg id "${trigger_id}" '.id == $id' >/dev/null

  "${ORCA_BIN}" agent triggers update "${trigger_id}" \
    --name "${resource_prefix}-trigger-updated" \
    --session-mode SESSION_PER_EVENT \
    --source-type cron \
    --schedule "30 0 1 1 *" \
    --timezone Etc/UTC \
    --payload "updated orca-cli trigger e2e ${run_suffix}" \
    --environment-id "${environment_id}" \
    --title-template "${resource_prefix}-trigger-session-updated" \
    --metadata "phase=updated" \
    --replicas 1 \
    --output json |
    jq --exit-status \
      --arg name "${resource_prefix}-trigger-updated" \
      '.name == $name and .source.schedule == "30 0 1 1 *" and .session.metadata.phase == "updated"' >/dev/null

  "${ORCA_BIN}" agent triggers unpause "${trigger_id}" |
    jq --exit-status '.status == "active"' >/dev/null
  "${ORCA_BIN}" agent triggers pause "${trigger_id}" |
    jq --exit-status '.status == "paused"' >/dev/null
  "${ORCA_BIN}" agent triggers sessions "${trigger_id}" --limit 10 --output json |
    jq --exit-status '.data | length == 0' >/dev/null
  "${ORCA_BIN}" agent triggers delete "${trigger_id}" |
    jq --exit-status --arg id "${trigger_id}" '.id == $id and .type == "trigger_deleted"' >/dev/null
  rm -f "${temp_dir}/trigger.json"
}

core_file_lifecycle() {
  local file_id

  printf 'orca-cli e2e file %s\n' "${run_suffix}" >"${temp_dir}/upload.txt"
  "${ORCA_BIN}" agent files create \
    --file "${temp_dir}/upload.txt" \
    --content-type text/plain \
    --output json >"${temp_dir}/file.json"
  file_id=$(e2e::json_id "${temp_dir}/file.json")
  "${ORCA_BIN}" agent files get "${file_id}" --output json |
    jq --exit-status --arg id "${file_id}" '.id == $id' >/dev/null
  if "${ORCA_BIN}" agent files content "${file_id}" \
    --output-file "${temp_dir}/download.txt" 2>"${temp_dir}/download-error.log"; then
    echo "uploaded file content unexpectedly remained downloadable" >&2
    exit 1
  fi
  grep --quiet 'status 403' "${temp_dir}/download-error.log"
}

core_session_lifecycle() {
  local agent_id
  local environment_id
  local session_id

  environment_id=$(e2e::json_id "${temp_dir}/environment.json")
  agent_id=$(e2e::json_id "${temp_dir}/agent.json")
  "${ORCA_BIN}" agent sessions create \
    --environment-id "${environment_id}" \
    --agent "${agent_id}" \
    --title "${resource_prefix}-session" \
    --output json >"${temp_dir}/session.json"
  session_id=$(e2e::json_id "${temp_dir}/session.json")
  "${ORCA_BIN}" agent sessions get "${session_id}" --output json |
    jq --exit-status --arg id "${session_id}" '.id == $id' >/dev/null
}

core_deterministic_execution() {
  local deadline
  local expected_reply
  local marker
  local session_id

  session_id=$(e2e::json_id "${temp_dir}/session.json")
  marker="KIND_HELM_CLI_${run_suffix}"
  expected_reply="MISSING_ECHO_TOOL ${marker}"
  "${ORCA_BIN}" agent sessions events send message \
    --session "${session_id}" \
    --text "Return the deterministic marker ${marker}." \
    --output json >"${temp_dir}/sent-events.json"
  jq --exit-status '.data | length == 1' "${temp_dir}/sent-events.json" >/dev/null

  deadline=$((SECONDS + 120))
  while true; do
    "${ORCA_BIN}" agent sessions events list \
      --session "${session_id}" \
      --limit 1000 \
      --output json >"${temp_dir}/events.json"
    if jq --exit-status \
      --arg expected "${expected_reply}" \
      'any(.data[]?; .type == "agent.message" and (tostring | contains($expected)))' \
      "${temp_dir}/events.json" >/dev/null; then
      break
    fi
    if jq --exit-status \
      'any(.data[]?; .type == "session.error" or .type == "session.status_error" or .type == "session.setup_failed")' \
      "${temp_dir}/events.json" >/dev/null; then
      echo "Managed Agents execution emitted a terminal error" >&2
      jq '.data' "${temp_dir}/events.json" >&2
      exit 1
    fi
    if ((SECONDS >= deadline)); then
      echo "timed out waiting for deterministic agent reply ${expected_reply}" >&2
      jq '.data' "${temp_dir}/events.json" >&2
      exit 1
    fi
    sleep 1
  done

  "${ORCA_BIN}" agent sessions events stream \
    --session "${session_id}" \
    --from-cursor 0 \
    --timeout 3s >"${temp_dir}/events.sse.jsonl"
  jq --exit-status --slurp \
    --arg expected "${expected_reply}" \
    'any(.[]; ((.data // .) | tostring | contains($expected)))' \
    "${temp_dir}/events.sse.jsonl" >/dev/null
}

core_resource_cleanup() {
  local agent_id
  local environment_id
  local file_id
  local session_id
  local trigger_id

  if [[ -s "${temp_dir}/trigger.json" ]]; then
    trigger_id=$(e2e::json_id "${temp_dir}/trigger.json")
    "${ORCA_BIN}" agent triggers delete "${trigger_id}" >/dev/null
  fi
  session_id=$(e2e::json_id "${temp_dir}/session.json")
  agent_id=$(e2e::json_id "${temp_dir}/agent.json")
  environment_id=$(e2e::json_id "${temp_dir}/environment.json")
  file_id=$(e2e::json_id "${temp_dir}/file.json")
  "${ORCA_BIN}" agent sessions archive "${session_id}" |
    jq --exit-status --arg id "${session_id}" \
      '.id == $id and (.archived_at | strings | length > 0)' >/dev/null
  "${ORCA_BIN}" agent archive "${agent_id}" >/dev/null
  "${ORCA_BIN}" agent environments archive "${environment_id}" >/dev/null
  "${ORCA_BIN}" agent files delete "${file_id}" >/dev/null
  rm -f \
    "${temp_dir}/session.json" \
    "${temp_dir}/trigger.json" \
    "${temp_dir}/agent.json" \
    "${temp_dir}/environment.json" \
    "${temp_dir}/file.json"
}

e2e::run_scenario "discovery and readiness" core_discovery_and_readiness
e2e::run_scenario "policy and pricing extension lifecycle" policy_pricing_extension_lifecycle
e2e::run_scenario "environment create, update, and get" core_environment_lifecycle
e2e::run_scenario "agent create, update, and get" core_agent_lifecycle
e2e::run_scenario "trigger create, update, lifecycle, sessions, and delete" core_trigger_lifecycle
e2e::run_scenario "file create, get, and denied content download" core_file_lifecycle
e2e::run_scenario "session create and get" core_session_lifecycle
if [[ "${ORCA_E2E_EXPECT_EXECUTION}" == "true" ]]; then
  e2e::run_scenario "deterministic execution and SSE replay" core_deterministic_execution
else
  printf '[SKIP] deterministic execution and SSE replay (disabled)\n'
fi
e2e::run_scenario "resource archival and deletion" core_resource_cleanup

e2e::finish_scenarios
echo "orca-cli core e2e passed (${ORCA_REGISTRY_URL})"
