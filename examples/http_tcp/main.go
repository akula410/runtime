// Package main demonstrates an HTTP server and a TCP echo server as managed services.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	runtime "github.com/akula410/runtime"
)

// HTTPService wraps net/http.Server as a runtime.Service.
type HTTPService struct {
	addr string
	srv  *http.Server
}

func NewHTTPService(addr string) *HTTPService {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "hello from HTTP service")
	})
	return &HTTPService{addr: addr, srv: &http.Server{Addr: addr, Handler: mux}}
}

func (s *HTTPService) Name() string { return "http" }
func (s *HTTPService) Start(ctx context.Context) error {
	log.Printf("[http] listening on %s", s.addr)
	if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
func (s *HTTPService) Stop(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
func (s *HTTPService) Health(_ context.Context) runtime.HealthStatus {
	return runtime.HealthStatus{Name: s.Name(), Message: "listening on " + s.addr}
}

// TCPService is a simple TCP echo server.
type TCPService struct {
	addr string
	ln   net.Listener
}

func (s *TCPService) Name() string { return "tcp-echo" }
func (s *TCPService) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln
	log.Printf("[tcp-echo] listening on %s", ln.Addr())

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go io.Copy(conn, conn) //nolint:errcheck
		}
	}()
	<-ctx.Done()
	return nil
}
func (s *TCPService) Stop(_ context.Context) error {
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}
func (s *TCPService) Health(_ context.Context) runtime.HealthStatus {
	return runtime.HealthStatus{Name: s.Name(), Message: "echo server running"}
}

func main() {
	app, err := runtime.New(
		runtime.WithShutdownTimeout(5 * time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := app.AddService(NewHTTPService("127.0.0.1:8080")); err != nil {
		log.Fatal(err)
	}
	if err := app.AddService(&TCPService{addr: "127.0.0.1:9000"}); err != nil {
		log.Fatal(err)
	}

	fmt.Println("starting HTTP+TCP services... press Ctrl+C to stop")
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
