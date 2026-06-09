package control

import (
	"encoding/json"
	"net/http"
	"strings"

	runtime "github.com/akula410/runtime"
)

func registerHandlers(mux *http.ServeMux, ctrl runtime.Controller, token string) {
	h := &handler{ctrl: ctrl, token: token}
	mux.HandleFunc("GET /status", h.appStatus)
	mux.HandleFunc("POST /stop", h.appStop)
	mux.HandleFunc("POST /restart", h.appRestart)
	mux.HandleFunc("GET /services/{name}/status", h.serviceStatus)
	mux.HandleFunc("GET /services/{name}/health", h.serviceHealth)
	mux.HandleFunc("POST /services/{name}/start", h.serviceStart)
	mux.HandleFunc("POST /services/{name}/stop", h.serviceStop)
	mux.HandleFunc("POST /services/{name}/restart", h.serviceRestart)
}

type handler struct {
	ctrl  runtime.Controller
	token string
}

func (h *handler) auth(w http.ResponseWriter, r *http.Request) bool {
	if h.token == "" {
		return true
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if got != h.token {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

func (h *handler) appStatus(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, h.ctrl.Status(r.Context()))
}

func (h *handler) appStop(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}
	if err := h.ctrl.Shutdown(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}

func (h *handler) appRestart(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}
	if err := h.ctrl.Restart(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

func (h *handler) serviceStatus(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}
	name := r.PathValue("name")
	hs, err := h.ctrl.ServiceStatus(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, hs)
}

func (h *handler) serviceHealth(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}
	name := r.PathValue("name")
	hs, err := h.ctrl.ServiceHealth(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, hs)
}

func (h *handler) serviceStart(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}
	name := r.PathValue("name")
	if err := h.ctrl.StartService(r.Context(), name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "starting"})
}

func (h *handler) serviceStop(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}
	name := r.PathValue("name")
	if err := h.ctrl.StopService(r.Context(), name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *handler) serviceRestart(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}
	name := r.PathValue("name")
	if err := h.ctrl.RestartService(r.Context(), name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
