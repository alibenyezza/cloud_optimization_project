#!/usr/bin/env bash
# Copie bin/ sur toutes les VMs de machines.csv et vérifie le déploiement.
# Usage : ./scripts/deploy_all.sh [machines.csv]
source "$(dirname "$0")/config.sh"
MACHINES="${1:-$MACHINES_FILE}"
VMS=$(tail -n +2 "$MACHINES" | tr -d '\r' | cut -d, -f1 | sed '/^$/d')

for VM in $VMS; do
  echo "=== $VM ==="
  gcloud compute ssh "$VM" --zone="$ZONE" --quiet \
    --command="mkdir -p ~/cloud-optimization" < /dev/null || { echo "Connexion impossible à $VM" >&2; continue; }
  gcloud compute scp --recurse bin "$VM":~/cloud-optimization/ --zone="$ZONE" --quiet
  gcloud compute ssh "$VM" --zone="$ZONE" --quiet \
    --command="chmod +x ~/cloud-optimization/bin/* && ls -l ~/cloud-optimization/bin" < /dev/null
done
