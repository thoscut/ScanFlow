# ScanFlow

Client-Server system for network document scanning with Paperless-NGX integration. The server runs on Linux (Raspberry Pi) with SANE, clients are cross-platform (Windows/macOS/Linux).

```
[Scanner] <--USB--> [Raspberry Pi + SANE] <--REST API--> [Client (Win/Mac/Linux)]
                            |
                  [Paperless-NGX / NAS]
```

## Features

- **SANE Scanner Integration** - Works with any SANE-compatible scanner (tested with Fujitsu ScanSnap)
- **Hardware Button Support** - Short press for standard scan, long press for oversize documents
- **Paperless-NGX Upload** - Direct upload with metadata, tags, and document types
- **SMB/CIFS Output** - Save scans directly to network shares
- **Multi-page Scanning** - ADF duplex support with page management
- **Optional OCR Processing** - Tesseract-based OCR (can be disabled per-scan or globally, e.g. when Paperless handles OCR)
- **PDF Generation** - PDF/A-2b compliant output
- **WebSocket Live Updates** - Real-time scan progress in client
- **Web UI** - Browser-based scanner control with settings management
- **Terminal UI** - Interactive Bubbletea-based TUI for the client
- **REST API** - Full API for custom integrations
- **Cross-platform Client** - Builds for Linux, macOS, and Windows

## Quick Start

### Prerequisites

**Server (Raspberry Pi / Linux):**

```bash
sudo apt install sane-utils libsane-dev tesseract-ocr tesseract-ocr-deu
```

**Client:** No system dependencies required.

### Build

```bash
# Build both server and client
make all

# Or build individually
make build-server
make build-client

# Cross-compile client for all platforms
make build-client-all

# Cross-compile server for ARM64 (Raspberry Pi)
make build-server-arm64
```

### Server Setup

```bash
# Test scanner detection
scanimage -L

# Configure
cp configs/server.toml /etc/scanflow/server.toml
# Edit /etc/scanflow/server.toml with your settings

# Run
./dist/scanflow-server -config /etc/scanflow/server.toml
```

### Client Setup

```bash
# Configure server connection
./dist/scanflow config set server.url http://scanserver.local:8080
./dist/scanflow config set server.api_key sk_live_...

# Start a scan
./dist/scanflow scan --profile standard --output paperless

# Interactive TUI mode
./dist/scanflow scan --interactive
```

## Project Structure

```
scanflow/
├── server/                 # Server (Go)
│   ├── cmd/scanflow-server/
│   ├── internal/
│   │   ├── api/           # REST API + WebSocket
│   │   ├── scanner/       # SANE integration
│   │   ├── processor/     # PDF + OCR pipeline
│   │   ├── output/        # Paperless, SMB, filesystem, email
│   │   ├── config/        # Configuration
│   │   └── jobs/          # Job queue
│   └── web/               # Web UI (HTML/CSS/JS)
├── client/                 # Client (Go)
│   ├── cmd/scanflow/
│   └── internal/
│       ├── client/        # API client
│       ├── cli/           # Cobra commands
│       ├── tui/           # Bubbletea TUI
│       └── config/
├── configs/               # Example configurations
├── deploy/                # Systemd, Docker, Ansible
└── docs/                  # Documentation
```
