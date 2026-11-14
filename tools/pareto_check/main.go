// pareto_check : calcule le VRAI front de Pareto (makespan, coût) en essayant TOUTES les allocations
// possibles avec le modèle commun de INTERFACES.md, puis le compare à NSGA-II et à Round-Robin.
//
// Pourquoi : avec 3 VMs et 12 tâches il n'y a que 3^12 = 531 441 allocations, ce qui se calcule en
// moins d'une seconde. On peut donc vérifier si NSGA-II trouve vraiment les meilleures solutions,
// sans rien payer sur GCP (tout est calculé avec profiles.csv).
//
// Compilation : go build -o bin/pareto_check tools/pareto_check/main.go
// Usage :
//   ./bin/pareto_check -tasks data/tasks.csv -machines data/machines.csv -profiles data/profiles.csv \
//       -rr data/allocation_rr.csv -nsga data/pareto/pareto.csv -out data/pareto_exact
// -rr et -nsga sont optionnels. Sorties dans -out :
//   pareto_exact.csv          front exact (même format que pareto.csv)
//   exact_sol_001.csv ...     une allocation par point du front exact (format allocation)
//   pareto_check.svg          figure : toutes les allocations, front exact, NSGA-II, Round-Robin
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type machine struct {
	vm, mtype     string
	vcpu          float64
	ramMB, priceH float64
}

type task struct{ id, workload, size string }

type point struct {
	mk, cost float64
	code     int64 // allocation codée en base len(machines)
}

const maxCombos = 6_000_000 // au-delà, l'énumération devient trop lourde : utiliser NSGA-II seul

func fail(msg string, a ...any) {
	fmt.Fprintf(os.Stderr, "Erreur : "+msg+"\n", a...)
	os.Exit(1)
}

func readCSV(path string, header []string) [][]string {
	f, err := os.Open(path)
	if err != nil {
		fail("%v", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		fail("lecture %s : %v", path, err)
	}
	if len(rows) == 0 {
		fail("%s est vide", path)
	}
	for i, h := range header {
		if i >= len(rows[0]) || strings.TrimSpace(strings.TrimPrefix(rows[0][i], "\ufeff")) != h {
			fail("%s : la colonne %d doit être %q (voir INTERFACES.md)", path, i+1, h)
		}
	}
	out := [][]string{}
	for _, row := range rows[1:] {
		if len(row) == 0 || strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
		out = append(out, row)
	}
	return out
}

func num(s, ctx string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		fail("nombre invalide %q (%s)", s, ctx)
	}
	return v
}

