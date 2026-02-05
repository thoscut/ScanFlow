# ScanFlow - Hardware-Button Integration

## Wichtig: Kein Hardware-Interrupt verfügbar

SANE bietet **keinen echten Hardware-Interrupt** für Scanner-Buttons. Sowohl `scanbd` als auch alle anderen Lösungen verwenden **Polling**. Um zwischen kurzem und langem Tastendruck zu unterscheiden, messen wir die Druckdauer.

## Kurzer vs. Langer Tastendruck

| Tastendruck | Dauer | Seitenhöhe | Verwendung |
|-------------|-------|------------|------------|
| **Kurz** | < 1 Sekunde | ~420mm (1,5× A4) | Normale Dokumente |
| **Lang** | ≥ 1 Sekunde | Unbegrenzt (0) | Kontoauszüge, lange Belege |

## Button-Status prüfen

Zunächst prüfen, ob SANE den Button erkennt:

```bash
# Scanner-Optionen und Sensoren anzeigen
scanimage -d "fujitsu:ScanSnap S1500:*" -A

# Erwartete Ausgabe (Auszug):
# Sensors:
#   --scan[=(yes|no)] [no] [hardware]
#       Scan button
```

Wenn Sie den Button gedrückt halten während Sie den Befehl ausführen, sollte `[yes]` erscheinen.

---

## Option A: Integrierte Button-Überwachung (Empfohlen)

Der ScanFlow Server überwacht den Button selbst – keine zusätzliche Software nötig. Die Druckdauer wird gemessen um zwischen kurz und lang zu unterscheiden.

### Architektur

```
┌─────────────────────────────────────────────────────────────────┐
│                    ScanFlow Server                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────┐     ┌─────────────────────────────┐   │
│  │  Button Watcher     │────►│  Job Queue                  │   │
│  │  (50ms Polling)     │     │  → Profil auswählen         │   │
│  │  Zeitmessung        │     │  → Scan starten             │   │
│  └──────────┬──────────┘     │  → PDF erstellen            │   │
│             │                │  → An Paperless senden      │   │
│             ▼                └─────────────────────────────┘   │
│  ┌─────────────────────┐                                       │
│  │  SANE API           │                                       │
│  │  GetOption("scan")  │                                       │
│  └─────────────────────┘                                       │
│                                                                 │
│  Druckdauer < 1s  → short_press_profile (page_height=420mm)    │
│  Druckdauer >= 1s → long_press_profile  (page_height=0)        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Implementierung

```go
// internal/scanner/button.go

package scanner

import (
    "context"
    "log/slog"
    "os/exec"
    "time"
)

type ButtonWatcher struct {
    scanner        *Scanner
    pollInterval   time.Duration  // 50ms für präzise Zeitmessung
    longPressDur   time.Duration  // 1 Sekunde = Schwelle für langen Druck
    onShortPress   func()         // Callback: kurzer Druck → Standard-Scan
    onLongPress    func()         // Callback: langer Druck → Überlängen-Scan
    beepEnabled    bool
    
    pressStart     time.Time
    isPressed      bool
    longPressBeep  bool
}

type ButtonConfig struct {
    Enabled           bool          `toml:"enabled"`
    PollInterval      time.Duration `toml:"poll_interval"`       // Default: 50ms
    LongPressDuration time.Duration `toml:"long_press_duration"` // Default: 1s
    ShortPressProfile string        `toml:"short_press_profile"` // z.B. "standard"
    LongPressProfile  string        `toml:"long_press_profile"`  // z.B. "oversize"
    Output            string        `toml:"output"`
    BeepOnLongPress   bool          `toml:"beep_on_long_press"`
}

