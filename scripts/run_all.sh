#!/usr/bin/env bash
# Lit le fichier d'allocation et lance une tâche par ligne, en arrière-plan.
# Usage : ./scripts/run_all.sh [fichier_allocation.csv]

source "$(dirname "$0")/config.sh"
ALLOC="${1:-data/allocation_manual.csv}"

if [ ! -f "$ALLOC" ]; then
  echo "Erreur : fichier $ALLOC introuvable." >&2
  exit 1
fi

# Vérifie que chaque task_id est unique
DUP=$(tail -n +2 "$ALLOC" | tr -d '\r' | cut -d, -f1 | sed '/^$/d' | sort | uniq -d)
if [ -n "$DUP" ]; then
  echo "Erreur : task_id en double : $DUP" >&2
  exit 1
fi

# Repart de résultats propres
mkdir -p results
rm -f results/*.out results/*.err results/*.status

N=0
while IFS=, read -r task_id workload size vm; do
  [ -z "$task_id$workload$size$vm" ] && continue    # NOUVEAU : ignore les lignes vides
  if [ -z "$task_id" ] || [ -z "$workload" ] || [ -z "$size" ] || [ -z "$vm" ]; then
    echo "Ligne incomplète ignorée : $task_id,$workload,$size,$vm" >&2
    continue
  fi
  ./scripts/run_task.sh "$task_id" "$workload" "$size" "$vm" "$ZONE" < /dev/null &
  N=$((N + 1))
done < <(tail -n +2 "$ALLOC" | tr -d '\r')

echo "$N tâches lancées, attente de la fin..."
wait
echo "All tasks completed"

# Bilan à partir des fichiers .status
FAILS=$(cat results/*.status | awk -F, '$5 != 0' | wc -l)
echo "Bilan : $N tâches, $FAILS en échec."
if [ "$FAILS" -ne 0 ]; then
  cat results/*.status | awk -F, '$5 != 0 {print "  ECHEC : " $1 " sur " $4 " (code " $5 ")"}' >&2
  exit 1
fi
exit 0
