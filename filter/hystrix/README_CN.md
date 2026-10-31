# Hystrix 过滤器

[English](./README.md) | 简体中文

Hystrix 过滤器为 dubbo-go consumer 和 provider 提供熔断能力。通过 dubbo-go extension API 配置后，扩展会自动把对应的 filter 加入 client 或 server 的 filter chain。

## 安装

```bash
go get github.com/apache/dubbo-go-extensions/filter/hystrix
```

## Consumer 配置

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

导入 `github.com/apache/dubbo-go-extensions/filter/hystrix` 并使用其配置选项即可。扩展会自动配置 Hystrix command，并添加 consumer filter。

## Provider 配置

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

server 会自动将 provider filter 加入 filter chain。

## YAML 配置

使用 `dubbo.Load()` 时，需要以副作用方式导入 Hystrix 扩展包来注册配置：

```go
import _ "github.com/apache/dubbo-go-extensions/filter/hystrix"
```

然后按生命周期 scope 配置 command。command 名称直接作为 `consumer` 或 `provider` 下的 key：

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

command key 格式为 `接口:分组:版本:方法`。例如 `greet.GreetService:::Greet` 的分组和版本为空。key 中的点号和冒号需要保留；不需要增加 `commands` 层，也不需要添加 `dubbo:consumer:` 或 `dubbo:provider:` 前缀。

配置优先级为默认值、YAML、typed options。consumer YAML 只在 client 初始化时生效，provider YAML 只在 server 初始化时生效。Hystrix 不支持 instance scope。

## Command 参数

| 参数 | 说明 | 默认值 |
| --- | --- | ---: |
| `timeout` | command 超时时间，单位毫秒 | 1000 |
| `max-concurrent-requests` | 最大并发执行数 | 10 |
| `request-volume-threshold` | 开始计算错误率前的最小请求数 | 20 |
| `sleep-window` | 熔断打开后尝试恢复前的等待时间，单位毫秒 | 5000 |
| `error-percent-threshold` | 触发熔断的错误百分比 | 50 |

每次 `WithConfig` 配置一个 command。配置多个 command 时调用多次；同一个 command 的 typed options 按声明顺序覆盖之前的选项和 YAML 值。
