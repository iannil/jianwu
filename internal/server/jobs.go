package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iannil/jianwu/internal/book"
)

// JobStatus is the lifecycle state of a background job.
type JobStatus string

const (
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

const (
	maxJobLogLines = 500
	maxJobHistory  = 100
)

// Job is one background operation (expand, factcheck, export, ...).
// Handlers mutate it through the exported setters; the API serves views.
type Job struct {
	ID     string    `json:"id"`
	Kind   string    `json:"kind"`
	Slug   string    `json:"slug,omitempty"`
	Status JobStatus `json:"status"`

	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	Progress int    `json:"progress"` // 0-100
	Message  string `json:"message,omitempty"`
	Err      string `json:"error,omitempty"`

	Result map[string]any `json:"result,omitempty"`

	mu     sync.Mutex
	log    []logLine
	usage  *book.TokenUsage
	cancel context.CancelFunc
}

// logLine is one timestamped job log entry.
type logLine struct {
	At   time.Time
	Text string
}

// jobView is the JSON projection of a job (avoids copying the mutex-bearing
// Job struct and hides scheduling internals).
type jobView struct {
	ID         string           `json:"id"`
	Kind       string           `json:"kind"`
	Slug       string           `json:"slug,omitempty"`
	Status     JobStatus        `json:"status"`
	CreatedAt  time.Time        `json:"created_at"`
	StartedAt  *time.Time       `json:"started_at,omitempty"`
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	Progress   int              `json:"progress"`
	Message    string           `json:"message,omitempty"`
	Err        string           `json:"error,omitempty"`
	Result     map[string]any   `json:"result,omitempty"`
	Log        string           `json:"log"`
	TokenUsage *book.TokenUsage `json:"token_usage,omitempty"`
}

// view returns a consistent JSON projection of the job.
func (j *Job) view() *jobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	logs := make([]string, len(j.log))
	for i, l := range j.log {
		logs[i] = l.At.Format("15:04:05") + " " + l.Text
	}
	return &jobView{
		ID:         j.ID,
		Kind:       j.Kind,
		Slug:       j.Slug,
		Status:     j.Status,
		CreatedAt:  j.CreatedAt,
		StartedAt:  j.StartedAt,
		FinishedAt: j.FinishedAt,
		Progress:   j.Progress,
		Message:    j.Message,
		Err:        j.Err,
		Result:     j.Result,
		Log:        strings.Join(logs, "\n"),
		TokenUsage: j.usage,
	}
}

// MarshalJSON delegates to view so raw *Job values serialize safely.
func (j *Job) MarshalJSON() ([]byte, error) { return json.Marshal(j.view()) }

// SetProgress updates the progress percent and status message.
func (j *Job) SetProgress(pct int, msg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Progress = clampPct(pct)
	if msg != "" {
		j.Message = msg
	}
}

// Logf appends a line to the job log (capped).
func (j *Job) Logf(format string, args ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.log = append(j.log, logLine{At: time.Now(), Text: fmt.Sprintf(format, args...)})
	if len(j.log) > maxJobLogLines {
		j.log = j.log[len(j.log)-maxJobLogLines:]
	}
}

// AddUsage merges provider-reported usage into the job total.
func (j *Job) AddUsage(u book.TokenUsage) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.usage == nil {
		j.usage = &book.TokenUsage{}
	}
	j.usage.Add(u)
}

// SetResult stores a key in the job result map.
func (j *Job) SetResult(key string, value any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Result == nil {
		j.Result = map[string]any{}
	}
	j.Result[key] = value
}

func clampPct(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// JobManager runs mutating jobs through a single worker so two jobs never
// write the same workspace at once (delivery constraint: no concurrent
// writers per book). FIFO queue; one job runs at a time.
type JobManager struct {
	mu       sync.Mutex
	jobs     map[string]*Job
	order    []string // insertion order, oldest first
	enqueued int      // queued but not yet acquired the worker slot
	active   int      // currently holding the worker slot
	sem      chan struct{}
}

// NewJobManager constructs a manager with one concurrent job slot.
func NewJobManager() *JobManager {
	return &JobManager{jobs: map[string]*Job{}, sem: make(chan struct{}, 1)}
}

// Start launches fn as a background job and returns its ID.
func (m *JobManager) Start(kind, slug string, fn func(ctx context.Context, j *Job) error) string {
	now := time.Now().UTC()
	j := &Job{
		ID:        "job-" + uuid.NewString()[:8],
		Kind:      kind,
		Slug:      slug,
		Status:    JobRunning,
		CreatedAt: now,
		Message:   "排队中",
	}
	ctx, cancel := context.WithCancel(context.Background())
	j.cancel = cancel

	m.mu.Lock()
	m.jobs[j.ID] = j
	m.order = append(m.order, j.ID)
	m.enqueued++
	// Evict oldest finished jobs beyond the history cap.
	for len(m.order) > maxJobHistory {
		oldest := m.order[0]
		if m.jobs[oldest].Status == JobRunning {
			break
		}
		delete(m.jobs, oldest)
		m.order = m.order[1:]
	}
	m.mu.Unlock()

	go func() {
		m.sem <- struct{}{}
		m.mu.Lock()
		m.enqueued--
		m.active++
		m.mu.Unlock()

		started := time.Now().UTC()
		j.mu.Lock()
		j.StartedAt = &started
		j.mu.Unlock()
		j.Logf("[%s] 开始", kind)

		err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("internal panic: %v", r)
				}
			}()
			return fn(ctx, j)
		}()

		finished := time.Now().UTC()
		j.mu.Lock()
		j.FinishedAt = &finished
		if ctx.Err() != nil {
			j.Status = JobFailed
			j.Err = "已取消"
			j.Message = "已取消"
		} else if err != nil {
			j.Status = JobFailed
			j.Err = err.Error()
			j.Message = "失败"
		} else {
			j.Status = JobSucceeded
			j.Progress = 100
			j.Message = "完成"
		}
		j.mu.Unlock()
		if err != nil && ctx.Err() == nil {
			j.Logf("失败: %s", err.Error())
		}
		j.Logf("[%s] 结束", kind)

		m.mu.Lock()
		m.active--
		m.mu.Unlock()
		<-m.sem
		cancel()
	}()
	return j.ID
}

// Cancel requests cancellation of a running job. Returns false if the job
// is unknown or already finished.
func (m *JobManager) Cancel(id string) bool {
	m.mu.Lock()
	j, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok || j.Status != JobRunning {
		return false
	}
	if j.cancel != nil {
		j.cancel()
	}
	return true
}

// Get returns a JSON view of one job.
func (m *JobManager) Get(id string) (*jobView, bool) {
	m.mu.Lock()
	j, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return nil, false
	}
	return j.view(), true
}

// List returns JSON views of all jobs, newest first.
func (m *JobManager) List() []*jobView {
	m.mu.Lock()
	ids := make([]string, len(m.order))
	copy(ids, m.order)
	m.mu.Unlock()

	out := make([]*jobView, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		if j, ok := m.jobs[ids[i]]; ok {
			out = append(out, j.view())
		}
	}
	return out
}

// waitIdle blocks until every job started so far has finished; test helper.
func (m *JobManager) waitIdle() {
	for {
		m.mu.Lock()
		settled := m.enqueued == 0 && m.active == 0
		m.mu.Unlock()
		if settled {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
