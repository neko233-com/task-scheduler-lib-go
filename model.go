// Package taskscheduler is the Go SDK for server-task-scheduler.
//
// It is intentionally tiny and dependency-free so it can be VENDORED into your
// project (copy the folder, or use as a git submodule). The wire contract
// matches the center exactly — see server-task-scheduler/ARCHITECTURE.md.
package taskscheduler

// RegisterRequest is sent to the center on startup.
type RegisterRequest struct {
	Name    string   `json:"name"`
	Address string   `json:"address"` // callback base URL, e.g. http://host:8081
	App     string   `json:"app"`
	Tasks   []string `json:"tasks"`
}

// RegisterResponse carries the assigned worker id and heartbeat TTL.
type RegisterResponse struct {
	WorkerID   string `json:"worker_id"`
	TTLSeconds int    `json:"ttl_seconds"`
}

// HeartbeatRequest keeps the worker alive and refreshes its task list.
type HeartbeatRequest struct {
	WorkerID string   `json:"worker_id"`
	Tasks    []string `json:"tasks"`
}

// GenericAck is returned by heartbeat/deregister.
type GenericAck struct {
	OK         bool `json:"ok"`
	TTLSeconds int  `json:"ttl_seconds"`
}

// ExecuteRequest is what the center sends to /execute.
type ExecuteRequest struct {
	JobID          string                 `json:"job_id"`
	TaskKey        string                 `json:"task_key"`
	Params         map[string]interface{} `json:"params"`
	Attempt        int                    `json:"attempt"`
	TimeoutSeconds int                    `json:"timeout_seconds"`
}

// ExecuteResponse is returned after running a task.
type ExecuteResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}
