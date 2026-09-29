#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

E2E_SCENARIO_COUNT=0
E2E_FAILED_SCENARIOS=()

e2e::require_command() {
  local command_name=$1
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "required command is not installed: ${command_name}" >&2
    return 1
  fi
}

e2e::require_env() {
  local variable_name=$1
  if [[ -z "${!variable_name:-}" ]]; then
    echo "required environment variable is not set: ${variable_name}" >&2
    return 1
  fi
}

e2e::wait_for_http() {
  local url=$1
  local timeout_seconds=${2:-300}
  local deadline=$((SECONDS + timeout_seconds))

  until curl --fail --silent --show-error --connect-timeout 2 --max-time 5 "${url}" >/dev/null; do
    if ((SECONDS >= deadline)); then
      echo "timed out waiting for ${url}" >&2
      return 1
    fi
    sleep 2
  done
}

e2e::json_id() {
  jq --exit-status --raw-output '.id | strings | select(length > 0)' "$1"
}

e2e::run_scenario() {
  local name=$1
  local scenario_function=$2
  local status

  E2E_SCENARIO_COUNT=$((E2E_SCENARIO_COUNT + 1))
  printf '[RUN] %s\n' "${name}"
  echo "::group::${name}"

  set +e
  (
    trap - EXIT
    set -euo pipefail
    "${scenario_function}"
  )
  status=$?
  set -e

  echo "::endgroup::"
  if ((status == 0)); then
    printf '[PASS] %s\n' "${name}"
    return 0
  fi

  E2E_FAILED_SCENARIOS+=("${name}")
  printf '[FAIL] %s (exit %d)\n' "${name}" "${status}" >&2
  echo "::error title=E2E scenario failed::${name} exited with status ${status}"
  if [[ -n "${E2E_FAILURE_FILE:-}" ]]; then
    printf '%s\t%s\n' "${E2E_SUITE_NAME:-e2e}" "${name}" >>"${E2E_FAILURE_FILE}"
  fi
  return 0
}

e2e::finish_scenarios() {
  local failure_count=${#E2E_FAILED_SCENARIOS[@]}

  if ((failure_count == 0)); then
    printf '[SUMMARY] all %d scenarios passed\n' "${E2E_SCENARIO_COUNT}"
    return 0
  fi

  printf '[SUMMARY] %d of %d scenarios failed:\n' \
    "${failure_count}" "${E2E_SCENARIO_COUNT}" >&2
  printf '  - %s\n' "${E2E_FAILED_SCENARIOS[@]}" >&2
  return 1
}
