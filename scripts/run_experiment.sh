#!/usr/bin/env bash
# Lance une expérience complète, éventuellement répétée : exécution + collecte + timeline.
# Usage : ./scripts/run_experiment.sh ALLOCATION.csv STRATEGY [REPETITIONS] [PRED_MAKESPAN_S] [PRED_COST_USD]
# Exemple : ./scripts/run_experiment.sh data/allocation_rr.csv rr 3
# File d'attente par VM (run_all_queue.sh) : QUEUE=1 ./scripts/run_experiment.sh data/allocation_rr.csv rr 3
source "$(dirname "$0")/config.sh"
if [ "$#" -lt 2 ]; then
  echo "Usage: $0 ALLOCATION.csv STRATEGY [REPETITIONS] [PRED_MAKESPAN_S] [PRED_COST_USD]" >&2
  exit 2
fi
ALLOC="$1"; STRATEGY="$2"; REP="${3:-1}"; PRED_MK="${4:-NA}"; PRED_COST="${5:-NA}"

if [ ! -f "$ALLOC" ]; then echo "Erreur : $ALLOC introuvable." >&2; exit 1; fi
if ! [[ "$STRATEGY" =~ ^[A-Za-z0-9_-]+$ ]]; then
  echo "Erreur : STRATEGY doit contenir seulement lettres, chiffres, _ ou - (ex. rr, nsga_knee)." >&2; exit 1
fi
if ! [[ "$REP" =~ ^[0-9]+$ ]] || [ "$REP" -lt 1 ]; then
  echo "Erreur : REPETITIONS doit être un entier >= 1." >&2; exit 1
fi
if [ ! -x bin/gantt ]; then go build -o bin/gantt tools/gantt/main.go || exit 1; fi

STAMP=$(date +%Y%m%d-%H%M)
IDS=""
for r in $(seq 1 "$REP"); do
  EXP="${STAMP}_${STRATEGY}_r${r}"
  echo "=============== $EXP ($r/$REP) ==============="
  if [ "${QUEUE:-0}" = 1 ]; then ./scripts/run_all_queue.sh "$ALLOC" "$EXP"; else ./scripts/run_all.sh "$ALLOC" "$EXP"; fi
  CODE=$?
  if [ ! -d "results/$EXP" ]; then
    echo "Erreur : l'expérience $EXP n'a pas pu démarrer (code $CODE), arrêt." >&2; exit 1
  fi
  ./scripts/collect_results.sh "$EXP" "$STRATEGY" "$PRED_MK" "$PRED_COST" || exit 1
  ./bin/gantt "results/$EXP/results.csv" "results/$EXP/gantt.svg"
  IDS="$IDS $EXP"
  [ "$r" -lt "$REP" ] && sleep 5      # laisse les VMs revenir au repos entre deux répétitions
done

echo "=============== Résumé ==============="
head -1 data/experiments.csv
for id in $IDS; do grep "^$id," data/experiments.csv; done
