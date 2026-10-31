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

## Development

Install [pre-commit](https://pre-commit.com/#install) and the import formatter,
then enable the Git hook in each clone:

```sh
uv tool install pre-commit
go install github.com/dubbogo/tools/cmd/imports-formatter@latest
export PATH="$(go env GOPATH)/bin:$PATH"
pre-commit install
```

Go and its `gofmt` command must also be available on `PATH`. If you set `GOBIN`,
add that directory to `PATH` instead of `$(go env GOPATH)/bin`.

When a commit includes Go files, the hook runs `gofmt -s -w .` followed by
`imports-formatter` from the repository root, matching CI. Both formatters scan
the working tree, including untracked Go files. If tracked files are changed,
the commit stops so you can review and stage the formatting changes before
committing again. Unstaged changes to tracked files are temporarily stashed by
pre-commit and restored afterward.

To run the checks manually:

```sh
pre-commit run --all-files
```

## License

Apache License 2.0 - See [LICENSE](./LICENSE) for details.
