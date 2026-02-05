# ScanFlow - Netzwerk-Scanner-System

## Projektübersicht

**ScanFlow** ist ein Client-Server-System für Dokumentenscanner wie den Fujitsu ScanSnap S1500. Ein Linux-Server (z.B. Raspberry Pi) steuert den Scanner via SANE und stellt eine REST-API bereit. Plattformunabhängige Clients können Scans auslösen, konfigurieren und an Paperless-NGX oder Netzwerkfreigaben weiterleiten.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Netzwerk                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────────────┐        ┌──────────────────────────────────┐  │
│  │   ScanFlow Server    │        │         ScanFlow Client          │  │
│  │   (Raspberry Pi)     │        │    (Windows/macOS/Linux/Web)     │  │
│  │                      │  REST  │                                  │  │
│  │  ┌────────────────┐  │◄──────►│  ┌────────────────────────────┐  │  │
│  │  │     SANE       │  │  API   │  │   CLI / TUI / Web-UI       │  │  │
│  │  │   + Scanner    │  │        │  └────────────────────────────┘  │  │
│  │  └────────────────┘  │        │                                  │  │
│  │          │           │        └──────────────────────────────────┘  │
│  │          ▼           │                                              │
│  │  ┌────────────────┐  │        ┌──────────────────────────────────┐  │
│  │  │  PDF Builder   │  │        │        Paperless-NGX             │  │
│  │  │  + OCR         │  │───────►│                                  │  │
│  │  └────────────────┘  │ Upload │                                  │  │
│  │          │           │        └──────────────────────────────────┘  │
│  │          ▼           │                                              │
│  │  ┌────────────────┐  │        ┌──────────────────────────────────┐  │
│  │  │ Output Handler │  │        │     NAS / Netzwerkfreigabe       │  │
│  │  │ (SMB/API/Mail) │  │───────►│        (SMB/CIFS)                │  │
│  │  └────────────────┘  │        │                                  │  │
│  └──────────────────────┘        └──────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Systemkomponenten

### 1. ScanFlow Server (Go)

Der Server läuft auf einem Linux-System (Raspberry Pi, NAS, etc.) und bietet:

- **Scanner-Steuerung** via SANE
- **REST-API** für Remote-Zugriff
- **PDF-Erstellung** mit optionaler OCR
- **Ausgabe-Handler** für Paperless-NGX, SMB, lokale Ablage
- **Hardware-Button-Support** (optional, via scanbd)

### 2. ScanFlow Client (Go)

Plattformunabhängiger Client (Windows, macOS, Linux):

- **CLI** für Scripting und Automatisierung
- **TUI** (Terminal UI) für interaktive Bedienung
- **Web-UI** (optional, vom Server gehostet)

### 3. Ausgabeziele

| Ziel | Beschreibung | Konfiguration |
|------|--------------|---------------|
| **Paperless-NGX** | Direkter API-Upload | URL + Token |
| **Netzwerkfreigabe** | SMB/CIFS-Share | Server/Share/Credentials |
| **Paperless Consume** | Ablage in Consume-Ordner | Pfad (lokal oder SMB) |
| **Lokaler Server** | Ablage auf dem Server | Pfad |
| **E-Mail** | Versand als Anhang | SMTP-Konfiguration |

---

## Server-Architektur

### Verzeichnisstruktur

```
scanflow-server/
├── cmd/
│   └── scanflow-server/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── server.go          # HTTP-Server + Router
│   │   ├── handlers.go        # API-Endpoints
│   │   ├── middleware.go      # Auth, Logging, CORS
│   │   └── websocket.go       # Live-Updates (Scan-Fortschritt)
│   ├── scanner/
│   │   ├── sane.go            # SANE-Wrapper
│   │   ├── device.go          # Device-Management
│   │   ├── options.go         # Scanner-Optionen
│   │   └── batch.go           # Batch-Scanning
│   ├── processor/
│   │   ├── pipeline.go        # Verarbeitungs-Pipeline
│   │   ├── image.go           # Bildoptimierung
│   │   ├── pdf.go             # PDF-Erstellung
│   │   └── ocr.go             # Tesseract-Integration
│   ├── output/
│   │   ├── handler.go         # Interface
│   │   ├── paperless.go       # Paperless-NGX API
│   │   ├── smb.go             # SMB/CIFS
│   │   ├── filesystem.go      # Lokale Ablage
│   │   └── email.go           # E-Mail-Versand
│   ├── config/
│   │   ├── config.go          # Konfigurationsverwaltung
│   │   └── profiles.go        # Scan-Profile
│   └── jobs/
│       ├── queue.go           # Job-Queue
│       └── job.go             # Job-Definition
├── web/                       # Eingebettete Web-UI (optional)
│   ├── static/
│   └── templates/
├── configs/
│   ├── server.toml            # Server-Konfiguration
│   └── profiles/
│       ├── standard.toml
│       ├── photo.toml
│       └── oversize.toml
├── go.mod
└── Makefile
```

