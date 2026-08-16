# Adaptive Service Throttling

English | [简体中文](./README_CN.md)

Adaptive service throttling for dubbo-go. The extension includes the provider-side adaptive service filter, the consumer-side adaptive service cluster, the P2C load balancer, and local method metrics used by the load balancer.

This extension keeps the same keys and behavior as the current built-in dubbo-go implementation, while letting applications depend on `github.com/apache/dubbo-go-extensions` explicitly.

## Installation

```bash
go get github.com/apache/dubbo-go-extensions/imports/adaptivesvc
```

Import `github.com/apache/dubbo-go-extensions/imports/adaptivesvc` to register
the provider filter, consumer cluster, and P2C load balancer together.

## Provider Side

The aggregate import registers the provider filter. It can also be registered
independently with a side-effect import:

```go
import (
    _ "github.com/apache/dubbo-go-extensions/filter/adaptivesvc"
)
```

Configure the provider filter key:

```text
padasvc
```

The provider filter only applies throttling when the invocation contains `adaptive-service.enabled=1`. When enabled, it uses the hill-climbing limiter and returns provider status through response attachments:

- `adaptive-service.remaining`
- `adaptive-service.inflight`

## Consumer Side

The aggregate import registers the adaptive service cluster and P2C load
balancer. They can also be registered independently with side-effect imports:

```go
import (
    _ "github.com/apache/dubbo-go-extensions/cluster/cluster/adaptivesvc"
    _ "github.com/apache/dubbo-go-extensions/cluster/loadbalance/p2c"
)
```

Configure the consumer with:

- cluster: `adaptiveService`
- load balance: `p2c`

The consumer cluster sets `adaptive-service.enabled=1` on outgoing invocations. The provider filter uses that attachment to decide whether to throttle and report capacity. The consumer cluster reads the provider's `adaptive-service.remaining` attachment and writes it to local method metrics. The P2C load balancer then chooses between two candidate providers by preferring the node with higher remaining hill-climbing capacity.

The adaptive service cluster currently supports only the `p2c` load balancer.

## Keys

- Provider filter: `padasvc`
- Cluster: `adaptiveService`
- Load balance: `p2c`
- Metrics key: `hill-climbing`
- Enable attachment: `adaptive-service.enabled`
- Enable value: `1`
- Remaining attachment: `adaptive-service.remaining`
- Inflight attachment: `adaptive-service.inflight`

## Verbose Limiter Logs

The limiter exposes a package-level `Verbose` switch for debug logs:

```go
import "github.com/apache/dubbo-go-extensions/filter/adaptivesvc/limiter"

func init() {
    limiter.Verbose = true
}
```

## Compatibility

After migration, adaptive service implementations are registered by this
extensions module. Applications using the migrated feature should import the
aggregate package above. The registered keys are:

- `padasvc`
- `adaptiveService`
- `p2c`

The implementation keeps the public configuration keys and adaptive service
behavior compatible with the former built-in implementation.
