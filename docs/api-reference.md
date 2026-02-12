# ScanFlow - API-Referenz

Base URL: `http://scanserver.local:8080`

## Authentifizierung

Alle API-Endpunkte (ausser Health) erfordern Authentifizierung:

```
Authorization: Bearer <api_key>
```

Oder:

```
X-API-Key: <api_key>
```

## Endpunkte

### System

#### GET /api/v1/health

Health-Check (keine Authentifizierung noetig).

**Response:**
```json
{
  "status": "ok",
  "version": "0.1.0"
}
```

#### GET /api/v1/status

Server-Status abfragen.

**Response:**
```json
{
  "status": "ok",
  "version": "0.1.0",
  "scanner": true,
  "devices": 1,
  "active_jobs": 0,
  "total_jobs": 5
}
```

### Scanner

#### GET /api/v1/scanner/devices

Verfuegbare Scanner auflisten.

**Response:**
```json
{
  "devices": [
    {
      "name": "fujitsu:ScanSnap S1500:*",
      "vendor": "FUJITSU",
      "model": "ScanSnap S1500",
      "type": "scanner"
    }
  ]
}
```

#### POST /api/v1/scanner/devices/{id}/open

Scanner oeffnen.

#### DELETE /api/v1/scanner/devices/{id}/close

Scanner schliessen.

### Scan-Operationen

#### POST /api/v1/scan

Neuen Scan starten.

**Request:**
```json
{
  "profile": "standard",
  "device_id": "",
  "options": {
    "resolution": 300,
    "mode": "color",
    "source": "adf_duplex"
  },
  "output": {
    "target": "paperless"
  },
  "metadata": {
    "title": "Rechnung 2024",
    "tags": [1, 3],
    "correspondent": 5
  },
  "ocr_enabled": true
}
```

Der Parameter `ocr_enabled` ist optional. Wenn gesetzt, ueberschreibt er die globale OCR-Einstellung fuer diesen einzelnen Scan. Nuetzlich wenn z.B. Paperless-NGX die OCR-Verarbeitung uebernimmt.

**Response (202):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "profile": "standard",
  "pages": [],
  "progress": 0,
  "created_at": "2024-01-15T14:30:52Z"
}
```

#### GET /api/v1/scan/{job_id}

Job-Status abfragen.

**Response:**
```json
{
  "id": "550e8400-...",
  "status": "scanning",
  "profile": "standard",
  "pages": [
    {"number": 1, "width": 2480, "height": 3508}
  ],
  "progress": 50,
  "created_at": "2024-01-15T14:30:52Z",
  "updated_at": "2024-01-15T14:30:55Z"
}
```

Status-Werte: `pending`, `scanning`, `processing`, `completed`, `failed`, `cancelled`

#### DELETE /api/v1/scan/{job_id}

Job abbrechen.

#### POST /api/v1/scan/{job_id}/continue

Weitere Seiten scannen.

#### POST /api/v1/scan/{job_id}/finish

Scan abschliessen und PDF erstellen.

**Request:**
```json
{
  "output": {"target": "paperless"},
  "metadata": {"title": "Finaler Titel"}
}
```

### Seiten-Management

#### GET /api/v1/scan/{job_id}/pages

Gescannte Seiten auflisten.

#### DELETE /api/v1/scan/{job_id}/pages/{n}

Seite loeschen.

#### POST /api/v1/scan/{job_id}/pages/reorder

Seiten umsortieren.

**Request:**
```json
{"order": [3, 1, 2]}
```

### Ausgabe

#### GET /api/v1/outputs

Konfigurierte Ausgabeziele auflisten.

**Response:**
```json
{
  "outputs": [
    {"name": "paperless", "type": "paperless", "enabled": true, "available": true},
    {"name": "smb", "type": "smb", "enabled": true, "available": true},
    {"name": "filesystem", "type": "filesystem", "enabled": true, "available": true}
  ]
}
```

### Profile

#### GET /api/v1/profiles

Profile auflisten.

#### GET /api/v1/profiles/{name}

Profil-Details abfragen.

#### POST /api/v1/profiles

Neues Profil erstellen.

#### PUT /api/v1/profiles/{name}

Profil aktualisieren.

### Einstellungen

#### GET /api/v1/settings

Aktuelle Einstellungen abfragen.

**Response:**
```json
{
  "ocr_enabled": true,
  "ocr_language": "deu+eng"
}
```

#### PUT /api/v1/settings

Einstellungen aktualisieren. Aenderungen werden sofort wirksam fuer neue Scans.

**Request:**
```json
{
  "ocr_enabled": false,
  "ocr_language": "deu+eng"
}
```

**Response:**
```json
{
  "ocr_enabled": false,
  "ocr_language": "deu+eng"
}
```

### WebSocket

#### WS /api/v1/ws

WebSocket-Verbindung fuer Live-Updates.

**Nachrichten:**
```json
{"type": "job_update", "job_id": "...", "status": "scanning", "progress": 50}
{"type": "page_complete", "job_id": "...", "page": 1}
{"type": "completed", "job_id": "...", "message": "Document processed"}
```

## Fehlercodes

| Code | Bedeutung |
|------|-----------|
| 200 | OK |
| 201 | Erstellt |
| 202 | Akzeptiert (Job gestartet) |
| 400 | Ungueltige Anfrage |
| 401 | Nicht autorisiert |
| 404 | Nicht gefunden |
| 500 | Server-Fehler |

## CLI-Nutzung

```bash
# Status pruefen
scanflow status

# Scanner anzeigen
scanflow devices

# Scan starten
scanflow scan --profile standard --output paperless

# Mit Metadaten
scanflow scan -t "Rechnung" --tags 1,3 --correspondent 5

# Interaktiv
scanflow scan -i

# TUI starten
scanflow tui
```