### REST-API Endpoints

```yaml
# Scanner-Verwaltung
GET    /api/v1/scanner/devices          # Verfügbare Scanner auflisten
GET    /api/v1/scanner/devices/{id}     # Scanner-Details + Optionen
POST   /api/v1/scanner/devices/{id}/open   # Scanner öffnen
DELETE /api/v1/scanner/devices/{id}/close  # Scanner schließen

# Scan-Operationen
POST   /api/v1/scan                     # Scan starten
GET    /api/v1/scan/{job_id}            # Job-Status abfragen
DELETE /api/v1/scan/{job_id}            # Job abbrechen
GET    /api/v1/scan/{job_id}/preview    # Vorschau-Bilder abrufen
POST   /api/v1/scan/{job_id}/continue   # Weitere Seiten scannen
POST   /api/v1/scan/{job_id}/finish     # Scan abschließen → PDF erstellen

# Seiten-Management (während Scan-Job)
GET    /api/v1/scan/{job_id}/pages      # Alle gescannten Seiten
DELETE /api/v1/scan/{job_id}/pages/{n}  # Seite löschen
POST   /api/v1/scan/{job_id}/pages/{n}/rescan  # Seite neu scannen
POST   /api/v1/scan/{job_id}/pages/reorder     # Seiten umsortieren

# Ausgabe
GET    /api/v1/outputs                  # Konfigurierte Ausgabeziele
POST   /api/v1/scan/{job_id}/send       # An Ausgabeziel senden

# Profile
GET    /api/v1/profiles                 # Verfügbare Profile
GET    /api/v1/profiles/{name}          # Profil-Details
POST   /api/v1/profiles                 # Profil erstellen
PUT    /api/v1/profiles/{name}          # Profil aktualisieren

# System
GET    /api/v1/status                   # Server-Status
GET    /api/v1/health                   # Health-Check
WS     /api/v1/ws                       # WebSocket für Live-Updates
```

### API-Datenmodelle

```go
// Scan-Anfrage
type ScanRequest struct {
    Profile     string            `json:"profile,omitempty"`      // Profil-Name
    DeviceID    string            `json:"device_id,omitempty"`    // Scanner-ID (optional)
    Options     *ScanOptions      `json:"options,omitempty"`      // Überschreibt Profil
    Output      *OutputConfig     `json:"output,omitempty"`       // Ausgabeziel
    Metadata    *DocumentMetadata `json:"metadata,omitempty"`     // Dokument-Metadaten
}

// Scanner-Optionen
type ScanOptions struct {
    Resolution  int     `json:"resolution"`   // DPI: 75, 150, 200, 300, 600
    Mode        string  `json:"mode"`         // color, gray, lineart
    Source      string  `json:"source"`       // adf_front, adf_back, adf_duplex, flatbed
    PageWidth   float64 `json:"page_width"`   // mm, 0 = auto
    PageHeight  float64 `json:"page_height"`  // mm, 0 = unlimited (Überlänge)
    Brightness  int     `json:"brightness"`   // -100 bis 100
    Contrast    int     `json:"contrast"`     // -100 bis 100
}

// Job-Status
type ScanJob struct {
    ID          string       `json:"id"`
    Status      JobStatus    `json:"status"`      // pending, scanning, processing, completed, failed
    Profile     string       `json:"profile"`
    Pages       []PageInfo   `json:"pages"`
    Progress    int          `json:"progress"`    // 0-100
    Error       string       `json:"error,omitempty"`
    CreatedAt   time.Time    `json:"created_at"`
    UpdatedAt   time.Time    `json:"updated_at"`
}

// Dokument-Metadaten (für Paperless)
type DocumentMetadata struct {
    Title           string   `json:"title,omitempty"`
    Created         string   `json:"created,omitempty"`          // ISO 8601
    Correspondent   int      `json:"correspondent,omitempty"`    // Paperless ID
    DocumentType    int      `json:"document_type,omitempty"`    // Paperless ID
    Tags            []int    `json:"tags,omitempty"`             // Paperless IDs
    ArchiveSerial   string   `json:"archive_serial_number,omitempty"`
}
```

---

## Server-Konfiguration

### Haupt-Konfiguration (`server.toml`)

