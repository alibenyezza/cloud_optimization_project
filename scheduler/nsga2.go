// nsga2 : NSGA-II pour placer les tâches sur les VMs, avec 2 objectifs à minimiser :
// le makespan et le coût. La mémoire de chaque VM est une contrainte.
// Evaluation = modèle commun de docs/INTERFACES.md (même calcul que tools/predict),
// avec le temps de préparation des VMs de data/vm_setup.csv.
// Compilation : go build -o bin/nsga2 scheduler/nsga2.go
// Usage       : ./bin/nsga2 data/tasks.csv data/machines.csv data/profiles.csv [SEED]
// Sorties     : data/pareto/ (pareto.csv, sol_XXX.csv, pareto.svg, run.txt) et data/allocation_nsga.csv
package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Paramètres de l'algo
const (
	popSize   = 100 // nombre de solutions dans la population
	nGen      = 300 // nombre de générations
	crossProb = 0.9 // probabilité de croisement
	closeProb = 0.1 // probabilité de la mutation "fermer une VM"
	outDir    = "data/pareto"
	chosenOut = "data/allocation_nsga.csv"
	setupPath = "data/vm_setup.csv"
)

type task struct{ id, workload, size string }

type vm struct {
	name, mtype  string
	ramMB, price float64
	setup        float64 // temps de préparation (création + SSH + déploiement), payé aussi
}

// Données du problème, lues une fois au début
var (
	tasks []task
	vms   []vm
	rt    [][]float64 // rt[i][v]  : temps médian de la tâche i sur la VM v (s)
	rss   [][]float64 // rss[i][v] : mémoire max de la tâche i sur la VM v (Mo)
)

type solution struct {
	alloc     []int   // chromosome : alloc[i] = indice de la VM de la tâche i
	makespan  float64 // objectif 1
	cost      float64 // objectif 2
	violation float64 // Mo au-dessus de 90 % de la RAM (0 = solution valide)
	nVMs      int     // nombre de VMs utilisées
	rank      int     // numéro du front (0 = le meilleur)
	crowd     float64 // distance de crowding (grand = solution isolée)
}

func fail(msg string, a ...any) {
	fmt.Fprintf(os.Stderr, "Erreur : "+msg+"\n", a...)
	os.Exit(1)
}

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
	return recs[1:]
}

func num(s, what string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		fail("%s invalide : %q", what, s)
	}
	return v
}

func load(tasksPath, machinesPath, profilesPath string) {
	// Préparation moyenne par machine_type (data/vm_setup.csv) ; type jamais mesuré : moyenne de tout
	setupSum, setupN := map[string]float64{}, map[string]float64{}
	all, count := 0.0, 0.0
	for _, r := range readCSV(setupPath) {
		t := num(r[6], "total_s")
		setupSum[r[2]] += t
		setupN[r[2]]++
		all += t
		count++
	}
	for _, r := range readCSV(machinesPath) {
		m := vm{name: r[0], mtype: r[1], ramMB: num(r[3], "ram_mb"), price: num(r[4], "price_per_hour_usd"), setup: all / count}
		if setupN[m.mtype] > 0 {
			m.setup = setupSum[m.mtype] / setupN[m.mtype]
		}
		vms = append(vms, m)
	}
	type prof struct{ runtime, rssMB float64 }
	profiles := map[string]prof{}
	for _, r := range readCSV(profilesPath) {
		profiles[r[0]+","+r[1]+","+r[2]] = prof{num(r[5], "median_runtime_s"), num(r[7], "max_rss_kb") / 1024}
	}
	for _, r := range readCSV(tasksPath) {
		t := task{r[0], r[1], r[2]}
		tasks = append(tasks, t)
		rowT := make([]float64, len(vms))
		rowM := make([]float64, len(vms))
		for v, m := range vms {
			p, ok := profiles[t.workload+","+t.size+","+m.mtype]
			if !ok {
				fail("pas de mesure pour %s %s sur %s", t.workload, t.size, m.mtype)
			}
			rowT[v], rowM[v] = p.runtime, p.rssMB
		}
		rt = append(rt, rowT)
		rss = append(rss, rowM)
	}
	if len(tasks) < 2 || len(vms) == 0 {
		fail("il faut au moins 2 tâches et 1 VM")
	}
}

