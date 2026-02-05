# CLAUDE.md - ScanFlow Projektdokumentation

## Projektübersicht

ScanFlow ist ein Client-Server-System für Netzwerk-Scanning mit Paperless-NGX-Integration. Der Server läuft auf Linux (Raspberry Pi) mit SANE, Clients sind plattformunabhängig.

```
[Scanner] ←USB→ [Raspberry Pi + SANE] ←REST API→ [Client (Win/Mac/Linux)]
                        ↓
              [Paperless-NGX / NAS]
```

## Repository-Struktur

```
scanflow/
├── server/                 # ScanFlow Server (Go)
│   ├── cmd/scanflow-server/
│   ├── internal/
│   │   ├── api/           # REST-API + WebSocket
│   │   ├── scanner/       # SANE-Integration
│   │   ├── processor/     # PDF + OCR Pipeline
│   │   ├── output/        # Paperless, SMB, etc.
│   │   ├── config/        # Konfiguration
│   │   └── jobs/          # Job-Queue
│   └── go.mod
├── client/                 # ScanFlow Client (Go)
│   ├── cmd/scanflow/
│   ├── internal/
│   │   ├── client/        # API-Client
│   │   ├── cli/           # Cobra Commands
│   │   ├── tui/           # Bubbletea TUI
│   │   └── config/
│   └── go.mod
├── configs/               # Beispiel-Konfigurationen
│   ├── server.toml
│   ├── client.toml
│   └── profiles/
├── deploy/                # Deployment-Dateien
│   ├── systemd/
│   ├── docker/
│   └── ansible/
├── docs/
└── Makefile
```

## Schnellstart

### Server (Raspberry Pi)

```bash
# Abhängigkeiten
sudo apt install sane-utils libsane-dev tesseract-ocr tesseract-ocr-deu

# Scanner testen
scanimage -L

# Server bauen und starten
cd server
go build -o scanflow-server ./cmd/scanflow-server
./scanflow-server -config ../configs/server.toml
```

### Client (beliebige Plattform)

```bash
cd client
go build -o scanflow ./cmd/scanflow

# Server-Verbindung konfigurieren
./scanflow config set server.url http://scanserver.local:8080
./scanflow config set server.api_key sk_live_...

# Scan starten
./scanflow scan --profile standard --output paperless
```

## Server-Entwicklung

### Wichtige Packages

| Package | Zweck |
|---------|-------|
| `github.com/tjgq/sane` | SANE Scanner-API |
| `github.com/go-chi/chi/v5` | HTTP Router |
| `github.com/gorilla/websocket` | WebSocket für Live-Updates |
| `github.com/pdfcpu/pdfcpu` | PDF-Verarbeitung |
| `github.com/hirochachacha/go-smb2` | SMB/CIFS Client |

### API-Endpunkte

```go
// Router-Setup (internal/api/server.go)
r := chi.NewRouter()
r.Use(middleware.Logger)
r.Use(AuthMiddleware(cfg.Auth.APIKeys))

// Scanner
r.Get("/api/v1/scanner/devices", h.ListDevices)
r.Post("/api/v1/scanner/devices/{id}/open", h.OpenDevice)

// Scan-Jobs
r.Post("/api/v1/scan", h.StartScan)
r.Get("/api/v1/scan/{job_id}", h.GetJobStatus)
r.Post("/api/v1/scan/{job_id}/continue", h.ContinueScan)
r.Post("/api/v1/scan/{job_id}/finish", h.FinishScan)
r.Delete("/api/v1/scan/{job_id}/pages/{n}", h.DeletePage)

// WebSocket
r.Get("/api/v1/ws", h.WebSocket)
```

### SANE-Integration

