# Hystrix Filter

English | [简体中文](./README_CN.md)

The Hystrix filter adds circuit breaking to dubbo-go consumers and providers. Configure it with the dubbo-go extension API; the extension adds the matching filter to the client or server filter chain automatically. Methods without an explicit command configuration pass through unless the scope has a `default` command configuration.

## Breaking change

The filter now applies only to commands declared through this extension's Go or YAML configuration. Existing `hystrix.ConfigureCommand` calls alone no longer enable a command in this filter, and methods that previously received hystrix-go's implicit defaults now pass through unless `default` is configured. Migrate those commands to `hystrix.WithConfig(...)` or the `dubbo.extensions.hystrix` YAML configuration.

## Installation

```bash
go get github.com/apache/dubbo-go-extensions/filter/hystrix
```

## Consumer Configuration

```go
import (
	"dubbo.apache.org/dubbo-go/v3/client"
	"github.com/apache/dubbo-go-extensions/filter/hystrix"
)

cli, err := client.NewClient(
	client.WithExtension(
		hystrix.WithConfig(
			hystrix.WithCommandName("greet.GreetService:::Greet"),
			hystrix.WithTimeout(1000),
			hystrix.WithMaxConcurrentRequests(10),
			hystrix.WithRequestVolumeThreshold(5),
			hystrix.WithSleepWindow(5000),
			hystrix.WithErrorPercentThreshold(50),
		),
	),
)
```

Import `github.com/apache/dubbo-go-extensions/filter/hystrix` as `hystrix` to use its options. The extension configures Hystrix commands and adds its consumer filter automatically.

## Provider Configuration

```go
import (
	"dubbo.apache.org/dubbo-go/v3/server"
	"github.com/apache/dubbo-go-extensions/filter/hystrix"
)

srv, err := server.NewServer(
	server.WithExtension(
		hystrix.WithConfig(
			hystrix.WithCommandName("greet.GreetService:::Greet"),
			hystrix.WithTimeout(1000),
			hystrix.WithMaxConcurrentRequests(10),
			hystrix.WithRequestVolumeThreshold(5),
			hystrix.WithSleepWindow(5000),
			hystrix.WithErrorPercentThreshold(50),
		),
	),
)
```

The server automatically adds the provider filter to its filter chain.

## YAML Configuration

When using `dubbo.Load()`, add a side-effect import so the Hystrix package registers its extension config:

```go
import _ "github.com/apache/dubbo-go-extensions/filter/hystrix"
```

Configure commands under the matching lifecycle scope. Each command name is a direct key under `consumer` or `provider`:

```yaml
dubbo:
  extensions:
    hystrix:
      consumer:
        "greet.GreetService:::Greet":
          timeout: 1000
          max-concurrent-requests: 10
          request-volume-threshold: 5
          sleep-window: 5000
          error-percent-threshold: 50
      provider:
        "greet.GreetService:::Greet":
          timeout: 1000
          max-concurrent-requests: 10
          request-volume-threshold: 5
          sleep-window: 5000
          error-percent-threshold: 50
```

The command key format is `interface:group:version:method`. For example, `greet.GreetService:::Greet` has an empty group and version. Keep the dots and colons in the key. Do not add a `commands` level or a `dubbo:consumer:` / `dubbo:provider:` prefix.

Use the reserved `default` key to apply settings to methods without their own command entry. A method-specific command overrides the default:

```yaml
consumer:
  default:
    timeout: 3000
    max-concurrent-requests: 200
  "greet.GreetService:::Greet":
    timeout: 500
```

The typed API can configure the same default with `WithConfig(WithCommandName("default"), ...)`.

Configuration precedence is defaults, YAML, then typed options. Consumer YAML is initialized for clients, and provider YAML is initialized for servers. Hystrix does not support the instance scope.

Hystrix keeps command settings and circuit state process-wide. Clients or servers that use the same command name in the same scope share the circuit, and the latest initialization sets its command settings and default policy. If that initialization omits `default`, unconfigured methods pass through.

## Command Parameters

| Parameter | Description | Default |
| --- | --- | ---: |
| `timeout` | Command timeout in milliseconds | 1000 |
| `max-concurrent-requests` | Maximum concurrent executions | 10 |
| `request-volume-threshold` | Minimum requests before evaluating the error percentage | 20 |
| `sleep-window` | Time before a trial request after the circuit opens, in milliseconds | 5000 |
| `error-percent-threshold` | Error percentage that opens the circuit | 50 |

Each `WithConfig` call configures one command or the reserved `default` entry. Use multiple calls to configure multiple commands. Within an entry, later typed options override earlier typed options and YAML values.
