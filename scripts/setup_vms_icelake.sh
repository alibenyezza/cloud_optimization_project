#!/usr/bin/env bash
# Même chose que setup_vms.sh, avec deux différences :
#  - les VMs n2 sont créées en Ice Lake (--min-cpu-platform). Sans ça, une n2 peut tomber sur un
#    Cascade Lake, 2,8 fois plus lent sur cpu_task (data/campagne_17-11/BILAN.md) ;
#  - si la zone de config.sh n'a plus de n2 (ZONE_RESOURCE_POOL_EXHAUSTED), on essaie les zones de
#    ZONES_SECOURS. Toutes les VMs vont dans la même zone.
# L'option n'existe pas pour les e2 : leur processeur n'est pas choisi. On l'affiche à la fin.
#
# Usage : ./scripts/setup_vms_icelake.sh ALLOCATION.csv [MACHINES.csv]
#         ./scripts/setup_vms_icelake.sh - [MACHINES.csv]      (toutes les VMs du fichier machines)
# Si une zone de secours est utilisée, mettre cette zone dans config.sh pour la suite (run, delete).
source "$(dirname "$0")/config.sh"
ALLOC="${1:?Usage : $0 ALLOCATION.csv|- [MACHINES.csv]}"
MACH="${2:-$MACHINES_FILE}"
LOG="data/vm_setup.csv"
CPU_N2="Intel Ice Lake"
ZONES_SECOURS="${ZONES_SECOURS:-europe-west9-b europe-west9-c europe-west1-b europe-west1-c europe-west1-d europe-west4-a europe-west4-b europe-west4-c}"

if [ "$ALLOC" = "-" ]; then
  VMS=$(tail -n +2 "$MACH" | tr -d '\r' | cut -d, -f1 | sed '/^$/d')
else
  [ -f "$ALLOC" ] || { echo "Erreur : $ALLOC introuvable." >&2; exit 1; }
  VMS=$(tail -n +2 "$ALLOC" | tr -d '\r' | cut -d, -f4 | sed '/^$/d' | sort -u)
fi
ls bin/*_task > /dev/null 2>&1 || { echo "Erreur : compile d'abord les workloads dans bin/." >&2; exit 1; }

type_de() { tr -d '\r' < "$MACH" | awk -F, -v vm="$1" '$1 == vm {print $2}'; }

# Création d'une VM ; les n2 en Ice Lake
creer() { # VM TYPE ZONE
  local opt=()
  [[ "$2" == n2-* ]] && opt=(--min-cpu-platform="$CPU_N2")
  gcloud compute instances create "$1" --zone="$3" --machine-type="$2" "${opt[@]}" \
    --image-family=debian-12 --image-project=debian-cloud --quiet < /dev/null > /dev/null 2> "/tmp/setup_$1.err"
}

PIDS=""
ZONE_CONFIG="$ZONE"
EXIST=$(gcloud compute instances list --zones="$ZONE" --format="value(name)" < /dev/null)
[ -f "$LOG" ] || echo "date,vm,machine_type,create_s,ssh_ready_s,deploy_s,total_s" > "$LOG"

# Choix de la zone : on crée d'abord la n2 (la plus rare), dans la 1re zone qui a de la place
PREMIERE=""
for VM in $VMS; do
  [[ "$(type_de "$VM")" == n2-* ]] && ! echo "$EXIST" | grep -qx "$VM" && { PREMIERE="$VM"; break; }
done
if [ -n "$PREMIERE" ]; then
  ZONE=""
  for z in $ZONE_CONFIG $ZONES_SECOURS; do
    t0_premiere=$(date +%s.%N)
    if creer "$PREMIERE" "$(type_de "$PREMIERE")" "$z"; then
      t1_premiere=$(date +%s.%N)
      ZONE="$z"
      break
    fi
    echo "[$PREMIERE] pas de place en $z : $(grep -o 'code: [A-Z_]*' "/tmp/setup_$PREMIERE.err" | head -1)"
  done
  [ -n "$ZONE" ] || { echo "Erreur : aucune zone n'a de $(type_de "$PREMIERE") $CPU_N2 libre. Rien n'a été créé." >&2; exit 1; }
  [ "$ZONE" = "$ZONE_CONFIG" ] || EXIST=$(gcloud compute instances list --zones="$ZONE" --format="value(name)" < /dev/null)
fi

for VM in $VMS; do
  TYPE=$(type_de "$VM")
  [ -n "$TYPE" ] || { echo "Erreur : $VM absente de $MACH" >&2; exit 1; }
  if [ "$VM" != "$PREMIERE" ] && echo "$EXIST" | grep -qx "$VM"; then
    echo "[$VM] déjà allumée, réutilisée"
    continue
  fi
  (
    if [ "$VM" = "$PREMIERE" ]; then
      t0=$t0_premiere; t1=$t1_premiere
    else
      t0=$(date +%s.%N)
      creer "$VM" "$TYPE" "$ZONE" \
        || { echo "[$VM] ECHEC de création :" >&2; grep -E "ERROR|message|-" "/tmp/setup_$VM.err" | head -5 >&2; exit 1; }
      t1=$(date +%s.%N)
    fi
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

# Processeur obtenu (Model 106 = Ice Lake, 85 = Cascade Lake, 79 = Broadwell, AMD = EPYC)
for VM in $VMS; do
  echo -n "[$VM] "
  gcloud compute ssh "$VM" --zone="$ZONE" --quiet --command="lscpu | grep -E '^Model( name)?:' | tr -s ' ' | tr '\n' ' '" < /dev/null 2> /dev/null
  echo
done
if [ "$ZONE" != "$ZONE_CONFIG" ]; then
  echo "Zone utilisée : $ZONE (pas celle de config.sh). Mettre ZONE=\"$ZONE\" dans scripts/config.sh avant la suite."
fi
if [ "$ECHECS" -gt 0 ]; then
  echo "Erreur : $ECHECS VM(s) non prête(s). Les VMs déjà créées restent allumées : ./scripts/delete_vms.sh" >&2
  exit 1
fi
