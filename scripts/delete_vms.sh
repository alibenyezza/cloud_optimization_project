#!/usr/bin/env bash
# Supprime toutes les VMs de machines.csv (et leurs disques) puis vérifie qu'il ne reste rien.
# Usage : ./scripts/delete_vms.sh [machines.csv]
source "$(dirname "$0")/config.sh"
MACHINES="${1:-$MACHINES_FILE}"
VMS=$(tail -n +2 "$MACHINES" | tr -d '\r' | cut -d, -f1 | sed '/^$/d')

gcloud compute instances delete $VMS --zone="$ZONE" --quiet
echo "--- Vérification (tout doit être vide) ---"
gcloud compute instances list
gcloud compute disks list
