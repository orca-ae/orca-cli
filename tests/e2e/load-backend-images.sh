#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

# Called only by trusted-main E2E. Never mount this auth configuration into Kind.
set -euo pipefail
: "${GH_TOKEN:?required}"
: "${KIND_CLUSTER_NAME:?required}"
: "${REGISTRY_IMAGE:?required}"
: "${REGISTRY_RESOLVED_IMAGE:?required}"
: "${HARNESS_IMAGE:?required}"
: "${HARNESS_RESOLVED_IMAGE:?required}"
: "${GATEWAY_IMAGE:?required}"

export DOCKER_CONFIG
DOCKER_CONFIG=$(mktemp -d)
cleanup() { rm -rf "${DOCKER_CONFIG}"; }
trap cleanup EXIT
username=$(gh api user --jq .login)
printf '%s' "${GH_TOKEN}" | docker login ghcr.io --username "${username}" --password-stdin
unset GH_TOKEN

docker pull --platform linux/amd64 "${REGISTRY_RESOLVED_IMAGE}"
docker tag "${REGISTRY_RESOLVED_IMAGE}" "${REGISTRY_IMAGE}"
docker pull --platform linux/amd64 "${HARNESS_RESOLVED_IMAGE}"
docker tag "${HARNESS_RESOLVED_IMAGE}" "${HARNESS_IMAGE}"
docker pull --platform linux/amd64 "${GATEWAY_IMAGE}"

# Remove registry authentication before exposing images to the test cluster.
cleanup
unset DOCKER_CONFIG
trap - EXIT
kind load docker-image --name "${KIND_CLUSTER_NAME}" \
  "${REGISTRY_IMAGE}" "${HARNESS_IMAGE}" "${GATEWAY_IMAGE}"
