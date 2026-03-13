# Upgrade and Migration Guide

This guide covers upgrading ScanFlow between versions, migrating configuration files, and rolling back if something goes wrong.

## General Upgrade Procedure

### 1. Back Up Before Upgrading

Always create a backup before upgrading:

```bash
# Stop the service
sudo systemctl stop scanflow

# Back up configuration
sudo cp -r /etc/scanflow /etc/scanflow.bak

# Back up persistent job storage
sudo cp -r /var/lib/scanflow /var/lib/scanflow.bak

# Back up the current binary
sudo cp /usr/local/bin/scanflow-server /usr/local/bin/scanflow-server.bak
```

### 2. Replace the Binary

Download the new release from the [GitHub releases page](https://github.com/thoscut/ScanFlow/releases) and replace the existing binary:

```bash
# Download and extract (adjust platform/arch as needed)
tar -xzf scanflow-server-linux-amd64.tar.gz

# Replace the binary
sudo cp scanflow-server-linux-amd64 /usr/local/bin/scanflow-server
sudo chmod +x /usr/local/bin/scanflow-server
```

### 3. Review the Release Notes

Check the release notes for any required configuration changes. New configuration keys are always optional and use sensible defaults, so existing configuration files continue to work.

### 4. Start the Service

```bash
sudo systemctl start scanflow
sudo systemctl status scanflow

# Verify the new version
curl -s http://localhost:8080/api/v1/health | jq .
```

## Docker Upgrades

For Docker deployments, pull the new image and restart:

```bash
# Pull the new version
docker pull ghcr.io/thoscut/scanflow-server:v1.2.0

# Or pull latest
docker pull ghcr.io/thoscut/scanflow-server:latest

# Restart the container
docker-compose -f deploy/docker/docker-compose.yml down
docker-compose -f deploy/docker/docker-compose.yml up -d
```

Persistent data stored in Docker volumes is preserved across upgrades.

## Client Upgrades

Clients are stateless and can be upgraded independently of the server. Replace the binary on each machine:

```bash
# Linux/macOS
sudo cp scanflow-linux-amd64 /usr/local/bin/scanflow
sudo chmod +x /usr/local/bin/scanflow

# Windows – replace scanflow.exe in your PATH
```

There is no dependency between client and server versions for the same major version. Clients from an older minor release work with a newer server and vice versa.

## Configuration Migration

### Adding New Sections

When a new release introduces configuration keys, they are always optional and populated with defaults. You do **not** need to add them to your existing `server.toml` unless you want to change the default behavior.

For example, if a release adds image filter settings:

```toml
# Optional – only add if you want to enable filters
[processing.image_filters]
auto_rotate = false
color_to_grayscale = false
brightness_adjust = 0.0
contrast_adjust = 0.0
normalize_exposure = false
```

### Verifying Configuration

After upgrading, validate your configuration:

```bash
scanflow-server -config /etc/scanflow/server.toml -version
```

If the configuration is invalid, the server exits with a clear error message describing all problems.

### Profile Compatibility

Scan profiles (TOML files in `/etc/scanflow/profiles/`) are forward-compatible. New profile fields are ignored by older versions and default to zero values in newer versions.

You can export profiles from one server and import them to another:

```bash
# Export
curl -H "Authorization: Bearer $KEY" \
  http://old-server:8080/api/v1/profiles/standard/export > standard.toml

# Import
curl -X POST -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/toml" \
  --data-binary @standard.toml \
  http://new-server:8080/api/v1/profiles/import
```

## Rollback Procedure

If an upgrade causes problems, roll back to the previous version:

```bash
# Stop the service
sudo systemctl stop scanflow

# Restore the binary
sudo cp /usr/local/bin/scanflow-server.bak /usr/local/bin/scanflow-server

# Restore configuration (if changed)
sudo cp -r /etc/scanflow.bak/* /etc/scanflow/

# Restore job storage (if schema changed)
sudo cp -r /var/lib/scanflow.bak/* /var/lib/scanflow/

# Start the old version
sudo systemctl start scanflow
```

### Docker Rollback

```bash
docker-compose -f deploy/docker/docker-compose.yml down
# Edit docker-compose.yml to pin the previous version tag
docker-compose -f deploy/docker/docker-compose.yml up -d
```

## Data Compatibility

### Job Storage

The persistent job store uses JSON files in `/var/lib/scanflow/documents/jobs/`. The format is forward-compatible: new fields are silently ignored by older versions. Downgrading may lose metadata for fields that were added in the newer release, but jobs themselves remain intact.

### Scan Profiles

TOML profile files are always additive. Unknown keys are silently ignored, so profiles created on a newer version can be used on older versions without errors (new features simply will not apply).

## Version Compatibility Matrix

| Server Version | Client Versions | Notes |
|---|---|---|
| v1.x | v1.x | Full compatibility within major version |
| v1.x | v0.x | Clients may lack newer API features |

## Checking Your Version

```bash
# Server
scanflow-server --version

# Client
scanflow --version

# Via API
curl http://localhost:8080/api/v1/health
```
