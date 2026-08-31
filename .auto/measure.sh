#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
go test -count=1 -v -run TestFixItScore ./core 2>&1 | grep -E "^METRIC "
