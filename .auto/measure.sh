#!/bin/bash
set -euo pipefail
export PATH="/d/Git/usr/bin:/d/Git/mingw64/bin:/usr/bin:$PATH"
export PATH="/c/Users/Administrator/scoop/apps/mingw/current/bin:$PATH"
cd "$(dirname "$0")/.."
core_out=$(go test -count=1 -v -run TestFixItScore ./core 2>&1)
ui_out=$(CGO_ENABLED=1 go test -count=1 -v -run TestFixItUI . 2>&1)
core=$(echo "$core_out" | grep -oE "fixit_core=[0-9]+" | cut -d= -f2)
ui=$(echo "$ui_out" | grep -oE "fixit_ui=[0-9]+" | cut -d= -f2)
echo "METRIC fixit_score=$((core + ui))"
echo "METRIC fixit_total=78"
echo "METRIC fixit_core=$core"
echo "METRIC fixit_ui=$ui"