func main() {
	tasksF := flag.String("tasks", "data/tasks.csv", "scénario")
	machF := flag.String("machines", "data/machines.csv", "machines")
	profF := flag.String("profiles", "data/profiles.csv", "mesures")
	rrF := flag.String("rr", "", "allocation de référence (optionnel), ex. data/allocation_rr.csv")
	nsgaF := flag.String("nsga", "", "front NSGA-II (optionnel), ex. data/pareto/pareto.csv")
	outD := flag.String("out", "data/pareto_exact", "dossier de sortie")
	flag.Parse()

	// --- Lecture des entrées
	var ms []machine
	vmIndex := map[string]int{}
	for _, r := range readCSV(*machF, []string{"vm", "machine_type", "vcpu", "ram_mb", "price_per_hour_usd"}) {
		vmIndex[r[0]] = len(ms)
		ms = append(ms, machine{r[0], r[1], num(r[2], "vcpu"), num(r[3], "ram_mb"), num(r[4], "price")})
	}
	var ts []task
	for _, r := range readCSV(*tasksF, []string{"task_id", "workload", "size"}) {
		ts = append(ts, task{r[0], r[1], r[2]})
	}
	type prof struct{ med, rssMB float64 }
	profiles := map[string]prof{}
	for _, r := range readCSV(*profF, []string{"workload", "size", "machine_type", "runs", "mean_runtime_s", "median_runtime_s", "std_runtime_s", "max_rss_kb"}) {
		profiles[r[0]+"|"+r[1]+"|"+r[2]] = prof{num(r[5], "median_runtime_s"), num(r[7], "max_rss_kb") / 1024}
	}
	nT, nM := len(ts), len(ms)
	if nT == 0 || nM == 0 {
		fail("aucune tâche ou aucune machine")
	}

	// Temps et mémoire de chaque tâche sur chaque VM (le modèle utilise la médiane)
	rt := make([][]float64, nT)
	rss := make([][]float64, nT)
	var missing []string
	for i, t := range ts {
		rt[i], rss[i] = make([]float64, nM), make([]float64, nM)
		for j, m := range ms {
			p, ok := profiles[t.workload+"|"+t.size+"|"+m.mtype]
			if !ok {
				missing = append(missing, t.workload+" "+t.size+" sur "+m.mtype)
				continue
			}
			rt[i][j], rss[i][j] = p.med, p.rssMB
		}
	}
	if len(missing) > 0 {
		fail("profiles.csv incomplet, il manque :\n  %s", strings.Join(missing, "\n  "))
	}

	combos := math.Pow(float64(nM), float64(nT))
	if combos > maxCombos {
		fail("%d^%d = %.0f allocations, trop pour une énumération complète (max %d). Réduire le scénario.", nM, nT, combos, maxCombos)
	}
	total := int64(combos)

	// Modèle commun (INTERFACES.md)
	sum := make([]float64, nM)
	longest := make([]float64, nM)
	mem := make([]float64, nM)
	eval := func(alloc []int) (mk, cost, viol float64) {
		for j := range ms {
			sum[j], longest[j], mem[j] = 0, 0, 0
		}
		for i, j := range alloc {
			sum[j] += rt[i][j]
			if rt[i][j] > longest[j] {
				longest[j] = rt[i][j]
			}
			mem[j] += rss[i][j]
		}
		for j, m := range ms {
			fin := math.Max(sum[j]/m.vcpu, longest[j])
			mk = math.Max(mk, fin)
			cost += m.priceH / 3600 * fin
			if over := mem[j] - 0.9*m.ramMB; over > 0 {
				viol += over
			}
		}
		return
	}
	decode := func(code int64, alloc []int) {
		for i := 0; i < nT; i++ {
			alloc[i] = int(code % int64(nM))
			code /= int64(nM)
		}
	}

	// --- Énumération de toutes les allocations (compteur en base nM)
	alloc := make([]int, nT)
	var feasible []point
	infeasible := int64(0)
	for code := int64(0); code < total; code++ {
		decode(code, alloc)
		mk, cost, viol := eval(alloc)
		if viol > 0 {
			infeasible++
			continue
		}
		feasible = append(feasible, point{mk, cost, code})
	}
	if len(feasible) == 0 {
		fail("aucune allocation ne respecte la contrainte mémoire")
	}

	// --- Front de Pareto exact : tri par makespan puis coût, on garde ce qui baisse le coût
	sort.Slice(feasible, func(a, b int) bool {
		if feasible[a].mk != feasible[b].mk {
			return feasible[a].mk < feasible[b].mk
		}
		return feasible[a].cost < feasible[b].cost
	})
	var front []point
	bestCost := math.Inf(1)
	for _, p := range feasible {
		if p.cost < bestCost-1e-12 {
			front = append(front, p)
			bestCost = p.cost
		}
	}

	if err := os.MkdirAll(*outD, 0755); err != nil {
		fail("%v", err)
	}
	pf, err := os.Create(filepath.Join(*outD, "pareto_exact.csv"))
	if err != nil {
		fail("%v", err)
	}
	fmt.Fprintln(pf, "solution_id,makespan_pred_s,cost_pred_usd,mem_violation_mb")
	for k, p := range front {
		id := fmt.Sprintf("exact_sol_%03d", k+1)
		fmt.Fprintf(pf, "%s,%.3f,%.6f,0\n", id, p.mk, p.cost)
		decode(p.code, alloc)
		af, err := os.Create(filepath.Join(*outD, id+".csv"))
		if err != nil {
			fail("%v", err)
		}
		fmt.Fprintln(af, "task_id,workload,size,vm")
		for i, t := range ts {
			fmt.Fprintf(af, "%s,%s,%s,%s\n", t.id, t.workload, t.size, ms[alloc[i]].vm)
		}
		af.Close()
	}
	pf.Close()

	fmt.Printf("Allocations testées : %d (%d respectent la mémoire, %d non)\n", total, len(feasible), infeasible)
	fmt.Printf("Front exact : %d solutions, makespan %.1f s -> %.1f s, coût %.5f -> %.5f USD\n",
		len(front), front[0].mk, front[len(front)-1].mk, front[0].cost, front[len(front)-1].cost)

	// Point de référence pour l'hypervolume : le pire makespan et le pire coût faisables, +5 %
	refMk, refCost := 0.0, 0.0
	for _, p := range feasible {
		refMk, refCost = math.Max(refMk, p.mk), math.Max(refCost, p.cost)
	}
	refMk, refCost = refMk*1.05, refCost*1.05
	hvExact := hypervolume(front, refMk, refCost)

	// --- Round-Robin (ou toute autre allocation de référence)
	var rrPt *point
	if *rrF != "" {
		a := make([]int, nT)
		byID := map[string]int{}
		for _, r := range readCSV(*rrF, []string{"task_id", "workload", "size", "vm"}) {
			j, ok := vmIndex[r[3]]
			if !ok {
				fail("%s : VM inconnue %q", *rrF, r[3])
			}
			byID[r[0]] = j
		}
		for i, t := range ts {
			j, ok := byID[t.id]
			if !ok {
				fail("%s : la tâche %s n'est pas allouée", *rrF, t.id)
			}
			a[i] = j
		}
		mk, cost, viol := eval(a)
		rrPt = &point{mk, cost, -1}
		fmt.Printf("\nRéférence (%s) : makespan prédit %.1f s, coût prédit %.5f USD", filepath.Base(*rrF), mk, cost)
		if viol > 0 {
			fmt.Printf(", DÉPASSE la mémoire de %.0f Mo", viol)
		}
		fmt.Println()
		// meilleure solution exacte qui ne coûte pas plus cher que la référence
		for _, p := range front {
			if p.cost <= cost+1e-12 {
				fmt.Printf("  -> pour un coût <= référence, l'optimum exact fait %.1f s (%.0f %% plus rapide)\n", p.mk, 100*(1-p.mk/mk))
				break
			}
		}
	}

	// --- NSGA-II
	var nsga []point
	if *nsgaF != "" {
		for _, r := range readCSV(*nsgaF, []string{"solution_id", "makespan_pred_s", "cost_pred_usd", "mem_violation_mb"}) {
			if num(r[3], "mem_violation_mb") > 0 {
				continue
			}
			nsga = append(nsga, point{num(r[1], "makespan"), num(r[2], "cost"), -1})
		}
		onFront := 0
		worstGap := 0.0
		for _, q := range nsga {
			// écart relatif au front exact : de combien il faudrait réduire q pour toucher le front
			gap := math.Inf(1)
			for _, p := range front {
				g := math.Max((q.mk-p.mk)/p.mk, (q.cost-p.cost)/p.cost)
				gap = math.Min(gap, math.Max(g, 0))
			}
			if gap < 1e-3 { // tolérance 0,1 % : les CSV sont arrondis (3 et 6 décimales)
				onFront++
			}
			worstGap = math.Max(worstGap, gap)
		}
		nf := paretoOf(nsga)
		fmt.Printf("\nNSGA-II : %d solutions faisables, %d sont exactement sur le front optimal\n", len(nsga), onFront)
		fmt.Printf("  plus grand écart au front exact : %.1f %%\n", 100*worstGap)
		fmt.Printf("  hypervolume NSGA-II / exact : %.1f %% (100 %% = front optimal trouvé)\n", 100*hypervolume(nf, refMk, refCost)/hvExact)
	}

	svg := filepath.Join(*outD, "pareto_check.svg")
	drawSVG(svg, feasible, front, nsga, rrPt)
	fmt.Printf("\nFichiers écrits dans %s/ (pareto_exact.csv, exact_sol_*.csv, pareto_check.svg)\n", *outD)
}