```go
// internal/scanner/sane.go
import "github.com/tjgq/sane"

type Scanner struct {
    conn *sane.Conn
    opts ScanOptions
}

func (s *Scanner) Open(deviceName string) error {
    if err := sane.Init(); err != nil {
        return fmt.Errorf("SANE init failed: %w", err)
    }
    
    conn, err := sane.Open(deviceName)
    if err != nil {
        return fmt.Errorf("cannot open scanner: %w", err)
    }
    s.conn = conn
    return nil
}

func (s *Scanner) ScanBatch(ctx context.Context) (<-chan *Page, error) {
    pages := make(chan *Page)
    
    go func() {
        defer close(pages)
        pageNum := 0
        
        for {
            select {
            case <-ctx.Done():
                return
            default:
                img, err := s.conn.ReadImage()
                if errors.Is(err, sane.ErrEmpty) {
                    return // ADF leer
                }
                if err != nil {
                    pages <- &Page{Err: err}
                    return
                }
                pageNum++
                pages <- &Page{Number: pageNum, Image: img}
            }
        }
    }()
    
    return pages, nil
}

// Scanner-Optionen setzen
func (s *Scanner) SetOptions(opts ScanOptions) error {
    if opts.Resolution > 0 {
        if _, err := s.conn.SetOption("resolution", opts.Resolution); err != nil {
            return err
        }
    }
    if opts.Mode != "" {
        if _, err := s.conn.SetOption("mode", opts.Mode); err != nil {
            return err
        }
    }
    if opts.Source != "" {
        if _, err := s.conn.SetOption("source", opts.Source); err != nil {
            return err
        }
    }
    // Überlängen: page-height = 0
    if opts.PageHeight == 0 {
        s.conn.SetOption("page-height", 0) // Unbegrenzt
    }
    return nil
}
```

### Job-Queue

```go
// internal/jobs/queue.go
type JobQueue struct {
    jobs    map[string]*Job
    pending chan *Job
    mu      sync.RWMutex
}

type Job struct {
    ID        string
    Status    JobStatus
    Profile   string
    Pages     []*Page
    Output    OutputConfig
    Metadata  DocumentMetadata
    CreatedAt time.Time
    UpdatedAt time.Time
    
    // Channels für Kommunikation
    progress  chan ProgressUpdate
    cancel    context.CancelFunc
}

type JobStatus string
const (
    StatusPending    JobStatus = "pending"
    StatusScanning   JobStatus = "scanning"
    StatusProcessing JobStatus = "processing"
    StatusCompleted  JobStatus = "completed"
    StatusFailed     JobStatus = "failed"
)
```

### Paperless-NGX Integration

```go
// internal/output/paperless.go
type PaperlessHandler struct {
    baseURL string
    token   string
    client  *http.Client
}

func (h *PaperlessHandler) Upload(ctx context.Context, doc *Document) error {
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    
    // PDF-Datei
    part, _ := writer.CreateFormFile("document", doc.Filename)
    io.Copy(part, doc.Reader)
    
    // Metadaten
    if doc.Title != "" {
        writer.WriteField("title", doc.Title)
    }
    for _, tag := range doc.Tags {
        writer.WriteField("tags", strconv.Itoa(tag))
    }
    writer.Close()
    
    req, _ := http.NewRequestWithContext(ctx, "POST",
        h.baseURL+"/api/documents/post_document/", body)
    req.Header.Set("Authorization", "Token "+h.token)
    req.Header.Set("Content-Type", writer.FormDataContentType())
    
    resp, err := h.client.Do(req)
    if err != nil {
        return fmt.Errorf("paperless upload: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("paperless error %d: %s", resp.StatusCode, body)
    }
    
    return nil
}
```

### Button-Watcher (Hardware-Taste mit Kurz/Lang-Erkennung)

**Wichtig**: SANE bietet keinen Hardware-Interrupt. Die Druckdauer wird durch Polling gemessen.

| Tastendruck | Dauer | Profil | Seitenhöhe |
|-------------|-------|--------|------------|
| Kurz | < 1s | standard | 420mm |
| Lang | ≥ 1s | oversize | unbegrenzt |

