#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

# Called only after Release Please has created the draft GitHub release.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"${ROOT}/scripts/release-metadata.sh" "$@" >/dev/null
tag="$1"
revision="$2"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GH_TOKEN:?GH_TOKEN is required}"

# matching-refs returns [] for an absent tag, while API/auth failures remain
# errors. Never reinterpret a failed lookup as permission to create a tag.
refs="$(gh api "repos/${GITHUB_REPOSITORY}/git/matching-refs/tags/${tag}")"
existing="$(jq -r --arg ref "refs/tags/${tag}" '
  .[] | select(.ref == $ref) | .object.sha
' <<<"${refs}")"
if [[ -n "${existing}" ]]; then
  if [[ "${existing}" != "${revision}" ]]; then
    echo "error: ${tag} already exists at ${existing}, not ${revision}; refusing to move it" >&2
    exit 1
  fi
  echo "${tag} already points to ${revision}."
  exit 0
fi

gh api --method POST "repos/${GITHUB_REPOSITORY}/git/refs" \
  -f "ref=refs/tags/${tag}" -f "sha=${revision}" >/dev/null