func NewButtonWatcher(scanner *Scanner, cfg ButtonConfig, 
                      onShort, onLong func()) *ButtonWatcher {
    if cfg.PollInterval == 0 {
        cfg.PollInterval = 50 * time.Millisecond
    }
    if cfg.LongPressDuration == 0 {
        cfg.LongPressDuration = 1 * time.Second
    }
    
    return &ButtonWatcher{
        scanner:      scanner,
        pollInterval: cfg.PollInterval,
        longPressDur: cfg.LongPressDuration,
        onShortPress: onShort,
        onLongPress:  onLong,
        beepEnabled:  cfg.BeepOnLongPress,
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
        return // Scanner möglicherweise busy
    }
    
    now := time.Now()
    
    if pressed && !w.isPressed {
        // ═══════════════════════════════════════════════════════════
        // Button wurde GERADE GEDRÜCKT → Zeitmessung starten
        // ═══════════════════════════════════════════════════════════
        w.pressStart = now
        w.isPressed = true
        w.longPressBeep = false
        slog.Debug("button pressed, measuring duration...")
        
    } else if pressed && w.isPressed {
        // ═══════════════════════════════════════════════════════════
        // Button wird GEHALTEN → Prüfen ob Schwelle erreicht
        // ═══════════════════════════════════════════════════════════
        if w.beepEnabled && !w.longPressBeep && 
           now.Sub(w.pressStart) >= w.longPressDur {
            w.playBeep()
            w.longPressBeep = true
            slog.Debug("long press threshold reached")
        }
        
    } else if !pressed && w.isPressed {
        // ═══════════════════════════════════════════════════════════
        // Button wurde LOSGELASSEN → Aktion auslösen
        // ═══════════════════════════════════════════════════════════
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

// Akustisches Feedback wenn Schwelle für langen Druck erreicht
func (w *ButtonWatcher) playBeep() {
    exec.Command("beep", "-f", "1000", "-l", "100").Run()
}

// Scanner-Methode zum Abfragen des Button-Status
func (s *Scanner) GetButtonState(buttonName string) (bool, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    if s.conn == nil {
        return false, ErrNotConnected
    }
    
    val, err := s.conn.GetOption(buttonName)
    if err != nil {
        return false, err
    }
    
    // SANE liefert bool für Sensoren
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
    
    // Kurzer Druck → Normal-Scan (begrenzte Seitenhöhe)
    onShortPress := func() {
        job := &jobs.Job{
            Profile:  cfg.Button.ShortPressProfile,  // "standard"
            Output:   cfg.Button.Output,
            Metadata: &DocumentMetadata{
                Title: fmt.Sprintf("Scan_%s", time.Now().Format("20060102_150405")),
            },
        }
        
        if err := jobQueue.Submit(job); err != nil {
            slog.Error("button scan failed", "error", err)
            return
        }
        slog.Info("normal scan started (short press)", "job_id", job.ID)
    }
    
    // Langer Druck → Überlängen-Scan (unbegrenzte Seitenhöhe)
    onLongPress := func() {
        job := &jobs.Job{
            Profile:  cfg.Button.LongPressProfile,  // "oversize"
            Output:   cfg.Button.Output,
            Metadata: &DocumentMetadata{
                Title: fmt.Sprintf("Scan_%s", time.Now().Format("20060102_150405")),
            },
        }
        
        if err := jobQueue.Submit(job); err != nil {
            slog.Error("button scan failed", "error", err)
            return
        }
        slog.Info("oversize scan started (long press)", "job_id", job.ID)
    }
    
    // Button-Watcher starten (wenn aktiviert)
    if cfg.Button.Enabled {
        buttonWatcher := scanner.NewButtonWatcher(
            scanner, cfg.Button, onShortPress, onLongPress)
        go buttonWatcher.Start(ctx)
    }
    
    // REST-API starten
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

# Optionale Metadaten
[button.metadata]
title_pattern = "Scan_{date}_{time}"  # Dynamischer Titel
# correspondent = 1                   # Fester Correspondent
# document_type = 2                   # Fester Dokumenttyp
# tags = [1, 3]                       # Feste Tags
```

### Profile für Kurz/Lang-Druck

```toml
# profiles/standard.toml - Kurzer Druck
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
# profiles/oversize.toml - Langer Druck
[profile]
name = "Überlänge"
description = "Für lange Dokumente (Kontoauszüge, Quittungsrollen)"

[scanner]
resolution = 200
mode = "gray"
source = "adf_duplex"
page_height = 0  # 0 = UNBEGRENZT
```

### LED-Feedback (Optional)

Falls der Scanner LED-Steuerung unterstützt:

```go
// Visuelles Feedback beim Scannen
func (s *Scanner) SetLED(state string) error {
    // Manche Scanner unterstützen LED-Kontrolle
    // state: "ready", "scanning", "error"
    _, err := s.conn.SetOption("led-mode", state)
    return err
}
```

---

## Option B: scanbd (Traditionell)

`scanbd` ist ein Daemon, der Scanner-Buttons überwacht und Scripts ausführt.

### Installation

```bash
# Debian/Ubuntu
sudo apt install scanbd

# Oder aus Source bauen
sudo apt install build-essential libconfuse-dev libusb-dev libudev-dev libdbus-1-dev libsane-dev
wget https://sourceforge.net/projects/scanbd/files/releases/scanbd-1.5.1.tgz
tar xzf scanbd-1.5.1.tgz
cd scanbd-1.5.1
./configure
make
sudo make install
```

### Konfiguration

```bash
# /etc/scanbd/scanbd.conf

global {
    debug = true
    debug-level = 2
    user = scanner
    group = scanner
    scriptdir = /etc/scanbd/scripts
    pidfile = /var/run/scanbd.pid
    timeout = 500
    
    environment {
        device = "SCANBD_DEVICE"
        action = "SCANBD_ACTION"
    }
}

device fujitsu {
    filter = "^fujitsu.*"
    
    action scan {
        filter = "^scan$"
        numerical-trigger {
            from-value = 0
            to-value = 1
        }
        desc = "Scan button pressed"
        script = "scanflow-button.sh"
    }
}
```

### SANE-Konfiguration für scanbd

scanbd benötigt exklusiven Zugriff auf den Scanner. Andere Anwendungen müssen über das Netzwerk-Backend zugreifen.

```bash
# /etc/sane.d/dll.conf (für andere Anwendungen)
# Nur net-Backend aktivieren
net

# /etc/sane.d/net.conf
localhost

# /etc/sane.d/saned.conf (Zugriff erlauben)
localhost
```

```bash
# /etc/scanbd/sane.d/dll.conf (für scanbd)
# Echte Backends aktivieren
fujitsu
```

### Scan-Script

```bash
#!/bin/bash
# /etc/scanbd/scripts/scanflow-button.sh

# Logging
exec >> /var/log/scanflow/button.log 2>&1
echo "$(date): Button pressed on device $SCANBD_DEVICE"

# ScanFlow API aufrufen
SCANFLOW_URL="${SCANFLOW_URL:-http://localhost:8080}"
SCANFLOW_API_KEY="${SCANFLOW_API_KEY:-$(cat /etc/scanflow/api_key)}"

# Scan starten via REST-API
response=$(curl -s -X POST "$SCANFLOW_URL/api/v1/scan" \
    -H "Authorization: Bearer $SCANFLOW_API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "profile": "standard",
        "output": {"target": "paperless"},
        "metadata": {"title": "Scan_'"$(date +%Y%m%d_%H%M%S)"'"}
    }')

job_id=$(echo "$response" | jq -r '.id')
echo "$(date): Started job $job_id"

# Optional: Auf Abschluss warten und Status loggen
while true; do
    status=$(curl -s "$SCANFLOW_URL/api/v1/scan/$job_id" \
        -H "Authorization: Bearer $SCANFLOW_API_KEY" | jq -r '.status')
    
    case "$status" in
        completed)
            echo "$(date): Job $job_id completed successfully"
            exit 0
            ;;
        failed)
            echo "$(date): Job $job_id failed"
            exit 1
            ;;
        *)
            sleep 2
            ;;
    esac
