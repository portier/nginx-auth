#!/usr/bin/env bash

# Uploads release packages created by `build-release.sh` to the matching
# release on GitHub.

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo >&2 "Usage: $0 VERSION"
  exit 64
fi

version="$1"

set -x
gh release upload "v${version}" \
  ./release/portier-nginx-auth-v${version}-*.{tar.gz,zip}
