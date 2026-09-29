#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT

cat >"${WORK}/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == "api repos/example/cli/git/matching-refs/tags/v0.1.0" ]]; then
  if [[ "${LOOKUP_FAIL:-false}" == true ]]; then
    exit 1
  fi
  printf '%s\n' "${REFS}"
elif [[ "$*" == "api --method POST repos/example/cli/git/refs -f ref=refs/tags/v0.1.0 -f sha=${REVISION}" ]]; then
  echo created >>"${CALLS}"
else
  echo "unexpected gh arguments: $*" >&2
  exit 1
fi
EOF
chmod +x "${WORK}/gh"
export PATH="${WORK}:${PATH}"
export GITHUB_REPOSITORY=example/cli GH_TOKEN=test-only-token
export REVISION=0123456789abcdef0123456789abcdef01234567
export CALLS="${WORK}/calls"
export REFS='[]'
script="${ROOT}/scripts/create-release-tag.sh"

"${script}" v0.1.0 "${REVISION}"
test "$(cat "${CALLS}")" = created
rm "${CALLS}"

REFS="$(jq -nc --arg sha "${REVISION}" '[{ref:"refs/tags/v0.1.0",object:{sha:$sha}}]')"
"${script}" v0.1.0 "${REVISION}"
test ! -e "${CALLS}"

export REFS='[{"ref":"refs/tags/v0.1.0","object":{"sha":"different"}}]'
if "${script}" v0.1.0 "${REVISION}" >/dev/null 2>&1; then
  echo "expected conflicting tag to fail" >&2
  exit 1
fi
test ! -e "${CALLS}"

export LOOKUP_FAIL=true
if "${script}" v0.1.0 "${REVISION}" >/dev/null 2>&1; then
  echo "expected API failure to fail closed" >&2
  exit 1
fi
test ! -e "${CALLS}"
unset LOOKUP_FAIL

# A prefix match is not the release tag itself.
export REFS='[{"ref":"refs/tags/v0.1.0-rc.1","object":{"sha":"different"}}]'
"${script}" v0.1.0 "${REVISION}"
test "$(cat "${CALLS}")" = created
rm "${CALLS}"

if "${script}" invalid-tag "${REVISION}" >/dev/null 2>&1; then
  echo "expected invalid tag to fail" >&2
  exit 1
fi
test ! -e "${CALLS}"
echo 'Release tag tests passed'
