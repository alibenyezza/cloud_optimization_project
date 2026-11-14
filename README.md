# Cloud Optimization – Ordonnancement de tâches sur GCP avec NSGA-II

Projet semestriel ESILV A5 CCC. Question centrale : **étant donné des tâches et des VMs,
où exécuter chaque tâche ?** On compare une allocation de référence (Round-Robin) à une
allocation optimisée par **NSGA-II** (algorithme génétique multi-objectif : makespan et coût).

## Architecture

    tasks.csv + machines.csv + profiles.csv
                    |
                    v
       SCHEDULER (Round-Robin ou NSGA-II)
                    |
                    v
             allocation_*.csv
                    |
                    v
    ORCHESTRATEUR (run_experiment.sh -> run_all.sh | run_all_queue.sh | run_all_dag.sh)
          |              |              |
          v              v              v
    cloudopt-vm1   cloudopt-vm2   cloudopt-vm3
          \              |              /
           v             v             v
    results/<id>/results.csv + gantt.svg  ->  data/experiments.csv

- **Workloads** (`workloads/`) : programmes Go compilés (cpu_task, memory_task, matrix_task, sort_task).
- **Scheduler** (`scheduler/`) : décide sur quelle VM tourne chaque tâche. Ne lance rien.
- **Orchestrateur** (`scripts/`) : exécute une allocation en parallèle par SSH et collecte les résultats.
  Il ne prend aucune décision : n'importe quelle allocation au bon format peut être exécutée.
  3 modes : tout en même temps (`run_all.sh`), file d'attente par VM (`run_all_queue.sh`),
  file d'attente + dépendances entre tâches (`run_all_dag.sh`).

Les formats de tous les fichiers sont définis dans **[docs/INTERFACES.md](docs/INTERFACES.md)**.

## Infrastructure

Projet GCP `cloud-optimization-511007`, zone `europe-west9-a` (Paris), image Debian 12.
Zone et projet sont définis une seule fois dans `scripts/config.sh`.

| VM | Type | vCPU | RAM (Linux) | CPU |
|---|---|---|---|---|
| cloudopt-vm1 | e2-highcpu-2 | 2 | 1976 Mo | Xeon 2.20 GHz |
| cloudopt-vm2 | e2-standard-2 | 2 | 7950 Mo | Xeon 2.20 GHz |
| cloudopt-vm3 | n2-standard-2 | 2 | 7950 Mo | Xeon 2.80 GHz |

Les VMs sont volontairement hétérogènes : vm1 est peu chère mais limitée en mémoire,
vm3 a un CPU plus rapide mais coûte plus cher. Fiches détaillées : `data/machine_<vm>.txt`.
Prix horaires (`data/machines.csv`) : tarifs à la demande europe-west9, relevés le 23/10/2026 sur gcloud-compute.com (à recouper avec la console GCP).

## Arborescence

    workloads/      sources Go des 4 workloads
    scheduler/      round_robin.go (baseline), nsga2.go (NSGA-II, sorties dans data/pareto/)
    tools/gantt/    timeline SVG d'une expérience (flèches de dépendance et chemin critique si tasks.csv)
    tools/predict/  makespan, coût et mémoire prévus d'une allocation (modèle de INTERFACES.md)
    tools/gen_tasks/ génération de scénarios reproductibles (graine), avec ou sans dépendances
    scripts/        config.sh, create_vms.sh, setup_vms.sh, deploy_all.sh, delete_vms.sh,
                    run_task.sh, run_all.sh, run_all_queue.sh, run_all_dag.sh,
                    collect_results.sh, run_experiment.sh, scaling_experiment.sh, demo.sh,
                    benchmark.sh, benchmark_vms.sh, make_profiles.sh
    data/           tasks.csv, machines.csv, profiles.csv, allocations, experiments.csv, pareto/
    results/        un dossier par expérience (.out, .err, .status, results.csv, gantt.svg)
    docs/           INTERFACES.md (contrat des fichiers), write-ups des TD1 à TD3, rapport Word
    bin/            exécutables compilés (non versionnés)

