// pareto_mesure : le front de Pareto prévu (data/pareto/pareto.csv) avec, par-dessus, les runs
// réellement mesurés (experiments.csv). Pour chaque run : une flèche du point PREVU vers le
// point MESURE. On voit tout de suite où et de combien le modèle se trompe.
// Les runs d'une même série (même stratégie, même prévision) sont regroupés : moyenne, "x3".
// Ne modifie rien : lit des CSV, écrit un SVG.
//
// Compilation : go build -o bin/pareto_mesure tools/pareto_mesure/main.go
// Usage       : ./bin/pareto_mesure [-note "HHMM=texte"]... PARETO.csv SORTIE.svg PREFIXES EXPERIMENTS.csv...
//
//	PREFIXES : préfixes de la colonne strategy à garder, séparés par des virgules (ex. s12_,demo_)
//	-note    : texte ajouté à l'étiquette des runs dont l'heure est HHMM (option répétable)
//
// Exemple     : ./bin/pareto_mesure -note "1729=vm3 Cascade Lake" data/pareto/pareto.csv /tmp/p.svg s12_ data/experiments.csv
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type point struct {
	strategy, hour           string
	predM, predC, mesM, mesC float64
	n                        int
}

type notes map[string]string

func (n notes) String() string { return "" }
func (n notes) Set(s string) error {
	k := strings.Index(s, "=")
	if k < 0 {
		return fmt.Errorf("format attendu HHMM=texte")
	}
	n[s[:k]] = s[k+1:]
	return nil
}

func fail(msg string, a ...any) {
	fmt.Fprintf(os.Stderr, "Erreur : "+msg+"\n", a...)
	os.Exit(1)
}

func read(path string) [][]string {
	f, err := os.Open(path)
	if err != nil {
		fail("%v", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		fail("%s vide ou illisible", path)
	}
	return rows
}

func num(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		fail("nombre invalide : %q", s)
	}
	return v
}

func colorOf(strategy string) string {
	switch {
	case strings.Contains(strategy, "nsga"):
		return "#1f5fbf"
	case strings.Contains(strategy, "heft"), strings.Contains(strategy, "greedy"):
		return "#e67e00"
	default:
		return "#d62728" // Round-Robin
	}
}

