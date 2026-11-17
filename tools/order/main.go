// order : réordonne les lignes d'une allocation par "chemin restant" (model.PriorityOrder).
// Les VMs ne changent pas, seulement l'ordre de priorité (l'ordre des lignes compte pour
// l'orchestrateur, voir docs/INTERFACES.md). Marche avec n'importe quelle allocation : RR, NSGA-II...
// Affiche la prédiction avant et après, avec le modèle commun.
// Compilation : go build -o bin/order tools/order/main.go
// Usage       : ./bin/order TASKS.csv MACHINES.csv PROFILES.csv ALLOCATION.csv SORTIE.csv
package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"cloudopt/model"
)

func fail(msg string, a ...any) {
	fmt.Fprintf(os.Stderr, "Erreur : "+msg+"\n", a...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 6 {
		fmt.Fprintln(os.Stderr, "Usage: order TASKS.csv MACHINES.csv PROFILES.csv ALLOCATION.csv SORTIE.csv")
		os.Exit(2)
	}
	p, err := model.Load(os.Args[1], os.Args[2], os.Args[3], "data/vm_setup.csv")
	if err != nil {
		fail("%v", err)
	}
	taskIndex := map[string]int{}
	for i, t := range p.Tasks {
		taskIndex[t.ID] = i
	}
	vmIndex := map[string]int{}
	for v, vm := range p.VMs {
		vmIndex[vm.Name] = v
	}

	// Lecture de l'allocation, mêmes contrôles que predict
	f, err := os.Open(os.Args[4])
	if err != nil {
		fail("%v", err)
	}
	rows, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil || len(rows) < 2 {
		fail("%s vide ou illisible", os.Args[4])
	}
	alloc := make([]int, len(p.Tasks))
	line := make([][]string, len(p.Tasks)) // ligne d'origine de chaque tâche
	var before []int
	for _, r := range rows[1:] {
		for k := range r {
			r[k] = strings.TrimSpace(r[k])
		}
		i, ok := taskIndex[r[0]]
		if !ok {
			fail("tâche %s absente de %s", r[0], os.Args[1])
		}
		if line[i] != nil {
			fail("tâche %s en double dans l'allocation", r[0])
		}
		v, ok := vmIndex[r[3]]
		if !ok {
			fail("VM %s absente de %s", r[3], os.Args[2])
		}
		alloc[i], line[i] = v, r
		before = append(before, i)
	}
	if len(before) != len(p.Tasks) {
		fail("l'allocation contient %d tâches sur %d", len(before), len(p.Tasks))
	}

	after := p.PriorityOrder()
	out := [][]string{rows[0]}
	for _, i := range after {
		out = append(out, line[i])
	}
	g, err := os.Create(os.Args[5])
	if err != nil {
		fail("%v", err)
	}
	w := csv.NewWriter(g)
	w.WriteAll(out)
	g.Close()
	if err := w.Error(); err != nil {
		fail("%v", err)
	}

	r1, r2 := p.Simulate(alloc, before), p.Simulate(alloc, after)
	fmt.Printf("ordre du fichier   : makespan %.2f s, coût %.6f USD\n", r1.Makespan, r1.Cost)
	fmt.Printf("ordre chemin restant : makespan %.2f s, coût %.6f USD (%+.1f %%)\n",
		r2.Makespan, r2.Cost, 100*(r2.Makespan-r1.Makespan)/r1.Makespan)
	fmt.Printf("prediction,%.2f,%.6f,%.0f\n", r2.Makespan, r2.Cost, r2.Violation)
}
