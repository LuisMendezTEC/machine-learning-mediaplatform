package coordinator

import (
	"sync"
	"time"

	"github.com/LuisMendezTEC/mediaplatform.git/internal/models"
)

const heartbeatTimeout = 15 * time.Second

// Registry maintains the state of all registered workers.
// It's thread-safe — multiple goroutines read and write to it.

type Registry struct {
	mu      sync.RWMutex
	workers map[string]*models.WorkerInfo
}

func NewRegistry() *Registry {
	return &Registry{
		workers: make(map[string]*models.WorkerInfo),
	}
}

// Register adds or updates a worker.
func (r *Registry) Register(w *models.WorkerInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w.LastSeen = time.Now()
	w.Status = "idle"
	r.workers[w.ID] = w
}

// Heartbeat updates the timestamp and metrics of a worker.
func (r *Registry) Heartbeat(id string, cpu, mem float64, activeJobs int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.workers[id]
	if !ok {
		return false
	}
	w.LastSeen = time.Now()
	w.CPUPercent = cpu
	w.MemPercent = mem
	w.ActiveJobs = activeJobs
	if activeJobs == 0 {
		w.Status = "idle"
	} else {
		w.Status = "busy"
	}
	return true
}

// LeastLoaded returns the worker with the least current load.
// Criteria: fewest active jobs; tiebreaker by CPU%.
// Returns nil if no workers are available.

func (r *Registry) LeastLoaded() *models.WorkerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best *models.WorkerInfo
	for _, w := range r.workers {
		if !r.isAlive(w) {
			continue
		}
		if best == nil {
			best = w
			continue
		}
		if w.ActiveJobs < best.ActiveJobs {
			best = w
		} else if w.ActiveJobs == best.ActiveJobs && w.CPUPercent < best.CPUPercent {
			best = w
		}
	}
	return best
}

// All returns a copy of all workers (alive and dead).
func (r *Registry) All() []*models.WorkerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*models.WorkerInfo, 0, len(r.workers))
	for _, w := range r.workers {
		copy := *w
		list = append(list, &copy)
	}
	return list
}

// EvictStale removes workers that haven't sent a heartbeat recently.
// It's called periodically by the scheduler.

func (r *Registry) EvictStale() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var evicted []string
	for id, w := range r.workers {
		if !r.isAlive(w) {
			evicted = append(evicted, id)
			delete(r.workers, id)
		}
	}
	return evicted
}

// isAlive checks if a worker is considered alive based on its last heartbeat.
func (r *Registry) isAlive(w *models.WorkerInfo) bool {
	return time.Since(w.LastSeen) < heartbeatTimeout
}
