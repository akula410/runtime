// Package main demonstrates starting a control server and querying it via the client.
//
// Run the server in one terminal:
//
//	go run ./examples/control
//
// Then in another terminal use the client (built into this example) or curl:
//
//	curl http://127.0.0.1:7070/status
//	curl -X POST http://127.0.0.1:7070/services/worker/restart
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	runtime "github.com/akula410/runtime"
	"github.com/akula410/runtime/control"
)

// workerService is a simple background worker.
type workerService struct{}

func (w *workerService) Name() string { return "worker" }
func (w *workerService) Start(ctx context.Context) error {
	log.Println("[worker] started")
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			log.Println("[worker] tick")
		case <-ctx.Done():
			log.Println("[worker] stopped")
			return nil
		}
	}
}
func (w *workerService) Stop(_ context.Context) error { return nil }
func (w *workerService) Health(_ context.Context) runtime.HealthStatus {
	return runtime.HealthStatus{Name: w.Name(), Message: "ticking"}
}

func main() {
	clientMode := flag.Bool("client", false, "run as control client instead of server")
	flag.Parse()

	if *clientMode {
		runClient()
		return
	}

	runServer()
}

func runServer() {
	app, err := runtime.New(runtime.WithShutdownTimeout(5 * time.Second))
	if err != nil {
		log.Fatal(err)
	}

	if err := app.AddService(&workerService{}); err != nil {
		log.Fatal(err)
	}

	// Control server is itself a service registered with the app.
	ctrlSrv := control.NewServer("127.0.0.1:7070", app, "")
	if err := app.AddService(ctrlSrv); err != nil {
		log.Fatal(err)
	}

	fmt.Println("server started. control API at http://127.0.0.1:7070")
	fmt.Println("run with -client to query it")
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func runClient() {
	client := control.NewClient("http://127.0.0.1:7070", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	st, err := client.Status(ctx)
	if err != nil {
		log.Fatalf("status: %v", err)
	}
	fmt.Println("=== App Status ===")
	for _, s := range st.Services {
		fmt.Printf("  %-20s %s\n", s.Name, s.State)
	}

	fmt.Println("restarting worker...")
	if err := client.RestartService(ctx, "worker"); err != nil {
		log.Fatalf("restart: %v", err)
	}
	fmt.Println("worker restarted")
}
