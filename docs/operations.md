# Operations Runbook

## Deployment

### Systemd Service

Install and manage the ScanFlow service:

```bash
# Install
sudo scanflow-server -install-service -config /etc/scanflow/server.toml

# Start / stop / restart
sudo systemctl start scanflow
sudo systemctl stop scanflow
sudo systemctl restart scanflow

# View logs
sudo journalctl -u scanflow -f
```

### Docker

```bash
docker build -t scanflow-server -f deploy/docker/Dockerfile .
docker run -d --name scanflow \
  --device /dev/bus/usb \
  -p 8080:8080 \
  -v /etc/scanflow:/etc/scanflow:ro \
  -v /var/lib/scanflow:/var/lib/scanflow \
  scanflow-server
```

## Configuration

### Validating configuration

ScanFlow validates the configuration at startup. To check without starting the server:

```bash
scanflow-server -config /etc/scanflow/server.toml -version
```

If the configuration is invalid, the server will report all errors and exit.

### Updating configuration

1. Edit `/etc/scanflow/server.toml`.
2. Restart the service: `sudo systemctl restart scanflow`
3. Verify the service started: `sudo systemctl status scanflow`

## Monitoring

### Health checks

| Endpoint | Auth | Purpose |
|----------|------|---------|
| `GET /api/v1/health` | No | Liveness probe — returns 200 if running |
| `GET /api/v1/ready` | No | Readiness probe — returns 200 if scanner connected |
| `GET /metrics` | No | Prometheus metrics |

### Key metrics to alert on

| Metric | Condition | Action |
|--------|-----------|--------|
| `scanflow_jobs_active` | > max_concurrent_jobs | Check for stuck jobs |
| `scanflow_jobs_total{status="failed"}` | Increasing | Check logs for output errors |
| `scanflow_http_requests_total{code="429"}` | Sustained | Possible brute-force attempt |
| `scanflow_http_requests_total{code="500"}` | Any | Check server logs immediately |

### Log analysis

ScanFlow uses structured JSON logging by default. Filter by severity:

```bash
# Errors only
journalctl -u scanflow | jq 'select(.level == "ERROR")'

# Job lifecycle
journalctl -u scanflow | jq 'select(.msg | startswith("job"))'
```

## Maintenance

### Job cleanup

ScanFlow automatically removes completed, failed, and cancelled jobs after the configured retention period (default: 30 days). The cleanup runs every minute.

To manually inspect persisted jobs:

```bash
ls /var/lib/scanflow/documents/jobs/
cat /var/lib/scanflow/documents/jobs/<job-id>.json | jq .
```

### TLS certificate renewal

When using ACME (Let's Encrypt), certificates are renewed automatically. Monitor renewal with:

```bash
journalctl -u scanflow | grep -i acme
```

### Backup

Back up the following paths:

| Path | Content |
|------|---------|
| `/etc/scanflow/` | Configuration and profiles |
| `/etc/scanflow/profiles/` | Custom scan profiles |
| `/var/lib/scanflow/` | Persistent job state and documents |
| ACME cert directory | TLS certificates |

Example backup script:

```bash
#!/bin/bash
tar -czf scanflow-backup-$(date +%Y%m%d).tar.gz \
  /etc/scanflow/ \
  /var/lib/scanflow/
```

### Upgrading

1. Stop the service: `sudo systemctl stop scanflow`
2. Replace the binary: `sudo cp scanflow-server /opt/scanflow/`
3. Start the service: `sudo systemctl start scanflow`
4. Verify: `curl http://localhost:8080/api/v1/health`

Job state is persisted to disk and will be restored on startup.

## Security

### API key management

- Store API keys in the configuration file with restricted permissions:
  ```bash
  sudo chmod 640 /etc/scanflow/server.toml
  sudo chown root:scanner /etc/scanflow/server.toml
  ```
- Rotate API keys periodically by updating the configuration and restarting.

### Rate limiting

The server limits requests to 10 per second per IP with a burst of 20. Clients exceeding the limit receive HTTP 429 responses.

### Network security

- Use TLS in production (either manual certificates or ACME).
- Restrict access to the scanner server to trusted networks.
- Use a reverse proxy (nginx, Caddy) for additional protection.
