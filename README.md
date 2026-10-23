# Cloud Optimization – Exécution distribuée (TD3)

Exécution de tâches indépendantes sur plusieurs VMs Google Compute Engine,
pilotées depuis une machine contrôleur (PC sous Ubuntu/WSL).

## Infrastructure

- Projet GCP : `cloud-optimization-511007`
- Zone : `europe-west9-b`
- VMs : `cloudopt-vm1`, `cloudopt-vm2`, `cloudopt-vm3` (e2-small, Debian 12)
- Workloads : `cpu_task`, `matrix_task`, `memory_task`, `sort_task` (dans `bin/`)

## Organisation

    bin/                        exécutables des workloads
    data/allocation_manual.csv  liste des tâches et VM choisie pour chacune
    scripts/deploy_all.sh       copie bin/ sur toutes les VMs
    scripts/run_task.sh         exécute UNE tâche sur UNE VM
    scripts/run_all.sh          lance toutes les tâches du CSV en parallèle
    results/                    un .out, un .err et un .status par tâche

L'allocation (quoi / où) est séparée de l'exécution (comment) :
changer de placement = modifier uniquement la colonne `vm` du CSV.

## Utilisation

1. Démarrer les VMs si elles sont arrêtées :
   `gcloud compute instances start cloudopt-vm1 cloudopt-vm2 cloudopt-vm3 --zone=europe-west9-b`
2. Déployer les binaires : `./scripts/deploy_all.sh`
3. Éditer `data/allocation_manual.csv` (format `task_id,workload,size,vm`)
4. Lancer : `./scripts/run_all.sh` (ou `./scripts/run_all.sh autre_fichier.csv`)

`run_all.sh` lance chaque tâche en arrière-plan (`&`), sans attendre la précédente,
puis attend la fin de toutes avec un seul `wait`. Plusieurs tâches sur la même VM
s'exécutent en même temps ; l'OS de la VM partage le CPU entre elles.

## Résultats et validation

Pour chaque tâche : `results/TX.out` (sortie, 1re ligne = nom de la VM réelle),
`results/TX.err` (erreurs) et `results/TX.status` (`task_id,workload,size,vm,code_retour`).

`run_all.sh` affiche un bilan et renvoie 1 si une tâche a échoué.

Vérifier que chaque tâche a tourné sur la bonne VM :

    tail -n +2 data/allocation_manual.csv | while IFS=, read -r id w s vm; do
      [ -z "$id" ] && continue
      echo "$id prévu=$vm réel=$(grep host results/$id.out | cut -d' ' -f2)"
    done

## Fin de session

Arrêter les VMs pour économiser les crédits :
`gcloud compute instances stop cloudopt-vm1 cloudopt-vm2 cloudopt-vm3 --zone=europe-west9-b`

## TD4 – Scheduler Round-Robin (baseline)

Le scheduler décide sur quelle VM chaque tâche s'exécute ; l'orchestrateur du TD3 (run_all.sh / run_task.sh) l'exécute sans modification d'architecture.

Fichiers d'entrée :
- data/tasks.csv : tâches à exécuter (task_id,workload,size), sans VM
- data/machines.csv : VMs disponibles (vm), dans l'ordre du Round-Robin

Compilation :
    go build -o bin/round_robin_scheduler scheduler/round_robin.go

Reproduire le workflow :
    ./bin/round_robin_scheduler data/tasks.csv data/machines.csv data/allocation_rr.csv
    ./scripts/run_all.sh data/allocation_rr.csv

Sans argument, run_all.sh utilise toujours data/allocation_manual.csv.
Les VMs sont en europe-west9-b ; déployer les binaires avec ./scripts/deploy_all.sh.

Round-Robin est une baseline de référence, pas la stratégie d'optimisation finale :
il ignore la charge des tâches, la capacité des VMs et le coût.
