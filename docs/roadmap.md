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

## Near Term

- Improve output handler coverage and regression tests
- Strengthen release artifacts with packaged archives and checksum publication
- Expand deployment ergonomics for the server, including built-in service installation
- Improve troubleshooting guidance for scanner permissions, OCR and Paperless connectivity
- Add rate limiting middleware to protect against brute-force API key guessing
- TLS certificate auto-renewal helpers (Let's Encrypt / ACME)

## Mid Term

- Add integration tests for output backends with mock services
- Expose more scanner capabilities in the Web UI and CLI
- Improve retry handling and observability for long-running uploads
- Add profile import/export helpers for easier rollout across multiple clients
- Implement job retention / automatic cleanup for old completed jobs
- Audit and pin third-party dependency versions with vulnerability scanning

## Long Term

- Native packaging for Linux distributions
- Better remote fleet management for multiple ScanFlow servers
- Richer document post-processing pipeline options
- Optional signed release artifacts and SBOM generation
- Role-based access control for multi-user environments
- Encrypted at-rest storage for scanned documents
