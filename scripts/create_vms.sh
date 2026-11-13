#!/usr/bin/env bash
# Crée une VM par ligne de machines.csv (vm,machine_type,...), dans la zone de config.sh
# Usage : ./scripts/create_vms.sh [machines.csv]
source "$(dirname "$0")/config.sh"
MACHINES="${1:-$MACHINES_FILE}"

tail -n +2 "$MACHINES" | tr -d '\r' | while IFS=, read -r vm type vcpu ram price; do
  [ -z "$vm" ] && continue
  echo "=== Création de $vm ($type) dans $ZONE"
  gcloud compute instances create "$vm" --zone="$ZONE" --machine-type="$type" \
    --image-family=debian-12 --image-project=debian-cloud < /dev/null
done
gcloud compute instances list
