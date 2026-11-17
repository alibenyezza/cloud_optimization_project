package model

import "testing"

// Cas fait à la main (d = sshDelay = 3,5 s, attente sans CPU) :
// VM a : X (20 s), Y (20 s), A (10 s) ; VM b : B (10 s) qui attend A.
//
// Ordre de tasks.csv (X, Y, A, B) : X et Y prennent les 2 places de a, partagent le coeur
// et finissent à d+40. A part ensuite (fin 2d+50), puis B sur b (fin 3d+60 = 70,5 s).
//
// Ordre chemin restant (A, X, Y, B) : A part tout de suite avec X et finit à d+20.
// B part alors sur b (fin 2d+30). Sur a, X finit à 40 et Y à d+50 = 53,5 s.
// Gain : 17 s, juste en changeant l'ordre, sans changer de VM.
func TestCheminRestant(t *testing.T) {
	p := tiny([]float64{20, 20, 10, 10}, []float64{1, 1, 1, 1}, [][]int{nil, nil, nil, {2}})
	d := sshDelay

	rank := p.RemainingPath()
	near(t, "chemin restant de A (A puis B)", rank[2], (10+d)+(10+d))
	near(t, "chemin restant de X", rank[0], 20+d)

	order := p.PriorityOrder()
	want := []int{2, 0, 1, 3}
	for k := range want {
		if order[k] != want[k] {
			t.Fatalf("ordre = %v, attendu %v (A, X, Y, B)", order, want)
		}
	}

	alloc := []int{0, 0, 0, 1}
	near(t, "makespan, ordre de tasks.csv", p.Simulate(alloc, Identity(4)).Makespan, 3*d+60)
	near(t, "makespan, ordre chemin restant", p.Simulate(alloc, order).Makespan, d+50)
}

// Sans dépendances : chemin restant = durée seule, donc on retrouve LPT (la plus longue d'abord)
func TestSansDependancesLPT(t *testing.T) {
	p := tiny([]float64{5, 30, 10}, []float64{1, 1, 1}, [][]int{nil, nil, nil})
	order := p.PriorityOrder()
	want := []int{1, 2, 0}
	for k := range want {
		if order[k] != want[k] {
			t.Fatalf("ordre = %v, attendu %v", order, want)
		}
	}
}
