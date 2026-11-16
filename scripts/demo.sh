#!/usr/bin/env bash
# Démo live de la soutenance, environ 3 min : Round-Robin contre NSGA-II sur les 12 vraies tâches.
# 1. les VMs  2. les tâches  3. les deux schedulers décident (NSGA-II en direct)
# 4. exécution réelle des deux allocations sur GCP  5. bilan + les deux Gantt.
# Les VMs doivent être allumées avant (./scripts/setup_vms.sh -), sinon on perd ~50 s en live.
# data/pareto/ et data/allocation_nsga.csv ne sont pas modifiés.
#
# Usage : ./scripts/demo.sh [TASKS.csv]      (défaut : data/tasks.csv)
source "$(dirname "$0")/config.sh"
TASKS="${1:-data/tasks.csv}"
OUT="data/demo"
[ -f "$TASKS" ] || { echo "Erreur : $TASKS introuvable." >&2; exit 1; }
mkdir -p "$OUT" bin

titre() { printf '\n\033[1;34m== %s ==\033[0m\n' "$*"; }
tableau() { if command -v column > /dev/null; then column -s, -t; else sed 's/,/   /g'; fi; }
ouvrir() { command -v explorer.exe > /dev/null && explorer.exe "$(wslpath -w "$1")" 2> /dev/null; }
for b in round_robin_scheduler:scheduler/round_robin.go nsga2:scheduler/nsga2.go predict:tools/predict/main.go; do
  go build -o "bin/${b%%:*}" "${b#*:}" || exit 1
done

titre "1. Les VMs (3 types différents, prix différents)"
gcloud compute instances list --zones="$ZONE" --format="table(name,machineType.basename(),status)" < /dev/null

titre "2. Les tâches à placer"
tableau < "$TASKS"

titre "3a. Round-Robin : chacun son tour, sans regarder les tâches"
./bin/round_robin_scheduler "$TASKS" "$MACHINES_FILE" "$OUT/allocation_rr.csv" > /dev/null || exit 1
cut -d, -f1,2,4 "$OUT/allocation_rr.csv" | tableau

titre "3b. NSGA-II : optimise temps ET coût (calcul en direct)"
sauve=$(mktemp -d)
cp -r data/pareto "$sauve/" 2> /dev/null; cp data/allocation_nsga.csv "$sauve/" 2> /dev/null
SECONDS=0
./bin/nsga2 "$TASKS" "$MACHINES_FILE" data/profiles.csv 42 | grep -E "front|sol_|Round-Robin" || exit 1
echo "  (calculé en $SECONDS s)"
cp data/allocation_nsga.csv "$OUT/allocation_nsga.csv"
rm -rf data/pareto; cp -r "$sauve/pareto" data/ 2> /dev/null; cp "$sauve/allocation_nsga.csv" data/ 2> /dev/null; rm -rf "$sauve"
cut -d, -f1,2,4 "$OUT/allocation_nsga.csv" | tableau

suivi() {   # compteur de tâches finies pendant l'exécution
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

lancer() {   # $1 = rr ou nsga
  local s="$1" alloc="$OUT/allocation_$1.csv"
  local n=$(tail -n +2 "$alloc" | sed '/^\s*$/d' | wc -l)
  local pred=$(./bin/predict "$TASKS" "$MACHINES_FILE" data/profiles.csv "$alloc" | tail -1)
  titre "4. Exécution réelle : $s (prévu $(echo "$pred" | cut -d, -f2) s)"
  SECONDS=0
  suivi "demo_$s" "$n" &
  local pid=$!
  QUEUE=1 ./scripts/run_experiment.sh "$alloc" "demo_$s" 1 "$(echo "$pred" | cut -d, -f2)" "$(echo "$pred" | cut -d, -f3)" \
    > "/tmp/demo_$s.log" 2>&1
  kill "$pid" 2> /dev/null; wait "$pid" 2> /dev/null; echo
  local dir=$(ls -td results/*_demo_"$s"_r1 2> /dev/null | head -1)
  grep -E "makespan =" "/tmp/demo_$s.log" | head -1
  local ok=0 total=0
  while IFS=, read -r id w sz vm; do
    [ -z "$id" ] && continue
    total=$((total + 1))
    [ "$(head -1 "$dir/$id.out" 2> /dev/null | cut -d' ' -f2 | cut -d. -f1)" = "$vm" ] && ok=$((ok + 1))
  done < <(tail -n +2 "$alloc" | tr -d '\r')
  echo "  Preuve : $ok/$total tâches ont tourné sur la VM prévue (ligne host: de chaque sortie)"
  eval "DIR_$s=\$dir"
}

lancer rr
lancer nsga

titre "5. Bilan"
{ echo "stratégie,makespan_s,coût_usd,prévu_s"
  for s in rr nsga; do grep -E "_demo_${s}_r1," data/experiments.csv | tail -1 | awk -F, -v s="$s" '{print s "," $3 "," $4 "," $5}'; done
} | tableau
ouvrir "$DIR_rr/gantt.svg"; ouvrir "$DIR_nsga/gantt.svg"
echo
echo "Résultats complets (3 répétitions, 12 et 40 tâches) : data/campagne/BILAN.md"
echo "Ne pas oublier : ./scripts/delete_vms.sh"
