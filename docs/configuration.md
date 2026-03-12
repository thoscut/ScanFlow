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

### [server.tls]

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| enabled | bool | false | TLS aktivieren |
| cert_file | string | "" | Zertifikat-Datei (PEM) |
| key_file | string | "" | Schluessel-Datei (PEM) |

### [server.tls.acme]

Automatische TLS-Zertifikate ueber Let's Encrypt (ACME). Wenn aktiviert, werden
Zertifikate automatisch bezogen und erneuert. Dies hat Vorrang vor manuellen
`cert_file`/`key_file` Einstellungen.

| Parameter | Typ | Standard | Beschreibung |
|-----------|-----|----------|-------------|
| enabled | bool | false | ACME aktivieren |
| email | string | "" | Kontakt-E-Mail fuer Let's Encrypt |
| domains | []string | [] | Domains fuer das Zertifikat |
| challenge | string | "http" | Challenge-Typ: "http" oder "dns" |
| cert_dir | string | "/var/lib/scanflow/certs" | Verzeichnis fuer Zertifikate |
| directory_url | string | "" | ACME-Directory (leer = Let's Encrypt Produktion) |
| dns_provider | string | "" | DNS-Provider: "cloudflare", "duckdns", "route53", "exec" |
| dns_propagation_wait | duration | "120s" | DNS-Propagierungs-Wartezeit |

#### HTTP-Challenge

Die HTTP-Challenge funktioniert automatisch. Der Server muss auf Port 80
erreichbar sein (fuer die Challenge-Validierung). Port 443 wird fuer HTTPS verwendet.

```toml
[server]
port = 443

[server.tls]
enabled = true

[server.tls.acme]
enabled = true
email = "admin@example.com"
domains = ["scanflow.example.com"]
challenge = "http"
```

#### DNS-Challenge mit Cloudflare

```toml
[server.tls.acme]
enabled = true
email = "admin@example.com"
domains = ["scanflow.example.com"]
challenge = "dns"
dns_provider = "cloudflare"

[server.tls.acme.cloudflare]
api_token_file = "/etc/scanflow/cloudflare_token"
# zone_id = ""  # Optional, wird automatisch erkannt
```

| Parameter | Typ | Beschreibung |
|-----------|-----|-------------|
| api_token_file | string | Pfad zur Cloudflare API-Token-Datei |
| zone_id | string | Cloudflare Zone-ID (optional, wird automatisch erkannt) |

#### DNS-Challenge mit DuckDNS

Besonders geeignet fuer Home-Server mit dynamischer IP.

```toml
[server.tls.acme]
enabled = true
email = "admin@example.com"
domains = ["myscanner.duckdns.org"]
challenge = "dns"
dns_provider = "duckdns"

[server.tls.acme.duckdns]
token_file = "/etc/scanflow/duckdns_token"
```

| Parameter | Typ | Beschreibung |
|-----------|-----|-------------|
| token_file | string | Pfad zur DuckDNS-Token-Datei |

#### DNS-Challenge mit AWS Route 53

```toml
[server.tls.acme]
enabled = true
email = "admin@example.com"
domains = ["scanflow.example.com"]
challenge = "dns"
dns_provider = "route53"

[server.tls.acme.route53]
access_key_id = "AKIAEXAMPLE"
secret_access_key_file = "/etc/scanflow/route53_secret"
hosted_zone_id = "Z0123456789"
region = "eu-central-1"
```

| Parameter | Typ | Beschreibung |
|-----------|-----|-------------|
| access_key_id | string | AWS Access Key ID |
| secret_access_key_file | string | Pfad zur Secret-Access-Key-Datei |
| hosted_zone_id | string | Route 53 Hosted Zone ID |
| region | string | AWS Region (Standard: us-east-1) |

#### DNS-Challenge mit externem Skript

Fuer DNS-Provider die nicht nativ unterstuetzt werden, koennen eigene Skripte
oder Tools wie certbot/lego verwendet werden.

```toml
[server.tls.acme]
enabled = true
email = "admin@example.com"
domains = ["scanflow.example.com"]
challenge = "dns"
dns_provider = "exec"

[server.tls.acme.exec]
create_command = "/usr/local/bin/dns-challenge-create"
cleanup_command = "/usr/local/bin/dns-challenge-cleanup"
```

Die Skripte werden mit folgenden Argumenten aufgerufen:
```
<create_command> <domain> <token> <key_auth>
<cleanup_command> <domain> <token> <key_auth>
```

| Parameter | Typ | Beschreibung |
|-----------|-----|-------------|
| create_command | string | Skript zum Erstellen des DNS-TXT-Eintrags |
| cleanup_command | string | Skript zum Loeschen des DNS-TXT-Eintrags |

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
