#!/usr/bin/env bash
# Campagne finale : Round-Robin contre NSGA-II, sur 2 scénarios, REP répétitions chacun.
#   1. 12 tâches sans dépendances (data/tasks.csv), mode file par VM (QUEUE=1)
#   2. 40 tâches avec dépendances (data/tasks_40_dag.csv), mode dépendances (DAG=)
# Pour chaque scénario : allocations RR et NSGA-II, prédictions (tools/predict), puis exécution.
# Les VMs sont créées au début et supprimées à la fin, même en cas d'erreur ou de Ctrl+C.
# Allocations et prédictions gardées dans data/campagne/ ; data/pareto/ n'est pas modifié.
#
# Usage : ./scripts/campaign.sh [REP] [SEED]      (défaut : 3 répétitions, graine 42)
# Durée : environ 35 min. Coût : environ 0,15 USD.
source "$(dirname "$0")/config.sh"
REP="${1:-3}"
SEED="${2:-42}"
OUT="data/campagne"
mkdir -p "$OUT" bin

echo "== Compilation"
for w in cpu_task memory_task matrix_task sort_task; do go build -o bin/$w workloads/$w.go || exit 1; done
go build -o bin/round_robin_scheduler scheduler/round_robin.go || exit 1
go build -o bin/nsga2 scheduler/nsga2.go || exit 1
go build -o bin/predict tools/predict/main.go || exit 1

# Prépare RR et NSGA-II pour un scénario : $1 = nom court, $2 = tasks.csv
preparer() {
  local nom="$1" tasks="$2"
  ./bin/round_robin_scheduler "$tasks" "$MACHINES_FILE" "$OUT/allocation_${nom}_rr.csv" > /dev/null || return 1
  # nsga2 écrit dans data/pareto/ et data/allocation_nsga.csv : on sauvegarde puis on remet en place
  local sauve=$(mktemp -d)
  cp -r data/pareto "$sauve/" 2> /dev/null; cp data/allocation_nsga.csv "$sauve/" 2> /dev/null
  ./bin/nsga2 "$tasks" "$MACHINES_FILE" data/profiles.csv "$SEED" > "$OUT/nsga_${nom}.txt" || return 1
  cp data/allocation_nsga.csv "$OUT/allocation_${nom}_nsga.csv"
  rm -rf data/pareto; cp -r "$sauve/pareto" data/ 2> /dev/null; cp "$sauve/allocation_nsga.csv" data/ 2> /dev/null
  rm -rf "$sauve"
  for s in rr nsga; do
    ./bin/predict "$tasks" "$MACHINES_FILE" data/profiles.csv "$OUT/allocation_${nom}_$s.csv" \
      | tail -1 > "$OUT/prediction_${nom}_$s.txt" || return 1
  done
  echo "  $nom : RR prévu $(cut -d, -f2 "$OUT/prediction_${nom}_rr.txt") s, NSGA-II prévu $(cut -d, -f2 "$OUT/prediction_${nom}_nsga.txt") s"
}

echo "== Allocations et prédictions"
preparer s12 data/tasks.csv || { echo "Erreur de préparation (12 tâches)" >&2; exit 1; }
preparer s40dag data/tasks_40_dag.csv || { echo "Erreur de préparation (40 tâches)" >&2; exit 1; }

# À partir d'ici les VMs coûtent : suppression garantie à la sortie
trap './scripts/delete_vms.sh' EXIT
echo "== Création des VMs"
./scripts/setup_vms.sh - || exit 1

lancer() {   # $1 = nom du scénario, $2 = stratégie, $3 = mode (QUEUE ou fichier tasks pour DAG)
  local nom="$1" s="$2" mode="$3"
  local mk=$(cut -d, -f2 "$OUT/prediction_${nom}_$s.txt") cost=$(cut -d, -f3 "$OUT/prediction_${nom}_$s.txt")
  if [ "$mode" = QUEUE ]; then
    QUEUE=1 ./scripts/run_experiment.sh "$OUT/allocation_${nom}_$s.csv" "${nom}_$s" "$REP" "$mk" "$cost"
  else
    DAG="$mode" ./scripts/run_experiment.sh "$OUT/allocation_${nom}_$s.csv" "${nom}_$s" "$REP" "$mk" "$cost"
  fi
}

echo "== Scénario 1 : 12 tâches, file par VM"
lancer s12 rr QUEUE
lancer s12 nsga QUEUE
echo "== Scénario 2 : 40 tâches avec dépendances"
lancer s40dag rr data/tasks_40_dag.csv
lancer s40dag nsga data/tasks_40_dag.csv

echo
echo "== Bilan de la campagne (data/experiments.csv)"
{ head -1 data/experiments.csv; grep -E '_s(12|40dag)_(rr|nsga)_r[0-9]+,' data/experiments.csv; } | cut -d, -f2-8 | sed 's/,/\t/g'
