# Cloud Optimization — Distributed task scheduling on GCP

ESILV A5 semester project. **Given a set of tasks and a set of VMs, where should each task run?**

```
data/tasks.csv + data/machines.csv (+ benchmark data)
        │
        ▼
   scheduler (Go)  ──►  data/allocation_*.csv  ──►  scripts/run_all.sh  ──►  VM1 / VM2 / VM3
                                                                                 │
                                                                                 ▼
                                                                   results/T*.out / .err / .status
```

| Component | Role | Files |
|---|---|---|
| Workloads | compiled Go programs with different resource profiles | `workloads/` |
| Infrastructure | 3 Compute Engine VMs | see below |
| Scheduler | decides where each task runs, writes an allocation CSV | `scheduler/` |
| Orchestrator | launches every task remotely and in parallel, collects outputs | `scripts/run_task.sh`, `scripts/run_all.sh` |

## Infrastructure

| Item | Value |
|---|---|
| Project ID | `cloud-optimization-511007` |
| Zone | `europe-west1-b` |
| VMs | `cloudopt-vm1`, `cloudopt-vm2`, `cloudopt-vm3` |
| Machine type | `e2-medium` (2 shared-core vCPUs, 4 GB RAM) |
| Image / disk | `ubuntu-2404-lts-amd64`, 20 GB `pd-balanced` |
| Controller | Linux (Ubuntu 24.04 under WSL2), Go 1.22, Google Cloud CLI |

## Repository layout

```
workloads/          reference workloads: cpu_task, memory_task, matrix_task, sort_task
scheduler/          round_robin.go — Round-Robin baseline scheduler
scripts/
  benchmark.sh            runs one workload N times, appends raw rows to a CSV
  benchmark_campaign.sh   full campaign: 4 workloads × 3 sizes × N runs
  summarize.sh            mean / std / min / max / RSS per configuration
  run_task.sh             runs one task on one VM through gcloud compute ssh
  run_all.sh              runs every task of an allocation in parallel (& + single wait)
  test_scheduler.sh       scheduler test cases, no VM needed
data/
  machine_local.txt, machine_cloud_vm1.txt         machine descriptions
  benchmark_local.csv, benchmark_cloud_vm1.csv     raw benchmark runs
  tasks.csv, machines.csv                          scheduler inputs
  allocation_manual.csv, allocation_manual_v2.csv  hand-written allocations
  allocation_rr.csv                                Round-Robin allocation (generated)
docs/               TD1–TD4 notes, results and answers
```

## Quick start

Requirements: Linux, Go, GNU time (`sudo apt install golang-go time`), and the Google Cloud CLI
authenticated on the project.

```bash
# 1. Build
mkdir -p bin
for w in cpu_task memory_task matrix_task sort_task; do go build -o bin/$w workloads/$w.go; done
go build -o bin/round_robin_scheduler scheduler/round_robin.go

# 2. Benchmark locally
./scripts/benchmark_campaign.sh data/benchmark_local.csv 3
./scripts/summarize.sh data/benchmark_local.csv

# 3. Start the VMs and deploy the binaries
gcloud config set compute/zone europe-west1-b
gcloud compute instances start cloudopt-vm1 cloudopt-vm2 cloudopt-vm3
for vm in cloudopt-vm1 cloudopt-vm2 cloudopt-vm3; do
  gcloud compute ssh $vm --command="mkdir -p ~/cloud-optimization"
  gcloud compute scp --recurse bin $vm:~/cloud-optimization/
done

# 4. Schedule and run
./scripts/test_scheduler.sh
./bin/round_robin_scheduler data/tasks.csv data/machines.csv data/allocation_rr.csv
./scripts/run_all.sh data/allocation_rr.csv

# 5. Stop the VMs when finished
gcloud compute instances stop cloudopt-vm1 cloudopt-vm2 cloudopt-vm3
```

## Progress

| TD | Topic | Status | Notes |
|---|---|---|---|
| 1 | Local workloads and benchmarking | ✅ | [docs/TD1.md](docs/TD1.md) |
| 2 | First Compute Engine deployment | ✅ | [docs/TD2.md](docs/TD2.md) |
| 3 | Distributed orchestration | ✅ | [docs/TD3.md](docs/TD3.md) |
| 4 | Round-Robin scheduler (baseline) | ✅ | [docs/TD4.md](docs/TD4.md) |
| 5 | Optimized scheduler, performance/cost trade-offs | ⏳ | |
| 6 | Final evaluation | ⏳ | |
