#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
version=${1:-}

if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: $0 <major.minor.patch>" >&2
  exit 1
fi

printf '%s\n' "$version" > "$root/VERSION"
npm --prefix "$root/web" version "$version" --no-git-tag-version --allow-same-version >/dev/null

echo "Version updated to $version."
