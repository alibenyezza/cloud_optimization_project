// Priorité des tâches : le "chemin restant" (upward rank, la priorité de HEFT).
// Nouveau fichier du package model : il ne change rien dans model.go.
//
// Chemin restant d'une tâche = sa durée + la plus longue chaîne de tâches qui dépendent d'elle.
// Une tâche qui bloque une longue chaîne doit partir tôt : sinon toute la chaîne attend.
// Sans dépendances, le chemin restant est la durée seule : on retrouve LPT (CM3).
package model

import "sort"

// taskWeight : durée moyenne de la tâche i sur les VMs (on ne sait pas encore où elle ira),
// plus l'attente SSH en mode dépendances
func (p *Problem) taskWeight(i int) float64 {
	s := 0.0
	for _, t := range p.RT[i] {
		s += t
	}
	w := s / float64(len(p.RT[i]))
	if p.HasDeps {
		w += sshDelay
	}
	return w
}

// RemainingPath : chemin restant de chaque tâche (s)
func (p *Problem) RemainingPath() []float64 {
	n := len(p.Tasks)
	succ := make([][]int, n) // succ[j] : tâches qui attendent j
	for i, t := range p.Tasks {
		for _, j := range t.Deps {
			succ[j] = append(succ[j], i)
		}
	}
	rank := make([]float64, n)
	done := make([]bool, n)
	var visit func(i int) float64
	visit = func(i int) float64 { // pas de cycle : vérifié dans Load
		if done[i] {
			return rank[i]
		}
		longest := 0.0
		for _, k := range succ[i] {
			if r := visit(k); r > longest {
				longest = r
			}
		}
		rank[i], done[i] = p.taskWeight(i)+longest, true
		return rank[i]
	}
	for i := range p.Tasks {
		visit(i)
	}
	return rank
}

// PriorityOrder : les tâches du plus long au plus court chemin restant
// (à égalité, l'ordre de tasks.csv). Une tâche passe toujours avant celles qui l'attendent.
func (p *Problem) PriorityOrder() []int {
	rank := p.RemainingPath()
	order := Identity(len(p.Tasks))
	sort.SliceStable(order, func(a, b int) bool { return rank[order[a]] > rank[order[b]] })
	return order
}