// evaluate applique le modèle commun à une allocation
func evaluate(alloc []int) solution {
	end := make([]float64, len(vms))
	mem := make([]float64, len(vms))
	used := make([]bool, len(vms))
	for i, v := range alloc {
		end[v] += rt[i][v]
		mem[v] += rss[i][v]
		used[v] = true
	}
	s := solution{alloc: alloc}
	for v := range vms {
		s.makespan = math.Max(s.makespan, end[v])
		if limit := 0.9 * vms[v].ramMB; mem[v] > limit {
			s.violation += mem[v] - limit
		}
	}
	// Une VM utilisée est payée de sa création jusqu'à la fin, une VM vide n'est pas créée
	for v := range vms {
		if used[v] {
			s.cost += vms[v].price / 3600 * (vms[v].setup + s.makespan)
			s.nVMs++
		}
	}
	return s
}

// dominates : a est meilleure que b.
// Contrainte mémoire (règle de Deb) : une solution valide bat une solution invalide,
// et entre deux invalides on garde celle qui dépasse le moins.
func dominates(a, b solution) bool {
	if a.violation != b.violation {
		return a.violation < b.violation
	}
	return a.makespan <= b.makespan && a.cost <= b.cost &&
		(a.makespan < b.makespan || a.cost < b.cost)
}

// sortFronts range la population en fronts : front 0 = personne ne la domine, etc.
func sortFronts(pop []solution) [][]int {
	n := len(pop)
	beats := make([][]int, n) // beats[p] : solutions dominées par p
	count := make([]int, n)   // count[p] : nombre de solutions qui dominent p
	var front []int
	for p := 0; p < n; p++ {
		for q := 0; q < n; q++ {
			if dominates(pop[p], pop[q]) {
				beats[p] = append(beats[p], q)
			} else if dominates(pop[q], pop[p]) {
				count[p]++
			}
		}
		if count[p] == 0 {
			pop[p].rank = 0
			front = append(front, p)
		}
	}
	var fronts [][]int
	for len(front) > 0 {
		fronts = append(fronts, front)
		var next []int
		for _, p := range front {
			for _, q := range beats[p] {
				count[q]--
				if count[q] == 0 {
					pop[q].rank = len(fronts)
					next = append(next, q)
				}
			}
		}
		front = next
	}
	return fronts
}

// crowding : pour chaque objectif, distance entre les deux voisins de la solution dans le front.
// Les deux bouts du front ont une distance infinie (on les garde toujours).
func crowding(pop []solution, front []int) {
	objectives := []func(s solution) float64{
		func(s solution) float64 { return s.makespan },
		func(s solution) float64 { return s.cost },
	}
	for _, i := range front {
		pop[i].crowd = 0
	}
	last := len(front) - 1
	for _, obj := range objectives {
		sort.Slice(front, func(a, b int) bool { return obj(pop[front[a]]) < obj(pop[front[b]]) })
		pop[front[0]].crowd = math.Inf(1)
		pop[front[last]].crowd = math.Inf(1)
		lo, hi := obj(pop[front[0]]), obj(pop[front[last]])
		if hi == lo {
			continue
		}
		for k := 1; k < last; k++ {
			pop[front[k]].crowd += (obj(pop[front[k+1]]) - obj(pop[front[k-1]])) / (hi - lo)
		}
	}
}

func rankAndCrowd(pop []solution) {
	for _, f := range sortFronts(pop) {
		crowding(pop, f)
	}
}

// better : meilleur front d'abord, puis la plus isolée
func better(a, b solution) bool {
	if a.rank != b.rank {
		return a.rank < b.rank
	}
	return a.crowd > b.crowd
}

// tournament : on tire 2 solutions au hasard et on garde la meilleure
func tournament(pop []solution, r *rand.Rand) solution {
	a, b := pop[r.Intn(len(pop))], pop[r.Intn(len(pop))]
	if better(a, b) {
		return a
	}
	return b
}

