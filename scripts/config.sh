#!/usr/bin/env bash
# Configuration commune, lue par tous les scripts avec : source "$(dirname "$0")/config.sh"
PROJECT="cloud-optimization-511007"
ZONE="europe-west9-a"
MACHINES_FILE="${MACHINES_FILE:-data/machines.csv}"
# Nombre de passages de memory_task sur sa mémoire (défaut du programme : 5).
# Avec 5, memory_task dure moins de 2 s même à 1 Go : impossible d'atteindre 30 s avec --size seul.
# Les mesures de profiles.csv (benchmark_vms.sh) ont été faites avec le défaut 5 :
# ne pas changer cette valeur sans remesurer memory_task.
MEM_PASSES="${MEM_PASSES:-5}"
