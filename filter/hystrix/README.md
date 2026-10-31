# Hystrix Filter

English | [简体中文](./README_CN.md)

The Hystrix filter adds circuit breaking to dubbo-go consumers and providers. Configure it with the dubbo-go extension API; the extension adds the matching filter to the client or server filter chain automatically.

## Installation

```bash
go get github.com/apache/dubbo-go-extensions/filter/hystrix
```

## Consumer Configuration

```go
cli, err := ins.NewClient(
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

Configuration precedence is defaults, YAML, then typed options. Consumer YAML is initialized for clients, and provider YAML is initialized for servers. Hystrix does not support the instance scope.

## Command Parameters

| Parameter | Description | Default |
| --- | --- | ---: |
| `timeout` | Command timeout in milliseconds | 1000 |
| `max-concurrent-requests` | Maximum concurrent executions | 10 |
| `request-volume-threshold` | Minimum requests before evaluating the error percentage | 20 |
| `sleep-window` | Time before a trial request after the circuit opens, in milliseconds | 5000 |
| `error-percent-threshold` | Error percentage that opens the circuit | 50 |

Each `WithConfig` call configures one command. Use multiple calls to configure multiple commands. Within a command, later typed options override earlier typed options and YAML values.
