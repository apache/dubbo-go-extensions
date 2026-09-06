# Adaptive Service Throttling

English | [简体中文](./README_CN.md)

Provider-side adaptive service throttling for dubbo-go.

This extension keeps the same keys and behavior as the current built-in dubbo-go implementation, while letting applications depend on `github.com/apache/dubbo-go-extensions` explicitly.

## Installation

```bash
go get github.com/apache/dubbo-go-extensions/filter/adaptivesvc
```

Import the extension and enable adaptive service through the option it
provides:

```go
import (
    adaptivesvc "github.com/apache/dubbo-go-extensions/filter/adaptivesvc"

    "dubbo.apache.org/dubbo-go/v3/server"
)

func main() {
    server.NewServer(
        adaptivesvc.WithServerAdaptiveService(),
    )
}
```

Importing the extension registers the `padasvc` filter. The extension option
adds that key to the provider filter chain without replacing filters that are
already configured.

The provider filter only applies throttling when the invocation contains `adaptive-service.enabled=1`. When enabled, it uses the hill-climbing limiter and returns provider status through response attachments:

- `adaptive-service.remaining`
- `adaptive-service.inflight`

The consumer side must set the enable attachment on outgoing invocations and
read the returned capacity attachments. Consumer-side adaptive service support
is outside the scope of this provider extension.

## Effect

The limiter maintains an adaptive concurrency limit for each service method.
When a method reaches its current limit, new requests are rejected by the
provider filter before entering the business handler. The limit is adjusted
from observed throughput and request latency, which keeps in-flight work within
the capacity learned for that method.

In the provider protection experiment from
[dubbo-go#3347](https://github.com/apache/dubbo-go/pull/3347), a client used 200
concurrent requests against a handler that took 200 ms. One sample at 10 seconds
reported 610 successful requests, 180 throttled requests, no other failures,
and a maximum of 53 requests executing in the handler concurrently:

```text
elapsed=10s started=840 success=610 rejected=180 failed=0 qps=61.0 reject_rate=21.4% avg=205ms p95=240ms server_active=50 server_max_active=53
```

The exact limit and rejection rate vary with workload and latency; the expected
effect is that excess requests are throttled before business execution instead
of allowing provider concurrency to grow without bound.

## Keys

- Provider filter: `padasvc`
- Enable attachment: `adaptive-service.enabled`
- Enable value: `1`
- Remaining attachment: `adaptive-service.remaining`
- Inflight attachment: `adaptive-service.inflight`

## Compatibility

The provider implementation remains available in dubbo-go. This extension lets
applications explicitly depend on the extensions module and register the same
`padasvc` provider filter key. The built-in and extension implementations
therefore coexist during the migration period.
