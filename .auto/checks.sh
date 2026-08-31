#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
go vet ./core
go build ./core
# 主包依赖 CGO（Fyne），本机无 gcc 无法编译；至少做语法检查。
gofmt -e main.go native_ui.go >/dev/null
