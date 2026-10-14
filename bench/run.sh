#!/usr/bin/env bash
# Runs the k6 scenarios against one implementation and keeps their summaries.
#
#   bash bench/run.sh <name> [base url] [scenario...]
#
# e.g. bash bench/run.sh go http://localhost:8080. The summaries go to bench/results/<name>-<scenario>.json,
# which bench/report.py turns into a table. VUS and DURATION are passed on to k6.
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "usage: $0 <name> [base url] [scenario...]" >&2
  exit 2
fi
NAME="$1"
BASE_URL="${2:-http://localhost:8080}"
shift $(($# < 2 ? $# : 2))
SCENARIOS=("$@")
[ ${#SCENARIOS[@]} -eq 0 ] && SCENARIOS=(browse auth checkout)

DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$DIR/results"

VUS="${VUS:-20}"
DURATION="${DURATION:-60s}"

for scenario in "${SCENARIOS[@]}"; do
  echo "=== $NAME: $scenario ==="
  k6 run --quiet \
    -e BASE_URL="$BASE_URL" -e SCENARIO="$scenario" -e VUS="$VUS" -e DURATION="$DURATION" \
    --summary-export "$DIR/results/$NAME-$scenario.json" \
    "$DIR/k6/store.js"
  # k6 rates cover setup() too; the report divides by the scenario's own duration instead.
  printf '{"vus": %s, "duration": "%s"}\n' "$VUS" "$DURATION" > "$DIR/results/$NAME-$scenario.run.json"
done