func main() {
	note := notes{}
	flag.Var(note, "note", "HHMM=texte, ajouté à l'étiquette des runs de cette heure")
	flag.Parse()
	args := flag.Args()
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: pareto_mesure [-note HHMM=texte]... PARETO.csv SORTIE.svg PREFIXES EXPERIMENTS.csv...")
		os.Exit(2)
	}
	prefixes := strings.Split(args[2], ",")

	// Front prévu : solution_id,makespan_pred_s,cost_pred_usd,mem_violation_mb,n_vms
	type sol struct {
		m, c float64
		n    string
	}
	var front []sol
	for _, r := range read(args[0])[1:] {
		front = append(front, sol{num(r[1]), num(r[2]), r[4]})
	}

	// Runs : experiment_id,strategy,makespan_s,cost_usd,pred_makespan_s,pred_cost_usd,...
	// Une série = même stratégie + même prévision : on fait la moyenne des mesures.
	var pts []*point
	series := map[string]*point{}
	for _, path := range args[3:] {
		for _, r := range read(path)[1:] {
			keep := false
			for _, pre := range prefixes {
				if strings.HasPrefix(r[1], strings.TrimSpace(pre)) {
					keep = true
				}
			}
			if !keep || r[4] == "" || r[5] == "" {
				continue
			}
			hour := ""
			if k := strings.Index(r[0], "-"); k >= 0 && len(r[0]) >= k+5 {
				hour = r[0][k+1 : k+5]
			}
			key := r[1] + "|" + r[4] + "|" + r[5]
			if strings.HasPrefix(r[1], "demo_") { // chaque démo reste un point à part
				key += "|" + r[0]
			}
			p, ok := series[key]
			if !ok {
				p = &point{strategy: r[1], hour: hour, predM: num(r[4]), predC: num(r[5])}
				series[key] = p
				pts = append(pts, p)
			} else if hour != p.hour { // série sur plusieurs heures : on ne garde que la 1re
				continue
			}
			p.mesM += num(r[2])
			p.mesC += num(r[3])
			p.n++
		}
	}
	if len(pts) == 0 {
		fail("aucun run ne correspond aux préfixes %q", args[2])
	}
	for _, p := range pts {
		p.mesM /= float64(p.n)
		p.mesC /= float64(p.n)
	}

	// Echelle commune ; la légende est à droite, hors du graphe
	x0, x1, y0, y1 := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	grow := func(m, c float64) {
		x0, x1 = math.Min(x0, m), math.Max(x1, m)
		y0, y1 = math.Min(y0, c), math.Max(y1, c)
	}
	for _, s := range front {
		grow(s.m, s.c)
	}
	for _, p := range pts {
		grow(p.predM, p.predC)
		grow(p.mesM, p.mesC)
	}
	dx, dy := (x1-x0)*0.08+1e-9, (y1-y0)*0.10+1e-12
	x0, x1, y0, y1 = x0-dx, x1+dx, y0-dy, y1+dy
	// Mise en page : le graphe en haut, la légende et la liste numérotée EN DESSOUS.
	// Dans le graphe, chaque run n'a qu'un numéro, placé dans le prolongement de sa flèche :
	// aucun texte long ne croise les flèches ni le front.
	const PW, PH, M, TOP = 760, 340, 80, 50 // largeur, hauteur du graphe, marge gauche, haut
	W := M + PW + 40
	H := TOP + PH + 70 + 2*20 + 12 + len(pts)*20 + 20
	bottom := float64(TOP + PH)
	px := func(v float64) float64 { return M + (v-x0)/(x1-x0)*PW }
	py := func(v float64) float64 { return bottom - (v-y0)/(y1-y0)*PH }
	fr := func(v float64, prec int) string {
		return strings.Replace(strconv.FormatFloat(v, 'f', prec, 64), ".", ",", 1)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" font-family="sans-serif" font-size="12">`+"\n", W, H)
	b.WriteString(`<defs><marker id="fl" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0,0 L10,5 L0,10 z" fill="#777"/></marker></defs>` + "\n")
	b.WriteString(`<rect width="100%" height="100%" fill="white"/>` + "\n")
	fmt.Fprintf(&b, `<text x="%d" y="28" font-size="15">Front de Pareto prévu et runs mesurés sur GCP (12 tâches)</text>`+"\n", M)
	fmt.Fprintf(&b, `<line x1="%d" y1="%.0f" x2="%d" y2="%.0f" stroke="black"/>`+"\n", M, bottom, M+PW, bottom)
	fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%.0f" stroke="black"/>`+"\n", M, TOP, M, bottom)
	for k := 0; k <= 4; k++ {
		xv, yv := x0+(x1-x0)*float64(k)/4, y0+(y1-y0)*float64(k)/4
		fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" text-anchor="middle">%.0f</text>`+"\n", px(xv), bottom+18, xv)
		fmt.Fprintf(&b, `<text x="%d" y="%.0f" text-anchor="end">%s</text>`+"\n", M-6, py(yv)+4, fr(yv, 4))
	}
	fmt.Fprintf(&b, `<text x="%d" y="%.0f" text-anchor="middle">makespan (s)</text>`+"\n", M+PW/2, bottom+38)
	fmt.Fprintf(&b, `<text x="20" y="%.0f" transform="rotate(-90 20 %.0f)" text-anchor="middle">coût (USD)</text>`+"\n", bottom-PH/2, bottom-PH/2)

	// Zones déjà occupées (centre, demi-largeur, demi-hauteur) : pour ne rien superposer
	type box struct{ x, y, w, h float64 }
	var used []box
	free := func(x, y, w, h float64) bool {
		for _, u := range used {
			if math.Abs(u.x-x) < u.w+w && math.Abs(u.y-y) < u.h+h {
				return false
			}
		}
		return true
	}

	// Front prévu (pointillés) ; "n VM" sous chaque point
	var line []string
	for _, s := range front {
		line = append(line, fmt.Sprintf("%.1f,%.1f", px(s.m), py(s.c)))
	}
	fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#1f5fbf" stroke-dasharray="4 3" opacity="0.7"/>`+"\n", strings.Join(line, " "))
	for _, s := range front {
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="4" fill="#1f5fbf"/>`+"\n", px(s.m), py(s.c))
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="#1f5fbf" font-size="11" text-anchor="middle">%s VM</text>`+"\n", px(s.m), py(s.c)+17, s.n)
		used = append(used, box{px(s.m), py(s.c) + 13, 18, 7})
	}

	// Flèches prévu -> mesuré, puis les points
	for _, p := range pts {
		c := colorOf(p.strategy)
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#777" marker-end="url(#fl)"/>`+"\n", px(p.predM), py(p.predC), px(p.mesM), py(p.mesC))
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="10" height="10" fill="white" stroke="%s" stroke-width="2"/>`+"\n", px(p.predM)-5, py(p.predC)-5, c)
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="5" fill="%s"/>`+"\n", px(p.mesM), py(p.mesC), c)
		used = append(used, box{px(p.mesM), py(p.mesC), 5, 5}, box{px(p.predM), py(p.predC), 5, 5})
	}

	// Numéros : dans le prolongement de la flèche, puis on tourne autour du point si la place est prise
	for k, p := range pts {
		mx, my := px(p.mesM), py(p.mesC)
		ang := math.Atan2(my-py(p.predC), mx-px(p.predM))
		if math.Hypot(my-py(p.predC), mx-px(p.predM)) < 1 {
			ang = 0
		}
		var x, y float64
		for step := 0; step < 24; step++ {
			a := ang + float64((step+1)/2)*math.Pi/6*float64(1-2*(step%2)) // 0, +30, -30, +60...
			r := 17.0 + float64(step/12)*12
			x, y = mx+r*math.Cos(a), my+r*math.Sin(a)
			if free(x, y, 9, 9) && x > M+10 && x < float64(M+PW)-10 && y > TOP+10 && y < bottom-10 {
				break
			}
		}
		used = append(used, box{x, y, 9, 9})
		c := colorOf(p.strategy)
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="9" fill="white" stroke="%s" stroke-width="1.5"/>`+"\n", x, y, c)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="%s" font-size="11" font-weight="bold" text-anchor="middle">%d</text>`+"\n", x, y+4, c, k+1)
	}

	// Légende des symboles (2 lignes), sous le graphe
	legend := [][2]string{
		{`<circle cx="0" cy="-4" r="4" fill="#1f5fbf"/>`, "front NSGA-II prévu (modèle)"},
		{`<rect x="-5" y="-9" width="10" height="10" fill="white" stroke="#555" stroke-width="2"/>`, "prévu pour le run"},
		{`<circle cx="0" cy="-4" r="5" fill="#555"/>`, "mesuré sur GCP"},
		{`<circle cx="0" cy="-4" r="5" fill="#d62728"/>`, "Round-Robin"},
		{`<circle cx="0" cy="-4" r="5" fill="#1f5fbf"/>`, "NSGA-II"},
		{`<circle cx="0" cy="-4" r="5" fill="#e67e00"/>`, "HEFT (glouton)"},
	}
	ly := bottom + 70
	for k, l := range legend {
		fmt.Fprintf(&b, `<g transform="translate(%d %.0f)">%s<text x="14" y="0">%s</text></g>`+"\n", M+(k%3)*250, ly+float64(k/3)*20, l[0], l[1])
	}

	// Liste numérotée : le détail de chaque run
	name := func(s string) string {
		switch {
		case strings.Contains(s, "nsga"):
			return "NSGA-II"
		case strings.Contains(s, "heft"), strings.Contains(s, "greedy"):
			return "HEFT"
		default:
			return "Round-Robin"
		}
	}
	ly += 2*20 + 12
	for k, p := range pts {
		label := fmt.Sprintf("%d. %sh%s %s", k+1, p.hour[:2], p.hour[2:], name(p.strategy))
		if p.n > 1 {
			label += fmt.Sprintf(" (moyenne de %d runs)", p.n)
		}
		if t, ok := note[p.hour]; ok {
			label += ", " + t
		}
		erreur := 100 * (p.predM - p.mesM) / p.mesM
		fmt.Fprintf(&b, `<text x="%d" y="%.0f" fill="%s">%s : mesuré %s s et %s USD ; prévu %s s (%s %%)</text>`+"\n",
			M, ly+float64(k)*20, colorOf(p.strategy), label, fr(p.mesM, 1), fr(p.mesC, 5), fr(p.predM, 1), fmt.Sprintf("%+.0f", erreur))
	}
	b.WriteString("</svg>\n")
	if err := os.WriteFile(args[1], []byte(b.String()), 0644); err != nil {
		fail("%v", err)
	}
	fmt.Printf("%d points (séries regroupées) dessinés dans %s\n", len(pts), args[1])
}
