#!/usr/bin/env bash
# TD3/TD4 — Lance toutes les tâches d'un fichier d'allocation, en parallèle.
#
# Usage : ./scripts/run_all.sh [ALLOCATION_CSV]   (défaut : data/allocation_manual.csv)
# Zone : variable ZONE, sinon la zone par défaut de gcloud.
#
# Une ligne = un run_task.sh lancé en arrière-plan (&). Un seul wait, après
# avoir tout lancé : plusieurs tâches d'une même VM tournent en même temps.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1

INPUT="${1:-data/allocation_manual.csv}"
ZONE="${ZONE:-$(gcloud config get-value compute/zone 2>/dev/null)}"

if [ ! -f "$INPUT" ]; then
    echo "error: allocation file '$INPUT' not found" >&2
    exit 1
fi
if [ -z "$ZONE" ]; then
    echo "error: no zone (set ZONE or run: gcloud config set compute/zone ...)" >&2
    exit 1
fi

HEADER="$(head -n 1 "$INPUT" | tr -d '\r')"
if [ "$HEADER" != "task_id,workload,size,vm" ]; then
    echo "error: $INPUT: header must be 'task_id,workload,size,vm', got '$HEADER'" >&2
    exit 1
fi

mkdir -p results
rm -f results/*.out results/*.err results/*.status

echo "Allocation: $INPUT  |  zone: $ZONE"
LAUNCHED=0
while IFS=, read -r task_id workload size vm || [ -n "$task_id" ]; do
    vm="${vm%$'\r'}"
    [ -z "$task_id" ] && continue
    ./scripts/run_task.sh "$task_id" "$workload" "$size" "$vm" "$ZONE" < /dev/null &
    echo "launched $task_id -> $vm (pid $!)"
    LAUNCHED=$((LAUNCHED + 1))
done < <(tail -n +2 "$INPUT")

echo "$LAUNCHED tasks launched, waiting..."
wait

FAILED=0
for status_file in results/*.status; do
    [ -e "$status_file" ] || continue
    IFS=, read -r _ _ code < "$status_file"
    [ "$code" = "0" ] || FAILED=$((FAILED + 1))
done
DONE=$(ls results/*.status 2>/dev/null | wc -l)

echo "All tasks completed: $DONE/$LAUNCHED finished, $FAILED failed."
[ "$FAILED" -eq 0 ] && [ "$DONE" -eq "$LAUNCHED" ]
