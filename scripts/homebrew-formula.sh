#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

# Prints the Homebrew formula for an ork release, built from the release's
# checksums.txt. The formula downloads the archives from the release on
# github.com/orca-ae/orca-cli, so it only installs once that release is public.

set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: $0 TAG CHECKSUMS_FILE" >&2
  exit 2
fi

tag="$1"
checksums="$2"
if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "error: ${tag} is not a release tag" >&2
  exit 2
fi
version="${tag#v}"
base_url="https://github.com/orca-ae/orca-cli/releases/download/${tag}"

sha256_of() {
  local archive="ork_${tag}_$1.tar.gz"
  local sum
  sum="$(awk -v name="${archive}" '$2 == name || $2 == "*" name { print $1 }' "${checksums}")"
  if [[ ! "${sum}" =~ ^[0-9a-f]{64}$ ]]; then
    echo "error: ${checksums} has no single SHA-256 for ${archive}" >&2
    return 1
  fi
  printf '%s' "${sum}"
}

darwin_arm64="$(sha256_of darwin_arm64)"
darwin_amd64="$(sha256_of darwin_amd64)"
linux_arm64="$(sha256_of linux_arm64)"
linux_amd64="$(sha256_of linux_amd64)"

cat <<EOF
# Generated from the checksums of ork ${tag} by orca-ae/orca-cli's release workflow.
class Ork < Formula
  desc "CLI to manage & interact with resources in Orca Agent Engine"
  homepage "https://runorca.ai"
  version "${version}"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "${base_url}/ork_${tag}_darwin_arm64.tar.gz"
      sha256 "${darwin_arm64}"
    end
    on_intel do
      url "${base_url}/ork_${tag}_darwin_amd64.tar.gz"
      sha256 "${darwin_amd64}"
    end
  end

  on_linux do
    on_arm do
      url "${base_url}/ork_${tag}_linux_arm64.tar.gz"
      sha256 "${linux_arm64}"
    end
    on_intel do
      url "${base_url}/ork_${tag}_linux_amd64.tar.gz"
      sha256 "${linux_amd64}"
    end
  end

  def install
    bin.install "ork"
    generate_completions_from_executable(bin/"ork", "completion")
  end

  test do
    assert_equal "ork version #{version}", shell_output("#{bin}/ork --version").strip
  end
end
EOF
