#!/usr/bin/env bash
# Exécute une allocation en respectant les dépendances entre tâches (graphe de précédence).
#
# Dépendances : colonne optionnelle depends_on de tasks.csv, ids séparés par ";" (vide = aucune).
#     task_id,workload,size,depends_on
#     T1,sort_task,80000000,
#     T2,matrix_task,2500,T1
#     T3,cpu_task,20000000,T1;T2
# Le format de l'allocation ne change pas (task_id,workload,size,vm).
#
# Règles (le modèle de prédiction doit simuler EXACTEMENT les mêmes) :
#  1. une tâche démarre quand toutes ses dépendances ont réussi, même si elles ont tourné sur une autre VM ;
#  2. au plus <vcpu> tâches en même temps par VM (vcpu de machines.csv) ;
#  3. quand une place se libère, on lance la première tâche prête dans l'ordre du fichier d'allocation ;
#  4. si une dépendance échoue, ses descendantes ne sont pas lancées (code 99, en rouge sur le Gantt).
# Le contrôleur (ce PC) décide qui part et quand ; chaque tâche est lancée par run_task.sh (une connexion SSH).
# Limite : dépendance logique uniquement, aucune donnée n'est transférée d'une tâche à l'autre.
#
# Usage : ./scripts/run_all_dag.sh ALLOCATION.csv TASKS.csv [EXPERIMENT_ID]
source "$(dirname "$0")/config.sh"
ALLOC="${1:?Usage : $0 ALLOCATION.csv TASKS.csv [EXPERIMENT_ID]}"
TASKS="${2:?Usage : $0 ALLOCATION.csv TASKS.csv [EXPERIMENT_ID]}"
EXP="${3:-}"
if [ -n "$EXP" ]; then OUT="results/$EXP"; else OUT="results"; fi
for f in "$ALLOC" "$TASKS"; do [ -f "$f" ] || { echo "Erreur : $f introuvable." >&2; exit 1; }; done
if [ -n "$EXP" ] && [ -d "$OUT" ]; then echo "Erreur : $OUT existe déjà." >&2; exit 1; fi

declare -A VM W S DEP STATE SLOTS USED
ORDER=()
while IFS=, read -r id w s vm; do
  [ -z "$id" ] && continue
  [ -n "${VM[$id]}" ] && { echo "Erreur : task_id en double : $id" >&2; exit 1; }
  VM[$id]=$vm; W[$id]=$w; S[$id]=$s; STATE[$id]=P; DEP[$id]=""
  ORDER+=("$id")
done < <(tail -n +2 "$ALLOC" | tr -d '\r')

while IFS=, read -r id w s dep _; do
  [ -z "$id" ] && continue
  [ -n "${VM[$id]}" ] || { echo "Erreur : $id est dans $TASKS mais pas dans l'allocation." >&2; exit 1; }
  DEP[$id]="${dep//;/ }"
done < <(tail -n +2 "$TASKS" | tr -d '\r')

# Vérifications avant de lancer quoi que ce soit : dépendances connues, pas de cycle, VMs connues
PAIRS=$(mktemp)
for id in "${ORDER[@]}"; do
  echo "$id $id" >> "$PAIRS"
  for d in ${DEP[$id]}; do
    [ -n "${VM[$d]}" ] || { echo "Erreur : $id dépend de $d, qui n'existe pas." >&2; rm -f "$PAIRS"; exit 1; }
    echo "$d $id" >> "$PAIRS"
  done
  v=${VM[$id]}
  if [ -z "${SLOTS[$v]}" ]; then
    SLOTS[$v]=$(tr -d '\r' < "$MACHINES_FILE" | awk -F, -v vm="$v" '$1 == vm {print $3}')
    [ -n "${SLOTS[$v]}" ] || { echo "Erreur : $v absente de $MACHINES_FILE" >&2; rm -f "$PAIRS"; exit 1; }
    USED[$v]=0
  fi
done
if ! tsort "$PAIRS" > /dev/null 2>&1; then
  echo "Erreur : les dépendances forment un cycle (A attend B qui attend A) :" >&2
  tsort "$PAIRS" 2>&1 >/dev/null | grep -v "^tsort: $PAIRS" | sed 's/^tsort: /  /' >&2
  rm -f "$PAIRS"; exit 1
fi
rm -f "$PAIRS"

mkdir -p "$OUT"
rm -f "$OUT"/*.out "$OUT"/*.err "$OUT"/*.status
cp "$ALLOC" "$OUT/allocation.csv"
cp "$TASKS" "$OUT/tasks.csv"      # garde les dépendances exactes de cette expérience

N=${#ORDER[@]}
echo "$N tâches, dépendances respectées, $(for v in "${!SLOTS[@]}"; do printf '%s:%s ' "$v" "${SLOTS[$v]}"; done)places"
while :; do
  pending=0; running=0
  # 1) tâches terminées : on libère leur place
  for id in "${ORDER[@]}"; do
    [ "${STATE[$id]}" = R ] || continue
    if [ -s "$OUT/$id.status" ]; then
      code=$(cut -d, -f5 "$OUT/$id.status")
      [ "$code" = 0 ] && STATE[$id]=D || STATE[$id]=F
      USED[${VM[$id]}]=$(( USED[${VM[$id]}] - 1 ))
    fi
  done
  # 2) tâches prêtes : dans l'ordre de l'allocation, s'il reste une place sur leur VM
  for id in "${ORDER[@]}"; do
    [ "${STATE[$id]}" = P ] || continue
    ready=1
    for d in ${DEP[$id]}; do
      case "${STATE[$d]}" in
        D) ;;
        F) ready=x; break ;;
        *) ready=0 ;;
      esac
    done
    if [ "$ready" = x ]; then      # une dépendance a échoué : on ne lance pas, on le note
      now=$(date +%s.%N)
      echo "$id,${W[$id]},${S[$id]},${VM[$id]},99,$now,$now" > "$OUT/$id.status"
      echo "dépendance échouée" > "$OUT/$id.err"
      echo "[$id] non lancée : une dépendance a échoué" >&2
      STATE[$id]=F
      continue
    fi
    v=${VM[$id]}
    if [ "$ready" = 1 ] && [ "${USED[$v]}" -lt "${SLOTS[$v]}" ]; then
      ./scripts/run_task.sh "$id" "${W[$id]}" "${S[$id]}" "$v" "$ZONE" "$OUT" < /dev/null &
      STATE[$id]=R; USED[$v]=$(( USED[$v] + 1 ))
    fi
  done
  for id in "${ORDER[@]}"; do
    case "${STATE[$id]}" in P) pending=$((pending + 1)) ;; R) running=$((running + 1)) ;; esac
  done
  [ "$pending" -eq 0 ] && [ "$running" -eq 0 ] && break
  sleep 1
done
wait

FAILS=$(cat "$OUT"/*.status | awk -F, '$5 != 0' | wc -l)
echo "Bilan : $N tâches, $FAILS en échec. Résultats dans $OUT/"
[ "$FAILS" -eq 0 ]
