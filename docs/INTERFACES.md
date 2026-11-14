# Contrat des fichiers (à respecter par les 3 membres)

Tout changement de format = une pull request qui modifie AUSSI ce fichier, relue par les 2 autres.

## Rôles
| Rôle | Membre | Produit | Utilise |
|---|---|---|---|
| A : infra et orchestrateur | Maxime | machines.csv, .status, results.csv, experiments.csv | allocations |
| B : benchmarks et modèle | ? | profiles.csv, fonction d'évaluation, validation prédit/réel | machines.csv |
| C : NSGA-II | ? | pareto.csv, sol_*.csv, allocation_nsga.csv | tasks, machines, profiles |

## Conventions
- Séparateur virgule, POINT décimal, une ligne d'en-tête, fins de ligne LF.
- Unités dans le nom des colonnes : _s, _kb, _mb, _usd.
- Zone et projet : uniquement dans scripts/config.sh (europe-west9-b).

## Infrastructure (data/machines.csv) – A
    vm,machine_type,vcpu,ram_mb,price_per_hour_usd
    cloudopt-vm1,e2-highcpu-2,2,1976,...
    cloudopt-vm2,e2-standard-2,2,7950,...
    cloudopt-vm3,n2-standard-2,2,7950,...
ram_mb = mémoire vue par Linux (free -m). Fiches détaillées : data/machine_<vm>.txt.
vm1 et vm2 : Xeon 2.20 GHz ; vm3 : Xeon 2.80 GHz.

## Scénario (data/tasks.csv)
    task_id,workload,size[,depends_on]
depends_on est optionnel : les tâches à attendre, séparées par ";" (vide = aucune). Ex. T3 attend T1 et T2 :
    T3,cpu_task,20000000,T1;T2
Les scénarios peuvent être générés : ./bin/gen_tasks -n 40 -seed 1 [-deps 0.4] (menu : data/task_menu.csv).
Le menu ne doit contenir que des couples (workload, taille) mesurés dans profiles.csv.
memory_task : nombre de passages fixé par MEM_PASSES dans scripts/config.sh, le même pour les mesures et les expériences.
IMPORTANT : le surcoût SSH mesuré est d'environ 3 s par tâche.
Choisir des tailles donnant 30 s à 2 min de calcul par tâche, sinon on mesure SSH et pas les machines.

## Mesures (data/profiles.csv) – B
    workload,size,machine_type,runs,mean_runtime_s,median_runtime_s,std_runtime_s,max_rss_kb
Chaque (workload, size) de tasks.csv doit être mesuré sur CHAQUE machine_type.
Le modèle utilise median_runtime_s (robuste aux pics). Temps = runtime_s du JSON.

## Allocation (data/allocation_*.csv) – scheduler -> orchestrateur
    task_id,workload,size,vm
Format inchangé depuis le TD3. Les dépendances restent dans tasks.csv, pas dans l'allocation.
L'ORDRE des lignes compte (files d'attente et dépendances) : c'est l'ordre de priorité.
Le scheduler peut donc choisir la VM ET l'ordre de chaque tâche.

## Exécution – A
3 modes, même format d'entrée et de sortie :
| Mode | Commande | Règle |
|---|---|---|
| tout en même temps | ./scripts/run_experiment.sh ALLOC strat N | toutes les tâches d'une VM partent ensemble (TD3) |
| file par VM | QUEUE=1 ./scripts/run_experiment.sh ALLOC strat N | au plus vcpu tâches à la fois par VM, 1 seule connexion SSH par VM |
| dépendances | DAG=TASKS.csv ./scripts/run_experiment.sh ALLOC strat N | file par VM + respect de depends_on |

Règles du mode dépendances (le modèle doit simuler exactement les mêmes) :
1. une tâche part quand toutes ses dépendances ont réussi, même si elles ont tourné sur une autre VM ;
2. au plus vcpu tâches en même temps par VM ;
3. quand une place se libère, on lance la première tâche prête dans l'ordre de l'allocation ;
4. si une dépendance échoue, ses descendantes ne partent pas (code 99) ;
5. un cycle dans les dépendances est refusé avant de lancer quoi que ce soit.
Dépendance logique seulement : aucune donnée n'est transférée entre tâches.

Surcoûts mesurés (à intégrer au modèle) :
- SSH : environ 3 s par tâche en mode dépendances (une connexion par tâche), environ 3 s par VM en mode file ;
- détection de fin d'une dépendance : jusqu'à 1 s ;
- préparation d'une VM (création + SSH prêt + déploiement) : environ 46 s, voir data/vm_setup.csv ;
- 2 vCPU = 1 seul coeur physique (2 threads) : 2 tâches de calcul sur la même VM se ralentissent
  (2 matrix_task 2200 ensemble : 29 s contre environ 24 s chacune seule).

## Préparation des VMs (data/vm_setup.csv) – A
    date,vm,machine_type,create_s,ssh_ready_s,deploy_s,total_s
./scripts/setup_vms.sh ALLOC crée seulement les VMs utilisées par l'allocation et ajoute une ligne par VM créée.
Quota du projet : 16 vCPU toutes régions confondues, donc 8 VMs à 2 vCPU au maximum.

## Sortie NSGA-II (data/pareto/) – C
- pareto.csv : solution_id,makespan_pred_s,cost_pred_usd,mem_violation_mb
- sol_001.csv, sol_002.csv... : une allocation par solution du front
- data/allocation_nsga.csv : la solution choisie pour l'exécution
- Toujours fixer et noter la graine aléatoire (seed) pour la reproductibilité.

## Résultats d'exécution – A
- results/<experiment_id>/<TASK_ID>.status :
      task_id,workload,size,vm,exit_code,start_epoch_s,end_epoch_s
  (horaires pris sur le PC contrôleur, SSH compris)
- results/<experiment_id>/results.csv :
      experiment_id,task_id,workload,size,vm,machine_type,exit_code,start_s,end_s,duration_s,runtime_s
- data/experiments.csv (une ligne par expérience) :
      experiment_id,strategy,makespan_s,cost_usd,pred_makespan_s,pred_cost_usd,n_tasks,n_failed
- experiment_id : AAAAMMJJ-HHMM_<strategie>_r<k>, ex. 20261023-1530_rr_r1

## Modèle commun de prédiction (identique pour tous)
A REVOIR (B) : la formule ci-dessous ne vaut que pour le mode "tout en même temps".
Avec les modes file et dépendances, le makespan se calcule en simulant les règles de la section Exécution.
Ancienne formule, mode "tout en même temps" (l'OS partage les vCPU) :
    fin(VM)  = max( somme des temps des tâches de la VM / vcpu , temps de la plus longue )
    makespan = max des fin(VM)
    coût     = somme sur les VMs de price_per_hour_usd / 3600 * fin(VM)
    mémoire  = somme des max_rss des tâches d'une VM <= 90 % de ram_mb   (contrainte)
En Go : allocation[i] = indice de la VM de la tâche i (c'est aussi le chromosome NSGA-II).

## Branches git
prod-<prénom> pour travailler, pull request relue par un autre membre pour fusionner.
