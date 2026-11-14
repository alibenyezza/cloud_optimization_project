#!/usr/bin/env bash
# Configuration commune, lue par tous les scripts avec : source "$(dirname "$0")/config.sh"
PROJECT="cloud-optimization-511007"
ZONE="europe-west9-b"
MACHINES_FILE="data/machines.csv"
# Nombre de passages de memory_task sur sa mémoire (défaut du programme : 5).
# Avec 5, memory_task dure moins de 2 s même à 1 Go : impossible d'atteindre 30 s avec --size seul.
# Doit être IDENTIQUE pour les mesures (profile_cloud.sh) et les expériences (run_task.sh).
MEM_PASSES="${MEM_PASSES:-5}"
