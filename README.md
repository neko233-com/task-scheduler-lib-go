# task-scheduler-lib-go

Go SDK for [server-task-scheduler](https://github.com/neko233-com/server-task-scheduler).
An embeddable worker that **auto-registers** with the center, sends heartbeats,
declares the tasks it can run, and serves `POST /execute` so the center can
dispatch scheduled jobs to it.

**Zero external dependencies. Vendored by design** — copy this folder into your
project, or use it as a git submodule. No package-registry publish required.

## Install / use

```bash
go get github.com/neko233-com/task-scheduler-lib-go
```

Or copy `model.go`, `client.go`, `worker.go` into your module.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	taskscheduler "github.com/neko233-com/task-scheduler-lib-go"
)

func main() {
	w := taskscheduler.New(
		"http://localhost:6432", // center
		"my-worker",             // name
		"my-app",                // app group
		":8081",                 // local bind
		"http://localhost:8081", // public callback URL
	)

	w.Handle("send_email", func(ctx context.Context, params map[string]interface{}) (string, error) {
		to, _ := params["to"].(string)
		return "queued email to " + to, nil
	})

	ctx := context.Background()
	w.Run(ctx) // blocks; registers, heartbeats, serves /execute
	fmt.Println("started")
}
```

## API

| Method | Description |
|--------|-------------|
| `New(serverURL, name, app, listenAddr, publicAddr)` | Construct a worker. |
| `Handle(taskKey, fn)` | Register a task handler. `fn(ctx, params) (output string, err error)`. |
| `Run(ctx)` | Register, start heartbeat + `/execute` server, block until ctx done. |

The center calls `POST {publicAddr}/execute` with
`{job_id, task_key, params, attempt, timeout_seconds}` and expects
`{success, output, error}`.

## Run the demo

```bash
go run ./example
# then on the center:
curl -X POST http://localhost:6432/api/v1/schedules -H 'Content-Type: application/json' \
  -d '{"name":"demo","task_key":"hello","cron":"*/30 * * * *","params":{"name":"world"},"enabled":true}'
```
