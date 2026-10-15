#!/usr/bin/env bash
# TD1 — Lance un workload plusieurs fois et ajoute une ligne CSV par exécution.
#
# Usage : ./scripts/benchmark.sh WORKLOAD SIZE RUNS [CSV]
# Ex.   : ./scripts/benchmark.sh cpu_task 500000 3
#   CSV par défaut : data/benchmark_local.csv (créé avec l'en-tête s'il est absent)
#
# Colonnes : workload,size,run,runtime_s,max_rss_kb
#   runtime_s   temps réel (wall-clock) mesuré autour du processus
#   max_rss_kb  pic de mémoire physique du processus, mesuré par GNU time (%M)
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1

if [ "$#" -lt 3 ] || [ "$#" -gt 4 ]; then
    echo "usage: $0 WORKLOAD SIZE RUNS [CSV]" >&2
    exit 2
fi

WORKLOAD=$1
SIZE=$2
RUNS=$3
CSV="${4:-data/benchmark_local.csv}"
BIN="bin/$WORKLOAD"

if [ ! -x "$BIN" ]; then
    echo "error: executable $BIN not found (compile it first)" >&2
    exit 1
fi
if ! [[ "$SIZE" =~ ^[0-9]+$ ]]; then
    echo "error: size must be a positive integer, got '$SIZE'" >&2
    exit 2
fi
if ! [[ "$RUNS" =~ ^[1-9][0-9]*$ ]]; then
    echo "error: runs must be >= 1, got '$RUNS'" >&2
    exit 2
fi
if [ ! -x /usr/bin/time ]; then
    echo "error: GNU time not found (sudo apt install time)" >&2
    exit 1
fi

mkdir -p "$(dirname "$CSV")"
if [ ! -s "$CSV" ]; then
    echo "workload,size,run,runtime_s,max_rss_kb" > "$CSV"
fi

TIME_OUT="$(mktemp)"
trap 'rm -f "$TIME_OUT"' EXIT

for run in $(seq 1 "$RUNS"); do
    start=$(date +%s%N)
    if ! /usr/bin/time -f "%M" -o "$TIME_OUT" "./$BIN" --size "$SIZE" > /dev/null; then
        echo "error: $WORKLOAD --size $SIZE failed on run $run" >&2
        exit 1
    fi
    end=$(date +%s%N)
    runtime=$(awk -v s="$start" -v e="$end" 'BEGIN { printf "%.4f", (e - s) / 1e9 }')
    rss=$(tail -n 1 "$TIME_OUT")
    echo "$WORKLOAD,$SIZE,$run,$runtime,$rss" >> "$CSV"
    echo "$WORKLOAD size=$SIZE run=$run runtime=${runtime}s max_rss=${rss}KB"
done