```toml
[server]
host = "0.0.0.0"
port = 8080
base_url = "http://scanserver.local:8080"

[server.auth]
enabled = true
# API-Key Authentifizierung
api_keys = [
    "sk_live_abc123...",  # Produktiv-Key
    "sk_dev_xyz789...",   # Entwicklungs-Key
]
# Optional: Basic Auth für Web-UI
basic_auth_user = "admin"
basic_auth_password_hash = "$2a$10$..."  # bcrypt

[server.tls]
enabled = false
cert_file = "/etc/scanflow/cert.pem"
key_file = "/etc/scanflow/key.pem"

[scanner]
device = ""  # Leer = Auto-Detect, oder "fujitsu:ScanSnap S1500:*"
auto_open = true  # Scanner beim Start öffnen

[scanner.defaults]
resolution = 300
mode = "color"
source = "adf_duplex"
page_width = 210.0   # A4
page_height = 297.0  # 0 für Überlängen

# --- Hardware-Button (Kurz/Lang-Druck-Erkennung) ---

[button]
enabled = true
poll_interval = "50ms"          # Schnelles Polling für präzise Zeitmessung
long_press_duration = "1s"      # Ab 1 Sekunde = langer Druck

# Kurzer Tastendruck → Normal-Scan (Seitenhöhe begrenzt)
short_press_profile = "standard"

# Langer Tastendruck → Überlängen-Scan (unbegrenzte Seitenhöhe)
long_press_profile = "oversize"

# Gemeinsames Ausgabeziel
output = "paperless"            # paperless, smb, paperless_consume

# Akustisches Feedback wenn Schwelle für langen Druck erreicht
beep_on_long_press = true

# Automatische Metadaten
[button.metadata]
title_pattern = "Scan_{date}_{time}"  # {date} = 20240115, {time} = 143052
# Optionale feste Werte für Paperless:
# correspondent = 1
# document_type = 2
# tags = [1, 3]

[processing]
temp_directory = "/tmp/scanflow"
max_concurrent_jobs = 2

[processing.pdf]
format = "PDF/A-2b"
compression = "jpeg"
jpeg_quality = 85

[processing.ocr]
enabled = true
language = "deu+eng"
tesseract_path = "/usr/bin/tesseract"

[storage]
# Lokaler Speicher für fertige Dokumente
local_directory = "/var/lib/scanflow/documents"
retention_days = 30  # Automatische Bereinigung

# --- Ausgabeziele ---

[output.paperless]
enabled = true
url = "https://paperless.example.com"
token_file = "/etc/scanflow/paperless_token"
# Alternative: Token direkt (nicht empfohlen)
# token = "abc123..."
verify_ssl = true
default_correspondent = 0
default_document_type = 0
default_tags = []

[output.smb]
enabled = true
server = "//nas.local/scans"
share = "scans"
username = "scanner"
password_file = "/etc/scanflow/smb_password"
directory = "incoming"
filename_pattern = "{date}_{time}_{title}"

[output.paperless_consume]
# Paperless Consume-Ordner (via SMB oder lokal gemountet)
enabled = false
path = "/mnt/paperless/consume"
# Oder via SMB:
# smb_server = "//paperless.local/consume"

[output.email]
enabled = false
smtp_host = "smtp.example.com"
smtp_port = 587
smtp_user = "scanner@example.com"
smtp_password_file = "/etc/scanflow/smtp_password"
from_address = "scanner@example.com"
default_recipient = ""

# --- Logging ---

[logging]
level = "info"  # debug, info, warn, error
format = "json"  # json, text
file = "/var/log/scanflow/server.log"
```

### Scan-Profile (`profiles/standard.toml`)

```toml
[profile]
name = "Standard Dokument"
description = "Farbscan 300 DPI, beidseitig, mit OCR"

[scanner]
resolution = 300
mode = "color"
source = "adf_duplex"
page_width = 210.0
page_height = 297.0

[processing]
optimize_images = true
deskew = true
remove_blank_pages = true
blank_threshold = 0.99  # 99% weiß = leer

[processing.ocr]
enabled = true
language = "deu"

[output]
default_target = "paperless"
```

### Profil für Überlängen (`profiles/oversize.toml`)

```toml
[profile]
name = "Überlänge"
description = "Für Dokumente länger als A4 (Kontoauszüge, etc.)"

[scanner]
resolution = 200
mode = "gray"
source = "adf_duplex"
page_width = 210.0
page_height = 0  # Unbegrenzt!

[processing]
optimize_images = true
deskew = true
remove_blank_pages = false  # Bei Überlängen nicht empfohlen

# Optional: Lange Seiten aufteilen
[processing.split]
enabled = false
threshold_mm = 350  # Ab dieser Länge aufteilen
overlap_mm = 10     # Überlappung zwischen Teilen

[output]
default_target = "smb"
```

---

## Client-Architektur

### Verzeichnisstruktur

```
scanflow-client/
├── cmd/
│   └── scanflow/
│       └── main.go
├── internal/
│   ├── client/
│   │   ├── api.go             # API-Client
│   │   ├── websocket.go       # WebSocket für Live-Updates
│   │   └── auth.go            # Authentifizierung
│   ├── cli/
│   │   ├── root.go            # Cobra Root-Command
│   │   ├── scan.go            # scan Command
│   │   ├── devices.go         # devices Command
│   │   ├── profiles.go        # profiles Command
│   │   ├── status.go          # status Command
│   │   └── config.go          # config Command
│   ├── tui/
│   │   ├── app.go             # Bubbletea App
│   │   ├── scan_view.go       # Scan-Ansicht
│   │   └── preview_view.go    # Vorschau-Ansicht
│   └── config/
│       └── config.go          # Client-Konfiguration
├── configs/
│   └── client.toml.example
├── go.mod
└── Makefile
```