// makeChild : croisement à un point puis mutation
func makeChild(p1, p2 solution, r *rand.Rand) []int {
	n := len(tasks)
	child := make([]int, n)
	copy(child, p1.alloc)
	if r.Float64() < crossProb {
		cut := 1 + r.Intn(n-1) // tâches avant cut : parent 1, après : parent 2
		copy(child[cut:], p2.alloc[cut:])
	}
	for i := range child {
		if r.Float64() < 1.0/float64(n) { // en moyenne une tâche change de VM
			child[i] = r.Intn(len(vms))
		}
	}
	// Mutation "fermer une VM" : toutes les tâches d'une VM partent sur une autre.
	// Sans elle, l'algo explore mal les solutions avec moins de VMs (il faudrait bouger
	// beaucoup de tâches en même temps par hasard).
	if r.Float64() < closeProb {
		from, to := r.Intn(len(vms)), r.Intn(len(vms))
		for i := range child {
			if child[i] == from {
				child[i] = to
			}
		}
	}
	return child
}

func nsga2(seed int64) []solution {
	r := rand.New(rand.NewSource(seed))
	pop := make([]solution, popSize)
	for k := range pop {
		alloc := make([]int, len(tasks))
		for i := range alloc {
			alloc[i] = r.Intn(len(vms))
		}
		pop[k] = evaluate(alloc)
	}
	rankAndCrowd(pop)

	for g := 0; g < nGen; g++ {
		all := append([]solution{}, pop...)
		for k := 0; k < popSize; k++ {
			all = append(all, evaluate(makeChild(tournament(pop, r), tournament(pop, r), r)))
		}
		// Elitisme : parents + enfants, on garde les popSize meilleurs.
		// Les doublons (même makespan et même coût) passent en dernier, sinon la population
		// se remplit de copies de la meilleure solution et l'algo n'explore plus.
		rankAndCrowd(all)
		sort.SliceStable(all, func(a, b int) bool { return better(all[a], all[b]) })
		var unique, copies []solution
		seen := map[string]bool{}
		for _, s := range all {
			if seen[key(s)] {
				copies = append(copies, s)
			} else {
				seen[key(s)] = true
				unique = append(unique, s)
			}
		}
		pop = append(unique, copies...)[:popSize]
		rankAndCrowd(pop)
	}
	return pop
}

func key(s solution) string { return fmt.Sprintf("%.4f,%.8f", s.makespan, s.cost) }

// paretoFront : solutions valides non dominées, une seule par couple (makespan, coût), triées par makespan
func paretoFront(sols []solution) []solution {
	var ok []solution
	for _, s := range sols {
		if s.violation == 0 {
			ok = append(ok, s)
		}
	}
	sort.SliceStable(ok, func(a, b int) bool {
		if ok[a].makespan != ok[b].makespan {
			return ok[a].makespan < ok[b].makespan
		}
		return ok[a].cost < ok[b].cost
	})
	var front []solution
	seen := map[string]bool{}
	best := math.Inf(1)
	for _, s := range ok {
		if s.cost < best && !seen[key(s)] {
			front = append(front, s)
			seen[key(s)] = true
			best = s.cost
		}
	}
	return front
}

// exactFront : essaie toutes les allocations (si moins de 2 millions) pour avoir le vrai front
func exactFront() []solution {
	total := 1
	for range tasks {
		total *= len(vms)
		if total > 2000000 {
			return nil
		}
	}
	type point struct {
		makespan, cost float64
		k              int
	}
	decode := func(k int) []int {
		alloc := make([]int, len(tasks))
		for i := range alloc {
			alloc[i] = k % len(vms)
			k /= len(vms)
		}
		return alloc
	}
	var pts []point
	for k := 0; k < total; k++ {
		if s := evaluate(decode(k)); s.violation == 0 {
			pts = append(pts, point{s.makespan, s.cost, k})
		}
	}
	sort.Slice(pts, func(a, b int) bool {
		if pts[a].makespan != pts[b].makespan {
			return pts[a].makespan < pts[b].makespan
		}
		return pts[a].cost < pts[b].cost
	})
	var cand []solution
	best := math.Inf(1)
	for _, p := range pts {
		if p.cost < best {
			cand = append(cand, evaluate(decode(p.k)))
			best = p.cost
		}
	}
	return paretoFront(cand)
}

