#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

e2e::require_env ORCA_BIN
e2e::require_env KIND_HELM_NAMESPACE
e2e::require_command kubectl

port_forward_log=${RUNNER_TEMP:-/tmp}/orca-managed-agents-port-forward.log
port_forward_pid=

cleanup() {
  local status=$?
  trap - EXIT
  if [[ -n "${port_forward_pid}" ]] && kill -0 "${port_forward_pid}" >/dev/null 2>&1; then
    kill "${port_forward_pid}" >/dev/null 2>&1 || true
    wait "${port_forward_pid}" >/dev/null 2>&1 || true
  fi
  if ((status != 0)) && [[ -f "${port_forward_log}" ]]; then
    cat "${port_forward_log}"
  fi
  exit "${status}"
}
trap cleanup EXIT

kubectl --namespace "${KIND_HELM_NAMESPACE}" port-forward \
  service/orca-managed-agents-registry 18080:8080 >"${port_forward_log}" 2>&1 &
port_forward_pid=$!
e2e::wait_for_http http://localhost:18080/healthz 180

api_key=orca_cli_e2e_local_key
"${SCRIPT_DIR}/seed-managed-agents-api-key.sh" \
  "${KIND_HELM_NAMESPACE}" orca-managed-agents-registry "${api_key}"

export ORCA_REGISTRY_URL=http://localhost:18080
export ORCA_API_KEY=${api_key}
unset ORCA_ACCESS_TOKEN
export ORCA_E2E_EXPECT_EXECUTION=true
"${SCRIPT_DIR}/core.sh"

echo "managed-agents direct topology passed"
