# Bilan : démos du 16/11 et campagne du 17/11 (Théotime)

Deux séries de mesures, après la campagne de Maxime (`data/campagne/BILAN.md`).
Toutes les lignes sont dans `data/experiments.csv` (stratégies `demo_*`, `be12_*`, `be40_*`),
les sorties et Gantt dans `results/20261116-*_demo_*` et `results/20261117-*`.

## 1. Expérience contrôlée (16/11 au soir, Paris) : seul le processeur de vm3 change

Même scénario (12 tâches), mêmes allocations (`demo.sh`). Entre les deux démos, vm3 a été recréée avec
`--min-cpu-platform="Intel Ice Lake"`. Sorties brutes de `lscpu` et des tests : `tests_directs_processeur.txt`.

| vm3 (n2-standard-2) | Modèle CPU | cpu_task 20M seul | Round-Robin | NSGA-II |
|---|---|---|---|---|
| Cascade Lake (17h29) | 85, 2,80 GHz | 19,9 s | 53,9 s | 86,9 s |
| Ice Lake (18h06) | 106, 2,60 GHz | 7,1 s | 78,4 s | 48,2 s |

cpu_task fait presque seulement des divisions entières : Ice Lake les fait 2,8 fois plus vite que
Cascade Lake. Les profils du 13/11 (7,2 s) ont donc été mesurés sur un Ice Lake. Avec un Cascade Lake,
NSGA-II (qui met les cpu_task sur vm3) perd ; avec un Ice Lake, il gagne (-38 % de temps, -49 % de coût).
Une n2 ne garantit pas son processeur, mais on peut l'imposer. Pas une e2.
Graphe : `pareto_mesure_16-11.svg`.

## 2. Campagne du 17/11 (Belgique) : mesurer, optimiser et exécuter dans la même session

Paris n'avait plus de n2 (`ZONE_RESOURCE_POOL_EXHAUSTED`). VMs créées en `europe-west1-b`
(zone changée temporairement dans `scripts/config.sh`, remise ensuite). vm3 forcée en Ice Lake.
vm1 et vm2 sont tombées sur des AMD EPYC 7B12 : cpu_task 20M en 5,2 s (25,4 s dans les profils du 13/11).

Profils remesurés sur ces VMs (`benchmark_vms.sh 5` puis `make_profiles.sh`) : `profiles_17-11.csv`
et `benchmark_*.csv`. Puis Round-Robin, glouton (`scheduler/greedy.go`) et NSGA-II (graine 42) recalculés
avec ces profils (`allocation_*.csv`, fronts dans `pareto_12/` et `pareto_40/`), et exécutés sur les mêmes VMs.

| Scénario | Stratégie | Makespan mesuré | Coût moyen | Prévu | VMs | Echecs |
|---|---|---|---|---|---|---|
| 12 tâches, file (3 runs) | Round-Robin | 26,4 s | 0,00504 USD | 30,7 s | 3 | 0 |
| 12 tâches, file (3 runs) | Glouton (HEFT) | 26,8 s | 0,00507 USD | 30,0 s | 3 | 0 |
| 12 tâches, file (3 runs) | NSGA-II | 35,5 s | 0,00320 USD | 41,0 s | 2 | 0 |
| 40 tâches, dépendances | Round-Robin | 106,1 s | 0,01053 USD | 142,1 s | 3 | 0 |
| 40 tâches, dépendances | Glouton (HEFT) | 85,6 s | 0,00912 USD | 108,5 s | 3 | 2 |
| 40 tâches, dépendances | NSGA-II | 86,0 s | 0,00914 USD | 101,2 s | 3 | 0 |

- 40 tâches : NSGA-II 19 % plus rapide et 13 % moins cher que Round-Robin, sans échec.
- 12 tâches : NSGA-II abandonne vm3 (2 fois plus chère, pas plus rapide ce jour-là). Solution choisie
  (point idéal) : 37 % moins chère, 35 % plus lente. Sa solution la plus rapide (prévue 27,8 s) n'a pas été exécutée.
- NSGA-II retrouve le vrai front sur 12 tâches (3 solutions sur 3, recherche exhaustive).
- Le modèle surestime le temps de 12 à 34 % (hyperthreading non compté) mais classe bien les stratégies.
- Graphe : `pareto_mesure_17-11.svg`.

## 3. Résultat inattendu : la mémoire dépend du moment

Glouton, 40 tâches : T07 (memory_task 1024, environ 1 Go) a été tuée par Linux sur vm1 (code 137,
`Killed` dans `results/20261117-1029_be40_greedy_r1/T07.err`). Elle tournait en même temps que T32
(sort_task 100 000 000, environ 0,8 Go) sur une VM de 2 Go. T30, qui l'attendait, n'a pas été lancée (code 99).
Le modèle prévoyait 0 dépassement : dans sa simulation, T07 et T32 ne se chevauchaient pas. Les tâches
ont fini plus tôt que prévu et les chevauchements ont changé. La mémoire n'est vérifiée qu'aux instants
prévus. Piste : vérifier le pire cas (les 2 tâches les plus gourmandes d'une VM ensemble).