// choose : la solution du front la plus proche du point idéal (makespan et coût ramenés entre 0 et 1)
func choose(front []solution) int {
	first, last := front[0], front[len(front)-1]
	best, bestD := 0, math.Inf(1)
	for i, s := range front {
		x, y := 0.0, 0.0
		if last.makespan > first.makespan {
			x = (s.makespan - first.makespan) / (last.makespan - first.makespan)
		}
		if first.cost > last.cost {
			y = (s.cost - last.cost) / (first.cost - last.cost)
		}
		if d := math.Hypot(x, y); d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

func writeCSV(path string, rows [][]string) {
	f, err := os.Create(path)
	if err != nil {
		fail("%v", err)
	}
	w := csv.NewWriter(f)
	w.WriteAll(rows)
	if err := w.Error(); err != nil {
		fail("%v", err)
	}
	f.Close()
}

func allocationRows(s solution) [][]string {
	rows := [][]string{{"task_id", "workload", "size", "vm"}}
	for i, t := range tasks {
		rows = append(rows, []string{t.id, t.workload, t.size, vms[s.alloc[i]].name})
	}
	return rows
}

// writeSVG : graphe coût en fonction du makespan
func writeSVG(path string, pop, front, exact []solution, chosen, rr solution) {
	const W, H, M = 720, 460, 70
	all := append(append(append([]solution{}, pop...), front...), exact...)
	all = append(all, rr)
	x0, x1, y0, y1 := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, s := range all {
		if s.violation > 0 {
			continue
		}
		x0, x1 = math.Min(x0, s.makespan), math.Max(x1, s.makespan)
		y0, y1 = math.Min(y0, s.cost), math.Max(y1, s.cost)
	}
	dx, dy := (x1-x0)*0.05+1e-9, (y1-y0)*0.05+1e-12
	x0, x1, y0, y1 = x0-dx, x1+dx, y0-dy, y1+dy
	px := func(v float64) float64 { return M + (v-x0)/(x1-x0)*(W-2*M) }
	py := func(v float64) float64 { return H - M - (v-y0)/(y1-y0)*(H-2*M) }

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" font-family="sans-serif" font-size="12">`+"\n", W, H)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="white"/>`+"\n")
	fmt.Fprintf(&b, `<text x="%d" y="25" font-size="15">Front de Pareto NSGA-II (prédiction)</text>`+"\n", M)
	fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="black"/>`+"\n", M, H-M, W-M, H-M)
	fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="black"/>`+"\n", M, M, M, H-M)
	for k := 0; k <= 4; k++ {
		xv, yv := x0+(x1-x0)*float64(k)/4, y0+(y1-y0)*float64(k)/4
		fmt.Fprintf(&b, `<text x="%.0f" y="%d" text-anchor="middle">%.0f</text>`+"\n", px(xv), H-M+18, xv)
		fmt.Fprintf(&b, `<text x="%d" y="%.0f" text-anchor="end">%.4f</text>`+"\n", M-6, py(yv)+4, yv)
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle">makespan prévu (s)</text>`+"\n", W/2, H-25)
	fmt.Fprintf(&b, `<text x="18" y="%d" transform="rotate(-90 18 %d)" text-anchor="middle">coût prévu (USD)</text>`+"\n", H/2, H/2)
	for _, s := range pop {
		if s.violation == 0 {
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="3" fill="#bbbbbb"/>`+"\n", px(s.makespan), py(s.cost))
		}
	}
	for _, s := range exact {
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="7" fill="none" stroke="black"/>`+"\n", px(s.makespan), py(s.cost))
	}
	for _, s := range front {
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="4" fill="#1f5fbf"/>`+"\n", px(s.makespan), py(s.cost))
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="#1f5fbf">%d VM</text>`+"\n", px(s.makespan)+10, py(s.cost)+16, s.nVMs)
	}
	fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="9" fill="none" stroke="#1a9c3c" stroke-width="3"/>`+"\n", px(chosen.makespan), py(chosen.cost))
	fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="9" height="9" fill="#d62728"/>`+"\n", px(rr.makespan)-4.5, py(rr.cost)-4.5)
	legend := []string{
		`<circle cx="0" cy="-4" r="4" fill="#1f5fbf"/>`, "front NSGA-II",
		`<circle cx="0" cy="-4" r="7" fill="none" stroke="black"/>`, "vrai front (recherche exhaustive)",
		`<circle cx="0" cy="-4" r="3" fill="#bbbbbb"/>`, "population finale",
		`<circle cx="0" cy="-4" r="7" fill="none" stroke="#1a9c3c" stroke-width="3"/>`, "solution choisie",
		`<rect x="-4.5" y="-8.5" width="9" height="9" fill="#d62728"/>`, "Round-Robin",
	}
	for k := 0; k < len(legend); k += 2 {
		y := M + 10 + k*10
		fmt.Fprintf(&b, `<g transform="translate(%d %d)">%s<text x="14" y="0">%s</text></g>`+"\n", M+40, y, legend[k], legend[k+1])
	}
	b.WriteString("</svg>\n")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		fail("%v", err)
	}
}

