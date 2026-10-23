#!/usr/bin/env bash
# Exécute UNE tâche sur UNE VM et enregistre stdout/stderr en local.
# Usage : ./scripts/run_task.sh TASK_ID WORKLOAD SIZE VM ZONE

if [ "$#" -ne 5 ]; then
  echo "Usage: $0 TASK_ID WORKLOAD SIZE VM ZONE" >&2
  exit 2
fi

TASK_ID="$1"
WORKLOAD="$2"
SIZE="$3"
VM="$4"
ZONE="$5"

mkdir -p results

echo "[$TASK_ID] lancement : $WORKLOAD --size $SIZE sur $VM"

gcloud compute ssh "$VM" --zone="$ZONE" --quiet \
  --command="echo host: \$(hostname); cd ~/cloud-optimization && ./bin/$WORKLOAD --size $SIZE" \
  > "results/$TASK_ID.out" 2> "results/$TASK_ID.err" < /dev/null
CODE=$?

echo "$TASK_ID,$WORKLOAD,$SIZE,$VM,$CODE" > "results/$TASK_ID.status"

if [ "$CODE" -eq 0 ]; then
  echo "[$TASK_ID] OK"
else
  echo "[$TASK_ID] ECHEC (code $CODE), voir results/$TASK_ID.err" >&2
fi
exit "$CODE"
