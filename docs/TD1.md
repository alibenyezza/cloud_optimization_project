# TD1 — Local workloads and benchmarking

## Local machine (`data/machine_local.txt`)

| Item | Value |
|---|---|
| OS | Ubuntu 24.04.3 LTS under WSL2 (kernel 6.6.87.2-microsoft-standard-WSL2), Windows 11 host |
| CPU | Intel Core Ultra 7 155H, x86_64 |
| Logical CPUs (as seen by WSL) | 16 (8 cores × 2 threads) |
| RAM (as seen by WSL) | 15 GiB |
| Storage | 1 TB virtual disk |
| Go | go1.22.2 linux/amd64 |

## Setup

```bash
sudo apt install -y git make build-essential time golang-go
mkdir -p ~/cloud-optimization/{workloads,bin,data,results,scripts}
go build -o bin/cpu_task workloads/cpu_task.go
go build -o bin/memory_task workloads/memory_task.go
go build -o bin/matrix_task workloads/matrix_task.go
go build -o bin/sort_task workloads/sort_task.go
```

## Benchmark runner

- `scripts/benchmark.sh WORKLOAD SIZE RUNS [CSV]` runs one workload `RUNS` times and appends one row per run:
  `workload,size,run,runtime_s,max_rss_kb`.
  - `runtime_s` is the wall-clock time measured around the process.
  - `max_rss_kb` is the peak resident memory reported by GNU time (`%M`).
- The script stops with a clear error if the binary is missing, if the size or run count is not a valid integer,
  if GNU time is not installed, or if a run fails.
- `scripts/benchmark_campaign.sh [CSV] [RUNS]` runs the full campaign: 4 workloads × 3 sizes × 3 runs.
- `scripts/summarize.sh CSV` computes the per-configuration statistics shown below.

```bash
./scripts/benchmark_campaign.sh data/benchmark_local.csv 3
./scripts/summarize.sh data/benchmark_local.csv
```

The suggested sizes were kept so that local and cloud results can be compared directly.
The smallest runs (about 20 ms) are short but still measurable with nanosecond timestamps.

## Results (`data/benchmark_local.csv`, 36 raw runs)

| workload | size | runs | mean_s | std_s | min_s | max_s | max_rss_mb |
|---|---:|---:|---:|---:|---:|---:|---:|
| cpu_task | 200000 | 3 | 0.021 | 0.001 | 0.020 | 0.022 | 2.0 |
| cpu_task | 500000 | 3 | 0.059 | 0.002 | 0.057 | 0.062 | 2.0 |
| cpu_task | 1000000 | 3 | 0.146 | 0.005 | 0.140 | 0.151 | 1.9 |
| memory_task | 128 | 3 | 0.137 | 0.001 | 0.136 | 0.138 | 130.3 |
| memory_task | 256 | 3 | 0.271 | 0.008 | 0.260 | 0.280 | 258.4 |
| memory_task | 512 | 3 | 0.613 | 0.083 | 0.522 | 0.724 | 515.2 |
| matrix_task | 200 | 3 | 0.016 | 0.002 | 0.014 | 0.019 | 3.0 |
| matrix_task | 400 | 3 | 0.076 | 0.001 | 0.074 | 0.076 | 5.8 |
| matrix_task | 600 | 3 | 0.224 | 0.016 | 0.203 | 0.242 | 10.5 |
| sort_task | 1000000 | 3 | 0.135 | 0.005 | 0.131 | 0.143 | 10.0 |
| sort_task | 5000000 | 3 | 0.696 | 0.026 | 0.660 | 0.716 | 40.4 |
| sort_task | 10000000 | 3 | 1.434 | 0.074 | 1.329 | 1.491 | 78.7 |

Single measurements with `/usr/bin/time -v`:

| Command | Wall | User | System | Max RSS |
|---|---:|---:|---:|---:|
| `cpu_task --size 500000` | 0.05 s | 0.06 s | 0.00 s | 2 MB |
| `memory_task --size 256` | 0.35 s | 0.14 s | 0.24 s | 259 MB |

## Answers

**Part 1**
1. A core is a physical execution unit. A logical CPU is a hardware thread that the OS can schedule;
   with hyper-threading, one core exposes 2 logical CPUs. WSL shows 8 cores × 2 = 16 logical CPUs.
2. 15 GiB are available to WSL.
3. Runtimes depend on the CPU, the memory, the OS and the background load, so a result is only
   comparable if the machine that produced it is known.

**Part 2**
1. `go build` produces a native, statically linked ELF x86-64 executable.
2. The `.go` file is human-readable source code. The executable is machine code for one OS and one CPU architecture.
3. A native executable needs no compiler or runtime on the target, starts instantly, and is deployed by copying one file.
4. Yes. The binary is self-contained, so the source is not needed to run it.

**Part 3**
1. `cpu_task`: pure computation with about 2 MB of memory.
2. `memory_task`: its resident memory grows with `--size` (about 515 MB for 512).
3. `--size` is the largest integer tested for `cpu_task`, the number of MB allocated for `memory_task`,
   the dimension N of the N×N matrices for `matrix_task`, and the number of integers for `sort_task`.
4. The problem size can be changed without recompiling, so experiments can be scripted.

**Part 4**
1. Wall-clock time is the real elapsed time. CPU time (user + system) is the time the CPU actually spent on
   the process. `memory_task` spends 0.24 s in system time, which is the kernel allocating memory pages.
2. `memory_task` has the largest maximum resident set size.
3. OS scheduling, other processes, CPU frequency changes, caches and page faults all vary from one run to the next.

**Part 5**
1. Automation gives the same procedure every time, with no typing mistakes, and makes the experiment reproducible.
2. Raw rows keep every measurement: means, variability and outliers are computed later.
3. Other heavy applications compete for the CPU and memory, which makes runtimes longer and more variable.
4. A missing executable, an invalid size or run count, a missing GNU time, and a workload that fails (non-zero exit code).

**Part 7**
1. `sort_task` with 10,000,000 integers is the slowest (1.43 s).
2. `memory_task` with 512 MB uses the most memory (515 MB RSS).
3. Doubling the input does not always double the runtime:
   - `cpu_task` 500k→1M gives ×2.5 (each number costs more to test);
   - `memory_task` 256→512 gives ×2.3 (roughly linear);
   - `sort_task` 5M→10M gives ×2.06 (n log n);
   - `matrix_task` 200→400 gives ×4.75 (cubic, with overhead on small sizes).
4. `matrix_task` shows the strongest growth, because its runtime grows as O(N³).
5. No. The repetitions vary, by up to ±0.08 s (14 %) for `memory_task` 512. WSL shares the PC with Windows.
6. Runtime and maximum RSS for each workload and size, together with the machine description.
   They are the baseline for the VM comparisons and the input data for a scheduler.

## Conclusion

The four workloads have clearly different profiles:
- `cpu_task` is pure CPU work with tiny memory use;
- `memory_task` is dominated by memory allocation, and its RSS equals its size;
- `matrix_task` grows cubically;
- `sort_task` is mixed, with n log n time and linear memory.

A scheduler must therefore treat tasks differently according to their workload and size.
