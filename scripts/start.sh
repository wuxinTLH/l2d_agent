#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ ! -f CoreServer/web/dist/index.html ]]; then
  printf 'Build first: bash scripts/build.sh\n' >&2
  exit 1
fi
go run ./cmd/studio "$@"