func paretoOf(pts []point) []point {
	s := append([]point(nil), pts...)
	sort.Slice(s, func(a, b int) bool {
		if s[a].mk != s[b].mk {
			return s[a].mk < s[b].mk
		}
		return s[a].cost < s[b].cost
	})
	var f []point
	best := math.Inf(1)
	for _, p := range s {
		if p.cost < best-1e-12 {
			f = append(f, p)
			best = p.cost
		}
	}
	return f
}

// hypervolume 2D (minimisation) : aire dominée par le front, bornée par le point de référence
func hypervolume(front []point, refMk, refCost float64) float64 {
	hv, prevCost := 0.0, refCost
	for _, p := range front { // trié par makespan croissant, coût décroissant
		if p.mk >= refMk || p.cost >= prevCost {
			continue
		}
		hv += (refMk - p.mk) * (prevCost - p.cost)
		prevCost = p.cost
	}
	return hv
}

func drawSVG(path string, all, front, nsga []point, rr *point) {
	const W, H, L, R, T, B = 820.0, 520.0, 90.0, 30.0, 50.0, 60.0
	minX, maxX, minY, maxY := math.Inf(1), 0.0, math.Inf(1), 0.0
	ext := append(append([]point(nil), front...), nsga...)
	if rr != nil {
		ext = append(ext, *rr)
	}
	// l'échelle suit les points intéressants (front, NSGA, RR) avec une marge, pas les pires allocations
	for _, p := range ext {
		minX, maxX = math.Min(minX, p.mk), math.Max(maxX, p.mk)
		minY, maxY = math.Min(minY, p.cost), math.Max(maxY, p.cost)
	}
	dx, dy := (maxX-minX)*0.6+1e-9, (maxY-minY)*0.6+1e-12
	minX, maxX, minY, maxY = minX-dx*0.15, maxX+dx, minY-dy*0.15, maxY+dy
	X := func(v float64) float64 { return L + (v-minX)/(maxX-minX)*(W-L-R) }
	Y := func(v float64) float64 { return H - B - (v-minY)/(maxY-minY)*(H-T-B) }
	in := func(p point) bool { return p.mk <= maxX && p.cost <= maxY }

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" font-family="sans-serif" font-size="12">`, W, H)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="white"/>`)
	fmt.Fprintf(&b, `<text x="%.0f" y="28" font-size="16" font-weight="bold">Makespan et coût prédits : toutes les allocations, front exact, NSGA-II, Round-Robin</text>`, L)
	// axes + graduations
	fmt.Fprintf(&b, `<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#333"/>`, L, H-B, W-R, H-B)
	fmt.Fprintf(&b, `<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#333"/>`, L, T, L, H-B)
	for k := 0; k <= 5; k++ {
		vx := minX + (maxX-minX)*float64(k)/5
		vy := minY + (maxY-minY)*float64(k)/5
		fmt.Fprintf(&b, `<text x="%.1f" y="%.0f" text-anchor="middle" fill="#555">%.0f</text>`, X(vx), H-B+18, vx)
		fmt.Fprintf(&b, `<text x="%.0f" y="%.1f" text-anchor="end" fill="#555">%.4f</text>`, L-6, Y(vy)+4, vy)
		fmt.Fprintf(&b, `<line x1="%.0f" y1="%.1f" x2="%.0f" y2="%.1f" stroke="#eee"/>`, L, Y(vy), W-R, Y(vy))
	}
	fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" text-anchor="middle">makespan prédit (s)</text>`, (L+W-R)/2, H-15)
	fmt.Fprintf(&b, `<text x="20" y="%.0f" transform="rotate(-90 20 %.0f)" text-anchor="middle">coût prédit (USD)</text>`, (T+H-B)/2, (T+H-B)/2)

	// nuage : un échantillon des allocations faisables (au plus 4000 points pour garder un SVG léger)
	rng := rand.New(rand.NewSource(1))
	idx := rng.Perm(len(all))
	drawn := 0
	for _, i := range idx {
		if drawn >= 4000 {
			break
		}
		if in(all[i]) {
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="1.6" fill="#c8c8c8"/>`, X(all[i].mk), Y(all[i].cost))
			drawn++
		}
	}
	// front exact en escalier
	var pts []string
	for k, p := range front {
		if k > 0 {
			pts = append(pts, fmt.Sprintf("%.1f,%.1f", X(p.mk), Y(front[k-1].cost)))
		}
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", X(p.mk), Y(p.cost)))
	}
	fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#1f6fb2" stroke-width="2"/>`, strings.Join(pts, " "))
	for _, p := range front {
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="4" fill="#1f6fb2"/>`, X(p.mk), Y(p.cost))
	}
	for _, p := range nsga {
		if in(p) {
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="9" height="9" fill="none" stroke="#e07b00" stroke-width="2"/>`, X(p.mk)-4.5, Y(p.cost)-4.5)
		}
	}
	if rr != nil {
		x, y := X(rr.mk), Y(rr.cost)
		fmt.Fprintf(&b, `<path d="M%.1f %.1f l7 7 m0 -7 l-7 7" transform="translate(-3.5 -3.5)" stroke="#c0392b" stroke-width="3"/>`, x, y)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="#c0392b" font-weight="bold">Round-Robin</text>`, x+9, y-6)
	}
	// légende
	lx, ly := W-R-230, T+10
	fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="225" height="78" fill="white" stroke="#ccc"/>`, lx, ly)
	fmt.Fprintf(&b, `<circle cx="%.0f" cy="%.0f" r="3" fill="#c8c8c8"/><text x="%.0f" y="%.0f">allocations possibles (échantillon)</text>`, lx+12, ly+16, lx+24, ly+20)
	fmt.Fprintf(&b, `<circle cx="%.0f" cy="%.0f" r="4" fill="#1f6fb2"/><text x="%.0f" y="%.0f">front de Pareto exact</text>`, lx+12, ly+34, lx+24, ly+38)
	fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="9" height="9" fill="none" stroke="#e07b00" stroke-width="2"/><text x="%.0f" y="%.0f">solutions NSGA-II</text>`, lx+7.5, ly+47.5, lx+24, ly+56)
	fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" fill="#c0392b" font-weight="bold">x</text><text x="%.0f" y="%.0f">Round-Robin</text>`, lx+8, ly+74, lx+24, ly+74)
	b.WriteString(`</svg>`)
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		fail("%v", err)
	}
}
