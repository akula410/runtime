package control_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	runtime "github.com/akula410/runtime"
	"github.com/akula410/runtime/control"
)

// fakeController is a test implementation of runtime.Controller.
type fakeController struct {
	status     runtime.AppStatus
	stopCalled bool
	startErr   map[string]error
	stopErr    map[string]error
}

func (f *fakeController) Status(_ context.Context) runtime.AppStatus { return f.status }
func (f *fakeController) Shutdown(_ context.Context) error           { f.stopCalled = true; return nil }
func (f *fakeController) ServiceStatus(_ context.Context, name string) (runtime.HealthStatus, error) {
	for _, s := range f.status.Services {
		if s.Name == name {
			return s, nil
		}
	}
	return runtime.HealthStatus{}, fmt.Errorf("runtime: service not found: %q", name)
}
func (f *fakeController) StartService(_ context.Context, name string) error {
	if err, ok := f.startErr[name]; ok {
		return err
	}
	return nil
}
func (f *fakeController) StopService(_ context.Context, name string) error {
	if err, ok := f.stopErr[name]; ok {
		return err
	}
	return nil
}
func (f *fakeController) RestartService(ctx context.Context, name string) error {
	if err := f.StopService(ctx, name); err != nil {
		return err
	}
	return f.StartService(ctx, name)
}

func startTestServer(t *testing.T, ctrl runtime.Controller, token string) (*control.Server, *control.Client) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	srv := control.NewServer(addr, ctrl, token)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() { _ = srv.Start(ctx) }()

	// wait for server to be ready
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	client := control.NewClient("http://"+addr, token)
	return srv, client
}

func TestControlServerStatus(t *testing.T) {
	ctrl := &fakeController{
		status: runtime.AppStatus{
			Services: []runtime.HealthStatus{
				{Name: "web", State: runtime.ServiceRunning},
			},
		},
	}
	_, client := startTestServer(t, ctrl, "")

	st, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(st.Services) != 1 || st.Services[0].Name != "web" {
		t.Fatalf("unexpected status: %+v", st)
	}
}

func TestControlServerStop(t *testing.T) {
	ctrl := &fakeController{}
	_, client := startTestServer(t, ctrl, "")

	if err := client.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !ctrl.stopCalled {
		t.Fatal("Shutdown was not called on controller")
	}
}

func TestControlServerServiceStatus(t *testing.T) {
	ctrl := &fakeController{
		status: runtime.AppStatus{
			Services: []runtime.HealthStatus{
				{Name: "worker", State: runtime.ServiceRunning, Message: "ok"},
			},
		},
	}
	_, client := startTestServer(t, ctrl, "")

	hs, err := client.ServiceStatus(context.Background(), "worker")
	if err != nil {
		t.Fatalf("ServiceStatus: %v", err)
	}
	if hs.Name != "worker" {
		t.Errorf("name = %q, want %q", hs.Name, "worker")
	}
}

func TestControlServerServiceNotFound(t *testing.T) {
	ctrl := &fakeController{}
	_, client := startTestServer(t, ctrl, "")

	_, err := client.ServiceStatus(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing service")
	}
}

func TestControlServerRestartService(t *testing.T) {
	ctrl := &fakeController{
		status: runtime.AppStatus{
			Services: []runtime.HealthStatus{{Name: "worker", State: runtime.ServiceRunning}},
		},
	}
	_, client := startTestServer(t, ctrl, "")

	if err := client.RestartService(context.Background(), "worker"); err != nil {
		t.Fatalf("RestartService: %v", err)
	}
}

func TestControlServerTokenAuth(t *testing.T) {
	ctrl := &fakeController{}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	srv := control.NewServer(addr, ctrl, "secret")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Start(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, dialErr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if dialErr == nil {
			c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	base := "http://" + addr
	goodClient := control.NewClient(base, "secret")
	badClient := control.NewClient(base, "wrong")

	if _, err := badClient.Status(context.Background()); err == nil {
		t.Fatal("expected unauthorized error for wrong token")
	}

	if _, err := goodClient.Status(context.Background()); err != nil {
		t.Fatalf("auth with correct token failed: %v", err)
	}
}
