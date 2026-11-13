// gantt : génère la timeline (diagramme de Gantt) d'une expérience au format SVG.
// Une ligne par tâche, regroupées par VM ; les tâches simultanées d'une VM sont empilées.
// Compilation : go build -o bin/gantt tools/gantt/main.go
// Usage       : ./bin/gantt results/<ID>/results.csv results/<ID>/gantt.svg
package main

import (
	"encoding/csv"
	"fmt"
	"html"
	"os"
	"sort"
	"strconv"
	"strings"
)

type task struct {
	id, workload, vm, mtype, code string
	start, end                    float64
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
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: gantt RESULTS.csv SORTIE.svg")
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
	fmt.Fprintf(&b, `<text x="%.0f" y="50" fill="#555">makespan = %.2f s, %d VMs</text>`+"\n", left, makespan, len(vms))

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
			if t.code != "0" {
				stroke = "#d62728"
			}
			x0, w := left+t.start*scale, (t.end-t.start)*scale
			ty := y + float64(t.lane)*laneH
			fmt.Fprintf(&b, `<g><title>%s %s : %.2f → %.2f s (%.2f s), code %s</title>`, t.id, t.workload, t.start, t.end, t.end-t.start, t.code)
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.0f" width="%.1f" height="%.0f" rx="3" fill="%s" stroke="%s" stroke-width="2"/>`, x0, ty+2, w, laneH-4, c, stroke)
			if w > 28 {
				fmt.Fprintf(&b, `<text x="%.1f" y="%.0f" fill="#ffffff">%s</text>`, x0+4, ty+laneH/2+4, html.EscapeString(t.id))
			}
			b.WriteString("</g>\n")
		}
		y += h + gap
	}

	// Légende
	lx := left
	for _, w := range []string{"cpu_task", "memory_task", "matrix_task", "sort_task"} {
		fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="14" height="14" fill="%s"/><text x="%.0f" y="%.0f">%s</text>`+"\n", lx, axisY+50, colors[w], lx+20, axisY+61, w)
		lx += 130
	}
	fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="14" height="14" fill="none" stroke="#d62728" stroke-width="2"/><text x="%.0f" y="%.0f">échec</text>`+"\n", lx, axisY+50, lx+20, axisY+61)
	b.WriteString("</svg>\n")

	if err := os.WriteFile(os.Args[2], []byte(b.String()), 0o644); err != nil {
		fail("%v", err)
	}
	fmt.Printf("Timeline écrite : %s (%d tâches, makespan %.2f s)\n", os.Args[2], len(recs)-1, makespan)
}
