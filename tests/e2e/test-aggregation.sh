#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

failure_file=$(mktemp)
trap 'rm -f "${failure_file}"' EXIT
export E2E_FAILURE_FILE=${failure_file}
export E2E_SUITE_NAME=fault-injection

passing() {
  echo PASSING_RAN
}

failing() {
  false
  echo SHOULD_NOT_RUN
}

after_failure() {
  echo AFTER_FAILURE_RAN
}

set +e
output=$(
  {
    e2e::run_scenario "passing scenario" passing
    e2e::run_scenario "failing scenario" failing
    e2e::run_scenario "after failure" after_failure
    e2e::finish_scenarios
  } 2>&1
)
status=$?
set -e

test "${status}" -eq 1
grep --fixed-strings --quiet '[PASS] passing scenario' <<<"${output}"
grep --fixed-strings --quiet '[FAIL] failing scenario' <<<"${output}"
grep --fixed-strings --quiet 'AFTER_FAILURE_RAN' <<<"${output}"
grep --fixed-strings --quiet '[PASS] after failure' <<<"${output}"
grep --fixed-strings --quiet '[SUMMARY] 1 of 3 scenarios failed' <<<"${output}"
grep --fixed-strings --quiet $'fault-injection\tfailing scenario' "${failure_file}"
if grep --fixed-strings --quiet 'SHOULD_NOT_RUN' <<<"${output}"; then
  echo "failed scenario continued after its first error" >&2
  exit 1
fi
