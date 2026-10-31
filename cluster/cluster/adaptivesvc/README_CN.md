# 自适应服务消费者端

[English](./README.md) | 简体中文

消费者侧 cluster 根据 provider 返回的剩余容量路由请求。Provider 需要启用
`padasvc` filter，参见[提供者端文档](../../../filter/adaptivesvc/README_CN.md)。

## Client 级接入

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

`WithClientAdaptiveService` 会注册 P2C，并为当前 client 的所有 reference
选择 `adaptiveService` cluster 和 `p2c` loadbalance。单个 reference 的
配置可以覆盖这些默认值。

也可以分别导入两个包进行注册，再使用 dubbo-go 的 typed client options：

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

仅需对单个 reference 启用时，创建 reference 时使用
`client.WithClusterAdaptiveService()` 和 `client.WithLoadBalanceP2C()`。

`client.WithExtension`（#3687 的 scoped extension 配置入口）只负责初始化
extension 配置和合并 filter 名称，不会自动选择 cluster 或 loadbalance。
因此消费者端仍需使用上述 client/reference options。

## 行为与配置键

Consumer cluster 在请求中设置 `adaptive-service.enabled=1`。Provider filter
按方法进行 hill-climbing 限流，并返回 `adaptive-service.remaining` 和
`adaptive-service.inflight`。Consumer 将剩余容量写入方法级
`hill-climbing` metrics，P2C 使用该指标比较两个 provider。没有配合使用的
provider 时，P2C 无法获得容量数据。

| 组件 | 配置键 |
| --- | --- |
| Provider filter | `padasvc` |
| Consumer cluster | `adaptiveService` |
| Loadbalance | `p2c` |
| 本地 metrics | `hill-climbing` |

迁移期间 dubbo-go 主仓仍保留内置实现。这些包允许应用显式依赖 extensions 仓库。
