#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0
#
# Checks that every tracked source file starts with the project license header,
# written in the file's own comment syntax:
#
#   // Copyright The Orca Authors
#   // SPDX-License-Identifier: Apache-2.0
#
# A shebang line stays above the header. Run with --fix to add missing headers;
# --fix also replaces an older one-line copyright notice in the header position.
set -euo pipefail

fix=false
case "${1:-}" in
  --fix) fix=true ;;
  "") ;;
  *)
    echo "usage: $0 [--fix]" >&2
    exit 2
    ;;
esac

cd "$(git rev-parse --show-toplevel)"

# comment_prefix prints the line-comment marker for a file, or nothing for
# files that are not checked (Markdown, JSON, go.mod, go.sum, and so on).
comment_prefix() {
  case "$1" in
    *.go) echo '//' ;;
    *.sql) echo '--' ;;
    *.sh | *.py | *.yml | *.yaml | Dockerfile | */Dockerfile) echo '#' ;;
  esac
}

missing=0
while IFS= read -r -d '' file; do
  [[ -f "$file" && ! -L "$file" ]] || continue
  prefix="$(comment_prefix "$file")"
  [[ -n "$prefix" ]] || continue

  copyright="$prefix Copyright The Orca Authors"
  spdx="$prefix SPDX-License-Identifier: Apache-2.0"
  offset=0
  if [[ "$(head -n 1 "$file")" == '#!'* ]]; then
    offset=1
  fi
  if [[ "$(sed -n "$((offset + 1))p" "$file")" == "$copyright" &&
    "$(sed -n "$((offset + 2))p" "$file")" == "$spdx" ]]; then
    continue
  fi

  if ! $fix; then
    echo "$file: missing license header" >&2
    missing=1
    continue
  fi

  tmp="$(mktemp)"
  awk -v copyright="$copyright" -v spdx="$spdx" -v old="$prefix Copyright" -v offset="$offset" '
    NR <= offset { print; next }
    state == "" {
      print copyright
      print spdx
      if (index($0, old) == 1) { state = "replaced"; next }
      print ""
      state = "body"
      if ($0 != "") print
      next
    }
    state == "replaced" {
      print ""
      state = "body"
      if ($0 != "") print
      next
    }
    { print }
    END {
      if (state == "") { print copyright; print spdx }
    }
  ' "$file" >"$tmp"
  chmod "$(stat -c '%a' "$file" 2>/dev/null || stat -f '%Lp' "$file")" "$tmp"
  mv "$tmp" "$file"
  echo "added license header: $file"
done < <(git ls-files -z)

if [[ "$missing" -ne 0 ]]; then
  echo "Run scripts/check-license-headers.sh --fix to add the missing headers." >&2
  exit 1
fi
