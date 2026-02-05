FROM golang:1.22-bookworm AS builder

WORKDIR /app
COPY server/ ./server/
COPY configs/ ./configs/

WORKDIR /app/server
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /scanflow-server ./cmd/scanflow-server

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    sane-utils libsane1 \
    tesseract-ocr tesseract-ocr-deu \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /etc/scanflow /var/lib/scanflow /var/log/scanflow /tmp/scanflow

COPY --from=builder /scanflow-server /usr/local/bin/
COPY configs/server.toml /etc/scanflow/
COPY configs/profiles/ /etc/scanflow/profiles/

RUN useradd -r -s /sbin/nologin scanner && \
    chown -R scanner:scanner /var/lib/scanflow /var/log/scanflow /tmp/scanflow

USER scanner

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/health || exit 1

CMD ["scanflow-server", "-config", "/etc/scanflow/server.toml"]
