# Adaptive Service Throttling

English | [简体中文](./README_CN.md)

Provider-side adaptive service throttling for dubbo-go.

This extension keeps the same keys and behavior as the current built-in dubbo-go implementation, while letting applications depend on `github.com/apache/dubbo-go-extensions` explicitly.

## Installation

```bash
go get github.com/apache/dubbo-go-extensions/filter/adaptivesvc
```

Register the provider filter with a side-effect import, then enable adaptive
service through the dubbo-go server option:

```go
import (
    _ "github.com/apache/dubbo-go-extensions/filter/adaptivesvc"

    "dubbo.apache.org/dubbo-go/v3/server"
)

func main() {
    server.NewServer(
        server.WithServerAdaptiveService(),
    )
}
```

When adaptive service is enabled, dubbo-go automatically adds the registered
filter key `padasvc` to the provider filter chain.

The provider filter only applies throttling when the invocation contains `adaptive-service.enabled=1`. When enabled, it uses the hill-climbing limiter and returns provider status through response attachments:

- `adaptive-service.remaining`
- `adaptive-service.inflight`

The consumer side must set the enable attachment on outgoing invocations and
read the returned capacity attachments. Consumer-side adaptive service support
is outside the scope of this provider extension.

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
