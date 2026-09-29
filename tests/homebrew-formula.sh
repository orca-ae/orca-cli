#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RENDER="${REPO_ROOT}/scripts/homebrew-formula.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT

fail() {
  echo "$1" >&2
  exit 1
}

assert_contains() {
  local output="$1"
  local expected="$2"
  if ! grep -Fq -- "${expected}" <<<"${output}"; then
    echo "formula is missing: ${expected}" >&2
    printf '%s\n' "${output}" >&2
    exit 1
  fi
}

tag=v0.5.0-rc.1
checksums="${WORK}/checksums.txt"
i=0
for platform in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64 windows_amd64 windows_arm64; do
  i=$((i + 1))
  extension=tar.gz
  if [[ "${platform}" == windows_* ]]; then
    extension=zip
  fi
  printf '%064d  ork_%s_%s.%s\n' "${i}" "${tag}" "${platform}" "${extension}" >>"${checksums}"
done

formula="$("${RENDER}" "${tag}" "${checksums}")"
assert_contains "${formula}" 'class Ork < Formula'
assert_contains "${formula}" 'version "0.5.0-rc.1"'
assert_contains "${formula}" 'license "Apache-2.0"'
assert_contains "${formula}" 'url "https://github.com/orca-ae/orca-cli/releases/download/v0.5.0-rc.1/ork_v0.5.0-rc.1_darwin_arm64.tar.gz"'
assert_contains "${formula}" "sha256 \"$(printf '%064d' 2)\""
assert_contains "${formula}" "sha256 \"$(printf '%064d' 4)\""
assert_contains "${formula}" 'assert_equal "ork version #{version}"'
if grep -Fq 'windows' <<<"${formula}"; then
  fail 'the formula must not reference Windows archives'
fi
if command -v ruby >/dev/null 2>&1; then
  ruby -c - <<<"${formula}" >/dev/null || fail 'the formula is not valid Ruby'
fi

grep -v 'linux_arm64' "${checksums}" >"${WORK}/incomplete.txt"
if "${RENDER}" "${tag}" "${WORK}/incomplete.txt" >/dev/null 2>&1; then
  fail 'expected a missing archive checksum to be rejected'
fi

if "${RENDER}" 0.5.0 "${checksums}" >/dev/null 2>&1; then
  fail 'expected a tag without the v prefix to be rejected'
fi

echo 'Homebrew formula tests passed'
