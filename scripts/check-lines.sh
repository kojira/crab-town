#!/usr/bin/env bash
# Fail if any tracked source file (*.go, *.html, *.js, *.css; go.sum excluded)
# has more than MAX_LINES lines (default 800).
set -euo pipefail
MAX_LINES="${MAX_LINES:-800}"
cd "$(git rev-parse --show-toplevel)"
fail=0
while IFS= read -r -d '' f; do
  [ "$(basename "$f")" = "go.sum" ] && continue
  [ -f "$f" ] || continue
  n=$(wc -l < "$f" | tr -d ' ')
  if [ "$n" -gt "$MAX_LINES" ]; then
    echo "FAIL: $f has $n lines (max $MAX_LINES)"
    fail=1
  else
    echo "ok:   $f $n"
  fi
done < <(git ls-files -z -- '*.go' '*.html' '*.js' '*.css')
exit "$fail"
