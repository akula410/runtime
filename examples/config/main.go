// Package main demonstrates loading per-environment config via a startup task.
//
// Usage:
//
//	go run ./examples/config --env=dev
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	runtime "github.com/akula410/runtime"
)

// AppConfig holds application configuration loaded from JSON.
type AppConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	LogLevel string `json:"log_level"`
}

func main() {
	env := flag.String("env", "dev", "environment directory name (dev, product, .local)")
	flag.Parse()

	var cfg AppConfig

	app, err := runtime.New(
		runtime.WithEnv(*env),
		runtime.WithConfigBase("configs"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Startup task loads config from configs/<env>/app.json.
	app.AddStartupTask(&runtime.StartupTaskFunc{
		TaskName: "load-config",
		Fn: func(ctx context.Context, a *runtime.App) error {
			if err := a.LoadConfig(ctx, &cfg); err != nil {
				return err
			}
			fmt.Printf("loaded config from %s: %+v\n", a.ConfigDir(), cfg)
			return nil
		},
	})

	// Override with runtime value (simulating CLI flag override).
	if *env == "dev" {
		cfg.LogLevel = "trace" // runtime override takes priority
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately after tasks run — no services needed in this example

	if err := app.Run(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("final config: %+v\n", cfg)
}
