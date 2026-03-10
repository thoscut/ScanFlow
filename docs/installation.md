# ScanFlow - Installationsanleitung

## Voraussetzungen

### Server (Raspberry Pi / Linux)

- Linux (Debian/Ubuntu empfohlen)
- Go 1.26+ (nur zum Bauen)
- SANE (Scanner-Treiber)
- Tesseract OCR (optional)

### Client

- Windows, macOS oder Linux
- Netzwerkzugang zum Server

## Server-Installation

### 1. System-Abhaengigkeiten installieren

```bash
sudo apt update
sudo apt install -y sane-utils libsane-dev tesseract-ocr tesseract-ocr-deu cifs-utils
```

### 2. Scanner testen

```bash
# Scanner-Berechtigungen
sudo usermod -aG scanner $USER

# Neu einloggen, dann:
scanimage -L
```

### 3. ScanFlow Server bauen

```bash
cd server
go build -o scanflow-server ./cmd/scanflow-server
```

Oder fuer Raspberry Pi (ARM64) cross-compilieren:

```bash
GOOS=linux GOARCH=arm64 go build -o scanflow-server-arm64 ./cmd/scanflow-server
```

### 4. Konfiguration

```bash
sudo mkdir -p /opt/scanflow /etc/scanflow /var/lib/scanflow /var/log/scanflow
sudo cp scanflow-server /opt/scanflow/
sudo cp configs/server.toml /etc/scanflow/
```

Konfiguration anpassen:

```bash
sudo nano /etc/scanflow/server.toml
```

Wichtige Einstellungen:
- `server.auth.api_keys` - API-Schluessel setzen
- `output.paperless.url` - Paperless-NGX URL
- `output.paperless.token_file` - Paperless Token

### 5. Paperless Token speichern

```bash
echo "dein-paperless-token" | sudo tee /etc/scanflow/paperless_token
sudo chmod 600 /etc/scanflow/paperless_token
```

### 6. Systemd Service installieren

```bash
sudo ./dist/scanflow-server -install-service -config /etc/scanflow/server.toml
sudo systemctl start scanflow
```

Der Server erzeugt dabei bei Bedarf automatisch:

- `/opt/scanflow/scanflow-server`
- `/etc/scanflow/server.toml`
- `/etc/systemd/system/scanflow.service`
- `/var/lib/scanflow`, `/var/log/scanflow`, `/tmp/scanflow`

Optional kann der Dienst direkt aktiviert und gestartet werden:

```bash
sudo ./dist/scanflow-server -install-service -start-service
```

### 7. Status pruefen

```bash
sudo systemctl status scanflow
curl http://localhost:8080/api/v1/health
```

## Client-Installation

### Aus Source bauen

```bash
cd client
go build -o scanflow ./cmd/scanflow
sudo cp scanflow /usr/local/bin/
```

### Konfiguration

```bash
scanflow config set server.url http://scanserver.local:8080
scanflow config set server.api_key sk_live_...
```

### Test

```bash
scanflow status
scanflow devices
```

## Docker-Installation

```bash
docker-compose -f deploy/docker/docker-compose.yml up -d
```

Wichtig: USB-Passthrough fuer Scanner-Zugriff erforderlich.

## Fehlerbehebung

| Problem | Loesung |
|---------|---------|
| Scanner nicht gefunden | `scanimage -L`, USB pruefen, Berechtigungen |
| Permission denied | `sudo usermod -aG scanner $USER` |
| Paperless 401 | Token pruefen |
| Verbindung verweigert | Firewall, Port pruefen |

## Hardware-Empfehlungen

### Bewaehrte Server-Hardware

- **Raspberry Pi 4 / 5 mit 4 GB RAM oder mehr** fuer kompakte Scan-Server
- **SSD statt SD-Karte** fuer OCR-Tempdaten und geringeren Verschleiss
- **USB-Scanner mit ADF und Duplex** fuer stabile Batch-Scans
- **Netzwerk per Ethernet** statt WLAN fuer grosse PDF-Uploads

### Geeignete Scanner-Merkmale

- SANE-kompatibler Treiber
- Dokumenteneinzug (ADF)
- Duplex-Scan
- Hardware-Button, wenn Kurz-/Langdruck direkt am Geraet genutzt werden soll

Vor dem produktiven Einsatz immer mit `scanimage -L` und `scanimage -A` pruefen, ob Scanner und Sensoren sauber erkannt werden.
