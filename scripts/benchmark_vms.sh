#!/usr/bin/env bash
# Mesure chaque couple (workload, size) de tasks.csv sur chaque VM de machines.csv.
# Sur une VM, les mesures passent une par une (benchmark.sh) ; les VMs tournent en parallèle.
# Résultat brut : data/benchmark_<machine_type>.csv
# Usage : ./scripts/benchmark_vms.sh [RUNS] [tasks.csv]
# Avant : create_vms.sh puis deploy_all.sh
source "$(dirname "$0")/config.sh"
RUNS="${1:-5}"
TASKS="${2:-data/tasks.csv}"
CONFIGS=$(tail -n +2 "$TASKS" | tr -d '\r' | cut -d, -f2,3 | sed '/^$/d' | sort -u | tr '\n' ' ')
echo "Couples mesurés : $CONFIGS"

while IFS=, read -r vm type vcpu ram price; do
  [ -z "$vm" ] && continue
  (
    gcloud compute scp scripts/benchmark.sh "$vm":~/cloud-optimization/ --zone="$ZONE" --quiet < /dev/null
    gcloud compute ssh "$vm" --zone="$ZONE" --quiet --command="
      sudo apt-get update -qq && sudo apt-get install -y -qq time > /dev/null
      cd ~/cloud-optimization && mkdir -p data && rm -f data/bench.csv
      for c in $CONFIGS; do bash benchmark.sh \${c%,*} \${c#*,} $RUNS data/bench.csv; done" \
      < /dev/null > /dev/null
    gcloud compute scp "$vm":~/cloud-optimization/data/bench.csv "data/benchmark_$type.csv" \
      --zone="$ZONE" --quiet < /dev/null
    echo "$vm ($type) : terminé, voir data/benchmark_$type.csv"
  ) &
done < <(tail -n +2 "$MACHINES_FILE" | tr -d '\r')

wait
echo "Mesures terminées. Ensuite : ./scripts/make_profiles.sh"