### Client-Konfiguration (`client.toml`)

```toml
[server]
url = "http://scanserver.local:8080"
api_key = "sk_live_abc123..."
# Oder aus Datei:
# api_key_file = "~/.config/scanflow/api_key"

[defaults]
profile = "standard"
output = "paperless"
interactive = false

[tui]
theme = "dark"  # dark, light
preview_quality = "medium"  # low, medium, high
```

### CLI-Befehle

```bash
# Server-Status prüfen
scanflow status

# Verfügbare Scanner auflisten
scanflow devices
scanflow devices --json

# Scan mit Standard-Profil starten
scanflow scan

# Scan mit spezifischem Profil
scanflow scan --profile dokument

# Scan mit Überlänge
scanflow scan --profile oversize

# Scan mit Metadaten für Paperless
scanflow scan --title "Rechnung 2024-001" \
              --correspondent 5 \
              --document-type 2 \
              --tags 1,3,7

# Interaktiver Modus (mehrere Stapel, Vorschau, etc.)
scanflow scan --interactive
scanflow scan -i

# Direkt an Paperless senden
scanflow scan --output paperless

# An Netzwerkfreigabe senden
scanflow scan --output smb --filename "Vertrag_Müller"

# An mehrere Ziele senden
scanflow scan --output paperless,smb

# Profile verwalten
scanflow profiles list
scanflow profiles show standard
scanflow profiles create mein-profil

# Konfiguration
scanflow config show
scanflow config set server.url http://192.168.1.100:8080

# TUI starten
scanflow tui
```

---

## Workflow: Interaktiver Scan

```
┌─────────────────────────────────────────────────────────────────┐
│  Client: scanflow scan -i                                       │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│  POST /api/v1/scan                                              │
│  { "profile": "standard" }                                      │
│  → Server erstellt Job, startet Scan                            │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│  WebSocket: Live-Updates                                        │
│  { "type": "progress", "job_id": "...", "page": 1 }            │
│  { "type": "page_complete", "preview_url": "..." }             │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│  ADF leer → Server sendet:                                      │
│  { "type": "feeder_empty", "pages_scanned": 5 }                │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│  Client zeigt Optionen:                                         │
│                                                                 │
│  ✓ 5 Seiten gescannt                                           │
│                                                                 │
│  [W] Weitere Seiten scannen                                     │
│  [V] Vorschau anzeigen                                          │
│  [D] Seite löschen                                              │
│  [R] Seite neu scannen                                          │
│  [F] Fertig → PDF erstellen                                     │
│  [A] Abbrechen                                                  │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼ [F] Fertig
┌─────────────────────────────────────────────────────────────────┐
│  POST /api/v1/scan/{job_id}/finish                             │
│  { "output": "paperless", "metadata": { "title": "..." } }     │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│  Server:                                                        │
│  1. Bilder optimieren (deskew, blank removal)                  │
│  2. PDF erstellen                                               │
│  3. OCR durchführen                                             │
│  4. An Paperless-NGX senden                                     │
│  5. Cleanup                                                     │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│  WebSocket: { "type": "completed", "paperless_task_id": "..." }│
│  Client: ✓ Dokument erfolgreich an Paperless übertragen        │
└─────────────────────────────────────────────────────────────────┘
```

---

## Paperless-NGX Integration

### Direkte API-Integration

```go
// internal/output/paperless.go

type PaperlessClient struct {
    baseURL    string
    token      string
    httpClient *http.Client
}

func (c *PaperlessClient) Upload(ctx context.Context, doc *Document) (*UploadResult, error) {
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    
    // Dokument-Datei
    part, _ := writer.CreateFormFile("document", doc.Filename)
    io.Copy(part, doc.Reader)
    
    // Metadaten
    if doc.Title != "" {
        writer.WriteField("title", doc.Title)
    }
    if doc.Correspondent > 0 {
        writer.WriteField("correspondent", strconv.Itoa(doc.Correspondent))
    }
    if doc.DocumentType > 0 {
        writer.WriteField("document_type", strconv.Itoa(doc.DocumentType))
    }
    for _, tag := range doc.Tags {
        writer.WriteField("tags", strconv.Itoa(tag))
    }
    if doc.Created != "" {
        writer.WriteField("created", doc.Created)
    }
    if doc.ArchiveSerial != "" {
        writer.WriteField("archive_serial_number", doc.ArchiveSerial)
    }
    
    writer.Close()
    
    req, _ := http.NewRequestWithContext(ctx, "POST", 
        c.baseURL+"/api/documents/post_document/", body)
    req.Header.Set("Authorization", "Token "+c.token)
    req.Header.Set("Content-Type", writer.FormDataContentType())
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("paperless upload failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("paperless returned status %d", resp.StatusCode)
    }
    
    var result struct {
        TaskID string `json:"task_id"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    
    return &UploadResult{TaskID: result.TaskID}, nil
}

