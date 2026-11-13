# Note pour l'équipe – où on en est

Salut, petit point sur ce que j'ai fait (partie A, infra + orchestrateur) et ce qu'il reste.

## Récupérer mon travail

Tout est sur la branche `prod-maxime`, pas encore fusionné. Pour l'avoir sur votre branche :

    git fetch
    git merge origin/prod-maxime

Avant de coder quoi que ce soit, lisez `docs/INTERFACES.md`. C'est là qu'on a fixé les formats
des CSV. Si vous voulez changer un format, prévenez les autres avant, sinon on casse le travail
de quelqu'un.

## Ce qui marche déjà

- 3 VMs différentes en europe-west9-a : vm1 (e2-highcpu-2, peu de RAM, pas chère),
  vm2 (e2-standard-2), vm3 (n2-standard-2, CPU plus rapide, plus chère).
- On lance une allocation, ça tourne en parallèle sur les VMs, et on récupère le makespan,
  le coût et une timeline (gantt.svg) pour chaque expérience.
- Testé avec Round-Robin, tout fonctionne.

## Utiliser les VMs

Je les ai supprimées pour ne pas cramer les crédits. Pour les recréer (5 min) :

    ./scripts/create_vms.sh
    sleep 60
    ./scripts/deploy_all.sh

Et quand vous avez fini, toujours :

    ./scripts/delete_vms.sh

Le projet GCP est sur mon compte : envoyez-moi votre mail Google pour que je vous donne l'accès.

## Lancer une expérience

    ./bin/round_robin_scheduler data/tasks.csv data/machines.csv data/allocation_rr.csv
    ./scripts/run_experiment.sh data/allocation_rr.csv rr 3

Ça lance 3 répétitions. Les résultats vont dans `results/<date>_rr_r1/` etc.,
et une ligne par expérience s'ajoute dans `data/experiments.csv`.

## Pour B (benchmarks)

- Il faut remplir `data/profiles.csv` (format dans INTERFACES.md).
- Chaque couple (workload, taille) du scénario doit être mesuré sur les 3 types de machines.
- Prenez de grosses tailles : 30 s à 2 min par tâche. La connexion SSH ajoute à elle seule
  environ 3 s par tâche, donc avec les petites tailles des TD on ne mesure que ça.
- Utilisez le `runtime_s` du JSON (temps de calcul pur) et au moins 5 répétitions.

## Pour C (NSGA-II)

- Entrées : `tasks.csv`, `machines.csv`, `profiles.csv`.
- Sortie : `data/pareto/pareto.csv`, une allocation par solution (`sol_001.csv`...),
  et `data/allocation_nsga.csv` pour celle qu'on exécute.
- Une allocation = une VM par tâche, donc le chromosome c'est juste `allocation[i] = indice de la VM`.
- Le makespan et le coût se calculent avec la formule de INTERFACES.md, la même pour tout le monde.
- Fixez la graine aléatoire (seed) et notez-la, sinon on ne pourra pas reproduire les résultats.
- En attendant les vraies mesures de B, faites un faux `profiles.csv` pour tester.

## Pièges qu'on s'est déjà pris

- Fins de ligne Windows dans les scripts → erreur "bad interpreter". Travaillez dans Ubuntu,
  pas dans un dossier C:\, ou faites `sed -i 's/\r$//' fichier`.
- Code 255 juste après avoir créé les VMs : SSH n'est pas encore prêt, attendez une minute.
- Ne commitez pas `bin/`, on recompile (commandes dans le README).
- La zone est définie uniquement dans `scripts/config.sh`, ne la remettez pas en dur ailleurs.

## Git

Chacun bosse sur sa branche `prod-<prénom>`. Quand un truc est fini et testé, pull request,
et un autre la relit avant de fusionner. On doit encore décider si on fusionne dans `main`
ou dans `prod`.

Pour l'oral, chacun doit pouvoir expliquer tout le projet, pas seulement sa partie.
Donc n'hésitez pas à poser des questions sur l'infra.

Maxime
