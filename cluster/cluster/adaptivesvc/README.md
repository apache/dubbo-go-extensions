# Adaptive Service Client

English | [简体中文](./README_CN.md)

The consumer cluster uses provider-reported remaining capacity to route requests.
The provider must enable the `padasvc` filter; see the [provider guide](../../../filter/adaptivesvc/README.md).

## Client-Wide Setup

```go
import (
    adaptivesvc "github.com/apache/dubbo-go-extensions/cluster/cluster/adaptivesvc"

    "dubbo.apache.org/dubbo-go/v3/client"
)

func main() {
    cli, err := client.NewClient(adaptivesvc.WithClientAdaptiveService())
    _ = cli
    _ = err
}
```

`WithClientAdaptiveService` registers P2C and selects the `adaptiveService`
cluster plus `p2c` load balancer for this client's references. Reference-level
cluster or load-balance options can override those defaults.

Alternatively, import both packages for registration and select the two
strategies explicitly using dubbo-go's typed client options:

```go
import (
    _ "github.com/apache/dubbo-go-extensions/cluster/cluster/adaptivesvc"
    _ "github.com/apache/dubbo-go-extensions/cluster/loadbalance/p2c"

    "dubbo.apache.org/dubbo-go/v3/client"
)

func main() {
    cli, err := client.NewClient(
        client.WithClientClusterAdaptiveService(),
        client.WithClientLoadBalanceP2C(),
    )
    _ = cli
    _ = err
}
```

For one reference rather than an entire client, use `client.WithClusterAdaptiveService()`
and `client.WithLoadBalanceP2C()` when creating that reference.

`client.WithExtension` (the scoped extension configuration API) only initializes
extension configuration and contributes filter names. It does not select a
cluster or load balancer. The options above are therefore required for client
adaptive service routing.

## Behavior And Keys

The consumer cluster sets `adaptive-service.enabled=1` on outgoing calls. The
provider filter applies its per-method hill-climbing limit and returns
`adaptive-service.remaining` and `adaptive-service.inflight`. The consumer
stores remaining capacity under the method-level `hill-climbing` metrics key;
P2C compares two providers using that metric. Without a cooperating provider,
P2C has no capacity data to compare.

| Component | Key |
| --- | --- |
| Provider filter | `padasvc` |
| Consumer cluster | `adaptiveService` |
| Load balancer | `p2c` |
| Local metrics | `hill-climbing` |

The built-in dubbo-go implementation remains available during migration.
These packages let applications explicitly depend on the extensions module.
