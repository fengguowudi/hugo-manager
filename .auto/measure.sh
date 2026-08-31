#!/bin/bash
set -euo pipefail
export PATH="/d/Git/usr/bin:/d/Git/mingw64/bin:/usr/bin:$PATH"
cd "$(dirname "$0")/.."
go test -count=1 -v -run TestFixItScore ./core 2>&1 | grep -E "^METRIC "
