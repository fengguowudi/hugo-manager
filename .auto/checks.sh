#!/bin/bash
set -euo pipefail
export PATH="/d/Git/usr/bin:/d/Git/mingw64/bin:/usr/bin:$PATH"
cd "$(dirname "$0")/.."
go vet ./core
go build ./core
# 主包依赖 CGO（Fyne），本机无 gcc 无法编译；至少做语法检查。
gofmt -e main.go native_ui.go >/dev/null
