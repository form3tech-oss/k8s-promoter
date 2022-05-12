#!/usr/bin/env bash
set -euo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

version="$1"
readonly version

os=$(uname -s | tr '[:upper:]' '[:lower:]')
readonly os

asset="golangci-lint-$version-$os-amd64"
readonly asset

url="https://github.com/golangci/golangci-lint/releases/download/v${version}/$asset.tar.gz"
readonly url

tarball="$(mktemp)"
readonly tarball

if [[ -x tools/golangci-lint && $(tools/golangci-lint version) == *"$version"* ]]; then
  echo "$version already installed"
  exit 0
fi

mkdir -p tools/
curl -sSfL "$url" -o "$tarball"
tar xf "$tarball" -C tools --strip-components 1 "$asset/golangci-lint"
rm -rf "$tarball"
tools/golangci-lint version
