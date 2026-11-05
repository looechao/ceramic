#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec go run github.com/egoist/mygo/cmd/mygo@v0.7.4 dev
