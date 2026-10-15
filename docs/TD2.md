# TD2 — Google Compute Engine

## Configuration

| Item | Value |
|---|---|
| gcloud account | `byezzaali@gmail.com` |
| Project ID | `cloud-optimization-511007` |
| Zone | `europe-west1-b` (Belgium) |
| VM | `cloudopt-vm1` (TD3 adds `cloudopt-vm2` and `cloudopt-vm3`, same configuration) |
| Machine type | `e2-medium`: 2 vCPUs (1/2 shared physical core), 4096 MB, x86_64 |
| Image | family `ubuntu-2404-lts-amd64` (`ubuntu-os-cloud`), resolved to `ubuntu-2404-noble-amd64-v20261009` |
| Boot disk | 20 GB `pd-balanced` |
| Network | VPC `default`, subnet `default`, internal IP `10.132.0.2`, ephemeral external IP |
| Guest CPU | Intel Xeon @ 2.20 GHz, 2 logical CPUs (1 core × 2 threads), 3.8 GiB RAM, no swap |

```bash
gcloud config set project cloud-optimization-511007
gcloud config set compute/zone europe-west1-b
gcloud compute instances create cloudopt-vm1 \
  --zone=europe-west1-b --machine-type=e2-medium \
  --image-family=ubuntu-2404-lts-amd64 --image-project=ubuntu-os-cloud \
  --boot-disk-size=20GB --boot-disk-type=pd-balanced \
  --labels=course=cloud-optimization,td=td2
```

Deployment: the binaries are statically linked x86-64 executables, built on the controller and copied with
`gcloud compute scp`. Go is not installed on the VMs. This works because the controller and the VM share the same OS
family and CPU architecture; a different architecture (e.g. ARM64) would require compiling on the VM.

Collected files: `data/machine_cloud_vm1.txt` (hostname, uname, lscpu, nproc, free, lsblk) and
`data/benchmark_cloud_vm1.csv` (same campaign as TD1: 36 raw runs).

## Local vs cloud comparison (mean of 3 runs, same binaries and sizes)

Local machine: Core Ultra 7 155H, 16 logical CPUs, 15 GiB (WSL2). Cloud machine: e2-medium, 2 shared vCPUs, 4 GB.

| workload | size | local mean_s | cloud mean_s | cloud / local | local std_s | cloud std_s | RSS local / cloud (MB) |
|---|---:|---:|---:|---:|---:|---:|---:|
| cpu_task | 200000 | 0.021 | 0.048 | 2.3× | 0.001 | 0.001 | 2.0 / 1.9 |
| cpu_task | 500000 | 0.059 | 0.149 | 2.5× | 0.002 | 0.000 | 2.0 / 1.8 |
| cpu_task | 1000000 | 0.146 | 0.381 | 2.6× | 0.005 | 0.000 | 1.9 / 1.9 |
| memory_task | 128 | 0.137 | 0.222 | 1.6× | 0.001 | 0.014 | 130.3 / 130.2 |
| memory_task | 256 | 0.271 | 0.419 | 1.5× | 0.008 | 0.010 | 258.4 / 258.3 |
| memory_task | 512 | 0.613 | 0.820 | 1.3× | 0.083 | 0.012 | 515.2 / 514.5 |
| matrix_task | 200 | 0.016 | 0.026 | 1.6× | 0.002 | 0.008 | 3.0 / 2.7 |
| matrix_task | 400 | 0.076 | 0.102 | 1.3× | 0.001 | 0.001 | 5.8 / 5.7 |
| matrix_task | 600 | 0.224 | 0.319 | 1.4× | 0.016 | 0.001 | 10.5 / 10.3 |
| sort_task | 1000000 | 0.135 | 0.151 | 1.1× | 0.005 | 0.009 | 10.0 / 9.6 |
| sort_task | 5000000 | 0.696 | 0.745 | 1.07× | 0.026 | 0.001 | 40.4 / 40.3 |
| sort_task | 10000000 | 1.434 | 1.541 | 1.07× | 0.074 | 0.001 | 78.7 / 78.4 |

## Answers

**Part 0**
1. A project is the administrative boundary for resources, permissions, quotas and billing.
   A region is a geographic area (`europe-west1`, Belgium). A zone is an isolated deployment area inside
   a region (`europe-west1-b`).
2. A VM instance is a zonal resource.
3. The machine type defines the virtual hardware (vCPUs, RAM). The image defines the operating system installed on the boot disk.
4. The image is a read-only template. The boot disk is the persistent disk created from it, which holds the
   OS and our files and is billed.
5. Zones do not all host the same hardware generations, and the capacity of a zone can temporarily run out.

**Part 1**
1. `byezzaali@gmail.com`.
2. `cloud-optimization-511007`.
3. Every resource and every charge goes to the selected project. With the wrong project, we would use the
   wrong credits, permissions and quotas.
4. It is the API that gcloud and the console call to create, list, stop and delete VMs. Without it, no
   `gcloud compute` command works.

**Part 3**
1. `e2-medium`: 2 vCPUs and 4 GB of RAM, on a shared core (`isSharedCpu: True`), x86_64, in `europe-west1-b`.
2. A vCPU is one hardware thread of a physical core, exposed to the VM.
3. CPU generation, clock frequency, shared versus dedicated cores and memory bandwidth all differ between machine types.
4. "Google Cloud VM" covers machines of very different power; only the exact type makes a result reproducible.

**Part 5**
1. `RUNNING`.
2. `e2-medium`.
3. Boot disk `cloudopt-vm1` (device `persistent-disk-0`), 20 GB `pd-balanced`.
4. Network `default`, subnet `default`.
5. Yes, an ephemeral one. It changes at every restart (for example `34.140.40.147`, then `35.195.144.114`).

**Part 6**
1. `10.132.0.2`.
2. Yes, an ephemeral external IP.
3. To reduce the attack surface. It can still be reached through IAP, a bastion host or Cloud NAT for outbound traffic.
4. The firewall must allow TCP port 22, sshd must be running, and the user needs SSH keys or OS Login permission.

**Part 12**
1. No. The e2-medium VM is slower for every workload: about 2.5× for `cpu_task` and 1.07× to 1.6× for the others.
2. "Cloud" covers many machine types. Here a small shared-core VM is compared with a recent laptop CPU.
   The machine type, not the word "cloud", determines performance.
3. Yes. The maximum RSS is almost identical, because it depends on the program, not on the machine.
4. No, but they are very stable: the standard deviation is mostly below 1 %, which is more stable than WSL.
   WSL shares the PC with Windows (up to 14 % variation for `memory_task` 512).
5. The machine type, CPU model, vCPU count, RAM, zone, image, disk, binary version, input sizes and number of runs.

## Lifecycle and cost

- `gcloud compute instances stop` moves the VMs to `TERMINATED`. vCPU and RAM are no longer billed,
  and the ephemeral external IP is released.
- The three 20 GB `pd-balanced` boot disks still exist while the VMs are stopped, and they are billed.
- `gcloud compute instances delete` would remove the VM and its auto-delete boot disk. Check the result
  afterwards with `gcloud compute disks list`.
- Status at the end of the session: the 3 VMs are stopped (`TERMINATED`).
