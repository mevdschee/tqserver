# SOCKS5 Proxy

TQServer includes a built-in SOCKS5 proxy for logging all outgoing API calls made by workers.

## Overview

When enabled, the SOCKS5 proxy:
- Runs on `127.0.0.1:1080` (configurable)
- Logs all outgoing connections with metadata
- Optionally inspects HTTPS traffic (dev mode only)

## Configuration

Enable in `config/server.yaml`:

```yaml
socks5:
  enabled: true
  port: 1080
  log_file: "logs/socks5_{date}.log"
  log_format: "json"  # or "text"
```

### HTTPS Inspection (Development Only)

For full request/response body logging of HTTPS connections:

```yaml
socks5:
  enabled: true
  https_inspection:
    enabled: true
    ca_cert: "config/tqserver-ca.crt"
    ca_key: "config/tqserver-ca.key"
    auto_generate: true
    log_body: true
    max_body_size: 1048576
```

> [!CAUTION]
> Only enable HTTPS inspection in development. It creates a CA certificate that must be trusted by workers and logs decrypted traffic.

## Environment Variables

When SOCKS5 is enabled, workers receive:

| Variable | Description |
|----------|-------------|
| `SOCKS5_PROXY` | `socks5://127.0.0.1:1080` |
| `ALL_PROXY` | `socks5://127.0.0.1:1080` |
| `TQSERVER_WORKER_UA` | `TQServer/{worker_name}` |
| `SSL_CERT_FILE` | CA cert path (if HTTPS inspection enabled) |
| `NODE_EXTRA_CA_CERTS` | CA cert path for Node/Bun (if HTTPS inspection enabled) |

## Log Format

### JSON (default)

```json
{"timestamp":"2026-01-10T01:35:00Z","worker_name":"api","dest_host":"api.stripe.com","dest_port":443,"protocol":"https","bytes_sent":1234,"bytes_recv":5678,"duration_ms":150}
```

### Text

```text
[2026-01-10 01:35:00] [api] CONNECT api.stripe.com:443 -> 1234 sent, 5678 recv, 150ms
```

## Worker Usage

Workers must configure their HTTP clients to use the proxy.

### Go Workers

```go
import (
    "net/url"
    "os"
    "golang.org/x/net/proxy"
)

proxyURL, _ := url.Parse(os.Getenv("SOCKS5_PROXY"))
dialer, _ := proxy.SOCKS5("tcp", proxyURL.Host, nil, proxy.Direct)
transport := &http.Transport{Dial: dialer.Dial}
client := &http.Client{Transport: transport}
```

### TypeScript/Bun Workers

```typescript
import { SocksProxyAgent } from 'socks-proxy-agent';

const agent = new SocksProxyAgent(process.env.SOCKS5_PROXY);
const response = await fetch('https://api.example.com', { agent });
```

### PHP Workers

```php
$client = new GuzzleHttp\Client([
    'proxy' => getenv('SOCKS5_PROXY'),
]);
```

## Correlation IDs

TQServer adds an `X-Correlation-ID` header to all incoming requests. This ID can be used to correlate incoming requests with outgoing API calls in the SOCKS5 logs.
