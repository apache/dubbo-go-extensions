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

## 开发

安装 [pre-commit](https://pre-commit.com/#install) 和 import 格式化工具，
然后在每个克隆的仓库中启用 Git 钩子：

```sh
uv tool install pre-commit
go install github.com/dubbogo/tools/cmd/imports-formatter@latest
export PATH="$(go env GOPATH)/bin:$PATH"
pre-commit install
```

Go 及其 `gofmt` 命令也需要在 `PATH` 中。如果设置了 `GOBIN`，请将该目录
加入 `PATH`，替代 `$(go env GOPATH)/bin`。

提交包含 Go 文件时，钩子会从仓库根目录依次运行 `gofmt -s -w .` 和
`imports-formatter`，与 CI 保持一致。两个工具都会扫描工作区，包括未跟踪的
Go 文件。如果已跟踪文件被修改，提交会中止；请检查并暂存格式化修改后重新提交。
pre-commit 会在检查前临时保存已跟踪文件中尚未暂存的修改，并在检查后恢复。

手动运行检查：

```sh
pre-commit run --all-files
```

## 许可证

Apache License 2.0 - 详见 [LICENSE](./LICENSE)
