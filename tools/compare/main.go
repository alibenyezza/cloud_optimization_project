// compare : résume data/experiments.csv par stratégie (moyenne, écart-type, prédit vs réel)
// et dessine une figure de comparaison (barres + barres d'erreur + valeur prédite).
//
// Compilation : go build -o bin/compare tools/compare/main.go
// Usage       : ./bin/compare data/experiments.csv data/comparaison.svg [strategie1 strategie2 ...]
// Sans liste de stratégies, toutes celles du fichier sont affichées (ordre d'apparition).
// Sortie texte : un tableau Markdown, à coller dans le rapport ou les slides.
package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type stat struct {
	name                       string
	mk, cost, predMk, predCost []float64
	failed                     int
}

func fail(msg string, a ...any) {
	fmt.Fprintf(os.Stderr, "Erreur : "+msg+"\n", a...)
	os.Exit(1)
}

func meanStd(v []float64) (float64, float64) {
	if len(v) == 0 {
		return math.NaN(), math.NaN()
	}
	m := 0.0
	for _, x := range v {
		m += x
	}
	m /= float64(len(v))
	if len(v) < 2 {
		return m, 0
	}
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return m, math.Sqrt(s / float64(len(v)-1))
}

func main() {
	if len(os.Args) < 3 {
		fail("usage : %s data/experiments.csv sortie.svg [strategies...]", os.Args[0])
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fail("%v", err)
	}
	rows, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil || len(rows) < 2 {
		fail("%s vide ou illisible", os.Args[1])
	}
	want := []string{"experiment_id", "strategy", "makespan_s", "cost_usd", "pred_makespan_s", "pred_cost_usd", "n_tasks", "n_failed"}
	for i, h := range want {
		if strings.TrimSpace(rows[0][i]) != h {
			fail("colonne %d attendue : %s", i+1, h)
		}
	}

	byName := map[string]*stat{}
	var order []string
	for _, r := range rows[1:] {
		if len(r) < 8 {
			continue
		}
		name := strings.TrimSpace(r[1])
		s, ok := byName[name]
		if !ok {
			s = &stat{name: name}
			byName[name] = s
			order = append(order, name)
		}
		nf, _ := strconv.Atoi(strings.TrimSpace(r[7]))
		if nf > 0 { // une expérience avec échec n'entre pas dans les moyennes, mais on la compte
			s.failed++
			continue
		}
		add := func(dst *[]float64, v string) {
			if x, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				*dst = append(*dst, x)
			}
		}
		add(&s.mk, r[2])
		add(&s.cost, r[3])
		add(&s.predMk, r[4])
		add(&s.predCost, r[5])
	}
	if len(os.Args) > 3 {
		order = os.Args[3:]
	}
	var sel []*stat
	for _, n := range order {
		s, ok := byName[n]
		if !ok {
			fail("stratégie %q absente de %s", n, os.Args[1])
		}
		sel = append(sel, s)
	}

	// --- Tableau Markdown
	base := sel[0]
	bMk, _ := meanStd(base.mk)
	bCost, _ := meanStd(base.cost)
	fmt.Println("| Stratégie | Runs OK | Échecs | Makespan réel (s) | Prédit (s) | Erreur modèle | Coût réel (USD) | vs " + base.name + " (temps / coût) |")
	fmt.Println("|---|---|---|---|---|---|---|---|")
	for _, s := range sel {
		m, sd := meanStd(s.mk)
		c, cs := meanStd(s.cost)
		p, _ := meanStd(s.predMk)
		errTxt := "NA"
		if !math.IsNaN(p) && m > 0 {
			errTxt = fmt.Sprintf("%+.1f %%", 100*(p-m)/m)
		}
		pTxt := "NA"
		if !math.IsNaN(p) {
			pTxt = fmt.Sprintf("%.1f", p)
		}
		fmt.Printf("| %s | %d | %d | %.1f +/- %.1f | %s | %s | %.5f +/- %.5f | %+.1f %% / %+.1f %% |\n",
			s.name, len(s.mk), s.failed, m, sd, pTxt, errTxt, c, cs, 100*(m-bMk)/bMk, 100*(c-bCost)/bCost)
	}

	drawSVG(os.Args[2], sel)
	fmt.Fprintf(os.Stderr, "\nFigure écrite : %s\n", os.Args[2])
}

