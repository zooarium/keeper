# Deployment Guide

This document describes how to deploy the Keeper API as a standalone service on a Linux server.

## 1. Build the Binary

The project includes a `Makefile` target to build a statically linked production binary inside a Docker container. This ensures the binary has all its dependencies bundled and can run on any compatible Linux system.

Run the following command on your build machine (requires Docker):

```bash
make build-prod
```

The resulting binary will be located at `bin/keeper`.

## 2. Prepare the Server

### Create a Dedicated User

For security, it's recommended to run the service under a dedicated non-root user.

```bash
sudo useradd -r -s /bin/false keeper
```

### Setup Directory Structure

Ensure the application directory exists and has the necessary subdirectories.

```bash
sudo mkdir -p /var/www/zoo/keeper/bin
sudo mkdir -p /var/www/zoo/keeper/config
sudo mkdir -p /var/www/zoo/keeper/data
sudo mkdir -p /var/www/zoo/keeper/log
```

### Deploy Files

Copy the binary and configuration files to the deployment directory.

```bash
sudo cp bin/keeper /var/www/zoo/keeper/bin/
sudo cp config/config.yaml /var/www/zoo/keeper/config/
```

### Set Permissions

Change the ownership of the application directory to the `keeper` user.

```bash
sudo chown -R keeper:keeper /var/www/zoo/keeper
sudo chmod +x /var/www/zoo/keeper/bin/keeper
```

## 3. Configure the Service

### Systemd Unit File

Create a new systemd unit file at `/etc/systemd/system/keeper.service`:

```ini
[Unit]
Description=Keeper User Management API
After=network.target

[Service]
User=keeper
Group=keeper
WorkingDirectory=/var/www/zoo/keeper

# Environment Variables
Environment="KEEPER_ENVIRONMENT=production"
Environment="KEEPER_SERVER_ADDR=:8080"
Environment="KEEPER_SERVER_HOST=localhost:8080"
Environment="KEEPER_SERVER_READ_TIMEOUT=5s"
Environment="KEEPER_SERVER_WRITE_TIMEOUT=10s"
Environment="KEEPER_SERVER_IDLE_TIMEOUT=120s"
Environment="KEEPER_DB_PATH=/var/www/zoo/keeper/data/keeper.db"
Environment="KEEPER_LOG_DIR=/var/www/zoo/keeper/log"
Environment="KEEPER_AUTH_JWT_SECRET=change-me-to-a-secure-random-string"
Environment="KEEPER_AUTH_JWT_EXPIRY=24h"
Environment="KEEPER_CORS_ALLOWED_ORIGINS=*"

ExecStart=/var/www/zoo/keeper/bin/keeper
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Environment Variables Reference

The application uses Viper for configuration, which maps nested keys to environment variables using underscores and a `KEEPER_` prefix (e.g., `SERVER.ADDR` becomes `KEEPER_SERVER_ADDR`).

| Variable | Description | Default |
|----------|-------------|---------|
| `KEEPER_ENVIRONMENT` | Deployment environment (production, development) | `production` |
| `KEEPER_SERVER_ADDR` | Port/Address to listen on | `:8080` |
| `KEEPER_SERVER_HOST` | Public host name for Swagger docs | `localhost:8080` |
| `KEEPER_SERVER_READ_TIMEOUT` | Max duration for reading the entire request | `5s` |
| `KEEPER_SERVER_WRITE_TIMEOUT` | Max duration before timing out writes of the response | `10s` |
| `KEEPER_SERVER_IDLE_TIMEOUT` | Max amount of time to wait for the next request | `120s` |
| `KEEPER_DB_PATH` | Path to the SQLite database file | `data/keeper.db` |
| `KEEPER_LOG_DIR` | Directory where logs will be stored | `log` |
| `KEEPER_AUTH_JWT_SECRET` | Secret key for signing JWT tokens | (Required) |
| `KEEPER_AUTH_JWT_EXPIRY` | Duration until JWT tokens expire | `24h` |
| `KEEPER_CORS_ALLOWED_ORIGINS` | Allowed origins for CORS (comma-separated) | `*` |

## 4. Start and Enable the Service

Reload systemd to recognize the new service, then start and enable it to run at boot.

```bash
sudo systemctl daemon-reload
sudo systemctl start keeper
sudo systemctl enable keeper
```

## 5. Verification

### Check Service Status

```bash
sudo systemctl status keeper
```

### Check Logs

```bash
tail -f /var/www/zoo/keeper/log/api.log
# OR
journalctl -u keeper -f
```

### Test the API

```bash
curl http://localhost:8080/health
```

## 6. Updating the Application

To update the application to a new version:

1. Build the new binary: `make build-prod`.
2. Stop the service: `sudo systemctl stop keeper`.
3. Replace the binary: `sudo cp bin/keeper /var/www/zoo/keeper/bin/`.
4. Start the service: `sudo systemctl start keeper`.

The application will automatically handle database migrations on startup.
