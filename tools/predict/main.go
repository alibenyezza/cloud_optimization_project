// predict : makespan, coût et mémoire prévus d'une allocation, avec le modèle commun
// (package model : simulation des règles de l'orchestrateur, voir docs/INTERFACES.md).
// Données : data/profiles.csv (temps mesurés) et data/vm_setup.csv (préparation des VMs).
// Compilation : go build -o bin/predict tools/predict/main.go
// Usage       : ./bin/predict data/tasks.csv data/machines.csv data/profiles.csv data/allocation_rr.csv
// Dernière ligne : prediction,<makespan_s>,<cost_usd>,<mem_violation_mb>
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
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "Usage: predict TASKS.csv MACHINES.csv PROFILES.csv ALLOCATION.csv")
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

	// Allocation : task_id,workload,size,vm ; l'ordre des lignes est l'ordre de priorité
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
	seen := make([]bool, len(p.Tasks))
	var order []int
	for _, r := range rows[1:] {
		id, vm := strings.TrimSpace(r[0]), strings.TrimSpace(r[3])
		i, ok := taskIndex[id]
		if !ok {
			fail("tâche %s absente de %s", id, os.Args[1])
		}
		if seen[i] {
			fail("tâche %s en double dans l'allocation", id)
		}
		v, ok := vmIndex[vm]
		if !ok {
			fail("VM %s absente de %s", vm, os.Args[2])
		}
		seen[i], alloc[i] = true, v
		order = append(order, i)
	}
	if len(order) != len(p.Tasks) {
		fail("l'allocation contient %d tâches sur %d", len(order), len(p.Tasks))
	}

	res := p.Simulate(alloc, order)
	fmt.Println("vm,machine_type,n_tasks,end_s,peak_mem_mb,mem_limit_mb")
	for v, vm := range p.VMs {
		fmt.Printf("%s,%s,%d,%.2f,%.0f,%.0f\n", vm.Name, vm.Type, res.Count[v], res.End[v], res.PeakMem[v], vm.MemLimit())
	}
	fmt.Printf("prediction,%.2f,%.6f,%.0f\n", res.Makespan, res.Cost, res.Violation)
}
