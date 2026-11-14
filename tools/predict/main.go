// predict : calcule le makespan, le coût et la mémoire prévus d'une allocation,
// avec le modèle commun de docs/INTERFACES.md et les mesures de data/profiles.csv.
// Compilation : go build -o bin/predict tools/predict/main.go
// Usage       : ./bin/predict data/tasks.csv data/machines.csv data/profiles.csv data/allocation_rr.csv
// Dernière ligne : prediction,<makespan_s>,<cost_usd>,<mem_violation_mb>
package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type machine struct {
	name, mtype string
	ramMB       float64
	price       float64
}

type profile struct {
	runtime float64 // médiane du runtime_s, en secondes
	rssMB   float64
}

func fail(msg string, a ...any) {
	fmt.Fprintf(os.Stderr, "Erreur : "+msg+"\n", a...)
	os.Exit(1)
}

// readCSV lit un fichier et renvoie ses lignes, en-tête comprise, sans espaces ni \r.
func readCSV(path string) [][]string {
	f, err := os.Open(path)
	if err != nil {
		fail("%v", err)
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil || len(recs) < 2 {
		fail("%s vide ou illisible", path)
	}
	for _, r := range recs {
		for i := range r {
			r[i] = strings.TrimSpace(r[i])
		}
	}
	return recs
}

func num(s, what string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		fail("%s invalide : %q", what, s)
	}
	return v
}

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "Usage: predict TASKS.csv MACHINES.csv PROFILES.csv ALLOCATION.csv")
		os.Exit(2)
	}

	// Machines : vm,machine_type,vcpu,ram_mb,price_per_hour_usd
	machines := map[string]machine{}
	var order []string
	for _, r := range readCSV(os.Args[2])[1:] {
		machines[r[0]] = machine{name: r[0], mtype: r[1],
			ramMB: num(r[3], "ram_mb"), price: num(r[4], "price_per_hour_usd")}
		order = append(order, r[0])
	}

	// Profils : workload,size,machine_type,runs,mean,median,std,max_rss_kb
	profiles := map[string]profile{}
	for _, r := range readCSV(os.Args[3])[1:] {
		key := r[0] + "," + r[1] + "," + r[2]
		profiles[key] = profile{runtime: num(r[5], "median_runtime_s"), rssMB: num(r[7], "max_rss_kb") / 1024}
	}

	// Tâches attendues : chacune doit apparaître une fois dans l'allocation
	expected := map[string]bool{}
	for _, r := range readCSV(os.Args[1])[1:] {
		expected[r[0]] = true
	}

	// Allocation : task_id,workload,size,vm -> somme des temps et mémoire par VM
	sum := map[string]float64{}
	mem := map[string]float64{}
	count := map[string]int{}
	seen := map[string]bool{}
	for _, r := range readCSV(os.Args[4])[1:] {
		id, w, size, vm := r[0], r[1], r[2], r[3]
		if !expected[id] {
			fail("tâche %s absente de %s", id, os.Args[1])
		}
		if seen[id] {
			fail("tâche %s en double dans l'allocation", id)
		}
		seen[id] = true
		m, ok := machines[vm]
		if !ok {
			fail("VM %s absente de %s", vm, os.Args[2])
		}
		p, ok := profiles[w+","+size+","+m.mtype]
		if !ok {
			fail("pas de mesure pour %s %s sur %s dans %s", w, size, m.mtype, os.Args[3])
		}
		sum[vm] += p.runtime
		mem[vm] += p.rssMB
		count[vm]++
	}
	if len(seen) != len(expected) {
		fail("l'allocation contient %d tâches sur %d", len(seen), len(expected))
	}

	// Modèle commun : fin(VM) = somme des temps de ses tâches (1 coeur physique par VM)
	fmt.Println("vm,machine_type,n_tasks,end_s,mem_mb,mem_limit_mb")
	makespan, cost, violation := 0.0, 0.0, 0.0
	for _, vm := range order {
		m := machines[vm]
		end := sum[vm]
		limit := 0.9 * m.ramMB
		fmt.Printf("%s,%s,%d,%.2f,%.0f,%.0f\n", vm, m.mtype, count[vm], end, mem[vm], limit)
		if end > makespan {
			makespan = end
		}
		if mem[vm] > limit {
			violation += mem[vm] - limit
		}
	}
	// Coût : une VM utilisée reste allumée jusqu'à la fin de l'expérience, une VM sans tâche n'est pas louée
	for _, vm := range order {
		if count[vm] > 0 {
			cost += machines[vm].price / 3600 * makespan
		}
	}
	fmt.Printf("prediction,%.2f,%.6f,%.0f\n", makespan, cost, violation)
}
