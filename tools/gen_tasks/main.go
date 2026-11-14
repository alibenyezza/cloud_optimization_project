// gen_tasks : génère un scénario (tasks.csv) de N tâches tirées au hasard dans un menu de
// couples (workload, taille). Même graine = même scénario : l'expérience est reproductible.
//
// Le menu (data/task_menu.csv) ne doit contenir que des couples MESURÉS dans profiles.csv,
// sinon le modèle ne peut pas prédire leur durée.
//
// Compilation : go build -o bin/gen_tasks tools/gen_tasks/main.go
// Usage       : ./bin/gen_tasks -n 40 -seed 1 -menu data/task_menu.csv -out data/tasks_40.csv
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
)

func fail(msg string, a ...any) {
	fmt.Fprintf(os.Stderr, "Erreur : "+msg+"\n", a...)
	os.Exit(1)
}

func main() {
	n := flag.Int("n", 40, "nombre de tâches")
	seed := flag.Int64("seed", 1, "graine aléatoire (à noter dans le rapport)")
	menuF := flag.String("menu", "data/task_menu.csv", "menu : workload,size[,weight]")
	outF := flag.String("out", "data/tasks_gen.csv", "fichier tasks.csv produit")
	flag.Parse()
	if *n < 1 {
		fail("-n doit être >= 1")
	}

	f, err := os.Open(*menuF)
	if err != nil {
		fail("%v", err)
	}
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.Comment = '#'
	rows, err := r.ReadAll()
	f.Close()
	if err != nil || len(rows) < 2 {
		fail("%s vide ou illisible", *menuF)
	}
	if strings.TrimSpace(rows[0][0]) != "workload" || strings.TrimSpace(rows[0][1]) != "size" {
		fail("%s : en-tête attendu workload,size[,weight]", *menuF)
	}

	type item struct {
		w, s   string
		weight float64
	}
	var menu []item
	total := 0.0
	for i, row := range rows[1:] {
		if len(row) < 2 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		it := item{strings.TrimSpace(row[0]), strings.TrimSpace(row[1]), 1}
		if _, err := strconv.Atoi(it.s); err != nil {
			fail("%s ligne %d : taille invalide %q", *menuF, i+2, it.s)
		}
		if len(row) >= 3 && strings.TrimSpace(row[2]) != "" {
			v, err := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
			if err != nil || v < 0 {
				fail("%s ligne %d : poids invalide %q", *menuF, i+2, row[2])
			}
			it.weight = v
		}
		menu = append(menu, it)
		total += it.weight
	}
	if len(menu) == 0 || total == 0 {
		fail("menu vide")
	}

	rng := rand.New(rand.NewSource(*seed))
	out, err := os.Create(*outF)
	if err != nil {
		fail("%v", err)
	}
	fmt.Fprintln(out, "task_id,workload,size")
	count := map[string]int{}
	width := len(strconv.Itoa(*n))
	for i := 1; i <= *n; i++ {
		x := rng.Float64() * total
		k := 0
		for ; k < len(menu)-1 && x >= menu[k].weight; k++ {
			x -= menu[k].weight
		}
		fmt.Fprintf(out, "T%0*d,%s,%s\n", width, i, menu[k].w, menu[k].s)
		count[menu[k].w+" "+menu[k].s]++
	}
	out.Close()

	fmt.Printf("%d tâches écrites dans %s (graine %d)\n", *n, *outF, *seed)
	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-24s x %d\n", k, count[k])
	}
}
