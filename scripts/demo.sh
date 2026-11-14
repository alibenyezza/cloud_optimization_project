#!/usr/bin/env bash
# Démo live de la soutenance, environ 2 min.
# Scénario à dépendances -> scheduler -> orchestrateur -> Gantt avec flèches.
# Les VMs doivent être allumées avant (setup_vms.sh), sinon on perd ~45 s en live.
#
# Usage : ./scripts/demo.sh [TASKS.csv]
# Si data/allocation_demo_nsga.csv existe, on l'exécute aussi pour comparer avec Round-Robin.
source "$(dirname "$0")/config.sh"
TASKS="${1:-data/tasks_demo_dag.csv}"
[ -f "$TASKS" ] || { echo "Erreur : $TASKS introuvable." >&2; exit 1; }

titre() { printf '\n\033[1;34m== %s ==\033[0m\n' "$*"; }
tableau() { if command -v column > /dev/null; then column -s, -t; else sed 's/,/   /g'; fi; }
ouvrir() { command -v explorer.exe > /dev/null && explorer.exe "$(wslpath -w "$1")" 2> /dev/null; }

titre "1. Les VMs"
gcloud compute instances list --zones="$ZONE" --format="table(name,machineType.basename(),status)" < /dev/null

titre "2. Les tâches (depends_on = ce qu'il faut attendre)"
tableau < "$TASKS"

titre "3. Le scheduler choisit la VM de chaque tâche"
./bin/round_robin_scheduler "$TASKS" "$MACHINES_FILE" data/allocation_demo_rr.csv > /dev/null || exit 1
tableau < data/allocation_demo_rr.csv

# compteur de tâches finies pendant l'exécution
suivi() {
  local strat="$1" n="$2" dir
  while :; do
    dir=$(ls -td results/*_"$strat"_r1 2> /dev/null | head -1)
    if [ -n "$dir" ]; then
      local fini=$(ls "$dir"/*.status 2> /dev/null | wc -l)
      printf '\r  %d/%d tâches finies (%d s)   ' "$fini" "$n" "$SECONDS"
      [ "$fini" -ge "$n" ] && return
    fi
    sleep 1
  done
}

lancer() {
  local alloc="$1" strat="$2"
  local n=$(tail -n +2 "$alloc" | sed '/^\s*$/d' | wc -l)
  titre "4. L'orchestrateur exécute ($strat)"
  SECONDS=0
  suivi "$strat" "$n" &
  local pid=$!
  DAG="$TASKS" ./scripts/run_experiment.sh "$alloc" "$strat" 1 > /tmp/demo_$strat.log 2>&1
  kill "$pid" 2> /dev/null; wait "$pid" 2> /dev/null; echo
  local dir=$(ls -td results/*_"$strat"_r1 2> /dev/null | head -1)
  grep -E "makespan =" /tmp/demo_$strat.log | head -1

  titre "5. Preuve : chaque tâche a tourné sur la VM prévue"
  while IFS=, read -r id w s vm; do
    [ -z "$id" ] && continue
    reel=$(head -1 "$dir/$id.out" 2> /dev/null | cut -d' ' -f2 | cut -d. -f1)
    [ "$reel" = "$vm" ] && ok="OK" || ok="NON"
    printf '  %-4s prévue %-13s  vue sur %-13s %s\n' "$id" "$vm" "${reel:-?}" "$ok"
  done < <(tail -n +2 "$alloc" | tr -d '\r')

  echo "  Gantt : $dir/gantt.svg"
  ouvrir "$dir/gantt.svg"
}

lancer data/allocation_demo_rr.csv demo_rr
[ -f data/allocation_demo_nsga.csv ] && lancer data/allocation_demo_nsga.csv demo_nsga

titre "6. Bilan"
{ head -1 data/experiments.csv; grep -E '_demo_(rr|nsga)_r1,' data/experiments.csv | tail -2; } | cut -d, -f1-4,7,8 | tableau
echo
echo "Ne pas oublier : ./scripts/delete_vms.sh"
