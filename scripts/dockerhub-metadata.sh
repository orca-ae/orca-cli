#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

# Resolve the optional Docker Hub mirror. The mirror is enabled only when
# DOCKERHUB_USERNAME, DOCKERHUB_TOKEN and DOCKERHUB_NAMESPACE are all set: the
# namespace never defaults to the login, so the image can't land under an
# account nobody chose for it. GitHub Actions appends stdout directly to
# GITHUB_OUTPUT, so diagnostics stay on stderr and stdout stays in key=value
# form.

set -euo pipefail

if [ "$#" -ne 0 ]; then
  echo "usage: $0" >&2
  exit 2
fi

username="${DOCKERHUB_USERNAME:-}"
token="${DOCKERHUB_TOKEN:-}"
namespace="${DOCKERHUB_NAMESPACE:-}"

credentials=false
dockerhub_username=''
dockerhub_image=''

if [[ -n "${username}" || -n "${token}" ]]; then
  if [[ -z "${username}" || -z "${token}" ]]; then
    echo '::error::Configure both DOCKERHUB_USERNAME and DOCKERHUB_TOKEN, or leave both unset to publish only to GHCR.' >&2
    exit 1
  fi

  if [[ -z "${namespace}" ]]; then
    echo '::warning::DOCKERHUB_NAMESPACE is not configured; publishing only to GHCR.' >&2
  else
    credentials=true
    dockerhub_username="$(printf '%s' "${username}" | tr '[:upper:]' '[:lower:]')"
    dockerhub_namespace="$(printf '%s' "${namespace}" | tr '[:upper:]' '[:lower:]')"
    dockerhub_image="docker.io/${dockerhub_namespace}/orca-cli"
  fi
else
  echo '::warning::Docker Hub credentials are not configured; publishing only to GHCR.' >&2
fi

cat <<EOF
credentials=${credentials}
dockerhub_username=${dockerhub_username}
dockerhub_image=${dockerhub_image}
EOF