```go
// internal/scanner/button.go
type ButtonWatcher struct {
    scanner        *Scanner
    pollInterval   time.Duration  // 50ms für präzise Messung
    longPressDur   time.Duration  // 1s = Schwelle für langen Druck
    onShortPress   func()         // Callback: kurzer Druck
    onLongPress    func()         // Callback: langer Druck
    
    pressStart     time.Time
    isPressed      bool
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
            return
        case <-ticker.C:
            w.poll()
        }
    }
}

func (w *ButtonWatcher) poll() {
    pressed, err := w.scanner.GetButtonState("scan")
    if err != nil {
        return // Scanner busy
    }
    
    now := time.Now()
    
    if pressed && !w.isPressed {
        // Button gerade gedrückt → Zeitmessung starten
        w.pressStart = now
        w.isPressed = true
        
    } else if !pressed && w.isPressed {
        // Button losgelassen → Dauer auswerten
        w.isPressed = false
        duration := now.Sub(w.pressStart)
        
        if duration >= w.longPressDur {
            slog.Info("long press", "duration", duration)
            go w.onLongPress()  // → Überlängen-Profil
        } else {
            slog.Info("short press", "duration", duration)
            go w.onShortPress() // → Standard-Profil
        }
    }
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

### Server mit Button-Integration (Kurz/Lang)

```go
// cmd/scanflow-server/main.go
func main() {
    cfg := loadConfig()
    scanner := scanner.New(cfg.Scanner)
    jobQueue := jobs.NewQueue()
    
    // Kurzer Druck → Standard-Scan (begrenzte Seitenhöhe)
    onShortPress := func() {
        job := &jobs.Job{
            Profile:  cfg.Button.ShortPressProfile,  // "standard"
            Output:   cfg.Button.Output,
            Metadata: &DocumentMetadata{
                Title: formatTitle(cfg.Button.Metadata.TitlePattern),
            },
        }
        slog.Info("normal scan (short press)", "profile", job.Profile)
        jobQueue.Submit(job)
    }
    
    // Langer Druck → Überlängen-Scan (unbegrenzte Höhe)
    onLongPress := func() {
        job := &jobs.Job{
            Profile:  cfg.Button.LongPressProfile,  // "oversize"
            Output:   cfg.Button.Output,
            Metadata: &DocumentMetadata{
                Title: formatTitle(cfg.Button.Metadata.TitlePattern),
            },
        }
        slog.Info("oversize scan (long press)", "profile", job.Profile)
        jobQueue.Submit(job)
    }
    
    // Button-Watcher starten
    if cfg.Button.Enabled {
        bw := scanner.NewButtonWatcher(scanner, cfg.Button, 
            onShortPress, onLongPress)
        go bw.Start(ctx)
    }
    
    // REST-API starten
    api.Start(cfg.Server, scanner, jobQueue)
}
```

### SMB-Ablage

```go
// internal/output/smb.go
import "github.com/hirochachacha/go-smb2"

type SMBHandler struct {
    server   string
    share    string
    user     string
    password string
}

func (h *SMBHandler) Upload(ctx context.Context, doc *Document) error {
    conn, err := net.Dial("tcp", h.server+":445")
    if err != nil {
        return err
    }
    defer conn.Close()
    
    d := &smb2.Dialer{
        Initiator: &smb2.NTLMInitiator{
            User:     h.user,
            Password: h.password,
        },
    }
    
    session, err := d.Dial(conn)
    if err != nil {
        return err
    }
    defer session.Logoff()
    
    share, err := session.Mount(h.share)
    if err != nil {
        return err
    }
    defer share.Umount()
    
    f, err := share.Create(doc.Filename)
    if err != nil {
        return err
    }
    defer f.Close()
    
    _, err = io.Copy(f, doc.Reader)
    return err
}
```

## Client-Entwicklung

### API-Client

```go
// client/internal/client/api.go
type Client struct {
    baseURL string
    apiKey  string
    http    *http.Client
}

func (c *Client) StartScan(ctx context.Context, req *ScanRequest) (*ScanJob, error) {
    body, _ := json.Marshal(req)
    
    httpReq, _ := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/api/v1/scan", bytes.NewReader(body))
    httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := c.http.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var job ScanJob
    json.NewDecoder(resp.Body).Decode(&job)
    return &job, nil
}

