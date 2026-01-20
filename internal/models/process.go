package models

// Process represents a supervised process
type Process struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Pid       int      `json:"pid"`
	Uptime    string   `json:"uptime"`
	Memory    string   `json:"memory"`
	CPU       string   `json:"cpu"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Directory string   `json:"directory"`
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Level     string `json:"level"`
	Worker    string `json:"worker,omitempty"`
}