done
```

### Systemd Service

```ini
# /etc/systemd/system/scanbd.service
[Unit]
Description=Scanner Button Daemon
After=network.target saned.socket

[Service]
Type=forking
ExecStart=/usr/sbin/scanbd -c /etc/scanbd/scanbd.conf
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

---

## Option C: insaned (Leichtgewichtig)

`insaned` ist ein einfacherer Daemon, der Buttons durch Polling überwacht.

### Installation

```bash
git clone https://github.com/abusenius/insaned.git
cd insaned
make
sudo make install
```

### Konfiguration

```bash
# /etc/insaned.conf
SANE_DEVICE="fujitsu:ScanSnap S1500:*"
POLL_INTERVAL=0.5
SCRIPTS_DIR=/etc/insaned/scripts
```

### Script

```bash
#!/bin/bash
# /etc/insaned/scripts/scan.sh
# Wird aufgerufen wenn "scan" Button gedrückt wird

curl -X POST "http://localhost:8080/api/v1/scan" \
    -H "Authorization: Bearer $SCANFLOW_API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"profile": "standard", "output": {"target": "paperless"}}'
```

---

## Empfehlung

| Ansatz | Vorteile | Nachteile |
|--------|----------|-----------|
| **A: Integriert** | Einfachste Installation, alles in einer App, keine Konflikte | Muss selbst implementiert werden |
| **B: scanbd** | Bewährt, viele Features | Komplexe Konfiguration, SANE-Netzwerkmodus nötig |
| **C: insaned** | Einfach, leichtgewichtig | Weniger Features, manuelles Polling |

