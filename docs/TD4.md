# TD4 — Round-Robin scheduler (baseline)

The **scheduler** decides *where* each task runs and only writes a CSV. The **orchestrator**
(`run_all.sh`, unchanged since TD3) executes it.

```bash
go build -o bin/round_robin_scheduler scheduler/round_robin.go
./scripts/test_scheduler.sh
./bin/round_robin_scheduler data/tasks.csv data/machines.csv data/allocation_rr.csv
./scripts/run_all.sh data/allocation_rr.csv
```

## What each part did

- **Part 0:** we separated the roles: the scheduler decides where each task runs, and the unchanged TD3
  orchestrator executes it.
- **Part 1:** we split `allocation_manual.csv` into `tasks.csv` (what to run) and `machines.csv` (the available VMs).
- **Part 2:** we studied the Round-Robin rule: task `i` goes to VM `i % number_of_VMs`, regardless of the task size
  or the VM capacity.
- **Part 3:** we compiled the Go scheduler and generated `allocation_rr.csv` in the TD3 format.
- **Part 4:** we checked the output, then ran test cases A to E and the task-order test.
- **Part 5:** `run_all.sh` now takes the allocation file as an argument
  (`INPUT="${1:-data/allocation_manual.csv}"`), and we ran `allocation_rr.csv` on the 3 VMs.
- **Part 6:** we explained why Round-Robin is not an optimization method.

## Generated allocation (`data/allocation_rr.csv`)

| VM | Tasks |
|---|---|
| cloudopt-vm1 | T1 cpu 500000 · T4 matrix 400 · T7 cpu 500000 |
| cloudopt-vm2 | T2 cpu 1000000 · T5 memory 512 · T8 memory 256 |
| cloudopt-vm3 | T3 memory 256 · T6 matrix 600 · T9 matrix 400 |

## Tests (`scripts/test_scheduler.sh`, 7/7 passed)

| Case | Input | Result |
|---|---|---|
| A | 6 tasks, 3 VMs | 2 tasks per VM |
| B | 5 tasks, 2 VMs | vm1 vm2 vm1 vm2 vm1 (3 and 2) |
| C | 2 tasks, 3 VMs | vm3 gets no task |
| D | 5 tasks, 0 VMs | `Error: no VMs available`, no file written |
| E | duplicate ID | `Error: duplicate task ID "T1"` |
| Order | reversed `tasks.csv` | T1 moves from vm1 to vm3 |
| Determinism | same input twice | identical output |

## Execution

- `./scripts/run_all.sh data/allocation_rr.csv` → **9/9 tasks finished, 0 failed** (about 6 s). Every `.err` was empty.
- Failure test: `run_task.sh X1 fake_task 10 cloudopt-vm1 europe-west1-b` is reported as `FAILED` (exit code 127).

## Answers

**2.2 — Before programming**
1. With 10 tasks and 3 VMs, vm1 gets 4 (T1, T4, T7, T10), and vm2 and vm3 get 3 each.
2. The number of tasks per VM is the same within one task. It is exactly equal only when the number of tasks is a multiple of the number of VMs.
3. No. Round-Robin uses only the position of a task in the file, never its size or workload.
4. Yes. Each task is launched in the background (`&`) with its own SSH connection, and the single `wait` comes after the
   loop. Tasks on the same VM therefore share its CPU and RAM.

**3.3 — Before proceeding**
- The circular assignment is the line `vm := vmNames[i%len(vmNames)]`.
- With 2 VMs, the index sequence becomes 0, 1, 0, 1… With 9 tasks, vm1 gets 5 and vm2 gets 4. The code does not change.
- The output uses the TD3 header `task_id,workload,size,vm`, so `run_all.sh` runs it unchanged.

**4.1 — Does changing the task order keep the same assignments?**
No. The assignment depends on the position of the task, not on its ID: in the reversed file, T1 moves from vm1 to vm3.
The output is still deterministic for the same input order.

**Part 6 — Discussion**
1. Two heavy CPU tasks on one VM share its 2 shared-core vCPUs. Both slow down and the last task finishes later,
   while another VM may be idle.
2. No. `machines.csv` only lists names, so a small VM would get as many tasks as a large one.
3. For each task: its estimated runtime and peak memory (`runtime_s` and `max_rss_kb` in
   `data/benchmark_cloud_vm1.csv`). For each VM: its vCPUs, RAM, relative speed and hourly price.
4. Candidate objective, to be confirmed by the team: minimize the makespan (the finish time of the last task)
   without exceeding the RAM of any VM.

**Limitation.** Round-Robin balances the **number** of tasks, not the **load**. Using the cloud benchmark means as
estimates, the work per VM is very uneven:
- vm2 gets 0.38 + 0.82 + 0.42 = 1.62 s (cpu 1M, memory 512, memory 256);
- vm1 gets 0.15 + 0.10 + 0.15 = 0.40 s;
- vm3 gets 0.42 + 0.32 + 0.10 = 0.84 s.

vm2 also holds both memory tasks, which compete for its memory bandwidth. During the run, the same
`memory_task 256` took 0.88 s on vm2 versus 0.47 s on vm3. To overcome this, the scheduler needs each task's estimated
demand (from the benchmarks) and each VM's capacity (vCPU and RAM columns in `machines.csv`). It could then place each
task on the least loaded VM that still has enough memory.
