package taskscheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Handler runs a task. It receives the schedule params and returns an output
// string (captured by the center) and an error (non-nil => task failed).
type Handler func(ctx context.Context, params map[string]interface{}) (string, error)

// Worker is an embeddable executor. It registers with the center, sends
// heartbeats to stay alive, and serves POST /execute so the center can dispatch
// tasks to it.
type Worker struct {
	serverURL  string
	name       string
	app        string
	listenAddr string // local bind, e.g. ":8081"
	publicAddr string // URL the center calls back, e.g. "http://host:8081"

	mu    sync.RWMutex
	tasks map[string]Handler

	client    *client
	workerID  string
	ttl       time.Duration
	httpSrv   *http.Server
}

// New constructs a Worker.
//
//	serverURL  dispatch center base, e.g. "http://localhost:6432"
//	name       human-readable worker name
//	app        application/group name
//	listenAddr local bind address, e.g. ":8081"
//	publicAddr outward URL the center reaches, e.g. "http://localhost:8081"
func New(serverURL, name, app, listenAddr, publicAddr string) *Worker {
	return &Worker{
		serverURL:  serverURL,
		name:       name,
		app:        app,
		listenAddr: listenAddr,
		publicAddr: publicAddr,
		tasks:      make(map[string]Handler),
		client:     newClient(serverURL),
		ttl:        30 * time.Second,
	}
}

// Handle registers a handler for taskKey. Call before Run.
func (w *Worker) Handle(taskKey string, fn Handler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.tasks[taskKey] = fn
}

func (w *Worker) taskKeys() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	keys := make([]string, 0, len(w.tasks))
	for k := range w.tasks {
		keys = append(keys, k)
	}
	return keys
}

// Run registers with the center, starts the heartbeat loop and the /execute
// HTTP server, then blocks until ctx is cancelled (on cancel it deregisters
// and shuts down the server).
func (w *Worker) Run(ctx context.Context) error {
	reg, err := w.client.register(RegisterRequest{
		Name:    w.name,
		App:     w.app,
		Address: w.publicAddr,
		Tasks:   w.taskKeys(),
	})
	if err != nil {
		return err
	}
	w.workerID = reg.WorkerID
	if reg.TTLSeconds > 0 {
		w.ttl = time.Duration(reg.TTLSeconds) * time.Second
	}
	log.Printf("[worker] registered id=%s ttl=%ds tasks=%v", w.workerID, reg.TTLSeconds, w.taskKeys())

	go w.heartbeatLoop(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /execute", w.handleExecute)
	w.httpSrv = &http.Server{Addr: w.listenAddr, Handler: mux}

	go func() {
		<-ctx.Done()
		_ = w.client.deregister(w.workerID)
		_ = w.httpSrv.Close()
	}()

	log.Printf("[worker] serving /execute on %s", w.listenAddr)
	if err := w.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (w *Worker) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(w.ttl / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.client.heartbeat(HeartbeatRequest{WorkerID: w.workerID, Tasks: w.taskKeys()}); err != nil {
				log.Printf("[worker] heartbeat failed: %v", err)
			}
		}
	}
}

func (w *Worker) handleExecute(rw http.ResponseWriter, r *http.Request) {
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, ExecuteResponse{Success: false, Error: "invalid JSON"})
		return
	}

	w.mu.RLock()
	fn, ok := w.tasks[req.TaskKey]
	w.mu.RUnlock()
	if !ok {
		writeJSON(rw, http.StatusOK, ExecuteResponse{Success: false, Error: "unknown task_key " + req.TaskKey})
		return
	}

	ctx := r.Context()
	if req.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	output, err := runHandler(ctx, fn, req.Params)
	if err != nil {
		writeJSON(rw, http.StatusOK, ExecuteResponse{Success: false, Output: output, Error: err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, ExecuteResponse{Success: true, Output: output})
}

// runHandler executes fn with panic recovery and context cancellation.
func runHandler(ctx context.Context, fn Handler, params map[string]interface{}) (string, error) {
	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	panicCh := make(chan interface{}, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				panicCh <- p
			}
		}()
		out, err := fn(ctx, params)
		done <- result{out, err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case p := <-panicCh:
		return "", fmt.Errorf("handler panicked: %v", p)
	case res := <-done:
		return res.out, res.err
	}
}

func writeJSON(rw http.ResponseWriter, code int, v interface{}) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(code)
	_ = json.NewEncoder(rw).Encode(v)
}
