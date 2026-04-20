// cmd/client/main.go
// Test client: single job submission and batch/concurrent load testing.
// Usage:
//
//	Single:  ./client -op convert -file /path/to/video.mp4 -priority 5
//	Batch:   ./client -batch -manifest dataset/manifest.json -concurrency 50
//	Watch:   ./client -watch -job <job-id>
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type submitRequest struct {
	FilePath  string `json:"file_path"`
	Operation string `json:"operation"`
	Priority  int    `json:"priority"`
}

type jobResponse struct {
	ID        string `json:"id"`
	FilePath  string `json:"file_path"`
	Operation string `json:"operation"`
	Status    string `json:"status"`
	Priority  int    `json:"priority"`
	Progress  int    `json:"progress"`
	WorkerID  string `json:"worker_id"`
	ErrorMsg  string `json:"error_msg"`
	ResultURL string `json:"result_url"`
	CreatedAt string `json:"created_at"`
}

type manifestFile struct {
	Filename   string   `json:"filename"`
	Path       string   `json:"path"`
	Type       string   `json:"type"`
	Format     string   `json:"format"`
	Operations []string `json:"operations"`
}

type manifest struct {
	Total int            `json:"total"`
	Files []manifestFile `json:"files"`
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func submitJob(coordinatorURL string, req submitRequest) (*jobResponse, error) {
	body, _ := json.Marshal(req)
	resp, err := http.Post(coordinatorURL+"/jobs", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("POST /jobs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("coordinator returned %d: %s", resp.StatusCode, string(raw))
	}
	var job jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &job, nil
}

func getJob(coordinatorURL, jobID string) (*jobResponse, error) {
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", coordinatorURL, jobID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var job jobResponse
	json.NewDecoder(resp.Body).Decode(&job)
	return &job, nil
}

// ── Single job mode ───────────────────────────────────────────────────────────

func runSingle(coordinatorURL, filePath, operation string, priority int, watch bool) {
	fmt.Printf("Submitting job: op=%s file=%s priority=%d\n", operation, filePath, priority)

	job, err := submitJob(coordinatorURL, submitRequest{
		FilePath:  filePath,
		Operation: operation,
		Priority:  priority,
	})
	if err != nil {
		log.Fatalf("Failed to submit job: %v", err)
	}

	fmt.Printf("✅ Job submitted: %s\n", job.ID)
	fmt.Printf("   Status:    %s\n", job.Status)
	fmt.Printf("   Operation: %s\n", job.Operation)

	if !watch {
		fmt.Println("\nTip: run with -watch to poll until completion.")
		return
	}

	fmt.Printf("\nWatching job %s...\n", job.ID)
	watchJob(coordinatorURL, job.ID)
}

func watchJob(coordinatorURL, jobID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	lastStatus := ""
	for range ticker.C {
		job, err := getJob(coordinatorURL, jobID)
		if err != nil {
			fmt.Printf("  poll error: %v\n", err)
			continue
		}
		if job.Status != lastStatus || job.Status == "running" {
			fmt.Printf("  [%s] status=%-10s progress=%3d%%  worker=%s\n",
				time.Now().Format("15:04:05"), job.Status, job.Progress, job.WorkerID)
			lastStatus = job.Status
		}
		if job.Status == "completed" {
			fmt.Printf("\n✅ Completed! Result URL:\n   %s\n", job.ResultURL)
			return
		}
		if job.Status == "failed" {
			fmt.Printf("\n❌ Failed: %s\n", job.ErrorMsg)
			return
		}
	}
}

// ── Batch mode ────────────────────────────────────────────────────────────────

func runBatch(coordinatorURL, manifestPath string, concurrency int, priorityOverride int) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		log.Fatalf("Cannot read manifest %s: %v", manifestPath, err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		log.Fatalf("Cannot parse manifest: %v", err)
	}

	// Expand: one request per (file, operation) pair
	type task struct {
		file manifestFile
		op   string
	}
	var tasks []task
	for _, f := range m.Files {
		for _, op := range f.Operations {
			tasks = append(tasks, task{file: f, op: op})
		}
	}

	total := len(tasks)
	fmt.Printf("Batch mode: %d tasks from %d files | concurrency=%d\n\n",
		total, len(m.Files), concurrency)

	var (
		submitted int64
		failed    int64
		wg        sync.WaitGroup
		sem       = make(chan struct{}, concurrency)
		start     = time.Now()
	)

	for i, t := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, tk task) {
			defer wg.Done()
			defer func() { <-sem }()

			priority := priorityOverride
			if priority == 0 {
				priority = 5 // default normal
			}

			_, err := submitJob(coordinatorURL, submitRequest{
				FilePath:  tk.file.Path,
				Operation: tk.op,
				Priority:  priority,
			})
			if err != nil {
				atomic.AddInt64(&failed, 1)
				fmt.Printf("  [%4d/%d] ❌ FAILED  %s %s: %v\n", idx+1, total, tk.op, tk.file.Filename, err)
				return
			}
			n := atomic.AddInt64(&submitted, 1)
			if n%10 == 0 || int(n) == total {
				fmt.Printf("  [%4d/%d] submitted %d/%d (%.1f jobs/s)\n",
					idx+1, total, n, total, float64(n)/time.Since(start).Seconds())
			}
		}(i, t)
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Printf("\n── Batch complete ──────────────────────────────\n")
	fmt.Printf("  Submitted: %d\n", submitted)
	fmt.Printf("  Failed:    %d\n", failed)
	fmt.Printf("  Time:      %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Rate:      %.1f jobs/s\n", float64(submitted)/elapsed.Seconds())
}

