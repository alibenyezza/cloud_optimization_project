#!/usr/bin/env bash
# Supprime toutes les VMs de machines.csv (et leurs disques) puis vérifie qu'il ne reste rien.
# Usage : ./scripts/delete_vms.sh [machines.csv]
source "$(dirname "$0")/config.sh"
MACHINES="${1:-$MACHINES_FILE}"
LISTE=$(tail -n +2 "$MACHINES" | tr -d '\r' | cut -d, -f1 | sed '/^$/d')
# On ne supprime que les VMs de la liste qui existent vraiment (sinon gcloud s'arrête en erreur)
EXIST=$(gcloud compute instances list --zones="$ZONE" --format="value(name)" < /dev/null)
VMS=$(for v in $LISTE; do echo "$EXIST" | grep -qx "$v" && echo "$v"; done)

if [ -n "$VMS" ]; then
  gcloud compute instances delete $VMS --zone="$ZONE" --quiet
else
  echo "Aucune VM de $MACHINES n'est allumée."
fi
echo "--- Vérification (tout doit être vide) ---"
gcloud compute instances list
gcloud compute disks list
