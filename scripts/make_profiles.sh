#!/usr/bin/env bash
# Construit data/profiles.csv à partir des mesures brutes data/benchmark_<machine_type>.csv.
# Une ligne par (workload, size, machine_type) : moyenne, médiane et écart-type du runtime_s, RSS max.
# Usage : ./scripts/make_profiles.sh
source "$(dirname "$0")/config.sh"
OUT="data/profiles.csv"
TYPES=$(tail -n +2 "$MACHINES_FILE" | tr -d '\r' | cut -d, -f2 | sed '/^$/d' | sort -u)

for type in $TYPES; do
  if [ ! -f "data/benchmark_$type.csv" ]; then
    echo "Erreur : data/benchmark_$type.csv introuvable (lancer benchmark_vms.sh)" >&2
    exit 1
  fi
done

echo "workload,size,machine_type,runs,mean_runtime_s,median_runtime_s,std_runtime_s,max_rss_kb" > "$OUT"
for type in $TYPES; do
  # Tri par couple puis par temps : la médiane se lit au milieu de chaque groupe
  tail -n +2 "data/benchmark_$type.csv" | tr -d '\r' | sort -t, -k1,1 -k2,2n -k4,4g |
  awk -F, -v type="$type" '
    function flush() {
      if (n == 0) return
      mean = sum / n
      med = (n % 2) ? t[(n + 1) / 2] : (t[n / 2] + t[n / 2 + 1]) / 2
      var = 0
      for (i = 1; i <= n; i++) var += (t[i] - mean) ^ 2
      std = (n > 1) ? sqrt(var / (n - 1)) : 0
      printf "%s,%s,%s,%d,%.3f,%.3f,%.3f,%d\n", w, s, type, n, mean, med, std, rss
    }
    $4 == "" { next }
    $1 != w || $2 != s { flush(); w = $1; s = $2; n = 0; sum = 0; rss = 0 }
    { n++; t[n] = $4; sum += $4; if ($5 > rss) rss = $5 }
    END { flush() }'
done >> "$OUT"

cat "$OUT"