// Task-Status abfragen
func (c *PaperlessClient) GetTaskStatus(ctx context.Context, taskID string) (*TaskStatus, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET",
        c.baseURL+"/api/tasks/?task_id="+taskID, nil)
    req.Header.Set("Authorization", "Token "+c.token)
    
    resp, err := c.httpClient.Do(req)
    // ...
}
```

### Alternative: Consume-Ordner

Paperless-NGX überwacht einen "Consume"-Ordner und verarbeitet dort abgelegte Dateien automatisch.

```go
// internal/output/paperless_consume.go

type PaperlessConsumeHandler struct {
    consumePath string
    smbClient   *smb2.Session  // Falls via SMB
}

func (h *PaperlessConsumeHandler) Upload(ctx context.Context, doc *Document) error {
    // Dateiname nach Paperless-Konvention für Metadaten:
    // [correspondent] - [date] - [title] - [tags].pdf
    filename := h.buildFilename(doc)
    targetPath := filepath.Join(h.consumePath, filename)
    
    // Kopieren (lokal oder via SMB)
    if h.smbClient != nil {
        return h.copyViaSMB(doc.Reader, targetPath)
    }
    return h.copyLocal(doc.Reader, targetPath)
}

// Paperless erkennt Metadaten aus dem Dateinamen
func (h *PaperlessConsumeHandler) buildFilename(doc *Document) string {
    parts := []string{}
    
    if doc.Correspondent != "" {
        parts = append(parts, doc.Correspondent)
    }
    if doc.Created != "" {
        parts = append(parts, doc.Created)
    }
    if doc.Title != "" {
        parts = append(parts, doc.Title)
    }
    
    name := strings.Join(parts, " - ")
    if name == "" {
        name = fmt.Sprintf("scan_%s", time.Now().Format("20060102_150405"))
    }
    
    return sanitizeFilename(name) + ".pdf"
}
```

---

## Hardware-Button Integration

Die Scan-Taste am ScanSnap S1500 löst automatisch einen Scan aus – **ohne** zusätzliche Software wie `scanbd`.

### Wichtig: Kein Hardware-Interrupt verfügbar

SANE bietet **keinen echten Hardware-Interrupt** für Scanner-Buttons. Der Button-Status wird durch Polling abgefragt. Um zwischen kurzem und langem Tastendruck zu unterscheiden, messen wir die **Druckdauer**.

### Kurzer vs. Langer Tastendruck

| Tastendruck | Dauer | Seitenhöhe | Verwendung |
|-------------|-------|------------|------------|
| **Kurz** | < 1 Sekunde | ~420mm (1,5× A4) | Normale Dokumente |
| **Lang** | ≥ 1 Sekunde | Unbegrenzt (0) | Kontoauszüge, lange Belege |

### Funktionsweise

```
┌─────────────────────────────────────────────────────────────────────┐
│  Button Watcher (Polling alle 50ms für präzise Zeitmessung)         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  SANE: GetOption("scan") → true?                                    │
│       │                                                             │
│       ▼ JA (Button gedrückt)                                        │
│  ┌─────────────────────┐                                            │
│  │ Startzeit speichern │                                            │
│  └──────────┬──────────┘                                            │
│             │                                                       │
│             ▼                                                       │
│  Warten bis Button losgelassen (GetOption → false)                  │
│             │                                                       │
│             ▼                                                       │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ Dauer = Endzeit - Startzeit                                 │   │
│  │                                                             │   │
│  │ if Dauer < 1s  → onShortPress() → Profil "standard"         │   │
│  │                  (page_height = 420mm)                      │   │
│  │                                                             │   │
│  │ if Dauer >= 1s → onLongPress()  → Profil "oversize"         │   │
│  │                  (page_height = 0 = unbegrenzt)             │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Implementierung

