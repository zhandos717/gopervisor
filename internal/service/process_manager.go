package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"pupervisor/internal/config"
	"pupervisor/internal/models"
)

var (
	ErrProcessNotFound       = errors.New("process not found")
	ErrProcessAlreadyRunning = errors.New("process already running")
	ErrProcessNotRunning     = errors.New("process not running")
)

type ProcessState struct {
	Config    config.ProcessConfig
	Cmd       *exec.Cmd
	Status    string
	Pid       int
	StartTime time.Time
	ExitCode  int
	cancel    context.CancelFunc
}

type ProcessManager struct {
	mu        sync.RWMutex
	processes map[string]*ProcessState
	logs      *LogBuffer
}

type LogBuffer struct {
	mu         sync.RWMutex
	entries    []models.LogEntry
	maxEntries int
}

func NewLogBuffer(maxEntries int) *LogBuffer {
	return &LogBuffer{
		entries:    make([]models.LogEntry, 0, maxEntries),
		maxEntries: maxEntries,
	}
}

func (lb *LogBuffer) Add(entry models.LogEntry) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.entries = append(lb.entries, entry)
	if len(lb.entries) > lb.maxEntries {
		lb.entries = lb.entries[len(lb.entries)-lb.maxEntries:]
	}
}

func (lb *LogBuffer) GetLast(n int) []models.LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if n <= 0 || len(lb.entries) == 0 {
		return []models.LogEntry{}
	}

	start := 0
	if len(lb.entries) > n {
		start = len(lb.entries) - n
	}

	result := make([]models.LogEntry, len(lb.entries[start:]))
	copy(result, lb.entries[start:])
	return result
}

func (lb *LogBuffer) GetByLevel(level string, n int) []models.LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var filtered []models.LogEntry
	for _, e := range lb.entries {
		if e.Level == level {
			filtered = append(filtered, e)
		}
	}

	if n <= 0 || len(filtered) == 0 {
		return filtered
	}

	start := 0
	if len(filtered) > n {
		start = len(filtered) - n
	}

	return filtered[start:]
}

func NewProcessManager(cfg *config.SupervisorConfig) *ProcessManager {
	pm := &ProcessManager{
		processes: make(map[string]*ProcessState),
		logs:      NewLogBuffer(1000),
	}

	for _, procCfg := range cfg.Processes {
		pm.processes[procCfg.Name] = &ProcessState{
			Config: procCfg,
			Status: "stopped",
		}
	}

	return pm
}

func (pm *ProcessManager) log(level, message string, processName string) {
	entry := models.LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Worker:    processName,
	}
	pm.logs.Add(entry)
}

func (pm *ProcessManager) StartProcess(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	state, ok := pm.processes[name]
	if !ok {
		return ErrProcessNotFound
	}

	if state.Status == "running" {
		return ErrProcessAlreadyRunning
	}

	ctx, cancel := context.WithCancel(context.Background())
	state.cancel = cancel

	cmd := exec.CommandContext(ctx, state.Config.Command, state.Config.Args...)

	if state.Config.Directory != "" {
		cmd.Dir = state.Config.Directory
	}

	if len(state.Config.Environment) > 0 {
		cmd.Env = os.Environ()
		for k, v := range state.Config.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		pm.log("error", fmt.Sprintf("Failed to create stdout pipe for %s: %v", name, err), name)
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		pm.log("error", fmt.Sprintf("Failed to create stderr pipe for %s: %v", name, err), name)
		return err
	}

	if err := cmd.Start(); err != nil {
		pm.log("error", fmt.Sprintf("Failed to start process %s: %v", name, err), name)
		return err
	}

	state.Cmd = cmd
	state.Status = "running"
	state.Pid = cmd.Process.Pid
	state.StartTime = time.Now()
	state.ExitCode = 0

	pm.log("info", fmt.Sprintf("Process %s started with PID %d", name, state.Pid), name)

	// Read stdout in goroutine
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			pm.log("info", fmt.Sprintf("[%s] %s", name, scanner.Text()), name)
		}
	}()

	// Read stderr in goroutine
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			pm.log("error", fmt.Sprintf("[%s] %s", name, scanner.Text()), name)
		}
	}()

	// Monitor process in goroutine
	go pm.monitorProcess(name, state)

	return nil
}

func (pm *ProcessManager) monitorProcess(name string, state *ProcessState) {
	if state.Cmd == nil {
		return
	}

	err := state.Cmd.Wait()

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if state.Cmd.ProcessState != nil {
		state.ExitCode = state.Cmd.ProcessState.ExitCode()
	}

	state.Status = "stopped"
	state.Pid = 0

	if err != nil {
		pm.log("warning", fmt.Sprintf("Process %s exited with error: %v", name, err), name)
	} else {
		pm.log("info", fmt.Sprintf("Process %s exited normally", name), name)
	}

	// Auto-restart if configured
	if state.Config.AutoRestart && state.cancel != nil {
		select {
		case <-time.After(time.Duration(state.Config.StartSecs) * time.Second):
			pm.mu.Unlock()
			pm.log("info", fmt.Sprintf("Auto-restarting process %s", name), name)
			pm.StartProcess(name)
			pm.mu.Lock()
		default:
		}
	}
}