// WebSocket für Live-Updates
func (c *Client) ConnectWebSocket(ctx context.Context, jobID string) (<-chan ProgressUpdate, error) {
    wsURL := strings.Replace(c.baseURL, "http", "ws", 1) + "/api/v1/ws"
    conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
    if err != nil {
        return nil, err
    }
    
    updates := make(chan ProgressUpdate)
    go func() {
        defer close(updates)
        defer conn.Close()
        
        for {
            var update ProgressUpdate
            if err := conn.ReadJSON(&update); err != nil {
                return
            }
            updates <- update
        }
    }()
    
    return updates, nil
}
```

### CLI mit Cobra

```go
// client/internal/cli/scan.go
var scanCmd = &cobra.Command{
    Use:   "scan",
    Short: "Dokument scannen",
    RunE:  runScan,
}

func init() {
    scanCmd.Flags().StringP("profile", "p", "standard", "Scan-Profil")
    scanCmd.Flags().StringP("output", "o", "", "Ausgabeziel (paperless, smb)")
    scanCmd.Flags().StringP("title", "t", "", "Dokumenttitel")
    scanCmd.Flags().BoolP("interactive", "i", false, "Interaktiver Modus")
    scanCmd.Flags().IntSlice("tags", nil, "Paperless Tag-IDs")
}

func runScan(cmd *cobra.Command, args []string) error {
    client := getClient()
    
    interactive, _ := cmd.Flags().GetBool("interactive")
    if interactive {
        return runInteractiveScan(cmd, client)
    }
    
    req := &ScanRequest{
        Profile: viper.GetString("profile"),
    }
    
    // Metadaten aus Flags
    if title, _ := cmd.Flags().GetString("title"); title != "" {
        req.Metadata = &DocumentMetadata{Title: title}
    }
    
    job, err := client.StartScan(cmd.Context(), req)
    if err != nil {
        return err
    }
    
    // Auf Abschluss warten
    return waitForJob(cmd.Context(), client, job.ID)
}
```

### TUI mit Bubbletea

```go
// client/internal/tui/scan_view.go
import tea "github.com/charmbracelet/bubbletea"

type ScanModel struct {
    client    *Client
    job       *ScanJob
    pages     []PagePreview
    status    string
    err       error
    quitting  bool
}

func (m ScanModel) Init() tea.Cmd {
    return m.startScan
}

func (m ScanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "w":
            return m, m.continueScan  // Weitere Seiten
        case "f":
            return m, m.finishScan    // Fertig
        case "d":
            return m, m.deletePage    // Seite löschen
        case "q", "ctrl+c":
            m.quitting = true
            return m, tea.Quit
        }
    case ProgressMsg:
        m.status = msg.Status
        return m, m.waitForProgress
    case ScanCompleteMsg:
        m.pages = append(m.pages, msg.Page)
        return m, nil
    case ErrorMsg:
        m.err = msg.Err
        return m, nil
    }
    return m, nil
}

func (m ScanModel) View() string {
    if m.err != nil {
        return fmt.Sprintf("Fehler: %v\n\nDrücke q zum Beenden.", m.err)
    }
    
    s := fmt.Sprintf("Status: %s\n", m.status)
    s += fmt.Sprintf("Seiten: %d\n\n", len(m.pages))
    
    s += "[W] Weitere Seiten  [F] Fertig  [D] Löschen  [Q] Abbrechen\n"
    
    return s
}
```

## Konfiguration

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
auto_open = true

[scanner.defaults]
resolution = 300
mode = "color"
source = "adf_duplex"

# Hardware-Button (Kurz/Lang-Druck-Erkennung)
[button]
enabled = true
poll_interval = "50ms"           # Schnelles Polling für Zeitmessung
long_press_duration = "1s"       # Ab 1s = langer Druck
short_press_profile = "standard" # Kurz → normale Seitenhöhe
long_press_profile = "oversize"  # Lang → unbegrenzte Seitenhöhe
output = "paperless"
beep_on_long_press = true        # Piepton bei Schwelle

[button.metadata]
title_pattern = "Scan_{date}_{time}"

[processing.ocr]
enabled = true
language = "deu+eng"

[output.paperless]
enabled = true
url = "https://paperless.local"
token_file = "/etc/scanflow/paperless_token"

[output.smb]
enabled = true
server = "nas.local"
share = "scans"
username = "scanner"
password_file = "/etc/scanflow/smb_password"
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

## Tests

```bash
# Server-Tests (mit SANE test device)
cd server
sudo sed -i 's/#test/test/' /etc/sane.d/dll.conf
go test -v ./...

