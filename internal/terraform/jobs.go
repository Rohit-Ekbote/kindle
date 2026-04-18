package terraform

import (
	"sync"
	"time"
)

const (
	StateProvisioning = "provisioning"
	StateDeleting     = "deleting"
	StateIdle         = "idle"
)

type JobStatus struct {
	State     string
	StartedAt time.Time
	Actor     string
	LogPath   string
	Err       error
}

type JobMap struct {
	mu   sync.RWMutex
	jobs map[string]JobStatus
}

func NewJobMap() *JobMap {
	return &JobMap{jobs: make(map[string]JobStatus)}
}

func (m *JobMap) Set(name string, status JobStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[name] = status
}

func (m *JobMap) Get(name string) (JobStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.jobs[name]
	return s, ok
}

func (m *JobMap) Delete(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, name)
}

func (m *JobMap) InFlight(name string) bool {
	s, ok := m.Get(name)
	if !ok {
		return false
	}
	return s.State == StateProvisioning || s.State == StateDeleting
}
