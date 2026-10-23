#!/usr/bin/env bash
# Copie bin/ sur toutes les VMs et vérifie le déploiement.
# Usage : ./scripts/deploy_all.sh

ZONE="europe-west1-b"
VMS="cloudopt-vm1 cloudopt-vm2 cloudopt-vm3"

for VM in $VMS; do
  echo "=== $VM ==="
  gcloud compute ssh "$VM" --zone="$ZONE" --quiet \
    --command="mkdir -p ~/cloud-optimization" || { echo "Connexion impossible à $VM" >&2; continue; }
  gcloud compute scp --recurse bin "$VM":~/cloud-optimization/ --zone="$ZONE" --quiet
  gcloud compute ssh "$VM" --zone="$ZONE" --quiet \
    --command="chmod +x ~/cloud-optimization/bin/* && ls -l ~/cloud-optimization/bin"
done
