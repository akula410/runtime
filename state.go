package runtime

// ServiceState represents the lifecycle state of a service.
type ServiceState string

const (
	ServiceStopped  ServiceState = "stopped"
	ServiceStarting ServiceState = "starting"
	ServiceRunning  ServiceState = "running"
	ServiceStopping ServiceState = "stopping"
	ServiceFailed   ServiceState = "failed"
)

// HealthStatus holds the health information for a single service.
type HealthStatus struct {
	Name    string       `json:"name"`
	State   ServiceState `json:"state"`
	Message string       `json:"message,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// AppStatus holds the overall application health status.
type AppStatus struct {
	Services []HealthStatus `json:"services"`
}
