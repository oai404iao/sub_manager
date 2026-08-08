#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
version=${1:-$(cat "$root/VERSION")}
version=${version#v}

if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid release version: $version" >&2
  exit 1
fi
if [[ $(cat "$root/VERSION") != "$version" ]]; then
  echo "VERSION does not match $version" >&2
  exit 1
fi
if [[ $(node -p "require('$root/web/package.json').version") != "$version" ]]; then
  echo "web/package.json does not match $version" >&2
  exit 1
fi

commit=${COMMIT:-$(git -C "$root" rev-parse HEAD 2>/dev/null || echo unknown)}
build_date=${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
dist="$root/dist"
package="sub-manager_${version}_linux_amd64"

rm -rf "${dist:?}"
mkdir -p "$dist/$package"

npm --prefix "$root/web" run build

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go -C "$root" build -trimpath \
    -ldflags="-s -w \
      -X github.com/oai404iao/sub_manager/internal/version.Version=$version \
      -X github.com/oai404iao/sub_manager/internal/version.Commit=$commit \
      -X github.com/oai404iao/sub_manager/internal/version.BuildDate=$build_date" \
    -o "$dist/$package/sub-manager" ./cmd/server

cp "$root/README.md" "$dist/$package/"
tar -C "$dist" -czf "$dist/$package.tar.gz" "$package"
rm -rf "${dist:?}/$package"
(
  cd "$dist"
  sha256sum "$package.tar.gz" > SHA256SUMS
)

echo "Release artifacts written to $dist."
