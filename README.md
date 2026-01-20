<p align="center">
  <h1 align="center">Pupervisor</h1>
  <p align="center">
    Lightweight process manager with modern web UI, written in Go
    <br />
    <a href="#features">Features</a>
    ·
    <a href="#installation">Installation</a>
    ·
    <a href="#screenshots">Screenshots</a>
    ·
    <a href="#api-reference">API</a>
  </p>
</p>

<p align="center">
  <img src="docs/images/dashboard.png" alt="Dashboard" width="800">
</p>

## About

Pupervisor is a supervisor-like process manager with a modern web interface. It allows you to start, stop, restart processes and view their logs in real-time. All static files are embedded into the binary - you only need one executable file to run.

## Features

- **Dashboard** — System overview with charts (status distribution, hourly activity)
- **Process Management** — Start, stop, restart with live stdout/stderr viewing
- **Bulk Operations** — Restart selected or all running processes at once
- **Search & Filter** — Quick process search by name and status filtering
- **Logs** — Worker and system logs with level filtering and worker badges
- **Crash History** — Track process crashes with exit codes and stderr output
- **SQLite Storage** — Persistent storage for crashes and settings
- **Settings** — Web-based configuration
- **No External Dependencies** — Custom CSS/JS, no CDN required
- **Single Binary** — All assets embedded, just run and go

## Screenshots

<details>
<summary>📊 Dashboard</summary>
<br>
<img src="docs/images/dashboard.png" alt="Dashboard" width="700">
<p>System overview with statistics, status distribution chart, activity graph, process list and recent logs.</p>
</details>

<details>
<summary>⚙️ Process Management</summary>
<br>
<img src="docs/images/processes.png" alt="Processes" width="700">
<p>Process cards with metrics (PID, Uptime, Memory, CPU), bulk selection and restart functionality.</p>
</details>

<details>
<summary>📋 Logs</summary>
<br>
<img src="docs/images/logs.png" alt="Logs" width="700">
<p>Worker and system logs with color-coded worker badges, level filtering, and worker filtering.</p>
</details>

<details>
<summary>🖥️ Process Output</summary>
<br>
<img src="docs/images/process-output.png" alt="Process Output" width="700">
<p>Real-time process output viewing with auto-scroll.</p>
</details>

## Installation

### Requirements

- Go 1.21 or higher

### From Source

```bash
# Clone the repository
git clone https://github.com/zhandos717/pupervisor
cd pupervisor

# Build
make build

# Run
./pupervisor --config pupervisor.yaml
```

### Using Go Install

```bash
go install github.com/zhandos717/pupervisor/cmd/server@latest
```

### Docker

```bash
# Using docker-compose
docker-compose up -d

# Or build manually
docker build -t pupervisor .
docker run -d -p 8080:8080 -v ./pupervisor.yaml:/app/config/pupervisor.yaml pupervisor
```

Open your browser: http://localhost:8080

## Configuration

Create a `pupervisor.yaml` file:

```yaml
processes:
  - name: my-worker
    command: python
    args:
      - worker.py
    directory: /app
    environment:
      PYTHONUNBUFFERED: "1"
    autostart: true
    autorestart: true
    startsecs: 3
    stopsignal: SIGTERM
    stoptimeout: 10

  - name: queue-processor
    command: php
    args:
      - artisan
      - queue:work
      - --sleep=3
    directory: /var/www/app
    autostart: true
    autorestart: true
```

### Process Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `name` | string | required | Process name |
| `command` | string | required | Command to execute |
| `args` | []string | [] | Command arguments |
| `directory` | string | "" | Working directory |
| `environment` | map | {} | Environment variables |
| `autostart` | bool | false | Start on supervisor launch |
| `autorestart` | bool | false | Restart on exit |
| `startsecs` | int | 1 | Seconds before considered started |
| `stopsignal` | string | SIGTERM | Signal to stop (SIGTERM, SIGINT, SIGKILL) |
| `stoptimeout` | int | 10 | Seconds to wait before SIGKILL |

## API Reference

### Processes

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/processes` | List all processes |
| POST | `/api/processes/{name}/start` | Start process |
| POST | `/api/processes/{name}/stop` | Stop process |
| POST | `/api/processes/{name}/restart` | Restart process |
| POST | `/api/processes/restart-all` | Restart all running |
| POST | `/api/processes/restart-selected` | Restart selected (JSON body) |

### Logs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/logs` | All logs |
| GET | `/api/logs/worker` | Worker output logs |
| GET | `/api/logs/system` | System event logs |
| GET | `/api/logs/worker/{name}` | Logs for specific worker |

### Crashes

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/crashes` | Crash history |
| GET | `/api/crashes/stats` | Crash statistics |
| GET | `/api/crashes/{name}` | Crashes for process |

### Settings & Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/settings` | Get settings |
| POST | `/api/settings` | Update settings |
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |

## Project Structure

```
pupervisor/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── api/
│   │   └── router.go            # HTTP routing
│   ├── config/
│   │   ├── config.go            # App configuration
│   │   └── process_config.go    # Process configuration
│   ├── handlers/
│   │   ├── health_handler.go    # Health endpoints
│   │   ├── process_handler.go   # Process API handlers
│   │   └── template_handler.go  # HTML rendering
│   ├── middleware/
│   │   └── logging.go           # Request logging
│   ├── models/
│   │   └── process.go           # Data models
│   ├── service/
│   │   └── process_manager.go   # Process management logic
│   └── storage/
│       └── storage.go           # SQLite storage
├── web/
│   ├── css/
│   │   └── style.css            # Styles (no CDN)
│   ├── templates/
│   │   ├── dashboard.html
│   │   ├── processes.html
│   │   ├── logs.html
│   │   ├── crashes.html
│   │   └── settings.html
│   └── embed.go                 # Static file embedding
├── docs/
│   └── images/                  # Screenshots
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── pupervisor.yaml              # Process configuration
└── README.md
```

## Development

```bash
# Run in development mode
make run-dev

# Run tests
make test

# Run linter
make lint

# Build for all platforms
make build-all

# See all available commands
make help
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

Inspired by [Supervisor](http://supervisord.org/) - the original process control system.
