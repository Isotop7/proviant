# Demo Instance

Public demo of Proviant with pre-seeded data that resets nightly.

## Demo Credentials

| Username | Password | Email Verified |
|----------|----------|----------------|
| `demo` | `demo` | Yes |
| `alice` | `AliceDemo123!` | Yes |
| `bob` | `BobDemo123!` | Yes |

## Configuration

Set these environment variables to enable demo mode:

| Variable | Description |
|----------|-------------|
| `PROVIANT_SERVER_DEMO_MODE=true` | Shows demo banner |
| `PROVIANT_SERVER_AUTH_SKIP_EMAIL_VERIFICATION=true` | Skips email verification |
| `PROVIANT_DEMO_MODE=true` | Triggers seed DB copy on startup |
| `NIGHTLY_RESET=true` | Forces fresh seed DB on next start |

## Docker Demo Image

```bash
docker build -f Dockerfile.demo -t proviant:demo .
docker run -p 8080:5050 \
  -e PROVIANT_SERVER_DEMO_MODE=true \
  -e PROVIANT_SERVER_AUTH_SKIP_EMAIL_VERIFICATION=true \
  -e PROVIANT_DEMO_MODE=true \
  proviant:demo
```

The demo Dockerfile builds a seed database from `seed.go` and copies it into the image at `/seed/demo_seed.db`. On startup with `PROVIANT_DEMO_MODE=true`, the container copies the seed DB to its data volume. Set `NIGHTLY_RESET=true` to force a fresh copy.