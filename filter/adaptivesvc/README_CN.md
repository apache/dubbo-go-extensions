# 自适应服务限流

[English](./README.md) | 简体中文

dubbo-go 自适应服务限流扩展。该扩展包含提供者侧 adaptive service filter、消费者侧 adaptive service cluster、P2C 负载均衡，以及 P2C 使用的本地方法级指标。

该扩展保留当前 dubbo-go 内置实现的 key 和行为，同时允许应用显式依赖 `github.com/apache/dubbo-go-extensions`。

## 安装

```bash
go get github.com/apache/dubbo-go-extensions/imports/adaptivesvc
```

导入 `github.com/apache/dubbo-go-extensions/imports/adaptivesvc`，即可同时注册
provider filter、consumer cluster 和 P2C loadbalance。

## 提供者侧

聚合导入会注册 provider filter；也可以单独通过副作用导入注册：

```go
import (
    _ "github.com/apache/dubbo-go-extensions/filter/adaptivesvc"
)
```

配置 provider filter key：

```text
padasvc
```

provider filter 只在 invocation 携带 `adaptive-service.enabled=1` 时执行限流。启用后，它会使用 hill-climbing limiter，并通过响应 attachment 返回 provider 状态：

- `adaptive-service.remaining`
- `adaptive-service.inflight`

## 消费者侧

聚合导入会注册 adaptive service cluster 和 P2C loadbalance；也可以单独通过副作用导入注册：

```go
import (
    _ "github.com/apache/dubbo-go-extensions/cluster/cluster/adaptivesvc"
    _ "github.com/apache/dubbo-go-extensions/cluster/loadbalance/p2c"
)
```

消费者侧配置：

- cluster: `adaptiveService`
- loadbalance: `p2c`

consumer cluster 会在发出的 invocation 上设置 `adaptive-service.enabled=1`。provider filter 根据该 attachment 判断是否限流并回传容量。consumer cluster 读取 provider 返回的 `adaptive-service.remaining` attachment，写入本地方法级 metrics。P2C loadbalance 随后在两个候选 provider 中选择 hill-climbing remaining capacity 更高的节点。

adaptive service cluster 当前只支持 `p2c` loadbalance。

## Key 列表

- Provider filter: `padasvc`
- Cluster: `adaptiveService`
- Loadbalance: `p2c`
- Metrics key: `hill-climbing`
- 启用 attachment: `adaptive-service.enabled`
- 启用值: `1`
- Remaining attachment: `adaptive-service.remaining`
- Inflight attachment: `adaptive-service.inflight`

## Verbose 日志

limiter 暴露了包级别的 `Verbose` 开关，可用于开启 debug 日志：

```go
import "github.com/apache/dubbo-go-extensions/filter/adaptivesvc/limiter"

func init() {
    limiter.Verbose = true
}
```

## 兼容性

迁移后 adaptive service 的实现由本 extensions 模块注册。使用迁移后能力的应用应导入上面的聚合包。注册的 key 为：

- `padasvc`
- `adaptiveService`
- `p2c`

实现保留了原内置实现的公开配置 key 和 adaptive service 行为。
