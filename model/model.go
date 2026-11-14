// Package model : le modèle commun de prédiction (docs/INTERFACES.md), utilisé par
// tools/predict et scheduler/nsga2.go pour qu'ils calculent exactement la même chose.
//
// Le modèle simule les règles de l'orchestrateur en mode file et en mode dépendances :
//  1. une tâche part quand toutes ses dépendances sont finies, même sur une autre VM ;
//  2. au plus vcpu tâches en même temps par VM ;
//  3. quand une place se libère, on lance la première tâche prête dans l'ordre de l'allocation.
//
// Vitesse : une VM à 2 vCPU n'a qu'un seul coeur physique. Avec k tâches de calcul en même
// temps, chacune avance à htGain/k de sa vitesse seule. Avec htGain = 1, une VM sans
// dépendances finit à la somme des temps de ses tâches (validé : +3 % sur Round-Robin).
package model

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const (
	htGain   = 1.0 // débit d'une VM avec 2 tâches, par rapport à 1 tâche (1 = pas de gain)
	sshDelay = 3.5 // mode dépendances : une connexion SSH par tâche (~3 s) + détection de fin (jusqu'à 1 s)
	memRatio = 0.9 // on garde 10 % de la RAM pour le système
)

type Task struct {
	ID, Workload, Size string
	Deps               []int // indices des tâches à attendre
}

type VM struct {
	Name, Type   string
	VCPU         int
	RAMMB, Price float64
	Setup        float64 // préparation (création + SSH + déploiement), payée aussi
}

type Problem struct {
	Tasks   []Task
	VMs     []VM
	RT      [][]float64 // RT[i][v]  : temps médian de la tâche i seule sur la VM v (s)
	RSS     [][]float64 // RSS[i][v] : mémoire max de la tâche i sur la VM v (Mo)
	HasDeps bool        // au moins une dépendance : mode dépendances (SSH par tâche)
}

type Result struct {
	Makespan, Cost float64
	Violation      float64   // Mo au-dessus de 90 % de la RAM au pic (0 = valide)
	NVMs           int       // VMs utilisées
	Start, Finish  []float64 // par tâche
	End, PeakMem   []float64 // par VM
	Count          []int     // tâches par VM
}

func readCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil || len(recs) < 2 {
		return nil, fmt.Errorf("%s vide ou illisible", path)
	}
	for _, rec := range recs {
		for i := range rec {
			rec[i] = strings.TrimSpace(rec[i])
		}
	}
	return recs[1:], nil
}

func num(s, what string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%s invalide : %q", what, s)
	}
	return v, nil
}

// Load lit tasks.csv (depends_on optionnel), machines.csv, profiles.csv et vm_setup.csv
func Load(tasksPath, machinesPath, profilesPath, setupPath string) (*Problem, error) {
	p := &Problem{}

	// Préparation moyenne par machine_type ; un type jamais mesuré prend la moyenne de tout
	rows, err := readCSV(setupPath)
	if err != nil {
		return nil, err
	}
	setupSum, setupN := map[string]float64{}, map[string]float64{}
	all := 0.0
	for _, r := range rows {
		t, err := num(r[6], "total_s")
		if err != nil {
			return nil, err
		}
		setupSum[r[2]] += t
		setupN[r[2]]++
		all += t
	}

	if rows, err = readCSV(machinesPath); err != nil {
		return nil, err
	}
	for _, r := range rows {
		vcpu, err1 := num(r[2], "vcpu")
		ram, err2 := num(r[3], "ram_mb")
		price, err3 := num(r[4], "price_per_hour_usd")
		for _, e := range []error{err1, err2, err3} {
			if e != nil {
				return nil, e
			}
		}
		if vcpu < 1 {
			return nil, fmt.Errorf("%s : vcpu doit être >= 1", r[0])
		}
		vm := VM{Name: r[0], Type: r[1], VCPU: int(vcpu), RAMMB: ram, Price: price, Setup: all / sumValues(setupN)}
		if setupN[vm.Type] > 0 {
			vm.Setup = setupSum[vm.Type] / setupN[vm.Type]
		}
		p.VMs = append(p.VMs, vm)
	}

	type prof struct{ rt, rss float64 }
	profiles := map[string]prof{}
	if rows, err = readCSV(profilesPath); err != nil {
		return nil, err
	}
	for _, r := range rows {
		rt, err1 := num(r[5], "median_runtime_s")
		rss, err2 := num(r[7], "max_rss_kb")
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("ligne invalide dans %s : %v", profilesPath, r)
		}
		profiles[r[0]+","+r[1]+","+r[2]] = prof{rt, rss / 1024}
	}

	if rows, err = readCSV(tasksPath); err != nil {
		return nil, err
	}
	index := map[string]int{}
	var depText []string
	for _, r := range rows {
		if _, dup := index[r[0]]; dup {
			return nil, fmt.Errorf("tâche %s en double dans %s", r[0], tasksPath)
		}
		index[r[0]] = len(p.Tasks)
		p.Tasks = append(p.Tasks, Task{ID: r[0], Workload: r[1], Size: r[2]})
		d := ""
		if len(r) > 3 {
			d = r[3]
		}
		depText = append(depText, d)
		rowT := make([]float64, len(p.VMs))
		rowM := make([]float64, len(p.VMs))
		for v, vm := range p.VMs {
			pr, ok := profiles[r[1]+","+r[2]+","+vm.Type]
			if !ok {
				return nil, fmt.Errorf("pas de mesure pour %s %s sur %s dans %s", r[1], r[2], vm.Type, profilesPath)
			}
			rowT[v], rowM[v] = pr.rt, pr.rss
		}
		p.RT = append(p.RT, rowT)
		p.RSS = append(p.RSS, rowM)
	}
	for i, d := range depText {
		for _, id := range strings.Split(d, ";") {
			if id = strings.TrimSpace(id); id == "" {
				continue
			}
			j, ok := index[id]
			if !ok {
				return nil, fmt.Errorf("%s dépend de %s, absente de %s", p.Tasks[i].ID, id, tasksPath)
			}
			p.Tasks[i].Deps = append(p.Tasks[i].Deps, j)
			p.HasDeps = true
		}
	}
	if err := p.checkCycle(); err != nil {
		return nil, err
	}
	return p, nil
}