```go
// internal/scanner/button.go

type ButtonWatcher struct {
    scanner        *Scanner
    pollInterval   time.Duration  // 50ms für präzise Messung
    longPressDur   time.Duration  // 1 Sekunde = langer Druck
    onShortPress   func()         // Callback: Kurzer Druck
    onLongPress    func()         // Callback: Langer Druck
    
    pressStart     time.Time
    isPressed      bool
    longPressBeep  bool           // Bereits gepiept?
}

func NewButtonWatcher(scanner *Scanner, cfg ButtonConfig, 
                      onShort, onLong func()) *ButtonWatcher {
    return &ButtonWatcher{
        scanner:      scanner,
        pollInterval: cfg.PollInterval,  // 50ms
        longPressDur: cfg.LongPressDuration,  // 1s
        onShortPress: onShort,
        onLongPress:  onLong,
    }
}

func (w *ButtonWatcher) Start(ctx context.Context) {
    slog.Info("button watcher started", 
        "poll_interval", w.pollInterval,
        "long_press_threshold", w.longPressDur)
    
    ticker := time.NewTicker(w.pollInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            slog.Info("button watcher stopped")
            return
        case <-ticker.C:
            w.poll()
        }
    }
}

func (w *ButtonWatcher) poll() {
    pressed, err := w.scanner.GetButtonState("scan")
    if err != nil {
        return // Scanner möglicherweise busy während Scan
    }
    
    now := time.Now()
    
    if pressed && !w.isPressed {
        // ══════════════════════════════════════════════════════
        // Button wurde GERADE GEDRÜCKT → Zeitmessung starten
        // ══════════════════════════════════════════════════════
        w.pressStart = now
        w.isPressed = true
        w.longPressBeep = false
        slog.Debug("button pressed, measuring duration...")
        
    } else if pressed && w.isPressed {
        // ══════════════════════════════════════════════════════
        // Button wird GEHALTEN → Prüfen ob Schwelle erreicht
        // ══════════════════════════════════════════════════════
        if !w.longPressBeep && now.Sub(w.pressStart) >= w.longPressDur {
            // Akustisches Feedback: Schwelle für langen Druck erreicht
            w.playBeep()
            w.longPressBeep = true
            slog.Debug("long press threshold reached")
        }
        
    } else if !pressed && w.isPressed {
        // ══════════════════════════════════════════════════════
        // Button wurde LOSGELASSEN → Aktion auslösen
        // ══════════════════════════════════════════════════════
        w.isPressed = false
        duration := now.Sub(w.pressStart)
        
        if duration >= w.longPressDur {
            slog.Info("long press detected", "duration", duration)
            if w.onLongPress != nil {
                go w.onLongPress()
            }
        } else {
            slog.Info("short press detected", "duration", duration)
            if w.onShortPress != nil {
                go w.onShortPress()
            }
        }
    }
}

// Optionales akustisches Feedback wenn Schwelle erreicht
func (w *ButtonWatcher) playBeep() {
    // System-Beep (falls verfügbar)
    exec.Command("beep", "-f", "1000", "-l", "100").Run()
}

// Button-Status über SANE abfragen
func (s *Scanner) GetButtonState(button string) (bool, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    if s.conn == nil {
        return false, ErrNotConnected
    }
    
    val, err := s.conn.GetOption(button)
    if err != nil {
        return false, err
    }
    
    if b, ok := val.(bool); ok {
        return b, nil
    }
    return false, nil
}
```

### Server-Integration

```go
// cmd/scanflow-server/main.go

func main() {
    cfg := loadConfig()
    scanner := scanner.New(cfg.Scanner)
    jobQueue := jobs.NewQueue()
    
    // ═══════════════════════════════════════════════════════════
    // Kurzer Tastendruck → Normal-Scan (Seitenhöhe begrenzt)
    // ═══════════════════════════════════════════════════════════
    onShortPress := func() {
        job := &jobs.Job{
            Profile:  cfg.Button.ShortPressProfile,  // "standard"
            Output:   cfg.Button.Output,
            Metadata: &DocumentMetadata{
                Title: formatTitle(cfg.Button.Metadata.TitlePattern),
            },
        }
        slog.Info("starting normal scan (short press)", 
            "profile", job.Profile)
        jobQueue.Submit(job)
    }
    
    // ═══════════════════════════════════════════════════════════
    // Langer Tastendruck → Überlängen-Scan (unbegrenzte Höhe)
    // ═══════════════════════════════════════════════════════════
    onLongPress := func() {
        job := &jobs.Job{
            Profile:  cfg.Button.LongPressProfile,  // "oversize"
            Output:   cfg.Button.Output,
            Metadata: &DocumentMetadata{
                Title: formatTitle(cfg.Button.Metadata.TitlePattern),
            },
        }
        slog.Info("starting oversize scan (long press)", 
            "profile", job.Profile)
        jobQueue.Submit(job)
    }
    
    // Button-Watcher starten
    if cfg.Button.Enabled {
        bw := scanner.NewButtonWatcher(
            scanner, cfg.Button, onShortPress, onLongPress)
        go bw.Start(ctx)
    }
    
    api.Start(cfg.Server, scanner, jobQueue)
}
```

### Konfiguration

