#!/usr/bin/env bash
# Lit un fichier d'allocation et lance une tâche par ligne, en arrière-plan, puis attend la fin.
# Usage : ./scripts/run_all.sh [ALLOCATION.csv] [EXPERIMENT_ID]
#   sans EXPERIMENT_ID : résultats dans results/ (écrasés à chaque lancement)
#   avec EXPERIMENT_ID : résultats dans results/<EXPERIMENT_ID>/ (jamais écrasés)
source "$(dirname "$0")/config.sh"
ALLOC="${1:-data/allocation_manual.csv}"
EXP="${2:-}"
if [ -n "$EXP" ]; then OUT="results/$EXP"; else OUT="results"; fi

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

# Une expérience nommée n'est jamais écrasée
if [ -n "$EXP" ] && [ -d "$OUT" ]; then
  echo "Erreur : $OUT existe déjà, choisis un autre EXPERIMENT_ID." >&2
  exit 1
fi

mkdir -p "$OUT"
rm -f "$OUT"/*.out "$OUT"/*.err "$OUT"/*.status
cp "$ALLOC" "$OUT/allocation.csv"     # garde l'allocation exacte utilisée

N=0
while IFS=, read -r task_id workload size vm; do
  [ -z "$task_id$workload$size$vm" ] && continue
  if [ -z "$task_id" ] || [ -z "$workload" ] || [ -z "$size" ] || [ -z "$vm" ]; then
    echo "Ligne incomplète ignorée : $task_id,$workload,$size,$vm" >&2
    continue
  fi
  ./scripts/run_task.sh "$task_id" "$workload" "$size" "$vm" "$ZONE" "$OUT" < /dev/null &
  N=$((N + 1))
done < <(tail -n +2 "$ALLOC" | tr -d '\r')

echo "$N tâches lancées, attente de la fin..."
wait
echo "All tasks completed"

# Bilan à partir des fichiers .status
FAILS=$(cat "$OUT"/*.status | awk -F, '$5 != 0' | wc -l)
echo "Bilan : $N tâches, $FAILS en échec. Résultats dans $OUT/"
if [ "$FAILS" -ne 0 ]; then
  cat "$OUT"/*.status | awk -F, '$5 != 0 {print "  ECHEC : " $1 " sur " $4 " (code " $5 ")"}' >&2
  exit 1
fi
exit 0
