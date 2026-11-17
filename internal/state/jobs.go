package state

import (
	"sort"
	"time"
)

// JobStatus describes the lifecycle stage of a background job.
type JobStatus string

const (
	JobRunning  JobStatus = "running"
	JobDone     JobStatus = "done"
	JobFailed   JobStatus = "failed"
	JobCanceled JobStatus = "canceled"
)

// Job tracks a background worker (scan, thumbnail batch, backup, staging).
// Progress is in [0,1]; Indeterminate marks jobs without a computable
// fraction.
type Job struct {
	ID            string
	Kind          string
	Label         string
	Status        JobStatus
	Progress      float64
	Indeterminate bool
	Detail        string
	StartedAt     time.Time
	FinishedAt    *time.Time
}

// Active reports whether the job is still running.
func (j *Job) Active() bool { return j.Status == JobRunning }

// StartJob registers a new running job, replacing any job with the same ID.
func (s *AppState) StartJob(id, kind, label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[id]; !exists {
		s.jobOrder = append(s.jobOrder, id)
	}
	s.jobs[id] = &Job{
		ID:            id,
		Kind:          kind,
		Label:         label,
		Status:        JobRunning,
		Indeterminate: true,
		StartedAt:     time.Now(),
	}
}

// UpdateJob adjusts progress (clamped to [0,1]), indeterminate mode, and
// the detail line of a running job. It is a no-op for unknown or finished
// jobs.
func (s *AppState) UpdateJob(id string, progress float64, indeterminate bool, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok || !j.Active() {
		return
	}
	j.Progress = clamp01(progress)
	j.Indeterminate = indeterminate
	j.Detail = detail
}

// FinishJob marks a job done, failed, or canceled.
func (s *AppState) FinishJob(id string, status JobStatus, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok || !j.Active() {
		return
	}
	now := time.Now()
	j.Status = status
	j.Detail = detail
	j.Progress = clamp01(j.Progress)
	j.FinishedAt = &now
}

// Job retrieves a copy of a job by ID.
func (s *AppState) Job(id string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *j, true
}

// Jobs returns copies of all jobs, oldest first, trimming terminal jobs
// beyond keepDone history entries.
func (s *AppState) Jobs(keepDone int) []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Job, 0, len(s.jobOrder))
	var done []Job
	for _, id := range s.jobOrder {
		j := s.jobs[id]
		if j.Active() {
			out = append(out, *j)
		} else {
			done = append(done, *j)
		}
	}
	// Most recent finished jobs first, then keep the requested history.
	sort.Slice(done, func(i, k int) bool {
		return done[i].StartedAt.After(done[k].StartedAt)
	})
	if keepDone >= 0 && len(done) > keepDone {
		done = done[:keepDone]
	}
	return append(out, done...)
}

// AnyActive reports whether any job is still running.
func (s *AppState) AnyActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, j := range s.jobs {
		if j.Active() {
			return true
		}
	}
	return false
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
