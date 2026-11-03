package taskscheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// client talks to the dispatch center over HTTP/JSON.
type client struct {
	serverURL string
	http      *http.Client
}

func newClient(serverURL string) *client {
	return &client{
		serverURL: serverURL,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *client) register(req RegisterRequest) (RegisterResponse, error) {
	var out RegisterResponse
	err := c.post("/api/v1/workers/register", req, &out)
	return out, err
}

func (c *client) heartbeat(req HeartbeatRequest) error {
	var ack GenericAck
	return c.post("/api/v1/workers/heartbeat", req, &ack)
}

func (c *client) deregister(workerID string) error {
	var ack GenericAck
	return c.post("/api/v1/workers/deregister", map[string]string{"worker_id": workerID}, &ack)
}

func (c *client) post(path string, body interface{}, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := c.serverURL + path
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("center returned status %d for %s", resp.StatusCode, path)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
