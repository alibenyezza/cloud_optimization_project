#!/usr/bin/env bash
# TD3 — Exécute UNE tâche sur UNE VM via gcloud compute ssh.
#
# Usage : ./scripts/run_task.sh TASK_ID WORKLOAD SIZE VM ZONE
# Ex.   : ./scripts/run_task.sh T1 cpu_task 500000 cloudopt-vm1 europe-west1-b
#
# Produit sur le contrôleur (pas sur la VM) :
#   results/TASK_ID.out     sortie standard du workload (JSON)
#   results/TASK_ID.err     erreur standard (workload + gcloud/ssh)
#   results/TASK_ID.status  "task_id,vm,exit_code" pour la validation
# Le code de retour du script est celui de la commande distante.
set -u

if [ "$#" -ne 5 ]; then
    echo "usage: $0 TASK_ID WORKLOAD SIZE VM ZONE" >&2
    exit 2
fi

TASK_ID=$1
WORKLOAD=$2
SIZE=$3
VM=$4
ZONE=$5

for name in TASK_ID WORKLOAD SIZE VM ZONE; do
    if [ -z "${!name}" ]; then
        echo "error: $name is empty" >&2
        exit 2
    fi
done

# WORKLOAD et SIZE sont insérés dans une commande distante : on refuse tout
# ce qui n'est pas un nom de binaire ou un entier.
if ! [[ "$WORKLOAD" =~ ^[a-z_]+$ ]]; then
    echo "error: invalid workload name '$WORKLOAD'" >&2
    exit 2
fi
if ! [[ "$SIZE" =~ ^[0-9]+$ ]]; then
    echo "error: size must be a positive integer, got '$SIZE'" >&2
    exit 2
fi

RESULTS_DIR="${RESULTS_DIR:-results}"
mkdir -p "$RESULTS_DIR"
OUT="$RESULTS_DIR/$TASK_ID.out"
ERR="$RESULTS_DIR/$TASK_ID.err"

# stdin redirigé : sinon ssh consomme l'entrée de la boucle de run_all.sh.
gcloud compute ssh "$VM" --zone="$ZONE" --quiet \
    --command="cd ~/cloud-optimization && ./bin/$WORKLOAD --size $SIZE" \
    < /dev/null > "$OUT" 2> "$ERR"
STATUS=$?

echo "$TASK_ID,$VM,$STATUS" > "$RESULTS_DIR/$TASK_ID.status"

if [ "$STATUS" -eq 0 ]; then
    echo "[$TASK_ID] OK      $WORKLOAD --size $SIZE on $VM"
else
    echo "[$TASK_ID] FAILED  $WORKLOAD --size $SIZE on $VM (exit $STATUS, see $ERR)" >&2
fi
exit "$STATUS"