## Prérequis

Ubuntu (testé sous WSL2), Go 1.26, Google Cloud CLI authentifié sur le projet
(`gcloud auth login --no-launch-browser` sous WSL, puis `gcloud config set project cloud-optimization-511007`).

## Reproduire une expérience

    # 1. Compiler
    for w in cpu_task memory_task matrix_task sort_task; do go build -o bin/$w workloads/$w.go; done
    go build -o bin/round_robin_scheduler scheduler/round_robin.go
    go build -o bin/gantt tools/gantt/main.go
    go build -o bin/predict tools/predict/main.go
    go build -o bin/nsga2 scheduler/nsga2.go

    # 2. (Seulement si profiles.csv n'est pas à jour pour ce tasks.csv) Mesurer chaque tâche
    #    sur chaque type de VM (5 fois, environ 20 min), puis faire profiles.csv
    ./scripts/create_vms.sh && sleep 60 && ./scripts/deploy_all.sh
    ./scripts/benchmark_vms.sh 5
    ./scripts/make_profiles.sh

    # 3. Calculer les allocations (Round-Robin et NSGA-II) et leurs prédictions
    ./bin/round_robin_scheduler data/tasks.csv data/machines.csv data/allocation_rr.csv
    ./bin/predict data/tasks.csv data/machines.csv data/profiles.csv data/allocation_rr.csv
    ./bin/nsga2 data/tasks.csv data/machines.csv data/profiles.csv 42     # -> data/allocation_nsga.csv
    ./bin/predict data/tasks.csv data/machines.csv data/profiles.csv data/allocation_nsga.csv

    # 4. Créer seulement les VMs utilisées par l'allocation, attendre SSH, déployer (temps mesuré)
    ./scripts/setup_vms.sh data/allocation_rr.csv

    # 5. Exécuter, collecter et tracer la timeline (3 répétitions, mode file), avec makespan et coût prévus
    QUEUE=1 ./scripts/run_experiment.sh data/allocation_rr.csv rr 3 MAKESPAN_PREVU COUT_PREVU
    QUEUE=1 ./scripts/run_experiment.sh data/allocation_nsga.csv nsga 3 MAKESPAN_PREVU COUT_PREVU

    # 6. Supprimer les VMs en fin de session (elles coûtent même inactives)
    ./scripts/delete_vms.sh

### Avec beaucoup de tâches et des dépendances

    go build -o bin/gen_tasks tools/gen_tasks/main.go
    ./bin/gen_tasks -n 40 -seed 1 -deps 0.4 -out data/tasks_40_dag.csv     # 40 tâches, 40 % avec dépendances
    ./bin/round_robin_scheduler data/tasks_40_dag.csv data/machines.csv data/allocation_rr.csv
    ./scripts/setup_vms.sh data/allocation_rr.csv                            # seulement les VMs utilisées, temps mesuré
    DAG=data/tasks_40_dag.csv ./scripts/run_experiment.sh data/allocation_rr.csv rr 3
    ./scripts/delete_vms.sh

- `QUEUE=1` à la place de `DAG=...` : file d'attente par VM sans dépendances.
- `./scripts/scaling_experiment.sh TASKS.csv "1 2 3 4 6"` : même lot sur 1, 2, 3... VMs identiques.
- `./scripts/demo.sh` : démo de soutenance (VMs allumées avant).
- Règles exactes de chaque mode et surcoûts mesurés : docs/INTERFACES.md, section Exécution.

Chaque répétition produit `results/<AAAAMMJJ-HHMM>_<strategie>_r<k>/` avec :
- `allocation.csv` : l'allocation exacte exécutée ;
- `<TASK>.out`, `.err`, `.status` : sortie, erreurs, code de retour et horaires de chaque tâche ;
- `results.csv` : une ligne par tâche, temps relatifs au début de l'expérience ;
- `gantt.svg` : timeline (une bande par VM, tâches simultanées empilées, échecs en rouge).

`data/experiments.csv` reçoit une ligne par expérience : makespan réel, coût réel,
valeurs prédites, nombre de tâches et d'échecs. C'est la base de la comparaison Round-Robin / NSGA-II.

