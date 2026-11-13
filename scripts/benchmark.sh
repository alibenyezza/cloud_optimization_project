#!/bin/bash
# scripts/benchmark.sh
#
# Script tout simple pour lancer un workload plusieurs fois
# et enregistrer le temps + la mémoire utilisée dans un CSV.
#
# Comment l'utiliser (3 arguments, dans cet ordre) :
#   ./benchmark.sh cpu_task 500000 3
#   ./benchmark.sh memory_task 256 3
#
#   1er argument = nom du workload (cpu_task, memory_task, matrix_task, sort_task)
#   2e argument  = la taille (--size)
#   3e argument  = combien de fois on répète le test

# On récupère les 3 arguments et on leur donne un nom clair
WORKLOAD=$1
SIZE=$2
REPETITIONS=$3
FICHIER=${4:-data/benchmark_local.csv}

# On construit le chemin vers l'exécutable, ex: bin/cpu_task
PROGRAMME="bin/$WORKLOAD"

# Vérification simple : est-ce que le fichier existe ?
if [ ! -f "$PROGRAMME" ]; then
    echo "Erreur : je ne trouve pas $PROGRAMME"
    echo "As-tu bien compilé les workloads avec go build ?"
    exit 1
fi

# On s'assure que le fichier CSV existe et a bien son en-tête
if [ ! -f "$FICHIER" ]; then
    echo "workload,size,run,runtime_s,max_rss_kb" > $FICHIER
fi

echo "Je lance $WORKLOAD avec --size $SIZE, $REPETITIONS fois"

# Boucle : on répète le test REPETITIONS fois
for run in $(seq 1 $REPETITIONS); do

    echo "  -> essai numero $run"

    # On lance le programme avec /usr/bin/time -v
    # -v donne des infos detaillees (temps, memoire)
    # On met tout ca dans un fichier temporaire pour le relire juste apres
    /usr/bin/time -v "$PROGRAMME" --size "$SIZE" > sortie_temp.txt 2> resultat_temp.txt

    # On lit le temps mesure par le programme (runtime_s dans son JSON), en secondes, ex: 0.096345277
    TEMPS=$(grep -o "runtime_s[^,}]*" sortie_temp.txt | cut -d: -f2)

    # On cherche la ligne de la memoire max utilisee, ex: "2432"
    MEMOIRE=$(grep "Maximum resident set size" resultat_temp.txt | awk '{print $NF}')

    # On ajoute une ligne dans le CSV avec toutes ces infos
    echo "$WORKLOAD,$SIZE,$run,$TEMPS,$MEMOIRE" >> $FICHIER

    # On supprime le fichier temporaire, on n'en a plus besoin
    rm resultat_temp.txt sortie_temp.txt

done

echo "Termine ! Regarde $FICHIER"
