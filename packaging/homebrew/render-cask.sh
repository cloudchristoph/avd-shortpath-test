#!/bin/sh
# Usage: render-cask.sh <version> <macos.tgz> <linux-amd64.tgz> <linux-arm64.tgz>
# Prints the cask with version and checksums filled in to stdout.
set -eu

if [ "$#" -ne 4 ]; then
  echo "usage: $0 <version> <macos.tgz> <linux-amd64.tgz> <linux-arm64.tgz>" >&2
  exit 2
fi

dir=$(dirname "$0")
sum() { shasum -a 256 "$1" | cut -d' ' -f1; }

sed \
  -e "s/@VERSION@/$1/" \
  -e "s/@SHA256_MACOS@/$(sum "$2")/" \
  -e "s/@SHA256_LINUX@/$(sum "$3")/" \
  -e "s/@SHA256_LINUX_ARM64@/$(sum "$4")/" \
  "$dir/avd-shortpath-test.rb.in"
