# Roadmap

## Completed

- Request body size limits to prevent denial-of-service via large payloads
- Security response headers (X-Content-Type-Options, X-Frame-Options, CSP, Referrer-Policy)
- Path traversal prevention in filesystem and Paperless consume output handlers
- MIME header injection prevention in the email output handler
- URL parameter injection prevention in the Paperless task status query
- OCR language input validation to block command injection via Tesseract arguments
- Authentication failure logging for security monitoring
- Configurable CORS origins (backward-compatible wildcard default)
- Rate limiting middleware to protect against brute-force API key guessing
- TLS certificate auto-renewal via Let's Encrypt with HTTP-01 and DNS-01 challenges
- Built-in service installation for Linux (systemd) and Windows
- Release artifact checksums (SHA-256) published with every release
- Startup configuration validation to catch invalid settings before serving traffic
- Scan and processing operation timeouts to prevent indefinite hangs
- Readiness probe endpoint (`/api/v1/ready`) for container orchestration
- Request ID propagation in logs and error responses
- Output handler regression tests and integration tests with mock services
- Troubleshooting guide for scanner permissions, OCR and Paperless connectivity
- Packaged release archives (tar.gz and zip) for each platform
- Scanner capabilities API endpoint for UI and CLI integration
- Profile import and export via REST API for easier rollout across clients
- Job retention and automatic cleanup for old completed jobs
- Dependency auditing with govulncheck and gosec in the CI pipeline
- Prometheus-compatible metrics endpoint for operational monitoring
- Persistent job storage to survive server restarts
- Graceful job queue draining during shutdown
- Security scanning (SAST and dependency audits) in the CI pipeline
- Docker image build and publish in the release workflow
- Retry handling with exponential backoff for long-running uploads
- Operations and troubleshooting runbook
- Richer document post-processing pipeline options (grayscale conversion, brightness/contrast adjustment, exposure normalization)
- SBOM generation in CycloneDX format for release artifacts
- Upgrade and migration path documentation
- Performance tuning guide for high-throughput environments

## Long Term

- Native packaging for Linux distributions
- Better remote fleet management for multiple ScanFlow servers
- Optional signed release artifacts
- Role-based access control for multi-user environments
- Encrypted at-rest storage for scanned documents
- Backup and disaster recovery tooling
- Horizontal scaling and high-availability support