// ── Stats mode ────────────────────────────────────────────────────────────────

func runStats(coordinatorURL string) {
	resp, err := http.Get(coordinatorURL + "/stats")
	if err != nil {
		log.Fatalf("GET /stats: %v", err)
	}
	defer resp.Body.Close()
	var stats map[string]int
	json.NewDecoder(resp.Body).Decode(&stats)

	fmt.Println("── System Stats ────────────────────────────────")
	for k, v := range stats {
		fmt.Printf("  %-12s %d\n", k, v)
	}

	resp2, err := http.Get(coordinatorURL + "/workers")
	if err != nil {
		return
	}
	defer resp2.Body.Close()
	var workers []map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&workers)

	fmt.Printf("\n── Workers (%d) ─────────────────────────────────\n", len(workers))
	for _, w := range workers {
		fmt.Printf("  %-12s  status=%-6s  active_jobs=%.0f  cpu=%.1f%%\n",
			w["id"], w["status"], w["active_jobs"], w["cpu_percent"])
	}
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	// Flags
	coordinatorURL := flag.String("coordinator", "", "Coordinator URL (default: $COORDINATOR_URL or http://localhost:8080)")

	// Single job flags
	filePath := flag.String("file", "", "Path to the input file")
	operation := flag.String("op", "convert", "Operation: convert | extract_audio | thumbnail")
	priority := flag.Int("priority", 5, "Job priority 1-10 (10=highest)")
	watch := flag.Bool("watch", false, "Poll until the job finishes")

	// Batch flags
	batch := flag.Bool("batch", false, "Enable batch mode (reads -manifest)")
	manifestPath := flag.String("manifest", "dataset/manifest.json", "Path to dataset manifest.json")
	concurrency := flag.Int("concurrency", 20, "Number of concurrent submissions in batch mode")

	// Stats flag
	statsMode := flag.Bool("stats", false, "Print system stats and worker list, then exit")

	flag.Parse()

	// Resolve coordinator URL
	url := *coordinatorURL
	if url == "" {
		url = os.Getenv("COORDINATOR_URL")
	}
	if url == "" {
		url = "http://localhost:8080"
	}

	fmt.Printf("[client] coordinator → %s\n\n", url)

	switch {
	case *statsMode:
		runStats(url)

	case *batch:
		if *manifestPath == "" {
			log.Fatal("batch mode requires -manifest <path>")
		}
		runBatch(url, *manifestPath, *concurrency, *priority)

	default:
		if *filePath == "" {
			fmt.Println("Usage:")
			fmt.Println("  Single job:  client -file <path> -op <operation> [-priority N] [-watch]")
			fmt.Println("  Batch load:  client -batch -manifest dataset/manifest.json [-concurrency 50]")
			fmt.Println("  Stats:       client -stats")
			fmt.Println("\nOperations: convert | extract_audio | thumbnail")
			os.Exit(1)
		}
		runSingle(url, *filePath, *operation, *priority, *watch)
	}
}
