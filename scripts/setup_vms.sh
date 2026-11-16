#!/usr/bin/env bash
# Prépare les VMs dont une allocation a besoin, et SEULEMENT celles-là : le nombre de VMs peut varier.
# Remplace "create_vms.sh + sleep 60 + deploy_all.sh" :
#  - crée les VMs en parallèle (une VM déjà allumée est réutilisée telle quelle) ;
#  - attend que SSH réponde vraiment, au lieu d'un sleep 60 fixe ;
#  - déploie les workloads ;
#  - MESURE le temps de chaque étape dans data/vm_setup.csv : c'est le surcoût fixe d'une VM,
#    celui qui fait qu'ajouter des VMs n'accélère pas toujours.
#
# Usage : ./scripts/setup_vms.sh ALLOCATION.csv [MACHINES.csv]
#         ./scripts/setup_vms.sh - [MACHINES.csv]      (toutes les VMs du fichier machines)
source "$(dirname "$0")/config.sh"
ALLOC="${1:?Usage : $0 ALLOCATION.csv|- [MACHINES.csv]}"
MACH="${2:-$MACHINES_FILE}"
LOG="data/vm_setup.csv"

if [ "$ALLOC" = "-" ]; then
  VMS=$(tail -n +2 "$MACH" | tr -d '\r' | cut -d, -f1 | sed '/^$/d')
else
  [ -f "$ALLOC" ] || { echo "Erreur : $ALLOC introuvable." >&2; exit 1; }
  VMS=$(tail -n +2 "$ALLOC" | tr -d '\r' | cut -d, -f4 | sed '/^$/d' | sort -u)
fi
ls bin/*_task > /dev/null 2>&1 || { echo "Erreur : compile d'abord les workloads dans bin/." >&2; exit 1; }

PIDS=""
EXIST=$(gcloud compute instances list --zones="$ZONE" --format="value(name)" < /dev/null)
[ -f "$LOG" ] || echo "date,vm,machine_type,create_s,ssh_ready_s,deploy_s,total_s" > "$LOG"

for VM in $VMS; do
  TYPE=$(tr -d '\r' < "$MACH" | awk -F, -v vm="$VM" '$1 == vm {print $2}')
  [ -n "$TYPE" ] || { echo "Erreur : $VM absente de $MACH" >&2; exit 1; }
  if echo "$EXIST" | grep -qx "$VM"; then
    echo "[$VM] déjà allumée, réutilisée"
    continue
  fi
  (
    t0=$(date +%s.%N)
    gcloud compute instances create "$VM" --zone="$ZONE" --machine-type="$TYPE" \
      --image-family=debian-12 --image-project=debian-cloud --quiet < /dev/null > /dev/null 2> "/tmp/setup_$VM.err" \
      || { echo "[$VM] ECHEC de création :" >&2; grep -E "ERROR|message|-" "/tmp/setup_$VM.err" | head -5 >&2; exit 1; }
    t1=$(date +%s.%N)
    echo "[$VM] créée, attente de SSH..."
    n=0
    until gcloud compute ssh "$VM" --zone="$ZONE" --quiet --command="true" < /dev/null > /dev/null 2>&1; do
      n=$((n + 1)); [ "$n" -ge 30 ] && { echo "[$VM] SSH toujours indisponible" >&2; exit 1; }
      sleep 5
    done
    t2=$(date +%s.%N)
    gcloud compute ssh "$VM" --zone="$ZONE" --quiet --command="mkdir -p ~/cloud-optimization/bin" < /dev/null > /dev/null 2>&1
    gcloud compute scp bin/*_task "$VM":~/cloud-optimization/bin/ --zone="$ZONE" --quiet < /dev/null > /dev/null 2>&1 \
      || { echo "[$VM] ECHEC du déploiement" >&2; exit 1; }
    gcloud compute ssh "$VM" --zone="$ZONE" --quiet --command="chmod +x ~/cloud-optimization/bin/*" < /dev/null > /dev/null 2>&1
    t3=$(date +%s.%N)
    awk -v d="$(date +%Y%m%d-%H%M)" -v vm="$VM" -v ty="$TYPE" -v a="$t0" -v b="$t1" -v c="$t2" -v e="$t3" \
      'BEGIN { printf "%s,%s,%s,%.1f,%.1f,%.1f,%.1f\n", d, vm, ty, b - a, c - b, e - c, e - a }' >> "$LOG"
    awk -v vm="$VM" -v a="$t0" -v e="$t3" 'BEGIN { printf "[%s] prête en %.0f s\n", vm, e - a }'
  ) &
  PIDS="$PIDS $!"
done
ECHECS=0
for p in $PIDS; do wait "$p" || ECHECS=$((ECHECS + 1)); done
echo "Temps de préparation enregistrés dans $LOG :"
tail -n +2 "$LOG" | tail -n "$(echo "$VMS" | wc -w)"
if [ "$ECHECS" -gt 0 ]; then
  echo "Erreur : $ECHECS VM(s) non prête(s). Les VMs déjà créées restent allumées : ./scripts/delete_vms.sh" >&2
  exit 1
fi
