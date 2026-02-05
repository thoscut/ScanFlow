VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: all
all: build-server build-client

# --- Server ---

.PHONY: build-server
build-server:
	cd server && go build $(LDFLAGS) -o ../dist/scanflow-server ./cmd/scanflow-server

.PHONY: run-server
run-server:
	cd server && go run ./cmd/scanflow-server -config ../configs/server.toml

# --- Client ---

.PHONY: build-client
build-client:
	cd client && go build $(LDFLAGS) -o ../dist/scanflow ./cmd/scanflow

.PHONY: build-client-all
build-client-all:
	cd client && \
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o ../dist/scanflow-linux-amd64 ./cmd/scanflow && \
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o ../dist/scanflow-linux-arm64 ./cmd/scanflow && \
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o ../dist/scanflow-darwin-amd64 ./cmd/scanflow && \
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o ../dist/scanflow-darwin-arm64 ./cmd/scanflow && \
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o ../dist/scanflow-windows-amd64.exe ./cmd/scanflow

# --- Server Cross-Compile ---

.PHONY: build-server-arm64
build-server-arm64:
	cd server && GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o ../dist/scanflow-server-arm64 ./cmd/scanflow-server

# --- Tests ---

.PHONY: test
test: test-server test-client

.PHONY: test-server
test-server:
	cd server && go test -v ./...

.PHONY: test-client
test-client:
	cd client && go test -v ./...

.PHONY: test-integration
test-integration:
	cd server && go test -v -tags=integration ./...

# --- Code Quality ---

.PHONY: vet
vet:
	cd server && go vet ./...
	cd client && go vet ./...

.PHONY: fmt
fmt:
	cd server && gofmt -w .
	cd client && gofmt -w .

# --- Clean ---

.PHONY: clean
clean:
	rm -rf dist/
	cd server && go clean
	cd client && go clean

# --- Docker ---

.PHONY: docker-build
docker-build:
	docker build -t scanflow-server -f deploy/docker/Dockerfile .

.PHONY: docker-run
docker-run:
	docker-compose -f deploy/docker/docker-compose.yml up

# --- Install ---

.PHONY: install-server
install-server: build-server
	sudo mkdir -p /opt/scanflow /etc/scanflow /var/lib/scanflow /var/log/scanflow
	sudo cp dist/scanflow-server /opt/scanflow/
	sudo cp configs/server.toml /etc/scanflow/
	sudo cp configs/profiles/*.toml /etc/scanflow/profiles/ 2>/dev/null || true
	sudo cp deploy/systemd/scanflow.service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo "ScanFlow server installed. Run: sudo systemctl enable --now scanflow"

.PHONY: install-client
install-client: build-client
	sudo cp dist/scanflow /usr/local/bin/
	@echo "ScanFlow client installed."
