#!/bin/bash
# Builds for this machine; pass -platform to build for others, such as
# ./scripts/build.sh -platform windows/amd64,linux/amd64
set -euo pipefail
cd "$(dirname "$0")/.."
exec go tool mygo build "$@"
