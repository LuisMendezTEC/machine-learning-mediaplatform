package coordinator

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/LuisMendezTEC/mediaplatform.git/internal/db"
	"github.com/LuisMendezTEC/mediaplatform.git/internal/models"
	"github.com/LuisMendezTEC/mediaplatform.git/internal/queue"
	"github.com/google/uuid"
)

// API groups all HTTP handlers of the coordinator.
type API struct {
	queue    *queue.Queue
	registry *Registry
	hub      *Hub
	db       *sql.DB
}

func NewAPI(q *queue.Queue, reg *Registry, hub *Hub, database *sql.DB) *API {
	return &API{queue: q, registry: reg, hub: hub, db: database}
}

// Router builds and returns the HTTP mux with all the routes.
func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// Jobs
	mux.HandleFunc("POST /jobs", a.submitJob)
	mux.HandleFunc("POST /batch", a.submitBatch)
	mux.HandleFunc("GET /jobs", a.listJobs)
	mux.HandleFunc("GET /jobs/{id}", a.getJob)

	// Workers
	mux.HandleFunc("POST /workers/register", a.registerWorker)
	mux.HandleFunc("POST /workers/{id}/heartbeat", a.workerHeartbeat)
	mux.HandleFunc("GET /workers", a.listWorkers)

	// Stats + WebSocket
	mux.HandleFunc("GET /stats", a.getStats)
	mux.HandleFunc("GET /ws", a.hub.ServeWS)

	return mux
}

// ── Job handlers ─────────────────────────────────────────────────────────────

func (a *API) submitJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FilePath  string           `json:"file_path"`
		Operation models.Operation `json:"operation"`
		Priority  int              `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	job := &models.Job{
		ID:         uuid.New().String(),
		FilePath:   req.FilePath,
		Operation:  req.Operation,
		Priority:   req.Priority,
		Status:     models.StatusPending,
		MaxRetries: 3,
		CreatedAt:  time.Now(),
	}
	if job.Priority == 0 {
		job.Priority = 5 // default: normal
	}

	if err := db.InsertJob(a.db, job); err != nil {
		log.Printf("[api] insert job: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if err := a.queue.Enqueue(r.Context(), job); err != nil {
		log.Printf("[api] enqueue: %v", err)
		http.Error(w, "queue error", http.StatusInternalServerError)
		return
	}

	log.Printf("[api] job submitted: %s op=%s priority=%d", job.ID, job.Operation, job.Priority)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (a *API) submitBatch(w http.ResponseWriter, r *http.Request) {
	var reqs []struct {
		FilePath  string           `json:"file_path"`
		Operation models.Operation `json:"operation"`
		Priority  int              `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	var jobs []*models.Job
	for _, req := range reqs {
		job := &models.Job{
			ID: uuid.New().String(), FilePath: req.FilePath,
			Operation: req.Operation, Priority: req.Priority,
			Status: models.StatusPending, MaxRetries: 3, CreatedAt: time.Now(),
		}
		if job.Priority == 0 {
			job.Priority = 5
		}
		db.InsertJob(a.db, job)
		a.queue.Enqueue(r.Context(), job)
		jobs = append(jobs, job)
	}
	log.Printf("[api] batch submitted: %d jobs", len(jobs))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(jobs)
}

func (a *API) listJobs(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	jobs, err := db.ListJobs(a.db, status)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (a *API) getJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := db.GetJob(a.db, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// ── Worker handlers ───────────────────────────────────────────────────────────

func (a *API) registerWorker(w http.ResponseWriter, r *http.Request) {
	var info models.WorkerInfo
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	a.registry.Register(&info)
	log.Printf("[api] worker registered: %s (%s)", info.ID, info.Hostname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}

func (a *API) workerHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var payload struct {
		CPU        float64 `json:"cpu_percent"`
		Mem        float64 `json:"mem_percent"`
		ActiveJobs int     `json:"active_jobs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if !a.registry.Heartbeat(id, payload.CPU, payload.Mem, payload.ActiveJobs) {
		// Worker no estaba registrado — que se registre primero
		http.Error(w, "worker not registered", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) listWorkers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a.registry.All())
}

func (a *API) getStats(w http.ResponseWriter, r *http.Request) {
	stats, _ := db.GetStats(a.db)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
