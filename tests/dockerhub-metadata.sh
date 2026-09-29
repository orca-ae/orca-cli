#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RESOLVER="${REPO_ROOT}/scripts/dockerhub-metadata.sh"

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

without_credentials="$(
  env -u DOCKERHUB_USERNAME -u DOCKERHUB_TOKEN -u DOCKERHUB_NAMESPACE \
    "${RESOLVER}" 2>/dev/null
)"
assert_line "${without_credentials}" 'credentials=false'
assert_line "${without_credentials}" 'dockerhub_image='

without_namespace="$(
  env -u DOCKERHUB_NAMESPACE \
    DOCKERHUB_USERNAME=ReleaseBot \
    DOCKERHUB_TOKEN=dummy-token \
    "${RESOLVER}" 2>/dev/null
)"
assert_line "${without_namespace}" 'credentials=false'
assert_line "${without_namespace}" 'dockerhub_username='
assert_line "${without_namespace}" 'dockerhub_image='

with_namespace="$(
  DOCKERHUB_USERNAME=ReleaseBot \
    DOCKERHUB_TOKEN=dummy-token \
    DOCKERHUB_NAMESPACE=ExampleOrg \
    "${RESOLVER}"
)"
assert_line "${with_namespace}" 'credentials=true'
assert_line "${with_namespace}" 'dockerhub_username=releasebot'
assert_line "${with_namespace}" 'dockerhub_image=docker.io/exampleorg/orca-cli'

if env -u DOCKERHUB_TOKEN \
  DOCKERHUB_USERNAME=releasebot DOCKERHUB_NAMESPACE=exampleorg "${RESOLVER}" >/dev/null 2>&1; then
  echo 'expected username without token to be rejected' >&2
  exit 1
fi

if env -u DOCKERHUB_USERNAME \
  DOCKERHUB_TOKEN=dummy-token DOCKERHUB_NAMESPACE=exampleorg "${RESOLVER}" >/dev/null 2>&1; then
  echo 'expected token without username to be rejected' >&2
  exit 1
fi

echo 'Docker Hub metadata tests passed'
