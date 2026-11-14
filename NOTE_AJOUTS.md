# Ajouts proposés (copie du repo, rien n'est poussé sur GitHub)

Tous les fichiers ci-dessous sont nouveaux, sauf `scripts/config.sh` et `scripts/run_task.sh`
(2 petites modifs, décrites plus bas). Les formats de INTERFACES.md ne changent pas.

| Fichier | Rôle | Critère de la grille visé |
|---|---|---|
| `scripts/profile_cloud.sh` | Remplit profiles.csv en mesurant sur les 3 VMs en parallèle | Metrics quality, reproductibilité |
| `tools/pareto_check/` | Calcule le VRAI front de Pareto en testant toutes les allocations, compare NSGA-II et Round-Robin | Method justification, Baseline vs optimized |
| `tools/compare/` | Tableau + figure Round-Robin vs NSGA-II à partir de experiments.csv | Baseline vs optimized, interprétation |
| `scripts/demo.sh` | Démo live de 1 à 2 min sur un petit scénario | Demonstration and visual evidence |
| `data/tasks_demo.csv` | Petit scénario pour la démo live (6 tâches, environ 15 s chacune) | |
| `propositions/tasks_final_proposition.csv` | Proposition de scénario final : 12 tâches, 6 couples distincts | |
| `propositions/profiles_ESTIMATION.csv` | ESTIMATIONS (pas des mesures) pour tester les outils avant les vraies mesures | ne pas mettre dans le rapport |

Compilation des nouveaux outils :

    go build -o bin/pareto_check tools/pareto_check/main.go
    go build -o bin/compare tools/compare/main.go

## 1. Problème trouvé : memory_task ne peut pas durer 30 s

Avec `--size` seul, memory_task fait 5 passages sur sa mémoire : 512 Mo = 0,77 s sur vm1.
Pour atteindre 30 s il faudrait environ 20 Go, impossible (vm1 a 2 Go).
Solution : utiliser l'option `--passes` qui existe déjà dans memory_task.go.

- `scripts/config.sh` : nouvelle variable `MEM_PASSES` (défaut 5 = comportement actuel inchangé).
- `scripts/run_task.sh` : ajoute `--passes $MEM_PASSES` quand le workload est memory_task.
- `profile_cloud.sh` utilise la même valeur : mesures et expériences restent cohérentes.

Pour le scénario final : `MEM_PASSES=400` dans config.sh (768 Mo environ 47 s, 1024 Mo environ 62 s sur vm1, estimé).

## 2. Mesurer les profils : profile_cloud.sh

    ./scripts/profile_cloud.sh data/tasks.csv 3 data/profiles.csv

- Les 3 VMs mesurent en même temps, et sur une VM les mesures passent une par une
  (sinon elles se partagent le CPU et les temps sont faux).
- Une seule connexion SSH par VM, pas une par mesure.
- Chaque couple (workload, taille) n'est mesuré qu'une fois, même s'il apparaît dans 3 tâches.
  Donc : durée = nb de couples DISTINCTS x répétitions x temps moyen. Le nombre de tâches ne compte pas.
- Installe `time` sur la VM si besoin (Debian 12 ne l'a pas de base, benchmark.sh en a besoin aussi).
- Mesures brutes gardées dans `data/profiles_raw/<vm>.csv`.

Testé avec un faux gcloud qui exécute tout en local (pas sur GCP).

## 3. Vérifier NSGA-II : pareto_check

Avec 3 VMs et 12 tâches il y a 3^12 = 531 441 allocations. On peut toutes les calculer
avec le modèle commun (0,2 s), donc on connaît le vrai front de Pareto.

    ./bin/pareto_check -tasks data/tasks.csv -machines data/machines.csv -profiles data/profiles.csv \
        -rr data/allocation_rr.csv -nsga data/pareto/pareto.csv -out data/pareto_exact

Il affiche :
- combien d'allocations respectent la mémoire ;
- le front exact (`pareto_exact.csv` + une allocation par point, exécutable directement) ;
- où se place Round-Robin et ce que l'optimum ferait au même coût ;
- pour NSGA-II : combien de solutions sont sur le front exact, le plus grand écart, et le ratio
  d'hypervolume (100 % = NSGA-II a trouvé tout le front optimal) ;
- la figure `pareto_check.svg`.

Pourquoi garder NSGA-II si on a l'exact ? À 20 tâches il y a 3^20 = 3,5 milliards d'allocations :
l'énumération ne passe plus, NSGA-II si. Le petit scénario sert à prouver que NSGA-II est fiable.
C'est une bonne réponse à la question "pourquoi NSGA-II ?".

## 4. Comparer les stratégies : compare

    ./bin/compare data/experiments.csv data/comparaison.svg rr nsga_rapide nsga_eco

Tableau Markdown (moyenne +/- écart-type, prédit vs réel, gain vs la 1re stratégie) et figure
avec barres d'erreur. Les expériences avec échec sont comptées mais exclues des moyennes.

## 5. Démo live : demo.sh

    ./scripts/demo.sh

Montre les VMs, lance le scheduler, affiche l'allocation, exécute en parallèle avec un compteur,
vérifie que chaque tâche a tourné sur la VM prévue, puis affiche la ligne de experiments.csv.
Si `data/allocation_demo_nsga.csv` existe, il est exécuté juste après pour comparer.

## Budget temps estimé (scénario proposé, à refaire avec les vraies mesures)

| Étape | Durée estimée |
|---|---|
| Créer les VMs + attente SSH + déploiement | environ 5 min |
| profile_cloud.sh, 6 couples x 3 répétitions | environ 20 min (environ 33 min avec 5 répétitions) |
| NSGA-II + pareto_check | quelques secondes |
| 3 stratégies x 3 répétitions (environ 2 à 3 min chacune) | environ 25 min |
| Supprimer les VMs | environ 2 min |
| Total | environ 55 min à 1 h 05, prévoir 1 h 30 avec les relances |

## 6. File d'attente par VM : run_all_queue.sh

    QUEUE=1 ./scripts/run_experiment.sh data/allocation_rr.csv rr 3

- Au plus <vcpu> tâches en même temps par VM (vcpu de machines.csv), les autres attendent.
- Une seule connexion SSH par VM pour toute sa file : plus de ~3 s perdues par tâche.
- Les tâches partent dans l'ordre du fichier d'allocation.
- Mêmes fichiers de sortie que run_all.sh : collect_results.sh et gantt marchent sans changement.
- Horaires pris sur la VM, SSH non compris.
- Sans QUEUE=1, rien ne change (run_all.sh).