Scripts utiles séparément :
- `./scripts/run_all.sh ALLOCATION.csv [EXPERIMENT_ID]` : exécution seule ;
- `./scripts/collect_results.sh EXPERIMENT_ID STRATEGY` : collecte seule ;
- `./bin/gantt results/<id>/results.csv sortie.svg` : timeline seule.

## Mesure : points de vigilance

- **Surcoût SSH d'environ 3 s par tâche** (mesuré) : les horaires sont pris sur le PC contrôleur,
  connexion SSH comprise. Le scénario utilise donc des tâches de 30 s à 2 min, pour que ce surcoût
  reste faible devant le calcul. Le temps de calcul pur reste disponible (`runtime_s`).
- **Partage du CPU** : `run_all.sh` lance en même temps toutes les tâches d'une VM ; `run_all_queue.sh`
  et `run_all_dag.sh` en lancent au plus vcpu à la fois. Une VM à 2 vCPU n'a qu'un seul coeur physique :
  deux tâches de calcul sur la même VM se ralentissent (voir INTERFACES.md, section Exécution).
- **Démarrage d'une VM** : environ 46 s (création + SSH prêt + déploiement), mesuré dans `data/vm_setup.csv`.
- **Variabilité** : un même scénario varie d'environ 1 à 2 % d'une exécution à l'autre ;
  chaque stratégie est donc répétée plusieurs fois.
- **Validation** : chaque `.out` commence par `host: <nom de la VM>`, ce qui prouve où la tâche a tourné.

## Historique

- TD1 : benchmarks locaux (`docs/write_up_TD1.txt`).
- TD2 : première VM GCP, comparaison local / cloud (`docs/write_up_TD2.txt`).
- TD3 : orchestration parallèle sur 3 VMs (`docs/write_up_TD3.txt`).
- TD4 : baseline Round-Robin (`scheduler/round_robin.go`, tests dans `tests_td4/`, résultats dans `results_rr/`).
- Partie B : scénario final, mesures sur les 3 types de VM, `profiles.csv` et validation du modèle (`docs/write_up_partie_B.txt`).
- TD5 : file d'attente par VM, VMs à la demande, dépendances entre tâches, générateur de scénarios,
  expérience sur le nombre de VMs (3 VMs plus lentes que 2 sur le scénario de démo).
- Partie C : NSGA-II (`scheduler/nsga2.go`), vérifié par recherche exhaustive sur le scénario de 12 tâches.

## Rapport (Word)

Le rapport est dans `docs/rapport_cloud_optimization.docx`. Les parties 3, 4, 5 et le début
de la 10 sont faites et à jour du TD5 (Maxime). Les parties marquées "A compléter" sont à écrire par chacun selon
sa partie du projet.

Règles d'écriture, pour que tout le rapport ait le même style :

- En français.
- Uniquement des caractères tapables sur un clavier AZERTY : accents et caractères spéciaux
  du clavier oui, mais pas de tiret long, de flèche, de guillemets « », de œ ni de majuscule
  accentuée. On écrit "-" , "->", "coeur", "A compléter".
- Style minimaliste : court, clair, concis. Des phrases simples, on écrit "on".
  Le texte doit sonner comme nous, pas comme une IA.
- Chaque choix technique est justifié avec son avantage ET son inconvénient.
- Chaque chiffre vient d'une vraie mesure (experiments.csv, results.csv, profiles.csv),
  jamais d'une estimation.
- Les limites et les résultats inattendus sont expliqués, pas cachés.
- On garde le plan en 11 parties : c'est celui que demande le sujet pour la soutenance.
- Objectif : chaque partie doit répondre à un critère de la grille de notation (fin du sujet).

Un fichier Word ne se fusionne pas avec git. Pour ne pas perdre de travail :

1. Prévenir les autres avant de l'ouvrir ("je prends le Word").
2. `git pull` avant de commencer.
3. Faire ses modifications.
4. `git add`, `git commit`, `git push` tout de suite, puis prévenir que c'est libre.
