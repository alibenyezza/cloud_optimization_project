// TD4 Round-Robin baseline scheduler (Go standard library only).
// Build: go build -o bin/round_robin_scheduler scheduler/round_robin.go
// Run:   ./bin/round_robin_scheduler data/tasks.csv data/machines.csv data/allocation_rr.csv
package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readCSV(path string, expectedHeader []string) ([][]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}
	if len(records[0]) != len(expectedHeader) {
		return nil, fmt.Errorf("%s: expected columns %s", path, strings.Join(expectedHeader, ","))
	}
	for i, name := range expectedHeader {
		if strings.TrimSpace(records[0][i]) != name {
			return nil, fmt.Errorf("%s: column %d must be %q", path, i+1, name)
		}
	}
	return records[1:], nil
}

func schedule(tasks [][]string, machines [][]string) ([][]string, error) {
	if len(machines) == 0 {
		return nil, fmt.Errorf("no VMs available: machines.csv must contain at least one VM")
	}

	vmNames := make([]string, 0, len(machines))
	seenMachines := make(map[string]bool)
	for i, row := range machines {
		vm := strings.TrimSpace(row[0])
		if vm == "" || seenMachines[vm] {
			return nil, fmt.Errorf("invalid or duplicated VM in machines.csv, data row %d", i+1)
		}
		seenMachines[vm] = true
		vmNames = append(vmNames, vm)
	}

	assigned := make([][]string, 0, len(tasks))
	seenTasks := make(map[string]bool)
	for i, row := range tasks {
		id := strings.TrimSpace(row[0])
		workload := strings.TrimSpace(row[1])
		size := strings.TrimSpace(row[2])
		if id == "" || workload == "" || size == "" {
			return nil, fmt.Errorf("missing task field in tasks.csv, data row %d", i+1)
		}
		if seenTasks[id] {
			return nil, fmt.Errorf("duplicate task ID %q", id)
		}
		seenTasks[id] = true

		vm := vmNames[i%len(vmNames)]
		assigned = append(assigned, []string{id, workload, size, vm})
	}
	return assigned, nil
}

func writeAllocation(path string, allocations [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Write to a temporary file, then rename, avoiding partially written CSVs.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rr-allocation-*")
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	writer := csv.NewWriter(tmp)
	err = writer.Write([]string{"task_id", "workload", "size", "vm"})
	if err == nil {
		err = writer.WriteAll(allocations)
	}
	writer.Flush()
	if err == nil {
		err = writer.Error()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("save output: %w", err)
	}
	return nil
}

func run(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: round_robin_scheduler <tasks.csv> <machines.csv> <allocation.csv>")
	}
	tasks, err := readCSV(args[0], []string{"task_id", "workload", "size"})
	if err != nil {
		return err
	}
	machines, err := readCSV(args[1], []string{"vm"})
	if err != nil {
		return err
	}
	assignments, err := schedule(tasks, machines)
	if err != nil {
		return err
	}
	if err := writeAllocation(args[2], assignments); err != nil {
		return err
	}
	fmt.Printf("Scheduled %d tasks across %d VMs: %s\n", len(tasks), len(machines), args[2])
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
