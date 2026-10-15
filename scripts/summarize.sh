#!/usr/bin/env bash
# Analyse d'un CSV de benchmark : une ligne par (workload, size), en Markdown.
#
# Usage : ./scripts/summarize.sh data/benchmark_local.csv
# Colonnes : runs, temps moyen, écart-type, min, max, RSS max moyenne (Mo).
set -u

if [ "$#" -ne 1 ] || [ ! -f "$1" ]; then
    echo "usage: $0 BENCHMARK_CSV" >&2
    exit 2
fi

awk -F, '
    NR == 1 { next }
    {
        key = $1 "," $2
        if (!(key in n)) order[++count] = key
        n[key]++
        sum[key] += $4
        sq[key] += $4 * $4
        rss[key] += $5
        if (!(key in lo) || $4 < lo[key]) lo[key] = $4
        if (!(key in hi) || $4 > hi[key]) hi[key] = $4
    }
    END {
        print "| workload | size | runs | mean_s | std_s | min_s | max_s | max_rss_mb |"
        print "|---|---:|---:|---:|---:|---:|---:|---:|"
        for (i = 1; i <= count; i++) {
            k = order[i]
            split(k, f, ",")
            mean = sum[k] / n[k]
            var = sq[k] / n[k] - mean * mean
            if (var < 0) var = 0
            printf "| %s | %s | %d | %.3f | %.3f | %.3f | %.3f | %.1f |\n",
                f[1], f[2], n[k], mean, sqrt(var), lo[k], hi[k], rss[k] / n[k] / 1024
        }
    }
' "$1"