**Empfehlung: Option A (Integrierte Lösung)**

Die integrierte Button-Überwachung ist am einfachsten zu verwenden:
- Keine zusätzliche Software
- Keine komplexe SANE-Netzwerkkonfiguration
- Direkter Zugriff auf Job-Queue und Scanner-Status
- Einfache Konfiguration in `server.toml`

---

## Erweiterte Features

### Mehrere Button-Aktionen

Manche Scanner haben mehrere Tasten (Scan, Copy, Email, etc.):

```toml
# server.toml

[buttons.scan]
enabled = true
profile = "standard"
output = "paperless"

[buttons.email]
enabled = true
profile = "standard"
output = "email"
email_recipient = "archiv@example.com"

[buttons.copy]
enabled = false  # Deaktiviert
```

```go
// Mehrere Buttons überwachen
func (w *ButtonWatcher) poll() {
    buttons := []string{"scan", "email", "copy", "file"}
    
    for _, btn := range buttons {
        pressed, err := w.scanner.GetButtonState(btn)
        if err != nil {
            continue
        }
        
        if pressed && w.isDebounced(btn) {
            w.handleButtonPress(btn)
        }
    }
}

func (w *ButtonWatcher) handleButtonPress(button string) {
    cfg, ok := w.buttonConfigs[button]
    if !ok || !cfg.Enabled {
        return
    }
    
    job := &jobs.Job{
        Profile: cfg.Profile,
        Output:  cfg.Output,
    }
    
    w.jobQueue.Submit(job)
}
```

### Status-Feedback via LED/Display

```go
// Scanner-Status anzeigen (falls unterstützt)
type ScannerStatus int

const (
    StatusReady ScannerStatus = iota
    StatusScanning
    StatusProcessing
    StatusError
)

func (s *Scanner) SetStatusIndicator(status ScannerStatus) {
    // Versuche LED zu setzen (nicht alle Scanner unterstützen das)
    switch status {
    case StatusReady:
        s.trySetOption("led-mode", "ready")
    case StatusScanning:
        s.trySetOption("led-mode", "scanning")
    case StatusError:
        s.trySetOption("led-mode", "error")
    }
}
```

### Button-Press während Scan

```go
// Während eines Scans: Button = weitere Seiten
func (w *ButtonWatcher) handleButtonDuringScan(job *jobs.Job) {
    // Button während aktivem Scan = "Noch mehr Seiten kommen"
    // Verlängert Timeout für nächsten Stapel
    job.ExtendTimeout(30 * time.Second)
    slog.Info("scan extended, waiting for more pages", "job_id", job.ID)
}
```

---

## Vollständige Server-Konfiguration mit Button

```toml
# /etc/scanflow/server.toml

[server]
host = "0.0.0.0"
port = 8080

[server.auth]
enabled = true
api_keys = ["sk_live_abc123..."]

[scanner]
device = ""  # Auto-detect
auto_open = true

[scanner.defaults]
resolution = 300
mode = "color"
source = "adf_duplex"

# === BUTTON-KONFIGURATION ===
[button]
enabled = true
poll_interval = "200ms"
debounce = "500ms"

# Standard-Aktion bei Button-Druck
profile = "standard"
output = "paperless"

# Automatischer Titel
[button.metadata]
title_pattern = "Scan_{date}_{time}"

# === ALTERNATIVE: Mehrere Buttons ===
# [buttons.scan]
# enabled = true
# profile = "standard"
# output = "paperless"
# 
# [buttons.email]
# enabled = true
# profile = "standard"
# output = "email"
# email_recipient = "archiv@example.com"

[processing.ocr]
enabled = true
language = "deu+eng"

[output.paperless]
enabled = true
url = "https://paperless.local"
token_file = "/etc/scanflow/paperless_token"
```

---

## Test der Button-Funktion

```bash
# 1. Scanner-Button prüfen
scanimage -d "fujitsu:ScanSnap S1500:*" -A | grep -A1 Sensors

# 2. Server starten mit Debug-Logging
SCANFLOW_LOG_LEVEL=debug ./scanflow-server -config server.toml

# 3. Button drücken und Log beobachten
# Erwartete Ausgabe:
# INFO button watcher started interval=200ms
# INFO scan button pressed
# INFO button scan started job_id=abc123

# 4. Job-Status prüfen
curl http://localhost:8080/api/v1/scan/abc123 \
    -H "Authorization: Bearer sk_live_..."
```
