#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RESOLVER="${REPO_ROOT}/scripts/release-metadata.sh"
REVISION="0123456789abcdef0123456789abcdef01234567"

assert_line() {
  local output="$1"
  local expected="$2"
  if ! grep -Fqx "${expected}" <<<"${output}"; then
    echo "missing output line: ${expected}" >&2
    echo "actual output:" >&2
    printf '  %s\n' "${output}" >&2
    exit 1
  fi
}

assert_rejected() {
  local tag="$1"
  if "${RESOLVER}" "${tag}" "${REVISION}" >/dev/null 2>&1; then
    echo "expected invalid release tag to be rejected: ${tag}" >&2
    exit 1
  fi
}

stable_output="$("${RESOLVER}" v0.2.0 "${REVISION}")"
assert_line "${stable_output}" "tag=v0.2.0"
assert_line "${stable_output}" "version=0.2.0"
assert_line "${stable_output}" "stable=true"
assert_line "${stable_output}" "git_sha=0123456789ab"

prerelease_output="$("${RESOLVER}" v1.4.0-rc.2 "${REVISION}")"
assert_line "${prerelease_output}" "tag=v1.4.0-rc.2"
assert_line "${prerelease_output}" "version=1.4.0-rc.2"
assert_line "${prerelease_output}" "stable=false"

assert_rejected "0.2.0"
assert_rejected "v0.2"
assert_rejected "v01.2.3"
assert_rejected "v1.2.3-rc.01"
assert_rejected "v1.2.3+build.1"

if "${RESOLVER}" v0.2.0 not-a-sha >/dev/null 2>&1; then
  echo "expected invalid git SHA to be rejected" >&2
  exit 1
fi

echo "release metadata tests passed"