func drawSVG(path string, sel []*stat) {
	const W, H = 940.0, 420.0
	palette := []string{"#c0392b", "#1f6fb2", "#e07b00", "#2e8b57", "#7d3c98", "#555555"}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" font-family="sans-serif" font-size="12">`, W, H)
	b.WriteString(`<rect width="100%" height="100%" fill="white"/>`)
	b.WriteString(`<text x="20" y="26" font-size="16" font-weight="bold">Baseline vs optimisé : moyenne +/- écart-type des répétitions, losange = prédiction du modèle</text>`)

	panel := func(x0 float64, title, unit string, get func(*stat) ([]float64, []float64), fmtv string) {
		const top, bottom, w = 70.0, 360.0, 380.0
		maxV := 0.0
		for _, s := range sel {
			real, pred := get(s)
			m, sd := meanStd(real)
			p, _ := meanStd(pred)
			for _, v := range []float64{m + sd, p} {
				if !math.IsNaN(v) {
					maxV = math.Max(maxV, v)
				}
			}
		}
		if maxV == 0 {
			maxV = 1
		}
		maxV *= 1.15
		Y := func(v float64) float64 { return bottom - v/maxV*(bottom-top) }
		fmt.Fprintf(&b, `<text x="%.0f" y="55" font-weight="bold">%s (%s)</text>`, x0, title, unit)
		fmt.Fprintf(&b, `<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#333"/>`, x0, bottom, x0+w, bottom)
		for k := 0; k <= 4; k++ {
			v := maxV * float64(k) / 4
			fmt.Fprintf(&b, `<line x1="%.0f" y1="%.1f" x2="%.0f" y2="%.1f" stroke="#eee"/>`, x0, Y(v), x0+w, Y(v))
			fmt.Fprintf(&b, `<text x="%.0f" y="%.1f" text-anchor="end" fill="#555" font-size="10">`+fmtv+`</text>`, x0-4, Y(v)+3, v)
		}
		bw := w / float64(len(sel)) * 0.55
		for i, s := range sel {
			real, pred := get(s)
			m, sd := meanStd(real)
			cx := x0 + w*(float64(i)+0.5)/float64(len(sel))
			col := palette[i%len(palette)]
			if !math.IsNaN(m) {
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" opacity="0.85"/>`, cx-bw/2, Y(m), bw, bottom-Y(m), col)
				fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#000"/>`, cx, Y(m-sd), cx, Y(m+sd))
				fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#000"/><line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#000"/>`,
					cx-6, Y(m-sd), cx+6, Y(m-sd), cx-6, Y(m+sd), cx+6, Y(m+sd))
				fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-weight="bold">`+fmtv+`</text>`, cx, Y(m+sd)-8, m)
			}
			if p, _ := meanStd(pred); !math.IsNaN(p) {
				fmt.Fprintf(&b, `<path d="M%.1f %.1f l7 7 l-7 7 l-7 -7 z" fill="white" stroke="#000" stroke-width="1.5"/>`, cx+bw/2+10, Y(p)-7)
			}
			fmt.Fprintf(&b, `<text x="%.1f" y="%.0f" text-anchor="middle">%s (n=%d)</text>`, cx, bottom+18, s.name, len(real))
		}
	}
	panel(70, "Makespan", "s", func(s *stat) ([]float64, []float64) { return s.mk, s.predMk }, "%.0f")
	panel(500, "Coût", "USD", func(s *stat) ([]float64, []float64) { return s.cost, s.predCost }, "%.4f")
	b.WriteString(`</svg>`)
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		fail("%v", err)
	}
}
