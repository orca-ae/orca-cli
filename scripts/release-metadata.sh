#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

# Resolve release metadata from one Git tag. GitHub Actions appends stdout
# directly to GITHUB_OUTPUT, so keep stdout in key=value form.

set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <vMAJOR.MINOR.PATCH[-PRERELEASE]> <git-sha>" >&2
  exit 2
fi

tag="$1"
revision="$2"
version="${tag#v}"

# Build metadata is rejected because '+' is not valid in every downstream
# version surface, notably Docker tags. Pre-release tags remain supported.
semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
if [[ "${tag}" != v* ]] || [[ ! "${version}" =~ ${semver} ]]; then
  echo "error: release tag must match vMAJOR.MINOR.PATCH[-PRERELEASE] without build metadata; got ${tag}" >&2
  exit 1
fi

if [[ "${version}" == *-* ]]; then
  prerelease="${version#*-}"
  IFS='.' read -r -a identifiers <<<"${prerelease}"
  for identifier in "${identifiers[@]}"; do
    if [[ "${identifier}" =~ ^0[0-9]+$ ]]; then
      echo "error: numeric pre-release identifiers must not contain leading zeroes; got ${tag}" >&2
      exit 1
    fi
  done
fi

if [[ ! "${revision}" =~ ^[0-9A-Fa-f]{12,64}$ ]]; then
  echo "error: git SHA must contain 12-64 hexadecimal characters; got ${revision}" >&2
  exit 1
fi

stable=true
if [[ "${version}" == *-* ]]; then
  stable=false
fi

cat <<EOF
tag=${tag}
version=${version}
stable=${stable}
git_sha=${revision:0:12}
revision=${revision}
EOF