func main() {
	if len(os.Args) != 4 && len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "Usage: nsga2 TASKS.csv MACHINES.csv PROFILES.csv [SEED]")
		os.Exit(2)
	}
	seed := int64(42)
	if len(os.Args) == 5 {
		seed = int64(num(os.Args[4], "seed"))
	}
	load(os.Args[1], os.Args[2], os.Args[3])

	pop := nsga2(seed)
	front := paretoFront(pop)
	if len(front) == 0 {
		fail("aucune solution ne respecte la mémoire")
	}
	c := choose(front)

	// Round-Robin pour comparer : tâche i sur la VM i modulo le nombre de VMs
	rrAlloc := make([]int, len(tasks))
	for i := range rrAlloc {
		rrAlloc[i] = i % len(vms)
	}
	rr := evaluate(rrAlloc)

	// Sorties : on efface d'abord les anciennes solutions
	os.MkdirAll(outDir, 0755)
	old, _ := filepath.Glob(filepath.Join(outDir, "sol_*.csv"))
	for _, f := range old {
		os.Remove(f)
	}
	rows := [][]string{{"solution_id", "makespan_pred_s", "cost_pred_usd", "mem_violation_mb", "n_vms"}}
	for i, s := range front {
		id := fmt.Sprintf("sol_%03d", i+1)
		rows = append(rows, []string{id, fmt.Sprintf("%.2f", s.makespan), fmt.Sprintf("%.6f", s.cost),
			fmt.Sprintf("%.0f", s.violation), strconv.Itoa(s.nVMs)})
		writeCSV(filepath.Join(outDir, id+".csv"), allocationRows(s))
	}
	writeCSV(filepath.Join(outDir, "pareto.csv"), rows)
	writeCSV(chosenOut, allocationRows(front[c]))

	exact := exactFront()
	var report strings.Builder
	fmt.Fprintf(&report, "seed=%d population=%d generations=%d croisement=%.1f mutation=1/%d fermer_vm=%.1f\n",
		seed, popSize, nGen, crossProb, len(tasks), closeProb)
	fmt.Fprintf(&report, "front NSGA-II : %d solutions\n", len(front))
	for i, s := range front {
		mark := ""
		if i == c {
			mark = "  <- choisie (data/allocation_nsga.csv)"
		}
		fmt.Fprintf(&report, "  sol_%03d  makespan %7.2f s  coût %.6f USD  %d VM(s)%s\n", i+1, s.makespan, s.cost, s.nVMs, mark)
	}
	fmt.Fprintf(&report, "Round-Robin : makespan %.2f s, coût %.6f USD, %d VM(s)\n", rr.makespan, rr.cost, rr.nVMs)
	if exact != nil {
		found := map[string]bool{}
		for _, s := range front {
			found[key(s)] = true
		}
		n := 0
		for _, s := range exact {
			if found[key(s)] {
				n++
			}
		}
		fmt.Fprintf(&report, "recherche exhaustive : vrai front = %d solutions, NSGA-II en a trouvé %d\n", len(exact), n)
		for _, s := range exact {
			fmt.Fprintf(&report, "  vrai     makespan %7.2f s  coût %.6f USD  %d VM(s)\n", s.makespan, s.cost, s.nVMs)
		}
	}
	fmt.Print(report.String())
	os.WriteFile(filepath.Join(outDir, "run.txt"), []byte(report.String()), 0644)
	writeSVG(filepath.Join(outDir, "pareto.svg"), pop, front, exact, front[c], rr)
}
