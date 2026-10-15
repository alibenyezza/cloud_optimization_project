# TD3 — Distributed orchestration

## Workers

Three identical VMs in `europe-west1-b`: `cloudopt-vm1`, `cloudopt-vm2` and `cloudopt-vm3`
(`e2-medium`, Ubuntu 24.04, 20 GB `pd-balanced`). Each VM has the binaries in `~/cloud-optimization/bin/`:

```bash
gcloud compute scp --recurse bin cloudopt-vm1:~/cloud-optimization/ --zone=europe-west1-b
gcloud compute ssh cloudopt-vm1 --zone=europe-west1-b --command="ls -lh ~/cloud-optimization/bin"
```

## Scripts

- `scripts/run_task.sh TASK_ID WORKLOAD SIZE VM ZONE` runs **one** task on **one** VM through `gcloud compute ssh`:
  - it checks that all 5 parameters are present, and that the workload is a binary name and the size an integer;
  - stdout and stderr are redirected directly on the controller to `results/TASK_ID.out` and `results/TASK_ID.err`;
  - `results/TASK_ID.status` stores `task_id,vm,exit_code`, and the script returns the remote exit code.
- `scripts/run_all.sh [ALLOCATION]` reads the allocation file (default `data/allocation_manual.csv`):
  - it checks the header and skips it;
  - it launches one `run_task.sh` per row **in the background (`&`)**;
  - it runs **a single `wait`** after the loop;
  - it prints `All tasks completed: N/N finished, F failed.` and returns 1 if any task failed.
- Several tasks assigned to the same VM run concurrently: each task has its own SSH connection, and there is no per-VM queue.
- The stdin of `gcloud compute ssh` is redirected to `/dev/null`. Otherwise ssh would read the next CSV rows,
  and the loop would silently skip tasks.

## Allocations and runs

| File | Placement | Result |
|---|---|---|
| `data/allocation_manual.csv` | vm1: T1 T4 T7 T8 · vm2: T2 T5 T9 · vm3: T3 T6 | 9/9 OK, about 4 s |
| `data/allocation_manual_v2.csv` (Part 11) | vm1: T1–T6 · vm2: T7 T8 · vm3: T9 | 9/9 OK, about 7 s |

For Part 11, only the `vm` column changed: the `task_id,workload,size` columns of both files are identical,
which was checked with `diff`. No script was modified.

## Validation (Part 10)

1. Each task was launched exactly once: one `launched Tx` line and one `.status` file per row.
2. Each task ran on the VM given in the allocation: the VM name is recorded in its `.status` file.
3. Each task produced its own `results/Tx.out` (workload JSON) and `results/Tx.err`, which is empty on success.
4. Errors stay visible. A missing binary gives exit code 127, and `No such file or directory` is kept in the `.err` file.
5. Failures are counted: `run_all.sh` returns 1 if any task failed.
6. `run_all.sh` returns only after `wait`, that is, after all background tasks have finished.
