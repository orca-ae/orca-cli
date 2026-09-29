#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

# Collects the license and notice files of every module compiled into ork into
# OUT_DIR, one directory per module, so release archives and the image can ship
# them. go-licenses skips github.com/orca-ae modules, which are this project's
# own; the Go SDK's license file is copied as-is because go-licenses can't
# classify it.

set -euo pipefail

if [[ "$#" -ne 1 ]]; then
  echo "usage: $0 OUT_DIR" >&2
  exit 2
fi

out_dir="$1"
go_licenses=(go run github.com/google/go-licenses/v2@v2.0.1)

rm -rf "${out_dir}"
"${go_licenses[@]}" save ./cmd/ork --ignore github.com/orca-ae --save_path "${out_dir}"

sdk_dir="$(go list -m -f '{{.Dir}}' github.com/orca-ae/orca-sdk-go)"
mkdir -p "${out_dir}/github.com/orca-ae/orca-sdk-go"
cp "${sdk_dir}/LICENSE" "${out_dir}/github.com/orca-ae/orca-sdk-go/LICENSE"

# Module-cache files are read-only; make the copies ordinary files.
chmod -R u+w,go+r "${out_dir}"
