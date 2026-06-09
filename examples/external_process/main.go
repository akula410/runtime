// Package main demonstrates running an external process as a managed service.
//
// Run:
//
//	go run ./examples/external_process
//
// This starts a long-running shell command as a managed service. Press Ctrl+C
// to trigger graceful shutdown, which sends SIGINT to the child process.
package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	rtime "github.com/akula410/runtime"
)

func main() {
	app, err := rtime.New(
		rtime.WithShutdownTimeout(10 * time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Choose a platform-appropriate long-running command.
	var svc *rtime.ExternalProcessService
	if runtime.GOOS == "windows" {
		svc = rtime.NewExternalProcessService("pinger", "cmd", "/c", "ping", "-t", "127.0.0.1")
	} else {
		svc = rtime.NewExternalProcessService("pinger", "ping", "-c", "60", "127.0.0.1")
	}
	svc.WithKillTimeout(3 * time.Second)

	if err := app.AddService(svc); err != nil {
		log.Fatal(err)
	}

	fmt.Println("external process service running — press Ctrl+C to stop")
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
