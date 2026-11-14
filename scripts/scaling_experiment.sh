#!/usr/bin/env bash
# Expérience "nombre de VMs" : le même lot de tâches exécuté avec 1, 2, 3... VMs identiques.
# But : montrer avec des mesures qu'ajouter des VMs n'accélère pas toujours
# (la plus longue tâche reste une limite, chaque VM a un surcoût fixe, et le coût monte).
#
# Les VMs sont toutes du même type pour que seul leur NOMBRE change.
# Ordonnancement : Round-Robin, exécution avec la file d'attente par VM.
# Chaque expérience s'appelle scale_k<nombre> dans data/experiments.csv.
# Les VMs déjà allumées sont réutilisées ; tout est supprimé à la fin.
#
# Usage   : ./scripts/scaling_experiment.sh TASKS.csv "1 2 3 4 6" [TYPE] [REPETITIONS]
# Exemple : ./scripts/scaling_experiment.sh data/tasks.csv "1 2 3 4 6" e2-standard-2 2
source "$(dirname "$0")/config.sh"
TASKS="${1:?Usage : $0 TASKS.csv \"1 2 3 4 6\" [TYPE] [REPETITIONS]}"
LIST="${2:-1 2 3 4 6}"
TYPE="${3:-e2-standard-2}"
REP="${4:-1}"
[ -f "$TASKS" ] || { echo "Erreur : $TASKS introuvable." >&2; exit 1; }
MAXK=$(echo $LIST | tr ' ' '\n' | sort -n | tail -1)
[ "$MAXK" -le 8 ] || { echo "Erreur : quota du projet = 16 vCPU, soit 8 VMs à 2 vCPU au maximum." >&2; exit 1; }

# Prix et RAM du type choisi, pris dans machines.csv (une VM de ce type doit y figurer)
REF=$(tr -d '\r' < "$MACHINES_FILE" | awk -F, -v t="$TYPE" '$2 == t {print $3 "," $4 "," $5; exit}')
[ -n "$REF" ] || { echo "Erreur : aucun $TYPE dans $MACHINES_FILE (il faut son prix et sa RAM)." >&2; exit 1; }

DIR="data/scaling"
mkdir -p "$DIR"
POOL="$DIR/machines_pool.csv"
echo "vm,machine_type,vcpu,ram_mb,price_per_hour_usd" > "$POOL"
for i in $(seq 1 "$MAXK"); do echo "cloudopt-s$i,$TYPE,$REF" >> "$POOL"; done

for k in $LIST; do
  echo "################ $k VM(s) ################"
  head -n $((k + 1)) "$POOL" > "$DIR/machines_k$k.csv"
  ./bin/round_robin_scheduler "$TASKS" "$DIR/machines_k$k.csv" "$DIR/allocation_k$k.csv" || exit 1
  ./scripts/setup_vms.sh "$DIR/allocation_k$k.csv" "$POOL" || exit 1
  MACHINES_FILE="$POOL" QUEUE=1 ./scripts/run_experiment.sh "$DIR/allocation_k$k.csv" "scale_k$k" "$REP" || true
done

echo "################ Suppression des VMs ################"
./scripts/delete_vms.sh "$POOL"
echo
echo "Résultats : lignes scale_k* de data/experiments.csv ; surcoût de démarrage : data/vm_setup.csv"
grep -E '^experiment_id|_scale_k[0-9]+_r' data/experiments.csv
