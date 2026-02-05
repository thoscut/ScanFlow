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
- **OCR Processing** - Tesseract-based OCR (German + English)
- **PDF Generation** - PDF/A-2b compliant output
- **WebSocket Live Updates** - Real-time scan progress in client
- **Terminal UI** - Interactive Bubbletea-based TUI for the client
- **Web UI** - Browser-based scanner control interface
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

## Configuration

### Server (`configs/server.toml`)

```toml
[server]
host = "0.0.0.0"
port = 8080

[server.auth]
enabled = true
api_keys = ["sk_live_..."]

[scanner]
device = ""  # Auto-detect

[scanner.defaults]
resolution = 300
mode = "color"
source = "adf_duplex"

[button]
enabled = true
poll_interval = "50ms"
long_press_duration = "1s"
short_press_profile = "standard"
long_press_profile = "oversize"

[output.paperless]
enabled = true
url = "https://paperless.local"
token_file = "/etc/scanflow/paperless_token"

[output.smb]
enabled = true
server = "nas.local"
share = "scans"
```

### Client (`~/.config/scanflow/client.toml`)

```toml
[server]
url = "http://scanserver.local:8080"
api_key = "sk_live_..."

[defaults]
profile = "standard"
output = "paperless"
```

### Scan Profiles

Profiles are defined in `configs/profiles/`:

| Profile | Resolution | Mode | Page Height | Use Case |
|---------|-----------|------|-------------|----------|
| `standard` | 300 DPI | Color | 420mm (A4) | Normal documents |
| `oversize` | 200 DPI | Grayscale | Unlimited | Long receipts, folded documents |
| `photo` | 600 DPI | Color | A4 | Photos and high-quality scans |

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/scanner/devices` | List available scanners |
| `POST` | `/api/v1/scanner/devices/{id}/open` | Open a scanner |
| `POST` | `/api/v1/scan` | Start a scan job |
| `GET` | `/api/v1/scan/{job_id}` | Get job status |
| `POST` | `/api/v1/scan/{job_id}/continue` | Scan more pages |
| `POST` | `/api/v1/scan/{job_id}/finish` | Finish and process job |
| `DELETE` | `/api/v1/scan/{job_id}/pages/{n}` | Delete a page |
| `GET` | `/api/v1/ws` | WebSocket for live updates |
| `GET` | `/api/v1/health` | Health check |

All endpoints (except health) require `Authorization: Bearer <api_key>` header.

## Deployment

### Systemd Service

```bash
make install-server
sudo systemctl enable --now scanflow
```

### Docker

```bash
make docker-build
docker-compose -f deploy/docker/docker-compose.yml up -d
```

## Development

### Tests

```bash
# Run all tests
make test

# Server tests only
make test-server

# Client tests only
make test-client

# Integration tests (requires SANE test device)
make test-integration
```

### Code Quality

```bash
make vet    # Go vet
make fmt    # Format code
```

### Project Structure

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
