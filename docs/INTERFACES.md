# Contrat des fichiers (à respecter par les 3 membres)

Tout changement de format = une pull request qui modifie AUSSI ce fichier, relue par les 2 autres.

## Rôles
| Rôle | Membre | Produit | Utilise |
|---|---|---|---|
| A : infra et orchestrateur | Maxime | machines.csv, .status, results.csv, experiments.csv | allocations |
| B : benchmarks et modèle | Ali | profiles.csv, fonction d'évaluation, validation prédit/réel | machines.csv |
| C : NSGA-II | ? | pareto.csv, sol_*.csv, allocation_nsga.csv | tasks, machines, profiles |

## Conventions
- Séparateur virgule, POINT décimal, une ligne d'en-tête, fins de ligne LF.
- Unités dans le nom des colonnes : _s, _kb, _mb, _usd.
- Zone et projet : uniquement dans scripts/config.sh (europe-west9-a).

## Infrastructure (data/machines.csv) – A
    vm,machine_type,vcpu,ram_mb,price_per_hour_usd
    cloudopt-vm1,e2-highcpu-2,2,1976,...
    cloudopt-vm2,e2-standard-2,2,7950,...
    cloudopt-vm3,n2-standard-2,2,7950,...
ram_mb = mémoire vue par Linux (free -m). Fiches détaillées : data/machine_<vm>.txt.
vm1 et vm2 : Xeon 2.20 GHz ; vm3 : Xeon 2.80 GHz.

## Scénario (data/tasks.csv)
    task_id,workload,size
IMPORTANT : le surcoût SSH mesuré est d'environ 3 s par tâche.
Choisir des tailles donnant 30 s à 2 min de calcul par tâche, sinon on mesure SSH et pas les machines.

## Mesures (data/profiles.csv) – B
    workload,size,machine_type,runs,mean_runtime_s,median_runtime_s,std_runtime_s,max_rss_kb
Chaque (workload, size) de tasks.csv doit être mesuré sur CHAQUE machine_type.
Le modèle utilise median_runtime_s (robuste aux pics). Temps = runtime_s du JSON.
Mesures brutes : data/benchmark_<machine_type>.csv (scripts/benchmark_vms.sh),
puis profiles.csv avec scripts/make_profiles.sh.

## Allocation (data/allocation_*.csv) – scheduler -> orchestrateur
    task_id,workload,size,vm
Format inchangé depuis le TD3 : run_all.sh lit n'importe quel fichier de ce format.

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
run_all.sh lance en même temps toutes les tâches d'une VM. Nos VMs ont 2 vCPU mais 1 seul coeur
physique (2 threads, voir lscpu) : deux tâches simultanées vont à peu près aussi vite qu'en série.
    fin(VM)  = somme des temps des tâches de la VM   (validé : +3 % sur le makespan Round-Robin)
    makespan = max des fin(VM)
    coût     = somme sur les VMs de price_per_hour_usd / 3600 * fin(VM)
    mémoire  = somme des max_rss des tâches d'une VM <= 90 % de ram_mb   (contrainte)
En Go : allocation[i] = indice de la VM de la tâche i (c'est aussi le chromosome NSGA-II).
Implémenté dans tools/predict/main.go (B). Dernière ligne affichée :
    prediction,<makespan_s>,<cost_usd>,<mem_violation_mb>

## Branches git
prod-<prénom> pour travailler, pull request relue par un autre membre pour fusionner.
