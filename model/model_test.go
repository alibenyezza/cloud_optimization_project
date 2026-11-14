package model

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// Petit problème fait à la main : 2 VMs à 2 vCPU, tâches de durée connue
func tiny(times []float64, rss []float64, deps [][]int) *Problem {
	p := &Problem{VMs: []VM{
		{Name: "a", Type: "t", VCPU: 2, RAMMB: 10000, Price: 3600, Setup: 0},
		{Name: "b", Type: "t", VCPU: 2, RAMMB: 10000, Price: 3600, Setup: 0},
	}}
	for i, t := range times {
		p.Tasks = append(p.Tasks, Task{ID: string(rune('A' + i)), Deps: deps[i]})
		p.RT = append(p.RT, []float64{t, t})
		p.RSS = append(p.RSS, []float64{rss[i], rss[i]})
		if len(deps[i]) > 0 {
			p.HasDeps = true
		}
	}
	return p
}

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %.4f, attendu %.4f", what, got, want)
	}
}

// Sans dépendances : une VM finit à la somme de ses tâches, même avec 2 places (1 seul coeur)
func TestSommeSansDependances(t *testing.T) {
	p := tiny([]float64{10, 20, 30}, []float64{1, 1, 1}, [][]int{nil, nil, nil})
	r := p.Simulate([]int{0, 0, 0}, Identity(3))
	near(t, "fin de la VM a", r.End[0], 60)
	near(t, "makespan", r.Makespan, 60)
	near(t, "coût (prix 3600 $/h = 1 $/s)", r.Cost, 60)
	if r.NVMs != 1 {
		t.Errorf("VMs utilisées = %d, attendu 1", r.NVMs)
	}
}

// Avec dépendance entre VMs : B attend la fin de A, chaque tâche paie le délai SSH
func TestDependanceEntreVMs(t *testing.T) {
	p := tiny([]float64{10, 5}, []float64{1, 1}, [][]int{nil, {0}})
	r := p.Simulate([]int{0, 1}, Identity(2))
	near(t, "fin de A", r.Finish[0], sshDelay+10)
	near(t, "début de B", r.Start[1], sshDelay+10)
	near(t, "makespan", r.Makespan, 2*sshDelay+15)
	near(t, "coût (2 VMs payées jusqu'à la fin)", r.Cost, 2*(2*sshDelay+15))
}

// Mémoire au pic : au plus 2 tâches en même temps, la 3e attend une place
func TestMemoireAuPic(t *testing.T) {
	p := tiny([]float64{10, 10, 10}, []float64{1000, 1000, 1000}, [][]int{nil, nil, nil})
	r := p.Simulate([]int{0, 0, 0}, Identity(3))
	near(t, "pic mémoire", r.PeakMem[0], 2000)
	near(t, "début de la 3e tâche", r.Start[2], 20)
}

// L'ordre de l'allocation décide qui part en premier quand une place se libère
func TestOrdreDePriorite(t *testing.T) {
	p := tiny([]float64{10, 10, 10}, []float64{1, 1, 1}, [][]int{nil, nil, nil})
	r := p.Simulate([]int{0, 0, 0}, []int{2, 1, 0})
	near(t, "début de C (1re dans l'ordre)", r.Start[2], 0)
	near(t, "début de A (dernière)", r.Start[0], 20)
}

func TestCycleRefuse(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	tasks := write("tasks.csv", "task_id,workload,size,depends_on\nT1,w,1,T2\nT2,w,1,T1\n")
	machines := write("machines.csv", "vm,machine_type,vcpu,ram_mb,price_per_hour_usd\nv,t,2,1000,1\n")
	profiles := write("profiles.csv", "workload,size,machine_type,runs,mean,median,std,max_rss_kb\nw,1,t,1,1,1,0,1024\n")
	setup := write("setup.csv", "date,vm,machine_type,create_s,ssh_ready_s,deploy_s,total_s\nd,v,t,1,1,1,3\n")
	if _, err := Load(tasks, machines, profiles, setup); err == nil {
		t.Error("un cycle T1 -> T2 -> T1 aurait dû être refusé")
	}
}
