// Package control provides a local HTTP control server and client for managing
// a running application. The server binds to 127.0.0.1 only by default.
package control

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	runtime "github.com/akula410/runtime"
)

const defaultAddr = "127.0.0.1:7070"

// Server is an HTTP control server that exposes management endpoints.
// It implements runtime.Service and can be registered with an App.
type Server struct {
	addr   string
	token  string
	ctrl   runtime.Controller
	srv    *http.Server
	listen net.Listener
}

// NewServer creates a Server that uses ctrl to manage the application.
// addr is the TCP address to listen on (default "127.0.0.1:7070").
// token is the optional bearer token for authentication (empty = no auth).
//
// Security: addr must be a loopback address. Passing a non-loopback address
// risks exposing management operations to the network.
func NewServer(addr string, ctrl runtime.Controller, token string) *Server {
	if addr == "" {
		addr = defaultAddr
	}
	return &Server{
		addr:  addr,
		token: token,
		ctrl:  ctrl,
	}
}

// Name implements runtime.Service.
func (s *Server) Name() string { return "control-server" }

// Start binds the listener and serves HTTP until ctx is cancelled.
// Implements runtime.Service.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("control: listen %s: %w", s.addr, err)
	}
	s.listen = ln

	mux := http.NewServeMux()
	registerHandlers(mux, s.ctrl, s.token)

	s.srv = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Stop shuts down the HTTP server gracefully.
// Implements runtime.Service.
func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// Health implements runtime.Service.
func (s *Server) Health(_ context.Context) runtime.HealthStatus {
	msg := ""
	if s.listen != nil {
		msg = "listening on " + s.listen.Addr().String()
	}
	return runtime.HealthStatus{
		Name:    s.Name(),
		Message: msg,
	}
}

// Addr returns the resolved listener address (available after Start).
func (s *Server) Addr() string {
	if s.listen != nil {
		return s.listen.Addr().String()
	}
	return s.addr
}
