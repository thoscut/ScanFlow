# ScanFlow

Client-Server-System for network document scanning with Paperless-NGX integration. The server runs on Linux (Raspberry Pi) with SANE, clients are cross-platform (Windows/macOS/Linux).

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
- **Optional OCR Processing** - Tesseract-based OCR (can be disabled when e.g. Paperless handles OCR)
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
```

### Run

```bash
# Server
./dist/scanflow-server -config configs/server.toml

# Client
./dist/scanflow config set server.url http://scanserver.local:8080
./dist/scanflow scan --profile standard --output paperless
```

## Documentation

Full documentation is available at [docs/](docs/):

- [Installation Guide](docs/installation.md)
- [Configuration Reference](docs/configuration.md)
- [API Reference](docs/api-reference.md)

Build and serve docs locally:

```bash
make docs-serve
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

## License

All rights reserved.
