# Performance Tuning Guide

This guide covers tuning ScanFlow for high-throughput environments where multiple scanners or heavy scan volumes require optimized resource usage.

## Concurrency Settings

### Max Concurrent Jobs

The most impactful setting is the number of jobs processed in parallel:

```toml
[processing]
max_concurrent_jobs = 4    # default: 2
```

**Guidelines:**

| Hardware | Recommended `max_concurrent_jobs` |
|---|---|
| Raspberry Pi 4 (4 GB) | 1–2 |
| Mid-range server (8 GB, 4 cores) | 2–4 |
| Dedicated server (32 GB, 8+ cores) | 4–8 |

Each concurrent job consumes:

- **RAM**: ~50–200 MB depending on scan resolution and page count
- **CPU**: Significant during OCR; low during scanning and PDF creation
- **Disk I/O**: Temporary files written per page

Monitor memory usage before increasing this value.

### Job Timeout

For large batch scans, increase the job timeout:

```toml
[processing]
job_timeout = "30m"    # default: 10m
```

## Scanner Optimization

### Resolution Selection

Higher resolutions produce larger images and slower processing. Choose the minimum resolution that meets your needs:

| Use Case | Resolution | Relative Speed |
|---|---|---|
| Text documents (OCR) | 200–300 DPI | Fast |
| Mixed text and images | 300 DPI | Normal |
| Photos / archival | 600 DPI | Slow |
| Draft / preview | 150 DPI | Very fast |

### Scan Mode

Grayscale scanning is significantly faster and produces smaller files than color:

```toml
# In a profile
[scanner]
mode = "gray"    # instead of "color"
```

For text-only documents, `lineart` mode is fastest but not suitable for OCR.

### Duplex vs. Simplex

Duplex (double-sided) scanning doesn't necessarily halve throughput. Many scanners scan both sides in a single pass. However, blank page removal becomes important with duplex:

```toml
[processing]
optimize_images = true
remove_blank_pages = true
blank_threshold = 0.98    # slightly lower threshold catches more blank backs
```

## OCR Performance

OCR is typically the slowest step in the pipeline. Optimization options:

### Disable OCR When Not Needed

```toml
[processing.ocr]
enabled = false
```

Or use profiles — enable OCR only on the standard text profile and disable it for photo scans.

### Language Selection

Fewer OCR languages mean faster processing:

```toml
[processing.ocr]
language = "deu"           # single language is faster
# language = "deu+eng"     # multi-language is slower
```

### ocrmypdf vs. Tesseract

ScanFlow prefers `ocrmypdf` over raw Tesseract. Install it for best results:

```bash
pip install ocrmypdf
```

`ocrmypdf` features like `--skip-text` (skip pages that already have text) and `--optimize` can significantly reduce processing time for mixed documents.

### Tesseract Thread Control

Tesseract uses multiple threads by default. On a shared system, limit parallelism:

```bash
export OMP_THREAD_LIMIT=2
```

Set this in the systemd unit file or Docker environment.

## PDF Creation

### JPEG Quality

Lower quality means smaller files and faster I/O at the cost of visual fidelity:

```toml
[processing.pdf]
jpeg_quality = 75    # default: 85, range 1-100
```

For text documents, quality 60–75 is usually acceptable. For photos, keep 85+.

### Image Filters

Image filters add per-page processing time. Enable only what you need:

```toml
[processing.image_filters]
auto_rotate = false           # adds rotation detection time
color_to_grayscale = false    # useful to shrink color scans
brightness_adjust = 0.0       # 0 = no change (fastest)
contrast_adjust = 0.0
normalize_exposure = false    # scans full image histogram
```

In profiles, set filters per use case rather than globally.

## Output Optimization

### Paperless-NGX

Paperless has its own processing queue. Avoid overwhelming it:

- Don't submit more than 2-3 documents per second
- Enable retry with backoff (built-in) so transient failures don't drop documents
- Use the Paperless consume folder for very high throughput — it avoids HTTP overhead

### SMB / Network Shares

For high-volume SMB output:

- Use a wired Ethernet connection (not Wi-Fi)
- Consider the `paperless_consume` output which writes directly to a directory, avoiding per-file SMB session setup

### Filesystem Output

For maximum throughput, write to a local SSD:

```toml
[output.filesystem]
directory = "/mnt/fast-ssd/scans"
```

## Memory Management

### Temporary Directory

Use a fast filesystem (SSD or tmpfs) for the processing temp directory:

```toml
[processing]
temp_directory = "/tmp/scanflow"
```

For systems with enough RAM, a tmpfs mount eliminates disk I/O during processing:

```bash
# Add to /etc/fstab
tmpfs /tmp/scanflow tmpfs size=2G,mode=1777 0 0
```

### Job Storage Retention

Reduce retention to limit disk usage:

```toml
[storage]
retention_days = 7    # default: 30
```

## Monitoring Performance

### Prometheus Metrics

ScanFlow exposes metrics at `/metrics`:

```bash
curl http://localhost:8080/metrics
```

Key metrics to watch:

| Metric | What It Tells You |
|---|---|
| `scanflow_jobs_total{status}` | Total jobs by outcome |
| `scanflow_jobs_active` | Currently processing jobs |
| `scanflow_scan_pages_total` | Total pages scanned |
| `scanflow_http_request_duration_seconds` | API latency |

### Alerting Thresholds

Suggested Prometheus alert rules:

```yaml
# Job processing taking too long
- alert: ScanFlowSlowJobs
  expr: scanflow_jobs_active > 0 and scanflow_http_request_duration_seconds_sum / scanflow_http_request_duration_seconds_count > 30
  for: 5m

# Queue depth growing
- alert: ScanFlowQueueBacklog
  expr: scanflow_jobs_pending > 10
  for: 5m
```

### System-Level Monitoring

Watch these OS-level metrics alongside ScanFlow:

- **CPU**: `top` or `htop` — OCR is CPU-intensive
- **Memory**: Free memory should stay above 500 MB
- **Disk I/O**: `iostat` — temp directory writes during processing
- **Network**: `iftop` — for SMB and Paperless uploads

## Hardware Sizing Guide

| Workload | CPU | RAM | Storage | Expected Throughput |
|---|---|---|---|---|
| Light (< 50 scans/day) | 2 cores | 2 GB | SD card | ~5 pages/min |
| Medium (50–200 scans/day) | 4 cores | 4 GB | SSD | ~15 pages/min |
| Heavy (200+ scans/day) | 8 cores | 8 GB | NVMe SSD | ~30 pages/min |

Throughput depends heavily on OCR usage and scan resolution.

## Example High-Throughput Configuration

```toml
[processing]
temp_directory = "/tmp/scanflow"
max_concurrent_jobs = 4
job_timeout = "20m"

[processing.pdf]
jpeg_quality = 75
compression = "jpeg"

[processing.ocr]
enabled = true
language = "deu"

[processing.image_filters]
auto_rotate = false
color_to_grayscale = true   # reduce file size
normalize_exposure = false

[storage]
retention_days = 7
```
