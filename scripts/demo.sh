#!/usr/bin/env bash
# Démo live pour la soutenance : toute la chaîne en ~1 à 2 min, sur un PETIT scénario.
#   scheduler -> allocation.csv -> orchestrateur (3 VMs en parallèle) -> results.csv + gantt.svg
# Les vrais résultats (gros scénario, 3 répétitions) sont calculés AVANT la soutenance :
# en live on prouve seulement que la chaîne marche de bout en bout.
#
# Prérequis : VMs créées et binaires déployés (create_vms.sh, deploy_all.sh), bin/ compilé.
# Usage     : ./scripts/demo.sh [TASKS_DEMO.csv]
# Optionnel : si data/allocation_demo_nsga.csv existe (NSGA-II lancé sur le même scénario),
#             il est exécuté juste après Round-Robin, pour comparer les deux timelines.
source "$(dirname "$0")/config.sh"
TASKS="${1:-data/tasks_demo.csv}"
[ -f "$TASKS" ] || { echo "Erreur : $TASKS introuvable." >&2; exit 1; }

tableau() { if command -v column > /dev/null; then column -s, -t; else sed 's/,/   /g'; fi; }
titre() { printf '\n\033[1;34m=== %s ===\033[0m\n' "$*"; }

titre "1. Les VMs (elles tournent, elles sont différentes)"
gcloud compute instances list --zones="$ZONE" --format="table(name,machineType.basename(),status)" < /dev/null

titre "2. Scheduler : il décide, il ne lance rien"
./bin/round_robin_scheduler "$TASKS" "$MACHINES_FILE" data/allocation_demo_rr.csv || exit 1
tableau < data/allocation_demo_rr.csv

# Affiche l'avancement pendant l'exécution : tâches terminées par VM
suivi() {
  local strat="$1" n="$2" dir=""
  while :; do
    dir=$(ls -td results/*_"$strat"_r1 2>/dev/null | head -1)
    if [ -n "$dir" ]; then
      local done=$(ls "$dir"/*.status 2>/dev/null | wc -l)
      printf '\r  %s : %d/%d tâches terminées (%s s)   ' "$strat" "$done" "$n" "$SECONDS"
      [ "$done" -ge "$n" ] && { echo; return; }
    fi
    sleep 2
  done
}

lancer() {
  local alloc="$1" strat="$2"
  local n=$(tail -n +2 "$alloc" | sed '/^\s*$/d' | wc -l)
  titre "3. Orchestrateur : $n tâches en parallèle sur les VMs ($strat)"
  SECONDS=0
  suivi "$strat" "$n" &
  local watcher=$!
  ./scripts/run_experiment.sh "$alloc" "$strat" 1 > /tmp/demo_$strat.log 2>&1
  local code=$?
  kill "$watcher" 2> /dev/null; wait "$watcher" 2> /dev/null; echo
  tail -n 6 /tmp/demo_$strat.log
  local dir=$(ls -td results/*_"$strat"_r1 2>/dev/null | head -1)
  titre "4. Preuve : chaque tâche a tourné là où le scheduler l'a dit"
  while IFS=, read -r id w s vm; do
    [ -z "$id" ] && continue
    reel=$(head -1 "$dir/$id.out" 2>/dev/null | cut -d' ' -f2 | cut -d. -f1)
    [ "$reel" = "$vm" ] && ok="OK" || ok="DIFFERENT"
    printf '  %-4s prévu %-13s exécuté sur %-13s %s\n' "$id" "$vm" "${reel:-?}" "$ok"
  done < <(tail -n +2 "$alloc" | tr -d '\r')
  echo "  Timeline : $dir/gantt.svg"
  return $code
}

lancer data/allocation_demo_rr.csv demo_rr
[ -f data/allocation_demo_nsga.csv ] && lancer data/allocation_demo_nsga.csv demo_nsga

titre "5. Bilan (data/experiments.csv)"
{ head -1 data/experiments.csv; grep -E '_demo_(rr|nsga)_r1,' data/experiments.csv | tail -2; } | tableau
echo
echo "Fin de démo. Pour ne rien payer ensuite : ./scripts/delete_vms.sh"