func sumValues(m map[string]float64) float64 {
	s := 0.0
	for _, v := range m {
		s += v
	}
	return s
}

// checkCycle : un cycle bloquerait l'exécution (l'orchestrateur le refuse aussi)
func (p *Problem) checkCycle() error {
	state := make([]int, len(p.Tasks)) // 0 pas vue, 1 en cours, 2 finie
	var visit func(i int) bool
	visit = func(i int) bool {
		state[i] = 1
		for _, j := range p.Tasks[i].Deps {
			if state[j] == 1 || (state[j] == 0 && !visit(j)) {
				return false
			}
		}
		state[i] = 2
		return true
	}
	for i := range p.Tasks {
		if state[i] == 0 && !visit(i) {
			return fmt.Errorf("cycle dans les dépendances autour de %s", p.Tasks[i].ID)
		}
	}
	return nil
}

// Simulate : alloc[i] = indice de la VM de la tâche i ; order = indices des tâches dans
// l'ordre du fichier d'allocation (priorité quand une place se libère).
func (p *Problem) Simulate(alloc, order []int) Result {
	n, nv := len(p.Tasks), len(p.VMs)
	res := Result{Start: make([]float64, n), Finish: make([]float64, n),
		End: make([]float64, nv), PeakMem: make([]float64, nv), Count: make([]int, nv)}

	delay := 0.0
	if p.HasDeps {
		delay = sshDelay
	}
	work := make([]float64, n) // calcul restant (en secondes "seule")
	wait := make([]float64, n) // attente SSH restante, ne prend pas le CPU
	state := make([]int8, n)   // 0 pas lancée, 1 en cours, 2 finie
	running := make([]int, nv) // tâches lancées par VM
	mem := make([]float64, nv) // mémoire des tâches en cours
	for i := range p.Tasks {
		work[i], wait[i] = p.RT[i][alloc[i]], delay
		res.Count[alloc[i]]++
	}

	t, done := 0.0, 0
	cpu := make([]int, nv)
	for done < n {
		// Règles 1 à 3 : lancer les tâches prêtes, dans l'ordre, tant qu'il y a de la place
		for _, i := range order {
			v := alloc[i]
			if state[i] != 0 || running[v] >= p.VMs[v].VCPU || !p.ready(i, state) {
				continue
			}
			state[i] = 1
			running[v]++
			res.Start[i] = t
			mem[v] += p.RSS[i][alloc[i]]
			res.PeakMem[v] = math.Max(res.PeakMem[v], mem[v])
		}
		// Vitesse de chaque tâche : le coeur est partagé entre les tâches qui calculent
		for v := range cpu {
			cpu[v] = 0
		}
		for i := range p.Tasks {
			if state[i] == 1 && wait[i] <= 0 {
				cpu[alloc[i]]++
			}
		}
		rate := func(i int) float64 {
			if wait[i] > 0 || cpu[alloc[i]] <= 1 {
				return 1
			}
			return htGain / float64(cpu[alloc[i]])
		}
		// Avancer jusqu'au prochain événement (fin d'attente SSH ou fin de tâche)
		dt := math.Inf(1)
		for i := range p.Tasks {
			if state[i] != 1 {
				continue
			}
			if wait[i] > 0 {
				dt = math.Min(dt, wait[i])
			} else {
				dt = math.Min(dt, work[i]/rate(i))
			}
		}
		if math.IsInf(dt, 1) { // impossible sans cycle (vérifié dans Load)
			panic("simulation bloquée : aucune tâche ne peut partir")
		}
		for i := range p.Tasks {
			if state[i] != 1 {
				continue
			}
			if wait[i] > 0 {
				if wait[i] -= dt; wait[i] < 1e-9 {
					wait[i] = 0
				}
				continue
			}
			if work[i] -= rate(i) * dt; work[i] < 1e-9 {
				state[i] = 2
				done++
				v := alloc[i]
				running[v]--
				mem[v] -= p.RSS[i][v]
				res.Finish[i] = t + dt
				res.End[v] = math.Max(res.End[v], t+dt)
			}
		}
		t += dt
	}

	for v, vm := range p.VMs {
		res.Makespan = math.Max(res.Makespan, res.End[v])
		if limit := memRatio * vm.RAMMB; res.PeakMem[v] > limit {
			res.Violation += res.PeakMem[v] - limit
		}
	}
	// Coût : une VM utilisée est payée de sa création jusqu'à la fin, une VM vide n'est pas créée
	for v, vm := range p.VMs {
		if res.Count[v] > 0 {
			res.Cost += vm.Price / 3600 * (vm.Setup + res.Makespan)
			res.NVMs++
		}
	}
	return res
}

func (p *Problem) ready(i int, state []int8) bool {
	for _, j := range p.Tasks[i].Deps {
		if state[j] != 2 {
			return false
		}
	}
	return true
}

// Identity : l'ordre de tasks.csv (celui qu'écrivent nsga2 et round_robin)
func Identity(n int) []int {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	return order
}

// MemLimit : mémoire utilisable d'une VM (Mo)
func (vm VM) MemLimit() float64 { return memRatio * vm.RAMMB }
