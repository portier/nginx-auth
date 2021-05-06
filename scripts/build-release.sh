#!/usr/bin/env bash

# Creates release packages for all supported platforms.

set -euo pipefail

if [[ $# -ne 1 ]]; then
	echo >&2 "Usage: $0 VERSION"
	exit 64
fi

version="$1"

# Build variants.
#
# PKGARCH here matches Docker naming on Linux, for convenience when used
# together with `upload-docker.sh`.
build_targets=(
	"GOOS=darwin  GOARCH=amd64       PKGARCH=amd64   "
	"GOOS=darwin  GOARCH=arm64       PKGARCH=arm64   "
	"GOOS=freebsd GOARCH=386         PKGARCH=386     "
	"GOOS=freebsd GOARCH=amd64       PKGARCH=amd64   "
	"GOOS=linux   GOARCH=386         PKGARCH=386     "
	"GOOS=linux   GOARCH=amd64       PKGARCH=amd64   "
	"GOOS=linux   GOARCH=arm GOARM=5 PKGARCH=arm32v5 "
	"GOOS=linux   GOARCH=arm GOARM=6 PKGARCH=arm32v6 "
	"GOOS=linux   GOARCH=arm GOARM=7 PKGARCH=arm32v7 "
	"GOOS=linux   GOARCH=arm64       PKGARCH=arm64v8 "
	"GOOS=linux   GOARCH=mips64le    PKGARCH=mips64le"
	"GOOS=linux   GOARCH=ppc64le     PKGARCH=ppc64le "
	"GOOS=linux   GOARCH=s390x       PKGARCH=s390x   "
	"GOOS=openbsd GOARCH=386         PKGARCH=386     "
	"GOOS=openbsd GOARCH=amd64       PKGARCH=amd64   "
	"GOOS=solaris GOARCH=amd64       PKGARCH=amd64   "
	"GOOS=windows GOARCH=386         PKGARCH=386     "
	"GOOS=windows GOARCH=amd64       PKGARCH=amd64   "
)

export CGO_ENABLED=0

for build_target in "${build_targets[@]}"; do
	eval export ${build_target}
	pkgname="portier-nginx-auth-v${version}-${GOOS}-${PKGARCH}"

	rm -fr "./release/${pkgname}"
	mkdir -p "./release/${pkgname}"

	echo "- Building ${pkgname}"
	if [[ "${GOOS}" = "windows" ]]; then
		go build -o "./release/${pkgname}/portier-nginx-auth.exe" .
		(cd ./release/ && zip -qr9 "./${pkgname}.zip" "./${pkgname}")
	else
		go build -o "./release/${pkgname}/portier-nginx-auth" .
		(cd ./release/ && tar -czf "${pkgname}.tar.gz" "./${pkgname}")
	fi
done