func (pm *ProcessManager) StopProcess(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	state, ok := pm.processes[name]
	if !ok {
		return ErrProcessNotFound
	}

	if state.Status != "running" || state.Cmd == nil || state.Cmd.Process == nil {
		return ErrProcessNotRunning
	}

	// Cancel context to stop auto-restart
	if state.cancel != nil {
		state.cancel()
		state.cancel = nil
	}

	// Send signal
	var sig syscall.Signal
	switch state.Config.StopSignal {
	case "SIGKILL":
		sig = syscall.SIGKILL
	case "SIGINT":
		sig = syscall.SIGINT
	default:
		sig = syscall.SIGTERM
	}

	pm.log("info", fmt.Sprintf("Sending %s to process %s (PID %d)", state.Config.StopSignal, name, state.Pid), name)

	if err := state.Cmd.Process.Signal(sig); err != nil {
		pm.log("error", fmt.Sprintf("Failed to send signal to %s: %v", name, err), name)
		return err
	}

	// Wait for process to stop with timeout
	done := make(chan struct{})
	go func() {
		state.Cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		pm.log("info", fmt.Sprintf("Process %s stopped", name), name)
	case <-time.After(time.Duration(state.Config.StopTimeout) * time.Second):
		pm.log("warning", fmt.Sprintf("Process %s did not stop in time, killing", name), name)
		state.Cmd.Process.Kill()
	}

	state.Status = "stopped"
	state.Pid = 0

	return nil
}

func (pm *ProcessManager) RestartProcess(name string) error {
	pm.mu.RLock()
	state, ok := pm.processes[name]
	isRunning := ok && state.Status == "running"
	pm.mu.RUnlock()

	if !ok {
		return ErrProcessNotFound
	}

	if isRunning {
		if err := pm.StopProcess(name); err != nil && !errors.Is(err, ErrProcessNotRunning) {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}

	return pm.StartProcess(name)
}

func (pm *ProcessManager) GetProcesses() []models.Process {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]models.Process, 0, len(pm.processes))
	for name, state := range pm.processes {
		uptime := "N/A"
		if state.Status == "running" && !state.StartTime.IsZero() {
			uptime = formatDuration(time.Since(state.StartTime))
		}

		result = append(result, models.Process{
			Name:   name,
			Status: state.Status,
			Pid:    state.Pid,
			Uptime: uptime,
			Memory: "N/A", // TODO: implement memory tracking
			CPU:    "N/A", // TODO: implement CPU tracking
		})
	}

	return result
}

func (pm *ProcessManager) GetProcess(name string) (models.Process, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	state, ok := pm.processes[name]
	if !ok {
		return models.Process{}, false
	}

	uptime := "N/A"
	if state.Status == "running" && !state.StartTime.IsZero() {
		uptime = formatDuration(time.Since(state.StartTime))
	}

	return models.Process{
		Name:   name,
		Status: state.Status,
		Pid:    state.Pid,
		Uptime: uptime,
		Memory: "N/A",
		CPU:    "N/A",
	}, true
}

func (pm *ProcessManager) GetLogs(limit int) []models.LogEntry {
	return pm.logs.GetLast(limit)
}

func (pm *ProcessManager) GetLogsByProcess(processName string, limit int) []models.LogEntry {
	all := pm.logs.GetLast(limit * 10) // Get more to filter
	var filtered []models.LogEntry
	for _, e := range all {
		if e.Worker == processName {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered
}

func (pm *ProcessManager) StartAll() {
	pm.mu.RLock()
	var toStart []string
	for name, state := range pm.processes {
		if state.Config.AutoStart {
			toStart = append(toStart, name)
		}
	}
	pm.mu.RUnlock()

	for _, name := range toStart {
		pm.log("info", fmt.Sprintf("Auto-starting process %s", name), name)
		if err := pm.StartProcess(name); err != nil {
			pm.log("error", fmt.Sprintf("Failed to auto-start %s: %v", name, err), name)
		}
	}
}

func (pm *ProcessManager) StopAll() {
	pm.mu.RLock()
	var toStop []string
	for name, state := range pm.processes {
		if state.Status == "running" {
			toStop = append(toStop, name)
		}
	}
	pm.mu.RUnlock()

	for _, name := range toStop {
		pm.log("info", fmt.Sprintf("Stopping process %s", name), name)
		if err := pm.StopProcess(name); err != nil {
			pm.log("error", fmt.Sprintf("Failed to stop %s: %v", name, err), name)
		}
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour

	hours := d / time.Hour
	d -= hours * time.Hour

	minutes := d / time.Minute
	d -= minutes * time.Minute

	seconds := d / time.Second

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
