#!/usr/bin/env bash
# TD1/TD2 — Campagne complète : chaque workload, 3 tailles, RUNS répétitions.
#
# Usage : ./scripts/benchmark_campaign.sh [CSV] [RUNS]
#   local (TD1) : ./scripts/benchmark_campaign.sh data/benchmark_local.csv 3
#   VM (TD2)    : ./scripts/benchmark_campaign.sh data/benchmark_cloud_vm1.csv 3
# Le CSV est recréé à chaque campagne pour ne pas mélanger deux séries.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1

CSV="${1:-data/benchmark_local.csv}"
RUNS="${2:-3}"

# workload  small  medium  large
CAMPAIGN="
cpu_task    200000  500000  1000000
memory_task 128     256     512
matrix_task 200     400     600
sort_task   1000000 5000000 10000000
"

rm -f "$CSV"
echo "$CAMPAIGN" | while read -r workload sizes; do
    [ -z "$workload" ] && continue
    for size in $sizes; do
        ./scripts/benchmark.sh "$workload" "$size" "$RUNS" "$CSV" < /dev/null || exit 1
    done
done || exit 1

echo "Campaign saved to $CSV ($(($(wc -l < "$CSV") - 1)) runs)"