```toml
# server.toml

[button]
enabled = true
poll_interval = "50ms"          # Schnelles Polling für präzise Zeitmessung
long_press_duration = "1s"      # Ab 1 Sekunde = langer Druck

# Kurzer Tastendruck → Normal-Scan
short_press_profile = "standard"

# Langer Tastendruck → Überlängen-Scan
long_press_profile = "oversize"

# Gemeinsames Ausgabeziel
output = "paperless"

# Akustisches Feedback wenn Schwelle erreicht
beep_on_long_press = true

[button.metadata]
title_pattern = "Scan_{date}_{time}"  # {date}=20240115, {time}=143052
```

### Scan-Profile

```toml
# profiles/standard.toml
[profile]
name = "Standard"
description = "Normaler Scan, Seitenhöhe ~1,5× A4"

[scanner]
resolution = 300
mode = "color"
source = "adf_duplex"
page_height = 420.0  # mm (wie ScanSnap Manager Standard)
```

```toml
# profiles/oversize.toml
[profile]
name = "Überlänge"
description = "Für lange Dokumente (Kontoauszüge, Quittungsrollen)"

[scanner]
resolution = 200      # Etwas niedriger für sehr lange Dokumente
mode = "gray"         # Oft reicht Graustufen für Kontoauszüge
source = "adf_duplex"
page_height = 0       # 0 = UNBEGRENZT, Scanner erkennt Länge automatisch
```

### Workflow: One-Touch mit Kurz/Lang-Unterscheidung

```
╔═══════════════════════════════════════════════════════════════════╗
║  KURZER DRUCK (< 1 Sekunde)                                       ║
║  → Normale Dokumente: Briefe, Rechnungen, A4-Seiten               ║
╠═══════════════════════════════════════════════════════════════════╣
║  1. Dokumente einlegen                                            ║
║  2. Taste KURZ drücken und loslassen                              ║
║  3. Scan startet mit page_height = 420mm                          ║
║  4. PDF → OCR → Paperless                                         ║
╚═══════════════════════════════════════════════════════════════════╝

╔═══════════════════════════════════════════════════════════════════╗
║  LANGER DRUCK (≥ 1 Sekunde)                                       ║
║  → Überlange Dokumente: Kontoauszüge, Kassenbons, Quittungen      ║
╠═══════════════════════════════════════════════════════════════════╣
║  1. Langes Dokument einlegen                                      ║
║  2. Taste GEDRÜCKT HALTEN bis Piepton (nach 1 Sek)                ║
║  3. Loslassen → Scan startet mit page_height = 0 (unbegrenzt)     ║
║  4. Scanner erfasst beliebige Länge automatisch                   ║
║  5. PDF → OCR → Paperless                                         ║
╚═══════════════════════════════════════════════════════════════════╝
```

### Button testen

```bash
# Prüfen ob SANE den Button erkennt
scanimage -d "fujitsu:ScanSnap S1500:*" -A | grep -A2 Sensors

# Erwartete Ausgabe:
# Sensors:
#   --scan[=(yes|no)] [no] [hardware]
#       Scan button

# Button gedrückt halten → [yes] sollte erscheinen

# Zeitmessung testen (Server mit Debug-Level)
SCANFLOW_LOG_LEVEL=debug ./scanflow-server

# Kurz drücken → "short press detected duration=XXXms"
# Lang drücken → "long press threshold reached" + "long press detected"
```

### Alternativen falls Polling nicht ausreicht

1. **scanbd**: Nutzt ebenfalls Polling, bietet aber D-Bus-Integration
2. **libinput**: Falls der Scanner als HID-Gerät erkannt wird (unwahrscheinlich)
3. **Kernel-Modul**: Theoretisch möglich, aber sehr aufwändig

Für den ScanSnap S1500 ist die Polling-Lösung mit Druckdauer-Messung die praktikabelste Variante.

### Alternative: scanbd (falls integrierte Lösung nicht funktioniert)

Für Scanner ohne direkte Button-Unterstützung via SANE kann `scanbd` verwendet werden:

```bash
#!/bin/bash
# /etc/scanbd/scripts/scanflow-button.sh

curl -X POST http://localhost:8080/api/v1/scan \
    -H "Authorization: Bearer $SCANFLOW_API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "profile": "standard",
        "output": {"target": "paperless"},
        "metadata": {"title": "Scan_'"$(date +%Y%m%d_%H%M%S)"'"}
    }'
```

---

## Deployment

### Raspberry Pi Setup

```bash
# System-Abhängigkeiten
sudo apt update
sudo apt install -y sane-utils libsane-dev tesseract-ocr tesseract-ocr-deu cifs-utils

# Scanner-Berechtigungen
sudo usermod -aG scanner $USER

# Scanner testen
scanimage -L
# Erwartet: device `fujitsu:ScanSnap S1500:...' is a FUJITSU ScanSnap S1500 scanner

# ScanFlow Server installieren
sudo mkdir -p /opt/scanflow /etc/scanflow /var/lib/scanflow /var/log/scanflow
sudo cp scanflow-server /opt/scanflow/
sudo cp server.toml /etc/scanflow/

