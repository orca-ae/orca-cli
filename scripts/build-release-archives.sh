#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

# Cross-compiles ork for every release platform. Each binary is packaged with
# LICENSE, NOTICE, README.md and THIRD_PARTY_LICENSES into
# dist/ork_<tag>_<os>_<arch>.tar.gz (.zip on Windows), and dist/checksums.txt
# lists the archives by file name so `sha256sum -c` works next to them. The
# Linux binaries and the license files are also staged under dist/docker/ for
# the container image. Run it from the repository root.

set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: $0 TAG VERSION" >&2
  exit 2
fi

tag="$1"
version="$2"

rm -rf build dist
mkdir -p build dist
./scripts/third-party-licenses.sh build/THIRD_PARTY_LICENSES

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  goos="${target%/*}"
  goarch="${target#*/}"
  binary=ork
  if [[ "${goos}" == windows ]]; then
    binary=ork.exe
  fi

  name="ork_${tag}_${goos}_${goarch}"
  mkdir -p "build/${name}"
  GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w -X main.version=${version}" \
    -o "build/${name}/${binary}" ./cmd/ork
  cp LICENSE NOTICE README.md "build/${name}/"
  cp -R build/THIRD_PARTY_LICENSES "build/${name}/THIRD_PARTY_LICENSES"

  if [[ "${goos}" == windows ]]; then
    (cd build && zip -qr "../dist/${name}.zip" "${name}")
  else
    tar -czf "dist/${name}.tar.gz" -C build "${name}"
  fi
done

(cd dist && shasum -a 256 ork_* >checksums.txt)

for arch in amd64 arm64; do
  mkdir -p "dist/docker/linux-${arch}"
  cp "build/ork_${tag}_linux_${arch}/ork" "dist/docker/linux-${arch}/ork"
  chmod 0755 "dist/docker/linux-${arch}/ork"
done
mkdir -p dist/docker/licenses
cp LICENSE NOTICE dist/docker/licenses/
cp -R build/THIRD_PARTY_LICENSES dist/docker/licenses/THIRD_PARTY_LICENSES
