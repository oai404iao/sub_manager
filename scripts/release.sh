#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
version=${1:-$(cat "$root/VERSION")}
version=${version#v}
tag="v$version"

if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid release version: $version" >&2
  exit 1
fi
if [[ $(cat "$root/VERSION") != "$version" ]]; then
  echo "VERSION does not match $version; run scripts/set-version.sh first" >&2
  exit 1
fi
if [[ $(node -p "require('$root/web/package.json').version") != "$version" ]]; then
  echo "web/package.json does not match $version" >&2
  exit 1
fi
if [[ $(git -C "$root" branch --show-current) != main ]]; then
  echo "releases must be created from main" >&2
  exit 1
fi
if [[ -n $(git -C "$root" status --porcelain) ]]; then
  echo "working tree must be clean" >&2
  exit 1
fi
if ! git -C "$root" remote get-url origin >/dev/null 2>&1; then
  echo "origin remote is not configured" >&2
  exit 1
fi

git -C "$root" fetch --prune origin main --tags
if [[ $(git -C "$root" rev-parse HEAD) != $(git -C "$root" rev-parse origin/main) ]]; then
  echo "local main must exactly match origin/main" >&2
  exit 1
fi
if git -C "$root" rev-parse --verify --quiet "refs/tags/$tag" >/dev/null; then
  echo "tag $tag already exists locally" >&2
  exit 1
fi
if git -C "$root" ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1; then
  echo "tag $tag already exists on origin" >&2
  exit 1
fi

npm --prefix "$root/web" ci
make -C "$root" ci
"$root/scripts/build-release.sh" "$version"
if [[ -n $(git -C "$root" status --porcelain) ]]; then
  echo "checks or builds changed tracked files; review them before releasing" >&2
  exit 1
fi

git -C "$root" tag -a "$tag" -m "Release $tag"
git -C "$root" push origin "$tag"

echo "Pushed $tag. The GitHub release workflow will publish binaries and the GHCR image."
