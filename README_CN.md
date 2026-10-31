# Dubbo-Go Extensions

[English](./README.md) | 简体中文

社区维护的 Apache Dubbo-Go v3 扩展集合。

## 可用扩展

### 过滤器 (Filters)

- **[hystrix](./filter/hystrix/)** - 基于 hystrix-go 的熔断器过滤器
- **[adaptivesvc](./filter/adaptivesvc/)** - 提供者侧自适应服务限流过滤器

### 集群 (Clusters)

- **[adaptiveService](./cluster/cluster/adaptivesvc/)** - 消费者侧自适应服务集群

### 负载均衡 (Load Balances)

- **[p2c](./cluster/loadbalance/p2c/)** - 使用 adaptive service metrics 的 Power of Two Choices 负载均衡

### 指标 (Metrics)

- **hill-climbing** - 自适应服务负载均衡使用的本地方法级指标 (`./cluster/metrics`)

### 配置中心 (Config Centers)

## 使用方法

Provider 与 consumer 扩展可分别启用。Provider 使用 `server.WithExtension`，
consumer 则在创建 client 时选择 cluster 与 loadbalance：

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

该 option 同时选择 `adaptiveService` 和 `p2c`。副作用导入只注册实现，
不会自动选择 consumer 策略。完整示例参见[消费者端文档](./cluster/cluster/adaptivesvc/README_CN.md)
和[提供者端文档](./filter/adaptivesvc/README_CN.md)。

详细文档请参见各个扩展目录。

## 许可证

Apache License 2.0 - 详见 [LICENSE](./LICENSE)
