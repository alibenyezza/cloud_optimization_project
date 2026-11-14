#!/usr/bin/env bash
# Agrège une expérience : results/<ID>/results.csv + une ligne dans data/experiments.csv
# Usage : ./scripts/collect_results.sh EXPERIMENT_ID STRATEGY [PRED_MAKESPAN_S] [PRED_COST_USD]
source "$(dirname "$0")/config.sh"
if [ "$#" -lt 2 ]; then
  echo "Usage: $0 EXPERIMENT_ID STRATEGY [PRED_MAKESPAN_S] [PRED_COST_USD]" >&2
  exit 2
fi
EXP="$1"; STRATEGY="$2"; PRED_MK="${3:-NA}"; PRED_COST="${4:-NA}"
DIR="results/$EXP"
EXPF="data/experiments.csv"
if ! ls "$DIR"/*.status > /dev/null 2>&1; then
  echo "Erreur : aucun .status dans $DIR" >&2
  exit 1
fi

# 1) Une ligne par tâche : status (7 col.) + runtime_s du JSON + machine_type + prix
TMP=$(mktemp)
for st in "$DIR"/*.status; do
  id=$(basename "$st" .status)
  line=$(tr -d '\r' < "$st")
  vm=$(echo "$line" | cut -d, -f4)
  rt=$(grep -o '"runtime_s":[0-9.eE+-]*' "$DIR/$id.out" 2>/dev/null | cut -d: -f2)
  mach=$(tr -d '\r' < "$MACHINES_FILE" | awk -F, -v vm="$vm" '$1 == vm {print $2 "," $5}')
  echo "$line,${rt:-NA},${mach:-NA,NA}" >> "$TMP"
done
# colonnes de TMP : 1 id, 2 workload, 3 size, 4 vm, 5 code, 6 start, 7 end, 8 runtime, 9 type, 10 prix

# Préparation moyenne d'une VM par machine_type (data/vm_setup.csv), payée en plus du makespan
SETUP=$(tail -n +2 data/vm_setup.csv 2> /dev/null | tr -d '\r' | awk -F, '
  { s[$3] += $7; n[$3]++; all += $7; c++ }
  END { for (t in s) printf "%s=%.3f;", t, s[t] / n[t]; if (c) printf "*=%.3f", all / c }')

# 2) results.csv (temps relatifs au premier début) + calcul du résumé
SUMMARY=$(awk -F, -v expid="$EXP" -v out="$DIR/results.csv" -v setup="$SETUP" '
  NR == FNR { if (min == "" || $6 < min) min = $6; next }
  FNR == 1  { print "experiment_id,task_id,workload,size,vm,machine_type,exit_code,start_s,end_s,duration_s,runtime_s" > out }
  {
    s = $6 - min; e = $7 - min
    printf "%s,%s,%s,%s,%s,%s,%s,%.3f,%.3f,%.3f,%s\n", expid, $1, $2, $3, $4, $9, $5, s, e, e - s, $8 > out
    if (e > fin[$4]) fin[$4] = e
    price[$4] = $10; type[$4] = $9
    if (e > mk) mk = e
    n++; if ($5 != 0) f++
  }
  END {
    k = split(setup, parts, ";")
    for (i = 1; i <= k; i++) if (split(parts[i], kv, "=") == 2) prep[kv[1]] = kv[2]
    cost = 0; na = 0
    for (v in fin) {
      p = (type[v] in prep) ? prep[type[v]] : prep["*"] + 0
      if (price[v] ~ /^[0-9.]+$/) cost += price[v] / 3600 * (p + mk); else na = 1
    }
    printf "%.3f,%s,%d,%d\n", mk, (na ? "NA" : sprintf("%.6f", cost)), n, f + 0
  }' "$TMP" "$TMP")
rm -f "$TMP"
IFS=, read -r MK COST N F <<< "$SUMMARY"
if [ -z "$MK" ]; then echo "Erreur : calcul impossible, rien ajouté à $EXPF" >&2; exit 1; fi

# 3) Ligne dans experiments.csv (jamais en double)
[ -f "$EXPF" ] || echo "experiment_id,strategy,makespan_s,cost_usd,pred_makespan_s,pred_cost_usd,n_tasks,n_failed" > "$EXPF"
if grep -q "^$EXP," "$EXPF"; then
  echo "Attention : $EXP est déjà dans $EXPF, ligne non ajoutée." >&2
else
  echo "$EXP,$STRATEGY,$MK,$COST,$PRED_MK,$PRED_COST,$N,$F" >> "$EXPF"
fi

echo "Expérience $EXP ($STRATEGY) : $N tâches, $F échec(s), makespan = $MK s, coût = $COST USD"
awk -F, 'NR > 1 { if ($9 > f[$5]) f[$5] = $9 } END { for (v in f) printf "  %s : fin à %.2f s\n", v, f[v] }' "$DIR/results.csv" | sort
