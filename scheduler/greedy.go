// greedy : 2e baseline proposée par Maxime, un glouton classique (HEFT, Heterogeneous Earliest Finish Time).
//  1. On trie les tâches par "chemin restant" (model.PriorityOrder) : celles qui bloquent
//     le plus de tâches passent d'abord.
//  2. Dans cet ordre, chaque tâche va sur la VM où elle FINIT LE PLUS TOT, d'après le modèle
//     commun (model.Simulate) appliqué aux tâches déjà placées + elle.
//  3. Filtrer avant de choisir (CM3, CM6) : une VM où la mémoire déborderait est écartée.
//
// Glouton : chaque choix est définitif, aucun retour en arrière (CM3). Rapide, mais il ne
// regarde que le temps : il ignore le coût (il utilise volontiers toutes les VMs).
// L'allocation est écrite dans l'ordre de priorité (l'ordre des lignes compte, INTERFACES.md).
//
// Compilation : go build -o bin/greedy scheduler/greedy.go
// Usage       : ./bin/greedy TASKS.csv MACHINES.csv PROFILES.csv SORTIE.csv
package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"

	"cloudopt/model"
)

func fail(msg string, a ...any) {
	fmt.Fprintf(os.Stderr, "Erreur : "+msg+"\n", a...)
	os.Exit(1)
}

// subProblem : les tâches déjà placées (dans l'ordre de priorité), indices renumérotés.
// Les dépendances d'une tâche sont toujours placées avant elle (PriorityOrder).
func subProblem(p *model.Problem, placed []int) *model.Problem {
	pos := map[int]int{}
	for k, i := range placed {
		pos[i] = k
	}
	s := &model.Problem{VMs: p.VMs, HasDeps: p.HasDeps}
	for _, i := range placed {
		t := p.Tasks[i]
		var deps []int
		for _, j := range t.Deps {
			deps = append(deps, pos[j])
		}
		s.Tasks = append(s.Tasks, model.Task{ID: t.ID, Workload: t.Workload, Size: t.Size, Deps: deps})
		s.RT = append(s.RT, p.RT[i])
		s.RSS = append(s.RSS, p.RSS[i])
	}
	return s
}

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "Usage: greedy TASKS.csv MACHINES.csv PROFILES.csv SORTIE.csv")
		os.Exit(2)
	}
	p, err := model.Load(os.Args[1], os.Args[2], os.Args[3], "data/vm_setup.csv")
	if err != nil {
		fail("%v", err)
	}

	order := p.PriorityOrder()
	var placed []int   // tâches déjà placées, dans l'ordre de priorité
	var subAlloc []int // leur VM
	alloc := make([]int, len(p.Tasks))
	for _, i := range order {
		placed = append(placed, i)
		s := subProblem(p, placed)
		bestV, bestFinish, bestViol := -1, math.Inf(1), math.Inf(1)
		for v := range p.VMs {
			r := s.Simulate(append(subAlloc, v), model.Identity(len(placed)))
			finish := r.Finish[len(placed)-1]
			// d'abord la mémoire (0 = valide), puis la fin la plus tôt
			if r.Violation < bestViol || (r.Violation == bestViol && finish < bestFinish) {
				bestV, bestFinish, bestViol = v, finish, r.Violation
			}
		}
		subAlloc = append(subAlloc, bestV)
		alloc[i] = bestV
	}

	rows := [][]string{{"task_id", "workload", "size", "vm"}}
	for _, i := range order {
		t := p.Tasks[i]
		rows = append(rows, []string{t.ID, t.Workload, t.Size, p.VMs[alloc[i]].Name})
	}
	f, err := os.Create(os.Args[4])
	if err != nil {
		fail("%v", err)
	}
	w := csv.NewWriter(f)
	w.WriteAll(rows)
	f.Close()
	if err := w.Error(); err != nil {
		fail("%v", err)
	}

	r := p.Simulate(alloc, order)
	fmt.Printf("HEFT : makespan %.2f s, coût %.6f USD, %d VM(s), dépassement mémoire %.0f Mo\n",
		r.Makespan, r.Cost, r.NVMs, r.Violation)
	fmt.Printf("prediction,%.2f,%.6f,%.0f\n", r.Makespan, r.Cost, r.Violation)
}
