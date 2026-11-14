// gantt : génère la timeline (diagramme de Gantt) d'une expérience au format SVG.
// Une ligne par tâche, regroupées par VM ; les tâches simultanées d'une VM sont empilées.
// Compilation : go build -o bin/gantt tools/gantt/main.go
// Usage       : ./bin/gantt results/<ID>/results.csv results/<ID>/gantt.svg [results/<ID>/tasks.csv]
// Avec tasks.csv (colonne depends_on) : une flèche par dépendance, et le chemin critique
// (la chaîne de tâches qui fixe le makespan) entouré en noir, ses flèches en rouge.
package main

import (
	"encoding/csv"
	"fmt"
	"html"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

type task struct {
	id, workload, vm, mtype, code string
	start, end, yc                float64
	lane                          int
}

var colors = map[string]string{
	"cpu_task": "#4C78A8", "memory_task": "#F58518",
	"matrix_task": "#54A24B", "sort_task": "#B279A2",
}

func fail(msg string, a ...any) {
	fmt.Fprintf(os.Stderr, "Erreur : "+msg+"\n", a...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 3 && len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "Usage: gantt RESULTS.csv SORTIE.svg [TASKS.csv]")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fail("%v", err)
	}
	recs, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil || len(recs) < 2 {
		fail("%s vide ou illisible", os.Args[1])
	}
	col := map[string]int{}
	for i, h := range recs[0] {
		col[strings.TrimSpace(h)] = i
	}
	for _, c := range []string{"experiment_id", "task_id", "workload", "vm", "machine_type", "exit_code", "start_s", "end_s"} {
		if _, ok := col[c]; !ok {
			fail("colonne %q manquante dans %s", c, os.Args[1])
		}
	}

	// Lecture des tâches
	exp := recs[1][col["experiment_id"]]
	byVM := map[string][]*task{}
	mtypes := map[string]string{}
	makespan := 0.0
	for _, r := range recs[1:] {
		s, err1 := strconv.ParseFloat(r[col["start_s"]], 64)
		e, err2 := strconv.ParseFloat(r[col["end_s"]], 64)
		if err1 != nil || err2 != nil {
			fail("start_s/end_s invalide pour %s", r[col["task_id"]])
		}
		t := &task{id: r[col["task_id"]], workload: r[col["workload"]], vm: r[col["vm"]],
			mtype: r[col["machine_type"]], code: r[col["exit_code"]], start: s, end: e}
		byVM[t.vm] = append(byVM[t.vm], t)
		mtypes[t.vm] = t.mtype
		if e > makespan {
			makespan = e
		}
	}
	if makespan <= 0 {
		makespan = 1
	}

	// Dépendances (optionnel) et chemin critique
	byID := map[string]*task{}
	for _, ts := range byVM {
		for _, t := range ts {
			byID[t.id] = t
		}
	}
	deps := map[string][]string{}
	if len(os.Args) == 4 {
		deps = readDeps(os.Args[3])
	}
	critical := map[string]bool{}
	if len(deps) > 0 {
		// on part de la tâche qui finit en dernier, et on remonte vers la dépendance qui a fini le plus tard
		var cur *task
		for _, t := range byID {
			if cur == nil || t.end > cur.end {
				cur = t
			}
		}
		for cur != nil {
			critical[cur.id] = true
			var prev *task
			for _, d := range deps[cur.id] {
				if p, ok := byID[d]; ok && (prev == nil || p.end > prev.end) {
					prev = p
				}
			}
			cur = prev
		}
	}
	nDeps := 0
	for _, ds := range deps {
		nDeps += len(ds)
	}
	vms := make([]string, 0, len(byVM))
	for vm := range byVM {
		vms = append(vms, vm)
	}
	sort.Strings(vms)

	// Empilement : chaque tâche va sur la première "voie" libre de sa VM
	lanes := map[string]int{}
	for _, vm := range vms {
		ts := byVM[vm]
		sort.Slice(ts, func(i, j int) bool { return ts[i].start < ts[j].start })
		var laneEnd []float64
		for _, t := range ts {
			t.lane = -1
			for k, le := range laneEnd {
				if le <= t.start {
					t.lane, laneEnd[k] = k, t.end
					break
				}
			}
			if t.lane < 0 {
				t.lane = len(laneEnd)
				laneEnd = append(laneEnd, t.end)
			}
		}
		lanes[vm] = len(laneEnd)
	}

	// Mise en page
	const width, left, right, top, laneH, gap = 1000.0, 190.0, 30.0, 70.0, 22.0, 16.0
	scale := (width - left - right) / makespan
	height := top
	for _, vm := range vms {
		height += float64(lanes[vm])*laneH + gap
	}
	axisY := height
	height += 80

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" font-family="Arial, sans-serif" font-size="12">`+"\n", width, height)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="#ffffff"/>`+"\n")
	fmt.Fprintf(&b, `<text x="%.0f" y="28" font-size="18" font-weight="bold">Timeline – %s</text>`+"\n", left, html.EscapeString(exp))
	sub := fmt.Sprintf("makespan = %.2f s, %d VMs", makespan, len(vms))
	if nDeps > 0 {
		sub += fmt.Sprintf(", %d dépendances, chemin critique : %d tâches (entourées en noir)", nDeps, len(critical))
	}
	fmt.Fprintf(&b, `<text x="%.0f" y="50" fill="#555">%s</text>`+"\n", left, html.EscapeString(sub))
	b.WriteString(`<defs><marker id="fl" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0,0 L10,5 L0,10 z" fill="#666"/></marker><marker id="flc" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0,0 L10,5 L0,10 z" fill="#c0392b"/></marker></defs>` + "\n")

	// Graduations de l'axe du temps
	step := 1.0
	for _, s := range []float64{0.5, 1, 2, 5, 10, 20, 30, 60, 120, 300, 600} {
		step = s
		if makespan/s <= 10 {
			break
		}
	}
	for x := 0.0; x <= makespan+1e-9; x += step {
		px := left + x*scale
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.0f" x2="%.1f" y2="%.0f" stroke="#e5e5e5"/>`+"\n", px, top-8, px, axisY)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.0f" text-anchor="middle" fill="#555">%g</text>`+"\n", px, axisY+16, x)
	}
	fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" text-anchor="middle" fill="#555">temps depuis le début de l'expérience (s)</text>`+"\n", left+(width-left-right)/2, axisY+34)

	// Une bande par VM
	y := top
	for _, vm := range vms {
		h := float64(lanes[vm]) * laneH
		fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" fill="#f5f5f5"/>`+"\n", left, y-2, width-left-right, h+4)
		fmt.Fprintf(&b, `<text x="10" y="%.0f" font-weight="bold">%s</text>`+"\n", y+h/2-2, html.EscapeString(vm))
		fmt.Fprintf(&b, `<text x="10" y="%.0f" fill="#555">%s</text>`+"\n", y+h/2+13, html.EscapeString(mtypes[vm]))
		for _, t := range byVM[vm] {
			c, ok := colors[t.workload]
			if !ok {
				c = "#888888"
			}
			stroke := "none"
			if critical[t.id] {
				stroke = "#000000"
			}
			if t.code != "0" {
				stroke = "#d62728"
			}
			x0, w := left+t.start*scale, (t.end-t.start)*scale
			ty := y + float64(t.lane)*laneH
			t.yc = ty + laneH/2
			fmt.Fprintf(&b, `<g><title>%s %s : %.2f → %.2f s (%.2f s), code %s</title>`, t.id, t.workload, t.start, t.end, t.end-t.start, t.code)
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.0f" width="%.1f" height="%.0f" rx="3" fill="%s" stroke="%s" stroke-width="2"/>`, x0, ty+2, w, laneH-4, c, stroke)
			if w > 28 {
				fmt.Fprintf(&b, `<text x="%.1f" y="%.0f" fill="#ffffff">%s</text>`, x0+4, ty+laneH/2+4, html.EscapeString(t.id))
			}
			b.WriteString("</g>\n")
		}
		y += h + gap
	}

	// Flèches de dépendance : de la fin de la dépendance au début de la tâche qui l'attendait
	for id, ds := range deps {
		t, ok := byID[id]
		if !ok {
			continue
		}
		for _, d := range ds {
			p, ok := byID[d]
			if !ok {
				continue
			}
			x1, y1 := left+p.end*scale, p.yc
			x2, y2 := left+t.start*scale, t.yc
			col, mk, sw := "#666", "fl", "1.2"
			if critical[id] && critical[d] {
				col, mk, sw = "#c0392b", "flc", "2"
			}
			dx := math.Max(20, (x2-x1)/2)
			fmt.Fprintf(&b, `<path d="M%.1f %.1f C%.1f %.1f %.1f %.1f %.1f %.1f" fill="none" stroke="%s" stroke-width="%s" marker-end="url(#%s)" opacity="0.85"/>`+"\n",
				x1, y1, x1+dx, y1, x2-dx, y2, x2, y2, col, sw, mk)
		}
	}

	// Légende
	lx := left
	for _, w := range []string{"cpu_task", "memory_task", "matrix_task", "sort_task"} {
		fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="14" height="14" fill="%s"/><text x="%.0f" y="%.0f">%s</text>`+"\n", lx, axisY+50, colors[w], lx+20, axisY+61, w)
		lx += 130
	}
	fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="14" height="14" fill="none" stroke="#d62728" stroke-width="2"/><text x="%.0f" y="%.0f">échec</text>`+"\n", lx, axisY+50, lx+20, axisY+61)
	if nDeps > 0 {
		lx += 80
		fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="14" height="14" fill="none" stroke="#000" stroke-width="2"/><text x="%.0f" y="%.0f">chemin critique</text>`+"\n", lx, axisY+50, lx+20, axisY+61)
	}
	b.WriteString("</svg>\n")

	if err := os.WriteFile(os.Args[2], []byte(b.String()), 0o644); err != nil {
		fail("%v", err)
	}
	fmt.Printf("Timeline écrite : %s (%d tâches, makespan %.2f s)\n", os.Args[2], len(recs)-1, makespan)
}

// readDeps lit la colonne depends_on d'un tasks.csv (ids séparés par ";"). Sans colonne : aucune dépendance.
func readDeps(path string) map[string][]string {
	f, err := os.Open(path)
	if err != nil {
		fail("%v", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil || len(recs) == 0 {
		fail("%s vide ou illisible", path)
	}
	c := -1
	for i, h := range recs[0] {
		if strings.TrimSpace(h) == "depends_on" {
			c = i
		}
	}
	out := map[string][]string{}
	if c < 0 {
		return out
	}
	for _, row := range recs[1:] {
		if c >= len(row) || strings.TrimSpace(row[c]) == "" {
			continue
		}
		for _, d := range strings.Split(row[c], ";") {
			if d = strings.TrimSpace(d); d != "" {
				out[strings.TrimSpace(row[0])] = append(out[strings.TrimSpace(row[0])], d)
			}
		}
	}
	return out
}
