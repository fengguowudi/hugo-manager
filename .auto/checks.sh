#!/bin/bash
set -euo pipefail
export PATH="/d/Git/usr/bin:/d/Git/mingw64/bin:/usr/bin:$PATH"
cd "$(dirname "$0")/.."
go vet ./core
go build ./core
# 有 gcc 时全量验证主包（Fyne/CGO）；否则只做语法检查。
export PATH="/c/Users/Administrator/scoop/apps/mingw/current/bin:$PATH"
if command -v gcc >/dev/null 2>&1; then
  CGO_ENABLED=1 go vet .
  CGO_ENABLED=1 go build -o "${TEMP:-/tmp}/hugo-manager-check.exe" .
  CGO_ENABLED=1 go test -count=1 -run TestMarkdownHelpers . 2>&1 | tail -2
else
  gofmt -e main.go native_ui.go >/dev/null
fi
