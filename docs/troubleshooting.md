# Troubleshooting

## Scanner Permissions

### Scanner not detected

If `scanimage -L` does not list your scanner:

1. Check the USB connection and verify the scanner is powered on.
2. Add your user to the `scanner` group:
   ```bash
   sudo usermod -aG scanner $USER
   ```
3. Reload udev rules or reconnect the USB cable:
   ```bash
   sudo udevadm control --reload-rules
   sudo udevadm trigger
   ```
4. Verify SANE detects the scanner:
   ```bash
   SANE_DEBUG_DLL=5 scanimage -L
   ```

### Permission denied

Ensure the service user has access to the scanner device:

```bash
ls -l /dev/bus/usb/001/
```

If needed, create a udev rule in `/etc/udev/rules.d/49-scanner.rules`:

```
SUBSYSTEM=="usb", ATTR{idVendor}=="04c5", MODE="0666"
```

Replace the vendor ID with the one for your scanner.

## OCR Issues

### Tesseract not found

Install Tesseract and the required language packs:

```bash
# Debian / Ubuntu
sudo apt install tesseract-ocr tesseract-ocr-deu tesseract-ocr-eng

# Verify installation
tesseract --version
```

### ocrmypdf not available

For best OCR results, install ocrmypdf:

```bash
pip install ocrmypdf
```

ScanFlow falls back to direct Tesseract if ocrmypdf is not available, but the output quality may be lower.

### OCR produces empty or garbled text

- Increase the scan resolution to 300 DPI or higher.
- Use `gray` or `lineart` mode for text-heavy documents.
- Ensure the correct language is configured in `server.toml`:
  ```toml
  [processing.ocr]
  language = "deu+eng"
  ```

## Paperless-NGX Connectivity

### Upload fails with 401

Verify the API token is correct:

```bash
curl -H "Authorization: Token YOUR_TOKEN" \
  https://paperless.example.com/api/documents/
```

Ensure the token file is readable by the scanner user:

```bash
sudo chmod 640 /etc/scanflow/paperless_token
sudo chown root:scanner /etc/scanflow/paperless_token
```

### Connection refused

- Verify the Paperless URL in `server.toml` is correct and reachable.
- Check firewall rules between the scanner server and Paperless.
- If using HTTPS, set `verify_ssl = false` for self-signed certificates (not recommended for production).

### Upload succeeds but document not visible

Paperless processes documents asynchronously. Check the Paperless task queue:

```bash
curl -H "Authorization: Token YOUR_TOKEN" \
  https://paperless.example.com/api/tasks/
```

## SMB Share Issues

### Connection timeout

- Verify the SMB server is reachable: `ping nas.local`
- Check that port 445 is open: `nc -zv nas.local 445`
- Ensure the share name is correct and the user has write permissions.

### Authentication failure

- Verify the username and password in the configuration.
- Check that the password file has the correct permissions:
  ```bash
  sudo chmod 640 /etc/scanflow/smb_password
  ```

## Server Issues

### Server fails to start

Check the configuration file for errors:

```bash
scanflow-server -config /etc/scanflow/server.toml
```

The server validates configuration at startup and reports all errors at once.

Common issues:
- Invalid port number (must be 1-65535)
- Missing TLS certificate files when TLS is enabled
- Invalid OCR language code
- Temp directory does not exist or is not writable

### High memory usage

- Reduce `max_concurrent_jobs` in the configuration.
- Lower the scan resolution for routine documents.
- Monitor with the `/metrics` endpoint for job queue depth.

### Jobs lost on restart

ScanFlow persists job state to disk. Verify the storage directory is writable:

```bash
ls -la /var/lib/scanflow/documents/jobs/
```

## Button Issues

### Button not detected

Verify SANE recognizes the scanner button:

```bash
scanimage -d "your_device" -A | grep -A2 Sensors
```

### Button press not triggering scan

- Check that `button.enabled = true` in `server.toml`.
- Verify the poll interval is set (default 50ms).
- Check server logs for button press events.

## Monitoring

### Prometheus metrics

ScanFlow exposes metrics at `/metrics` (no authentication required):

```bash
curl http://localhost:8080/metrics
```

Available metrics:
- `scanflow_jobs_total{status}` — job counts by status
- `scanflow_jobs_active` — currently processing jobs
- `scanflow_scan_pages_total` — total pages scanned
- `scanflow_http_requests_total{code}` — HTTP request counts
- `scanflow_http_request_duration_seconds` — request latency

### Health and readiness

- `GET /api/v1/health` — basic liveness check
- `GET /api/v1/ready` — readiness check (scanner connected)
