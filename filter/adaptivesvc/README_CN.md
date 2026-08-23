# 自适应服务限流

[English](./README.md) | 简体中文

dubbo-go 提供者侧自适应服务限流扩展。

该扩展保留当前 dubbo-go 内置实现的 key 和行为，同时允许应用显式依赖 `github.com/apache/dubbo-go-extensions`。

## 安装

```bash
go get github.com/apache/dubbo-go-extensions/filter/adaptivesvc
```

通过副作用导入注册 provider filter，然后使用 dubbo-go server option 启用
adaptive service：

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

启用 adaptive service 后，dubbo-go 会自动将注册 key `padasvc` 加入 provider
filter chain。

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
