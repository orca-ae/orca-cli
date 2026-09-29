# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

# Release CI cross-compiles static Linux binaries and stages them, with the
# license files, under dist/docker/ (scripts/build-release-archives.sh). Keeping
# compilation outside Docker avoids QEMU-bound Go builds while still producing
# one multi-platform image.
FROM alpine:3.22

ARG TARGETARCH
ARG ORCA_CLI_VERSION=dev
ARG ORCA_CLI_REVISION=unknown

LABEL org.opencontainers.image.title="ork"
LABEL org.opencontainers.image.description="CLI to manage & interact with resources in Orca Agent Engine"
LABEL org.opencontainers.image.url="https://runorca.ai"
LABEL org.opencontainers.image.source="https://github.com/orca-ae/orca-cli"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.version="${ORCA_CLI_VERSION}"
LABEL org.opencontainers.image.revision="${ORCA_CLI_REVISION}"

# Keep a shell and BusyBox utilities so the image can back a long-running
# Kubernetes toolset pod and support kubectl exec. Credentials are injected at
# runtime; no credential ARG or ENV belongs in this image.
# Keep ca-certificates on the latest security revision available to Alpine 3.22.
# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 1000 orca \
    && adduser -S -D -H -u 1000 -G orca -h /home/orca orca \
    && mkdir -p /home/orca /workspace \
    && chown -R orca:orca /home/orca /workspace

COPY dist/docker/licenses/ /usr/share/doc/ork/
COPY --chmod=0755 dist/docker/linux-${TARGETARCH}/ork /usr/local/bin/ork

ENV HOME=/home/orca
USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/ork"]
