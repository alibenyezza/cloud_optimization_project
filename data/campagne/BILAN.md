# Bilan de la campagne finale (16/11/2026)

Lancée avec `./scripts/campaign.sh 3`. Tout est dans `data/experiments.csv` (lignes `s12_*` et `s40dag_*`),
les allocations et prédictions dans `data/campagne/`, les timelines dans `results/20261116-*/gantt.svg`.
Profils utilisés : `profiles.csv` du 13/11. Zone europe-west9-a. 0 tâche en échec sur 12 expériences.

## Résultats (3 répétitions)

| Scénario | Stratégie | VMs | Makespan mesuré | Moyenne | Coût moyen | Prévu |
|---|---|---|---|---|---|---|
| 12 tâches, file par VM | Round-Robin | 3 | 51,5 / 51,5 / 51,7 s | 51,6 s | 0,00674 USD | 83,0 s |
| 12 tâches, file par VM | NSGA-II | 2 | 47,6 / 47,6 / 47,6 s | 47,6 s | 0,00441 USD | 55,6 s |
| 40 tâches, dépendances | Round-Robin | 3 | 164,3 / 163,2 / 163,9 s | 163,8 s | 0,01446 USD | 301,7 s |
| 40 tâches, dépendances | NSGA-II | 3 | 170,7 / 201,5 / 165,1 s | 179,1 s | 0,01552 USD | 145,9 s |

- 12 tâches : NSGA-II est 7,7 % plus rapide et 34,5 % moins cher (2 VMs au lieu de 3).
- 40 tâches : NSGA-II ne gagne pas. Sans le run à 201,5 s, il fait 167,9 s, soit 2,5 % de plus que Round-Robin.
- Stabilité : écart-type 0,1 s sur 12 tâches, 0,6 s pour Round-Robin sur 40 tâches.
- Le run NSGA-II à 201,5 s : les 3 VMs ont fini plus tard en même temps (170,7 / 199,0 / 201,5 s),
  ce qui ressemble à un ralentissement passager du cloud, pas à l'allocation.

## Résultat inattendu : vm2 a changé de processeur

vm2 (e2-standard-2) n'avait pas le même processeur que le 13/11. Preuve dans les `runtime_s`
(`results/20261116-1347_s12_rr_r1/results.csv`) :

| Tâche | VM | 13/11, seule (profiles.csv) | 16/11, en partageant la VM |
|---|---|---|---|
| cpu_task 40 000 000 | vm2 e2-standard-2 | 67,2 s | 34,1 s |
| cpu_task 20 000 000 | vm1 e2-highcpu-2 | 25,4 s | 35,8 s (ralentie par le partage, normal) |
| cpu_task 20 000 000 | vm3 n2-standard-2 | 7,2 s | 7,2 s |

Conséquences :
- Le modèle croyait vm2 lente en calcul, donc il surestime Round-Robin (83 s prévus, 51,6 s mesurés ;
  302 s prévus, 163,8 s mesurés), qui met beaucoup de calcul sur vm2.
- NSGA-II évitait vm2 pour le calcul. Ce choix ne sert plus quand vm2 est rapide : sur 40 tâches,
  son avantage prévu (146 s contre 302 s) disparaît.
- Sur 12 tâches, NSGA-II reste meilleur, surtout en coût, parce qu'il n'utilise que 2 VMs.

A retenir : une VM e2 ne garantit pas son processeur, et une optimisation n'est jamais meilleure que
les mesures sur lesquelles elle repose. Pour une comparaison juste, il faudrait mesurer, optimiser et
exécuter dans une même session de VMs. On ne l'a pas refait : la limite est expliquée (partie 10).

## Pour le rapport

- Partie 5.4 (Maxime) : campagne et tableau ci-dessus, déjà dans le Word.
- Partie 10 (Maxime) : limite sur le processeur des e2, déjà dans le Word.
- Parties 7, 8, 9 : ce fichier contient tous les chiffres. Les Gantt à montrer :
  `results/20261116-1347_s12_rr_r1/gantt.svg` et `results/20261116-1350_s12_nsga_r1/gantt.svg`
  (12 tâches), `results/20261116-1353_s40dag_rr_r1/gantt.svg` et `results/20261116-1401_s40dag_nsga_r3/gantt.svg`
  (40 tâches, flèches de dépendance et chemin critique).
