#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
npm --prefix CoreServer/web ci
npm --prefix CoreServer/web run build
mkdir -p bin
go build -o bin/studio.exe ./cmd/studio
go build -o bin/oc.exe ./cmd/oc
go build -o bin/l2d.exe ./cmd/l2d
printf '\nBuild complete. Run ./bin/studio.exe from the project root.\n'
