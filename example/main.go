// Command demo is a runnable example worker for server-task-scheduler.
//
// Usage:
//   go run ./example
// Then create a schedule on the center with task_key "hello":
//   curl -X POST http://localhost:6432/api/v1/schedules -H 'Content-Type: application/json' \
//     -d '{"name":"demo","task_key":"hello","cron":"*/30 * * * *","params":{"name":"world"},"enabled":true}'
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	taskscheduler "github.com/neko233-com/task-scheduler-lib-go"
)

func main() {
	serverURL := envOr("SCHEDULER_URL", "http://localhost:6432")
	w := taskscheduler.New(serverURL, "demo-go", "demo", ":8081", "http://localhost:8081")

	w.Handle("hello", func(ctx context.Context, params map[string]interface{}) (string, error) {
		name, _ := params["name"].(string)
		if name == "" {
			name = "world"
		}
		msg := fmt.Sprintf("hello, %s! (worker=demo-go, time=%s)", name, time.Now().Format(time.RFC3339))
		fmt.Println(msg)
		return msg, nil
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log := func(v string) { fmt.Fprintln(os.Stderr, v) }
	log("demo-go worker starting; register at " + serverURL)
	if err := w.Run(ctx); err != nil {
		fmt.Println("worker error:", err)
		os.Exit(1)
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
