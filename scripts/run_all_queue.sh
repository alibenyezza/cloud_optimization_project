#!/usr/bin/env bash
# Comme run_all.sh, mais avec une FILE D'ATTENTE par VM :
#  - au plus <vcpu> tâches en même temps sur une VM (vcpu lu dans machines.csv), les autres attendent ;
#  - UNE seule connexion SSH par VM pour toute sa file (plus de ~3 s perdues par tâche) ;
#  - les tâches partent dans l'ordre du fichier d'allocation (le scheduler peut donc choisir l'ordre).
# Sorties identiques à run_all.sh (.out, .status) : collect_results.sh et gantt marchent sans changement.
# Horaires : pris sur la VM (horloges GCP synchronisées), SSH non compris.
#
# Usage : ./scripts/run_all_queue.sh ALLOCATION.csv [EXPERIMENT_ID]
source "$(dirname "$0")/config.sh"
ALLOC="${1:-data/allocation_manual.csv}"
EXP="${2:-}"
if [ -n "$EXP" ]; then OUT="results/$EXP"; else OUT="results"; fi
[ -f "$ALLOC" ] || { echo "Erreur : fichier $ALLOC introuvable." >&2; exit 1; }
DUP=$(tail -n +2 "$ALLOC" | tr -d '\r' | cut -d, -f1 | sed '/^$/d' | sort | uniq -d)
[ -z "$DUP" ] || { echo "Erreur : task_id en double : $DUP" >&2; exit 1; }
if [ -n "$EXP" ] && [ -d "$OUT" ]; then echo "Erreur : $OUT existe déjà." >&2; exit 1; fi
mkdir -p "$OUT"
rm -f "$OUT"/*.out "$OUT"/*.err "$OUT"/*.status
cp "$ALLOC" "$OUT/allocation.csv"

VMS=$(tail -n +2 "$ALLOC" | tr -d '\r' | cut -d, -f4 | sed '/^$/d' | awk '!seen[$0]++')
N=$(tail -n +2 "$ALLOC" | tr -d '\r' | sed '/^$/d' | wc -l)

for VM in $VMS; do
  SLOTS=$(tr -d '\r' < "$MACHINES_FILE" | awk -F, -v vm="$VM" '$1 == vm {print $3}')
  [ -n "$SLOTS" ] || { echo "Erreur : $VM absente de $MACHINES_FILE" >&2; exit 1; }
  TASKS=$(tail -n +2 "$ALLOC" | tr -d '\r' | awk -F, -v vm="$VM" '$4 == vm {print $1, $2, $3}')
  NT=$(echo "$TASKS" | wc -l)
  echo "[$VM] $NT tâches, $SLOTS à la fois"
  # Script exécuté sur la VM : xargs -P lance au plus SLOTS tâches en parallèle
  REMOTE=$(cat <<EOF
cd ~/cloud-optimization || exit 1
run() {
  extra=""; [ "\$2" = memory_task ] && extra="--passes $MEM_PASSES"
  st=\$(date +%s.%N); out=\$(./bin/\$2 --size \$3 \$extra 2> /dev/null); c=\$?; en=\$(date +%s.%N)
  printf 'OUT|%s|%s\n' "\$1" "\$out"
  printf 'ST|%s|%s|%s|%s\n' "\$1" "\$c" "\$st" "\$en"
}
export -f run
echo "HOST|\$(hostname)"
printf '%s\n' "$TASKS" | xargs -P $SLOTS -L 1 bash -c 'run \$0 \$1 \$2'
EOF
)
  B64=$(printf '%s' "$REMOTE" | base64 -w0)
  (
    gcloud compute ssh "$VM" --zone="$ZONE" --quiet --command="echo $B64 | base64 -d | bash" \
      < /dev/null > "$OUT/.$VM.log" 2> "$OUT/.$VM.err"
    CODE=$?
    HOST=$(grep '^HOST|' "$OUT/.$VM.log" | cut -d'|' -f2)
    while read -r id w s; do
      [ -z "$id" ] && continue
      line=$(grep "^ST|$id|" "$OUT/.$VM.log")
      if [ -n "$line" ]; then
        IFS='|' read -r _ _ c st en <<< "$line"
        { echo "host: $HOST"; grep "^OUT|$id|" "$OUT/.$VM.log" | cut -d'|' -f3-; } > "$OUT/$id.out"
      else   # la tâche n'a pas tourné (SSH impossible, VM absente...) : échec visible, jamais caché
        c=$CODE; [ "$c" -eq 0 ] && c=1; st=$(date +%s.%N); en=$st
        cp "$OUT/.$VM.err" "$OUT/$id.err"
      fi
      [ -f "$OUT/$id.err" ] || : > "$OUT/$id.err"
      echo "$id,$w,$s,$VM,$c,$st,$en" > "$OUT/$id.status"
      [ "$c" -eq 0 ] && echo "[$id] OK sur $VM" || echo "[$id] ECHEC (code $c) sur $VM" >&2
    done <<< "$TASKS"
  ) &
done
echo "$N tâches lancées, attente de la fin..."
wait
rm -f "$OUT"/.*.log "$OUT"/.*.err 2> /dev/null
FAILS=$(cat "$OUT"/*.status | awk -F, '$5 != 0' | wc -l)
echo "Bilan : $N tâches, $FAILS en échec. Résultats dans $OUT/"
[ "$FAILS" -eq 0 ]
