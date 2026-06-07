# Configuration

Configuration is provided via environment variables.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HASS_URL` | Home Assistant URL | required |
| `HASS_TOKEN` | Long-lived access token | required |
| `HASS_TIMEOUT` | HTTP timeout for backup operations (e.g. `10m`, `1h`, `30s`) | `10m` |
| `HASS_INSECURE` | Skip TLS verification for self-signed certs | `false` |
| `STORAGE_URL` | Storage backend URL (see below) | `file://./backups` |
| `STORAGE_PREFIX` | Object key prefix for backup files | `home-assistant/` |
| `LOG_LEVEL` | Log level: `debug`, `info`, `warn`, `error` | `info` |
| `LOG_FORMAT` | Log format: `pretty`, `text`, `json` | `pretty` |
| `RETENTION_KEEP_LAST` | Number of backups to keep (0 = unlimited) | `30` |

## Storage Backends

| Scheme | Description | Example |
|--------|-------------|---------|
| `file://` | Local filesystem | `file://./backups` |
| `s3://` | Amazon S3 or S3-compatible (e.g. Ceph RGW) | `s3://bucket-name` |
| `gs://` | Google Cloud Storage | `gs://bucket-name` |

### S3-Compatible Storage

For S3-compatible backends (e.g. Ceph RGW, MinIO), set `AWS_ENDPOINT_URL`:

```bash
export STORAGE_URL=s3://my-bucket?use_path_style=true
export AWS_ENDPOINT_URL=https://ceph.example.com
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
```

Append `?use_path_style=true` to `STORAGE_URL` for backends that don't support virtual-hosted-style addressing (Ceph RGW, MinIO without wildcard DNS, etc.). Without it the AWS SDK rewrites the endpoint as `<bucket>.<endpoint>` and the DNS lookup will fail with `no such host`. Plain AWS S3 does not need this flag.
