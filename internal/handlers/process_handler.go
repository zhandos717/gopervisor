package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"pupervisor/internal/models"
	"pupervisor/internal/service"

	"github.com/gorilla/mux"
)

type ProcessHandler struct {
	pm *service.ProcessManager
}

func NewProcessHandler(pm *service.ProcessManager) *ProcessHandler {
	return &ProcessHandler{pm: pm}
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type SuccessResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (h *ProcessHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func (h *ProcessHandler) writeError(w http.ResponseWriter, status int, err error, message string) {
	h.writeJSON(w, status, ErrorResponse{
		Error:   err.Error(),
		Message: message,
	})
}

func (h *ProcessHandler) GetProcesses(w http.ResponseWriter, r *http.Request) {
	processes := h.pm.GetProcesses()
	h.writeJSON(w, http.StatusOK, processes)
}

func (h *ProcessHandler) StartProcess(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	if err := h.pm.StartProcess(name); err != nil {
		if errors.Is(err, service.ErrProcessNotFound) {
			h.writeError(w, http.StatusNotFound, err, "Process not found: "+name)
			return
		}
		if errors.Is(err, service.ErrProcessAlreadyRunning) {
			h.writeError(w, http.StatusConflict, err, "Process already running: "+name)
			return
		}
		h.writeError(w, http.StatusInternalServerError, err, "Failed to start process")
		return
	}

	h.writeJSON(w, http.StatusOK, SuccessResponse{
		Status:  "started",
		Message: "Process " + name + " started successfully",
	})
}

func (h *ProcessHandler) StopProcess(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	if err := h.pm.StopProcess(name); err != nil {
		if errors.Is(err, service.ErrProcessNotFound) {
			h.writeError(w, http.StatusNotFound, err, "Process not found: "+name)
			return
		}
		if errors.Is(err, service.ErrProcessNotRunning) {
			h.writeError(w, http.StatusConflict, err, "Process not running: "+name)
			return
		}
		h.writeError(w, http.StatusInternalServerError, err, "Failed to stop process")
		return
	}

	h.writeJSON(w, http.StatusOK, SuccessResponse{
		Status:  "stopped",
		Message: "Process " + name + " stopped successfully",
	})
}

func (h *ProcessHandler) RestartProcess(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	if err := h.pm.RestartProcess(name); err != nil {
		if errors.Is(err, service.ErrProcessNotFound) {
			h.writeError(w, http.StatusNotFound, err, "Process not found: "+name)
			return
		}
		h.writeError(w, http.StatusInternalServerError, err, "Failed to restart process")
		return
	}

	h.writeJSON(w, http.StatusOK, SuccessResponse{
		Status:  "restarted",
		Message: "Process " + name + " restarted successfully",
	})
}

type BulkRestartRequest struct {
	Names []string `json:"names"`
}

type BulkRestartResponse struct {
	Status    string `json:"status"`
	Restarted int    `json:"restarted"`
	Failed    int    `json:"failed"`
	Message   string `json:"message"`
}

func (h *ProcessHandler) RestartAllProcesses(w http.ResponseWriter, r *http.Request) {
	restarted, failed := h.pm.RestartAll()

	h.writeJSON(w, http.StatusOK, BulkRestartResponse{
		Status:    "completed",
		Restarted: restarted,
		Failed:    failed,
		Message:   fmt.Sprintf("Restarted %d processes, %d failed", restarted, failed),
	})
}

func (h *ProcessHandler) RestartSelectedProcesses(w http.ResponseWriter, r *http.Request) {
	var req BulkRestartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, err, "Invalid JSON")
		return
	}

	if len(req.Names) == 0 {
		h.writeError(w, http.StatusBadRequest, errors.New("no processes specified"), "Please select at least one process")
		return
	}

	restarted, failed := h.pm.RestartSelected(req.Names)

	h.writeJSON(w, http.StatusOK, BulkRestartResponse{
		Status:    "completed",
		Restarted: restarted,
		Failed:    failed,
		Message:   fmt.Sprintf("Restarted %d processes, %d failed", restarted, failed),
	})
}

func (h *ProcessHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	logs := h.pm.GetLogs(100)
	h.writeJSON(w, http.StatusOK, logs)
}

func (h *ProcessHandler) GetWorkerLogs(w http.ResponseWriter, r *http.Request) {
	allLogs := h.pm.GetLogs(200)
	// Filter worker output logs (messages starting with [processname])
	workerLogs := make([]models.LogEntry, 0)
	for _, log := range allLogs {
		if log.Worker != "" && len(log.Message) > 0 && log.Message[0] == '[' {
			workerLogs = append(workerLogs, log)
		}
	}
	h.writeJSON(w, http.StatusOK, workerLogs)
}

func (h *ProcessHandler) GetSystemLogs(w http.ResponseWriter, r *http.Request) {
	allLogs := h.pm.GetLogs(200)
	// Filter system event logs (not worker output)
	systemLogs := make([]models.LogEntry, 0)
	for _, log := range allLogs {
		if log.Worker == "" || (len(log.Message) > 0 && log.Message[0] != '[') {
			systemLogs = append(systemLogs, log)
		}
	}
	h.writeJSON(w, http.StatusOK, systemLogs)
}

func (h *ProcessHandler) GetWorkerSpecificLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workerName := vars["workerName"]

	logs := h.pm.GetLogsByProcess(workerName, 50)
	h.writeJSON(w, http.StatusOK, logs)
}

// Crash history endpoints

func (h *ProcessHandler) GetCrashes(w http.ResponseWriter, r *http.Request) {
	store := h.pm.GetStorage()
	if store == nil {
		h.writeJSON(w, http.StatusOK, []struct{}{})
		return
	}

	crashes, err := store.GetCrashes(100)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err, "Failed to get crash history")
		return
	}

	h.writeJSON(w, http.StatusOK, crashes)
}

func (h *ProcessHandler) GetCrashesByProcess(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	store := h.pm.GetStorage()
	if store == nil {
		h.writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	crashes, err := store.GetCrashesByProcess(name, 50)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err, "Failed to get crash history")
		return
	}

	h.writeJSON(w, http.StatusOK, crashes)
}

func (h *ProcessHandler) GetCrashStats(w http.ResponseWriter, r *http.Request) {
	store := h.pm.GetStorage()
	if store == nil {
		h.writeJSON(w, http.StatusOK, map[string]int{})
		return
	}

	stats, err := store.GetCrashStats()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err, "Failed to get crash stats")
		return
	}

	h.writeJSON(w, http.StatusOK, stats)
}

// Settings endpoints

func (h *ProcessHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	store := h.pm.GetStorage()
	if store == nil {
		h.writeJSON(w, http.StatusOK, map[string]string{})
		return
	}

	settings, err := store.GetAllSettings()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err, "Failed to get settings")
		return
	}

	h.writeJSON(w, http.StatusOK, settings)
}

func (h *ProcessHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	store := h.pm.GetStorage()
	if store == nil {
		h.writeError(w, http.StatusInternalServerError, errors.New("storage not available"), "Storage not initialized")
		return
	}

	var settings map[string]string
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		h.writeError(w, http.StatusBadRequest, err, "Invalid JSON")
		return
	}

	for key, value := range settings {
		if err := store.SetSetting(key, value); err != nil {
			h.writeError(w, http.StatusInternalServerError, err, "Failed to save setting: "+key)
			return
		}
	}

	h.writeJSON(w, http.StatusOK, SuccessResponse{
		Status:  "saved",
		Message: "Settings saved successfully",
	})
}
