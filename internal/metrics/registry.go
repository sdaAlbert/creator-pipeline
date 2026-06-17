package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"creator-pipeline/internal/queue"
	"creator-pipeline/internal/task"
)

type Registry struct {
	mu            sync.Mutex
	queue         queue.Queue
	created       int
	byStatus      map[task.Status]int
	retries       int
	running       int
	durations     []float64
	modelFailures int
}

func NewRegistry(q queue.Queue) *Registry {
	return &Registry{
		queue:    q,
		byStatus: make(map[task.Status]int),
	}
}

func (r *Registry) TaskCreated() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created++
}

func (r *Registry) TaskFinished(status task.Status, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byStatus[status]++
	r.durations = append(r.durations, duration.Seconds())
}

func (r *Registry) Retry() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retries++
}

func (r *Registry) WorkerRunning(delta int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running += delta
	if r.running < 0 {
		r.running = 0
	}
}

func (r *Registry) ModelFailure() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modelFailures++
}

func (r *Registry) Prometheus() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var b strings.Builder
	fmt.Fprintf(&b, "# TYPE creator_tasks_created_total counter\ncreator_tasks_created_total %d\n", r.created)
	fmt.Fprintf(&b, "# TYPE creator_tasks_finished_total counter\n")
	for _, status := range []task.Status{task.StatusSucceeded, task.StatusFailed, task.StatusTimeout, task.StatusCanceled} {
		fmt.Fprintf(&b, "creator_tasks_finished_total{status=%q} %d\n", status, r.byStatus[status])
	}
	fmt.Fprintf(&b, "# TYPE creator_task_retry_total counter\ncreator_task_retry_total %d\n", r.retries)
	fmt.Fprintf(&b, "# TYPE creator_queue_depth gauge\ncreator_queue_depth %d\n", r.queue.Len())
	fmt.Fprintf(&b, "# TYPE creator_worker_running_jobs gauge\ncreator_worker_running_jobs %d\n", r.running)
	fmt.Fprintf(&b, "# TYPE creator_model_call_failed_total counter\ncreator_model_call_failed_total %d\n", r.modelFailures)
	fmt.Fprintf(&b, "# TYPE creator_task_duration_p95_seconds gauge\ncreator_task_duration_p95_seconds %.6f\n", percentile(r.durations, 0.95))
	return b.String()
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	idx := int(float64(len(cp)-1) * p)
	return cp[idx]
}
