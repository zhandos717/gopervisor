package storage

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sql.DB
}

// CrashRecord represents a process crash event
type CrashRecord struct {
	ID          int64     `json:"id"`
	ProcessName string    `json:"process_name"`
	ExitCode    int       `json:"exit_code"`
	Signal      string    `json:"signal,omitempty"`
	ErrorMsg    string    `json:"error_message,omitempty"`
	Stdout      string    `json:"stdout,omitempty"`
	Stderr      string    `json:"stderr,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CrashedAt   time.Time `json:"crashed_at"`
	Uptime      string    `json:"uptime"`
}

// Settings represents user settings
type Settings struct {
	ID        int64  `json:"id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

// ErrorLog represents a system error log
type ErrorLog struct {
	ID        int64     `json:"id"`
	Level     string    `json:"level"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func New(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}

	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS crashes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		process_name TEXT NOT NULL,
		exit_code INTEGER,
		signal TEXT,
		error_message TEXT,
		stdout TEXT,
		stderr TEXT,
		started_at DATETIME,
		crashed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		uptime TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_crashes_process ON crashes(process_name);
	CREATE INDEX IF NOT EXISTS idx_crashes_time ON crashes(crashed_at DESC);

	CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT UNIQUE NOT NULL,
		value TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS error_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT NOT NULL,
		source TEXT,
		message TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_errors_time ON error_logs(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_errors_level ON error_logs(level);
	`

	_, err := s.db.Exec(schema)
	return err
}

func (s *Storage) Close() error {
	return s.db.Close()
}

// Crash operations

func (s *Storage) SaveCrash(crash *CrashRecord) error {
	query := `
		INSERT INTO crashes (process_name, exit_code, signal, error_message, stdout, stderr, started_at, crashed_at, uptime)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := s.db.Exec(query,
		crash.ProcessName,
		crash.ExitCode,
		crash.Signal,
		crash.ErrorMsg,
		crash.Stdout,
		crash.Stderr,
		crash.StartedAt,
		crash.CrashedAt,
		crash.Uptime,
	)
	if err != nil {
		return err
	}

	crash.ID, _ = result.LastInsertId()
	return nil
}

func (s *Storage) GetCrashes(limit int) ([]CrashRecord, error) {
	query := `
		SELECT id, process_name, exit_code, signal, error_message, stdout, stderr, started_at, crashed_at, uptime
		FROM crashes
		ORDER BY crashed_at DESC
		LIMIT ?
	`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var crashes []CrashRecord
	for rows.Next() {
		var c CrashRecord
		var signal, errMsg, stdout, stderr sql.NullString
		var startedAt, crashedAt sql.NullTime
		var uptime sql.NullString

		err := rows.Scan(&c.ID, &c.ProcessName, &c.ExitCode, &signal, &errMsg, &stdout, &stderr, &startedAt, &crashedAt, &uptime)
		if err != nil {
			return nil, err
		}

		c.Signal = signal.String
		c.ErrorMsg = errMsg.String
		c.Stdout = stdout.String
		c.Stderr = stderr.String
		if startedAt.Valid {
			c.StartedAt = startedAt.Time
		}
		if crashedAt.Valid {
			c.CrashedAt = crashedAt.Time
		}
		c.Uptime = uptime.String

		crashes = append(crashes, c)
	}

	return crashes, rows.Err()
}

func (s *Storage) GetCrashesByProcess(processName string, limit int) ([]CrashRecord, error) {
	query := `
		SELECT id, process_name, exit_code, signal, error_message, stdout, stderr, started_at, crashed_at, uptime
		FROM crashes
		WHERE process_name = ?
		ORDER BY crashed_at DESC
		LIMIT ?
	`
	rows, err := s.db.Query(query, processName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var crashes []CrashRecord
	for rows.Next() {
		var c CrashRecord
		var signal, errMsg, stdout, stderr sql.NullString
		var startedAt, crashedAt sql.NullTime
		var uptime sql.NullString

		err := rows.Scan(&c.ID, &c.ProcessName, &c.ExitCode, &signal, &errMsg, &stdout, &stderr, &startedAt, &crashedAt, &uptime)
		if err != nil {
			return nil, err
		}

		c.Signal = signal.String
		c.ErrorMsg = errMsg.String
		c.Stdout = stdout.String
		c.Stderr = stderr.String
		if startedAt.Valid {
			c.StartedAt = startedAt.Time
		}
		if crashedAt.Valid {
			c.CrashedAt = crashedAt.Time
		}
		c.Uptime = uptime.String

		crashes = append(crashes, c)
	}

	return crashes, rows.Err()
}

func (s *Storage) GetCrashStats() (map[string]int, error) {
	query := `
		SELECT process_name, COUNT(*) as count
		FROM crashes
		GROUP BY process_name
		ORDER BY count DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, err
		}
		stats[name] = count
	}

	return stats, rows.Err()
}

// Settings operations

func (s *Storage) GetSetting(key string) (string, error) {
	var value sql.NullString
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value.String, nil
}

func (s *Storage) SetSetting(key, value string) error {
	query := `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(query, key, value)
	return err
}

func (s *Storage) GetAllSettings() (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key string
		var value sql.NullString
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value.String
	}

	return settings, rows.Err()
}

// Error log operations

func (s *Storage) SaveError(level, source, message string) error {
	query := `INSERT INTO error_logs (level, source, message) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, level, source, message)
	return err
}

func (s *Storage) GetErrors(limit int) ([]ErrorLog, error) {
	query := `
		SELECT id, level, source, message, created_at
		FROM error_logs
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errors []ErrorLog
	for rows.Next() {
		var e ErrorLog
		var source sql.NullString
		if err := rows.Scan(&e.ID, &e.Level, &source, &e.Message, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Source = source.String
		errors = append(errors, e)
	}

	return errors, rows.Err()
}

func (s *Storage) GetErrorsByLevel(level string, limit int) ([]ErrorLog, error) {
	query := `
		SELECT id, level, source, message, created_at
		FROM error_logs
		WHERE level = ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := s.db.Query(query, level, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errors []ErrorLog
	for rows.Next() {
		var e ErrorLog
		var source sql.NullString
		if err := rows.Scan(&e.ID, &e.Level, &source, &e.Message, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Source = source.String
		errors = append(errors, e)
	}

	return errors, rows.Err()
}

func (s *Storage) ClearOldErrors(daysToKeep int) error {
	query := `DELETE FROM error_logs WHERE created_at < datetime('now', '-' || ? || ' days')`
	_, err := s.db.Exec(query, daysToKeep)
	return err
}

func (s *Storage) ClearOldCrashes(daysToKeep int) error {
	query := `DELETE FROM crashes WHERE crashed_at < datetime('now', '-' || ? || ' days')`
	_, err := s.db.Exec(query, daysToKeep)
	return err
}
