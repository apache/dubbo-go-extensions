# 自适应服务限流

[English](./README.md) | 简体中文

dubbo-go 提供者侧自适应服务限流扩展。

该扩展保留当前 dubbo-go 内置实现的 key 和行为，同时允许应用显式依赖 `github.com/apache/dubbo-go-extensions`。

## 安装

```bash
go get github.com/apache/dubbo-go-extensions/filter/adaptivesvc
```

导入扩展，并通过扩展提供的 option 启用 adaptive service：

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

导入扩展会注册 `padasvc` filter。扩展提供的 option 会将该 key 加入 provider
filter chain，并保留已经配置的其他 filter。

provider filter 只在 invocation 携带 `adaptive-service.enabled=1` 时执行限流。启用后，它会使用 hill-climbing limiter，并通过响应 attachment 返回 provider 状态：

- `adaptive-service.remaining`
- `adaptive-service.inflight`

consumer 侧需要在发出的 invocation 上设置启用 attachment，并读取 provider 返回的容量 attachment。consumer 侧 adaptive service 支持不在本 provider extension 的范围内。

## 限流效果

limiter 会为每个服务方法维护独立的自适应并发上限。当方法的当前并发达到上限时，
provider filter 会在请求进入业务处理逻辑前将其拒绝。该上限会根据实际吞吐量和请求
延迟动态调整，从而把执行中的请求数控制在该方法当前可承受的容量内。

在 [dubbo-go#3347](https://github.com/apache/dubbo-go/pull/3347) 的 provider
保护实验中，客户端以 200 并发请求一个处理耗时为 200 ms 的方法。运行到第 10 秒时，
一组示例结果为：成功 610 次、限流拒绝 180 次、其他失败为 0，进入业务处理逻辑的
最大并发数为 53：

```text
elapsed=10s started=840 success=610 rejected=180 failed=0 qps=61.0 reject_rate=21.4% avg=205ms p95=240ms server_active=50 server_max_active=53
```

实际并发上限和拒绝比例会随业务负载及请求延迟动态变化。预期效果是让超出容量的请求
在进入业务逻辑前被限流，避免 provider 的执行并发无限增长。

## Key 列表

- Provider filter: `padasvc`
- 启用 attachment: `adaptive-service.enabled`
- 启用值: `1`
- Remaining attachment: `adaptive-service.remaining`
- Inflight attachment: `adaptive-service.inflight`

## 兼容性

提供者侧实现仍保留在 dubbo-go 中。本 extension 允许应用显式依赖 extensions 模块，并注册相同的 `padasvc` provider filter key。因此在迁移期间，主仓内置实现与 extension 实现会同时存在。
