# Home Assistant Backup

A Golang application to automatically back up your Home Assistant instance to S3-compatible storage or Google Cloud Storage.

See [CONFIGURATION.md](CONFIGURATION.md) for setup details.

A Docker image is available at `ghcr.io/connorsapps/hass-backup:latest`

## Quick Start

```bash
# Set credentials
export HASS_URL=https://your-homeassistant
export HASS_TOKEN=your-long-lived-access-token

# Run (backups save to ./backups by default)
go run github.com/ConnorsApps/hass-backup

# Run with Docker
docker run --rm \
  -e HASS_URL=https://your-homeassistant \
  -e HASS_TOKEN=your-long-lived-access-token \
  -e STORAGE_URL=s3://my-bucket \
  -e AWS_ACCESS_KEY_ID=... \
  -e AWS_SECRET_ACCESS_KEY=... \
  ghcr.io/connorsapps/hass-backup:latest
```

## Requirements

- Home Assistant with the Supervisor API (Home Assistant OS or Supervised)
- A long-lived access token from your Home Assistant profile
