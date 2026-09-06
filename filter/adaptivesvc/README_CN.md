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

## Key 列表

- Provider filter: `padasvc`
- 启用 attachment: `adaptive-service.enabled`
- 启用值: `1`
- Remaining attachment: `adaptive-service.remaining`
- Inflight attachment: `adaptive-service.inflight`

## 兼容性

提供者侧实现仍保留在 dubbo-go 中。本 extension 允许应用显式依赖 extensions 模块，并注册相同的 `padasvc` provider filter key。因此在迁移期间，主仓内置实现与 extension 实现会同时存在。
