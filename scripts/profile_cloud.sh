#!/usr/bin/env bash
# Mesure chaque couple (workload, taille) du scénario sur CHAQUE VM, pour remplir profiles.csv.
#
# Ce qui change par rapport à benchmark.sh :
#  - ça tourne sur les VMs (benchmark.sh mesure le PC local) ;
#  - les 3 VMs mesurent EN MÊME TEMPS (durée totale = celle de la VM la plus lente, pas la somme) ;
#  - sur une VM, les mesures passent UNE PAR UNE (sinon elles se partagent le CPU et les temps sont faux) ;
#  - une seule connexion SSH par VM (pas 3 s perdues à chaque mesure) ;
#  - chaque couple (workload, taille) n'est mesuré qu'une fois, même s'il apparaît dans plusieurs tâches.
#
# Usage   : ./scripts/profile_cloud.sh [TASKS.csv] [REPETITIONS] [SORTIE]
# Exemple : ./scripts/profile_cloud.sh data/tasks.csv 3 data/profiles.csv
# Brut    : data/profiles_raw/<vm>.csv (une ligne par mesure, pour vérifier ou refaire les stats)
#
# Durée ~= (nb de couples distincts) x REPETITIONS x (temps moyen d'une tâche sur la VM la plus lente).
source "$(dirname "$0")/config.sh"

TASKS="${1:-data/tasks.csv}"
REPS="${2:-3}"
OUTF="${3:-data/profiles.csv}"
RAW="data/profiles_raw"

[ -f "$TASKS" ] || { echo "Erreur : $TASKS introuvable." >&2; exit 1; }
[[ "$REPS" =~ ^[0-9]+$ ]] && [ "$REPS" -ge 1 ] || { echo "Erreur : REPETITIONS doit être un entier >= 1." >&2; exit 1; }
if [ -f "$OUTF" ] && [ "$(wc -l < "$OUTF")" -gt 1 ]; then
  echo "Erreur : $OUTF contient déjà des mesures. Choisis un autre fichier de sortie ou supprime-le." >&2
  exit 1
fi

# Couples (workload, taille) distincts du scénario
PAIRS=$(tail -n +2 "$TASKS" | tr -d '\r' | awk -F, 'NF >= 3 && $2 != "" {print $2 " " $3}' | sort -u)
NP=$(echo "$PAIRS" | sed '/^$/d' | wc -l)
VMS=$(tail -n +2 "$MACHINES_FILE" | tr -d '\r' | cut -d, -f1 | sed '/^$/d')
echo "$NP couples (workload, taille) x $REPS répétitions, sur : $(echo $VMS)"
echo "memory_task lancé avec --passes $MEM_PASSES (voir config.sh)"
mkdir -p "$RAW"

# Script exécuté sur chaque VM : mesures en série, une ligne CSV par mesure sur stdout
JOB=$(mktemp)
{
  echo 'cd ~/cloud-optimization || exit 1'
  echo 'if [ ! -x /usr/bin/time ]; then sudo apt-get -qq update > /dev/null && sudo apt-get -qq install -y time > /dev/null; fi'
  echo 'TYPE=$(curl -s -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/machine-type | awk -F/ "{print \$NF}")'
  echo "for pair in $(echo "$PAIRS" | sed '/^$/d' | tr ' ' ':' | tr '\n' ' '); do"
  echo '  w=${pair%%:*}; s=${pair#*:}; extra=""'
  echo "  [ \"\$w\" = memory_task ] && extra=\"--passes $MEM_PASSES\""
  echo "  for r in \$(seq 1 $REPS); do"
  echo '    /usr/bin/time -v ./bin/$w --size $s $extra > ./p.out 2> ./p.err'
  echo '    rt=$(grep -o "\"runtime_s\":[0-9.eE+-]*" ./p.out | cut -d: -f2)'
  echo '    rss=$(grep "Maximum resident set size" ./p.err | awk "{print \$NF}")'
  echo '    echo "$TYPE,$w,$s,$r,${rt:-NA},${rss:-NA}"'
  echo '    echo "  [$(hostname)] $w $s essai $r : ${rt:-ECHEC} s" >&2'
  echo '  done'
  echo 'done'
} > "$JOB"

START=$(date +%s)
for VM in $VMS; do
  (
    gcloud compute scp "$JOB" "$VM":~/cloud-optimization/profile_job.sh --zone="$ZONE" --quiet < /dev/null > /dev/null 2>&1 \
      || { echo "Copie impossible vers $VM" >&2; exit 1; }
    echo "machine_type,workload,size,run,runtime_s,max_rss_kb" > "$RAW/$VM.csv"
    gcloud compute ssh "$VM" --zone="$ZONE" --quiet --command="bash ~/cloud-optimization/profile_job.sh" \
      < /dev/null >> "$RAW/$VM.csv"
    echo "=== $VM terminé"
  ) &
done
wait
rm -f "$JOB"
echo "Mesures terminées en $(( ($(date +%s) - START) / 60 )) min $(( ($(date +%s) - START) % 60 )) s"

# Agrégation : moyenne, médiane, écart-type, RSS max, par (workload, taille, type de machine)
echo "workload,size,machine_type,runs,mean_runtime_s,median_runtime_s,std_runtime_s,max_rss_kb" > "$OUTF"
cat "$RAW"/*.csv | grep -v '^machine_type,' | awk -F, '$5 != "NA"' \
  | sort -t, -k2,2 -k3,3n -k1,1 -k5,5g \
  | awk -F, '
    function flush() {
      if (n == 0) return
      m = s / n; v = 0
      for (i = 1; i <= n; i++) v += (x[i] - m) ^ 2
      sd = (n > 1) ? sqrt(v / (n - 1)) : 0
      med = (n % 2) ? x[(n + 1) / 2] : (x[n / 2] + x[n / 2 + 1]) / 2
      printf "%s,%s,%s,%d,%.6f,%.6f,%.6f,%d\n", w, sz, t, n, m, med, sd, rss
    }
    { key = $2 "," $3 "," $1
      if (key != prev) { flush(); prev = key; n = 0; s = 0; rss = 0; w = $2; sz = $3; t = $1 }
      x[++n] = $5; s += $5; if ($6 + 0 > rss) rss = $6 + 0 }
    END { flush() }' >> "$OUTF"

FAILS=$(cat "$RAW"/*.csv | grep -v '^machine_type,' | awk -F, '$5 == "NA"' | wc -l)
echo "Écrit : $OUTF ($(($(wc -l < "$OUTF") - 1)) lignes). Mesures en échec : $FAILS"
[ "$FAILS" -eq 0 ] || echo "Attention : des mesures ont échoué (souvent mémoire insuffisante sur vm1), voir $RAW/" >&2
