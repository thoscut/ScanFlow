# ScanFlow - Konfigurationsreferenz

## Server-Konfiguration

Datei: `/etc/scanflow/server.toml`

### [server]

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| host | string | "0.0.0.0" | Bind-Adresse |
| port | int | 8080 | HTTP-Port |
| base_url | string | "" | Oeffentliche URL |

### [server.auth]

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| enabled | bool | true | Authentifizierung aktivieren |
| api_keys | []string | [] | Gueltige API-Schluessel |

### [scanner]

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| device | string | "" | Scanner-Device (leer = Auto) |
| auto_open | bool | true | Automatisch verbinden |

### [scanner.defaults]

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| resolution | int | 300 | DPI (75-600) |
| mode | string | "color" | color, gray, lineart |
| source | string | "adf_duplex" | Papiereinzug |
| page_width | float | 210.0 | Seitenbreite in mm |
| page_height | float | 297.0 | Seitenhoehe (0 = unbegrenzt) |

### [button]

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| enabled | bool | true | Button-Ueberwachung |
| poll_interval | duration | "50ms" | Polling-Intervall |
| long_press_duration | duration | "1s" | Schwelle fuer langen Druck |
| short_press_profile | string | "standard" | Profil bei kurzem Druck |
| long_press_profile | string | "oversize" | Profil bei langem Druck |
| output | string | "paperless" | Ausgabeziel |
| beep_on_long_press | bool | true | Akustisches Feedback |

### [processing]

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| temp_directory | string | "/tmp/scanflow" | Temporaeres Verzeichnis |
| max_concurrent_jobs | int | 2 | Max. parallele Jobs |

### [processing.ocr]

OCR ist optional und kann global in der Konfiguration, ueber die Web-UI (Einstellungen) oder pro Scan deaktiviert werden. Dies ist nuetzlich wenn z.B. Paperless-NGX die OCR-Verarbeitung uebernimmt.

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| enabled | bool | true | OCR standardmaessig aktivieren |
| language | string | "deu+eng" | Tesseract-Sprachen |
| tesseract_path | string | "/usr/bin/tesseract" | Tesseract-Pfad |

OCR kann auch zur Laufzeit ueber die Settings-API geaendert werden:

```bash
# OCR-Status abfragen
curl http://scanserver.local:8080/api/v1/settings

# OCR deaktivieren
curl -X PUT http://scanserver.local:8080/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{"ocr_enabled": false, "ocr_language": "deu+eng"}'
```

Einzelne Scans koennen die OCR-Einstellung ueberschreiben:

```bash
curl -X POST http://scanserver.local:8080/api/v1/scan \
  -H "Content-Type: application/json" \
  -d '{"profile": "standard", "ocr_enabled": false}'
```

### [output.paperless]

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| enabled | bool | false | Paperless aktivieren |
| url | string | "" | Paperless URL |
| token_file | string | "" | Token-Datei (chmod 600) |
| verify_ssl | bool | true | SSL pruefen |

### [output.smb]

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| enabled | bool | false | SMB aktivieren |
| server | string | "" | SMB-Server |
| share | string | "" | Freigabename |
| username | string | "" | Benutzername |
| password_file | string | "" | Passwort-Datei |

## Scan-Profile

Verzeichnis: `/etc/scanflow/profiles/` oder `configs/profiles/`

### Beispiel: standard.toml

```toml
[profile]
name = "Standard"
description = "Farbscan 300 DPI"

[scanner]
resolution = 300
mode = "color"
source = "adf_duplex"
page_height = 420.0

[processing]
optimize_images = true
deskew = true
remove_blank_pages = true
blank_threshold = 0.99

[processing.ocr]
enabled = true
language = "deu"

[output]
default_target = "paperless"
```

## Client-Konfiguration

Datei: `~/.config/scanflow/client.toml`

### [server]

| Parameter | Typ | Beschreibung |
|-----------|-----|-------------|
| url | string | Server-URL |
| api_key | string | API-Schluessel |

### [defaults]

| Parameter | Typ | Beschreibung |
|-----------|-----|-------------|
| profile | string | Standard-Profil |
| output | string | Standard-Ausgabeziel |
