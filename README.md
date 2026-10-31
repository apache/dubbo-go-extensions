# Dubbo-Go Extensions

English | [简体中文](./README_CN.md)

Community-maintained extensions for Apache Dubbo-Go v3.

## Available Extensions

### Filters

- **[hystrix](./filter/hystrix/)** - Circuit breaker filter using hystrix-go
- **[adaptivesvc](./filter/adaptivesvc/)** - Provider-side adaptive service throttling filter

### Clusters

- **[adaptiveService](./cluster/cluster/adaptivesvc/)** - Consumer-side adaptive service cluster

### Load Balances

- **[p2c](./cluster/loadbalance/p2c/)** - Power of Two Choices load balancer using adaptive service metrics

### Metrics

- **hill-climbing** - Local method metrics for adaptive service load balancing (`./cluster/metrics`)

### Config Centers

## Usage

Provider and consumer extensions can be enabled independently. Use the provider
option with `server.WithExtension`, and the consumer option with `client.NewClient`:

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

The consumer option selects both `adaptiveService` and `p2c`. Side-effect
imports only register implementations; they do not select client strategies.
See the [consumer guide](./cluster/cluster/adaptivesvc/README.md) and
[provider guide](./filter/adaptivesvc/README.md) for complete examples.

See individual extension directories for detailed documentation.

## License

Apache License 2.0 - See [LICENSE](./LICENSE) for details.