# Integration-Tests
go test -v -tags=integration ./...

# Client-Tests
cd client
go test -v ./...
```

### Test mit SANE Test-Device

```go
func TestScannerDiscovery(t *testing.T) {
    scanner := NewScanner()
    devices, err := scanner.Discover()
    require.NoError(t, err)
    
    // Test-Device sollte vorhanden sein
    found := false
    for _, d := range devices {
        if strings.Contains(d.Name, "test") {
            found = true
            break
        }
    }
    assert.True(t, found, "SANE test device not found")
}
```

## Debugging

### Button testen

```bash
# Prüfen ob SANE den Button erkennt
scanimage -d "fujitsu:ScanSnap S1500:*" -A | grep -A2 Sensors

# Erwartete Ausgabe:
# Sensors:
#   --scan[=(yes|no)] [no] [hardware]
#       Scan button

# Button gedrückt halten und erneut ausführen → [yes]
```

### Server

```bash
# SANE Debug
SANE_DEBUG_DLL=5 SANE_DEBUG_FUJITSU=5 ./scanflow-server

# API Debug
curl -v http://localhost:8080/api/v1/scanner/devices \
    -H "Authorization: Bearer sk_live_..."

# Scanner testen
scanimage -d "fujitsu:ScanSnap S1500:*" -A
```

### Häufige Probleme

| Problem | Lösung |
|---------|--------|
| Scanner nicht gefunden | `scanimage -L`, USB-Kabel, Berechtigungen |
| "Permission denied" | User zur `scanner` Gruppe hinzufügen |
| Paperless 401 | Token prüfen, API-Zugriff in Paperless aktiviert? |
| SMB-Verbindung fehlgeschlagen | Firewall, Credentials, Share-Berechtigungen |

## Build & Release

```makefile
# Makefile
VERSION := $(shell git describe --tags --always)

.PHONY: build-server
build-server:
	cd server && go build -ldflags "-X main.version=$(VERSION)" \
		-o ../dist/scanflow-server ./cmd/scanflow-server

.PHONY: build-client
build-client:
	cd client && go build -ldflags "-X main.version=$(VERSION)" \
		-o ../dist/scanflow ./cmd/scanflow

.PHONY: build-client-all
build-client-all:
	cd client && \
	GOOS=linux GOARCH=amd64 go build -o ../dist/scanflow-linux-amd64 ./cmd/scanflow && \
	GOOS=linux GOARCH=arm64 go build -o ../dist/scanflow-linux-arm64 ./cmd/scanflow && \
	GOOS=darwin GOARCH=amd64 go build -o ../dist/scanflow-darwin-amd64 ./cmd/scanflow && \
	GOOS=darwin GOARCH=arm64 go build -o ../dist/scanflow-darwin-arm64 ./cmd/scanflow && \
	GOOS=windows GOARCH=amd64 go build -o ../dist/scanflow-windows-amd64.exe ./cmd/scanflow

.PHONY: test
test:
	cd server && go test -v ./...
	cd client && go test -v ./...
```

## Deployment auf Raspberry Pi

```bash
# Cross-compile für ARM64
GOOS=linux GOARCH=arm64 go build -o scanflow-server-arm64 ./cmd/scanflow-server

# Kopieren
scp scanflow-server-arm64 pi@scanserver.local:/opt/scanflow/scanflow-server

# Service neu starten
ssh pi@scanserver.local "sudo systemctl restart scanflow"
```

## Referenzen

- [SANE API](http://www.sane-project.org/html/)
- [Go SANE Bindings](https://pkg.go.dev/github.com/tjgq/sane)
- [Paperless-NGX API](https://docs.paperless-ngx.com/api/)
- [Chi Router](https://go-chi.io/)
- [Bubbletea TUI](https://github.com/charmbracelet/bubbletea)
- [pdfcpu](https://pdfcpu.io/)
