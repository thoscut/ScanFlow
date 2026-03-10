# Hardware Recommendations

## Recommended Server Platforms

### Compact appliance

- Raspberry Pi 4 or 5
- 4 GB RAM minimum, 8 GB preferred for OCR-heavy workloads
- SSD over USB for temporary files and retention storage
- Wired Ethernet for large uploads to Paperless-NGX or SMB shares

### Small office / shared scanner

- Mini PC or thin client with x86_64 Linux
- 8 GB RAM or more
- Local SSD for scan buffering
- USB 3.0 ports for high-throughput scanners

## Scanner Recommendations

Choose a scanner that provides:

- proven SANE compatibility
- ADF support
- duplex scanning
- a hardware scan button if you want short/long press workflows directly on the device

## Practical Sizing Guidance

- **Occasional home use**: Raspberry Pi + one duplex ADF scanner
- **Frequent OCR and multi-user use**: x86_64 mini PC
- **Long document / oversize workflows**: scanner with reliable continuous feed and stable sensor reporting

## Validation Checklist

Before committing to hardware, verify:

```bash
scanimage -L
scanimage -A
```

These commands confirm that the scanner is visible to SANE and whether button/sensor options are exposed.