# Paperless Token speichern (sicher!)
echo "your-paperless-token" | sudo tee /etc/scanflow/paperless_token
sudo chmod 600 /etc/scanflow/paperless_token

# Systemd Service
sudo cp scanflow.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable scanflow
sudo systemctl start scanflow
```

### Systemd Service (`scanflow.service`)

```ini
[Unit]
Description=ScanFlow Scanner Server
After=network.target

[Service]
Type=simple
User=scanner
Group=scanner
ExecStart=/opt/scanflow/scanflow-server -config /etc/scanflow/server.toml
Restart=always
RestartSec=5

# Sicherheit
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/scanflow /var/log/scanflow /tmp/scanflow

[Install]
WantedBy=multi-user.target
```

### Docker (Alternative)

```dockerfile
FROM golang:1.22-bookworm AS builder
WORKDIR /app
COPY . .
RUN go build -o scanflow-server ./cmd/scanflow-server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y \
    sane-utils libsane1 tesseract-ocr tesseract-ocr-deu \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/scanflow-server /usr/local/bin/
COPY configs/server.toml /etc/scanflow/

# USB-Passthrough erforderlich!
# docker run --device=/dev/bus/usb

EXPOSE 8080
CMD ["scanflow-server", "-config", "/etc/scanflow/server.toml"]
```

```yaml
# docker-compose.yml
version: "3.8"
services:
  scanflow:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./config:/etc/scanflow:ro
      - scanflow-data:/var/lib/scanflow
      - /mnt/nas/scans:/mnt/output  # SMB-Mount
    devices:
      - /dev/bus/usb:/dev/bus/usb  # USB-Passthrough
    privileged: true  # Für Scanner-Zugriff
    restart: unless-stopped

volumes:
  scanflow-data:
```

---

## Sicherheit

### API-Authentifizierung

```go
// internal/api/middleware.go

func AuthMiddleware(validKeys []string) func(http.Handler) http.Handler {
    keySet := make(map[string]bool)
    for _, k := range validKeys {
        keySet[k] = true
    }
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Bearer Token
            auth := r.Header.Get("Authorization")
            if strings.HasPrefix(auth, "Bearer ") {
                token := strings.TrimPrefix(auth, "Bearer ")
                if keySet[token] {
                    next.ServeHTTP(w, r)
                    return
                }
            }
            
            // API Key Header
            apiKey := r.Header.Get("X-API-Key")
            if keySet[apiKey] {
                next.ServeHTTP(w, r)
                return
            }
            
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
        })
    }
}
```

### Netzwerk-Sicherheit

- Server nur im lokalen Netzwerk erreichbar (Firewall)
- Optional: TLS für verschlüsselte Verbindungen
- API-Keys mit ausreichender Entropie
- Passwörter/Tokens in separaten Dateien mit `chmod 600`

---

## Abhängigkeiten

### Server (Go)

```go
require (
    github.com/tjgq/sane v0.0.0-...      // SANE bindings
    github.com/go-chi/chi/v5 v5.0.0      // HTTP router
    github.com/gorilla/websocket v1.5.0  // WebSocket
    github.com/pdfcpu/pdfcpu v0.6.0      // PDF processing
    github.com/hirochachacha/go-smb2 v1.1.0  // SMB client
    github.com/pelletier/go-toml/v2 v2.1.0   // TOML config
    github.com/rs/zerolog v1.31.0        // Logging
)
```

### Client (Go)

```go
require (
    github.com/spf13/cobra v1.8.0        // CLI framework
    github.com/spf13/viper v1.18.0       // Configuration
    github.com/charmbracelet/bubbletea v0.25.0  // TUI
    github.com/charmbracelet/lipgloss v0.9.0    // TUI styling
    github.com/gorilla/websocket v1.5.0  // WebSocket
    github.com/go-resty/resty/v2 v2.11.0 // HTTP client
)
```

### System (Raspberry Pi)

```bash
# Basis
sane-utils libsane-dev tesseract-ocr tesseract-ocr-deu

# Optional für SMB
cifs-utils

# Optional für E-Mail
msmtp  # Oder direkt via Go
```

---

## Nächste Schritte

1. **Server-Grundgerüst**: HTTP-Server, SANE-Integration, Job-Queue
2. **Scanner-Modul**: Device-Discovery, Batch-Scanning, Optionen
3. **PDF-Pipeline**: Bildverarbeitung, PDF-Erstellung, OCR
4. **Output-Handler**: Paperless-NGX API, SMB, Consume-Ordner
5. **Client-CLI**: Grundbefehle, API-Client
6. **Client-TUI**: Interaktive Oberfläche
7. **Tests**: Unit-Tests, Integration-Tests
8. **Dokumentation**: Installation, Konfiguration, API-Referenz
9. **Web-UI**: Eingebettete Oberfläche (optional)
10. **Hardware-Button**: scanbd-Integration (optional)
