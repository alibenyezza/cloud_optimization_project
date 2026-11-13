#!/usr/bin/env bash
# Exécute UNE tâche sur UNE VM et enregistre sortie, erreurs, code de retour et horaires.
# Usage : run_task.sh TASK_ID WORKLOAD SIZE VM ZONE [RESULTS_DIR]
# .status : task_id,workload,size,vm,exit_code,start_epoch_s,end_epoch_s
if [ "$#" -lt 5 ] || [ "$#" -gt 6 ]; then
  echo "Usage: $0 TASK_ID WORKLOAD SIZE VM ZONE [RESULTS_DIR]" >&2
  exit 2
fi
TASK_ID="$1"; WORKLOAD="$2"; SIZE="$3"; VM="$4"; ZONE="$5"
OUT="${6:-results}"
mkdir -p "$OUT"
echo "[$TASK_ID] lancement : $WORKLOAD --size $SIZE sur $VM"

START=$(date +%s.%N)
gcloud compute ssh "$VM" --zone="$ZONE" --quiet \
  --command="echo host: \$(hostname); cd ~/cloud-optimization && ./bin/$WORKLOAD --size $SIZE" \
  > "$OUT/$TASK_ID.out" 2> "$OUT/$TASK_ID.err" < /dev/null
CODE=$?
END=$(date +%s.%N)

echo "$TASK_ID,$WORKLOAD,$SIZE,$VM,$CODE,$START,$END" > "$OUT/$TASK_ID.status"
if [ "$CODE" -eq 0 ]; then echo "[$TASK_ID] OK"
else echo "[$TASK_ID] ECHEC (code $CODE), voir $OUT/$TASK_ID.err" >&2; fi
exit "$CODE"
