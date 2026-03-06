# AnyTLS Go Node

这是一个基于 Go 的 AnyTLS 服务端节点实现，当前以 PPanel v1 节点模式运行。

仓库当前重点解决三件事：

- 从 PPanel 拉取节点配置和用户列表
- 运行 AnyTLS 入站服务并转发流量
- 上报在线用户、流量和节点状态

## 当前能力

- 协议：AnyTLS
- 启动方式：TOML 配置文件 + PPanel v1
- 用户认证：基于面板下发用户列表
- 运行时能力：设备数限制、入站 TCP 连接数限制、流量统计、状态上报
- 日志：标准输出 + 按天切分文件日志

当前代码已初步拆成两层：

- 通用节点层：负责配置加载、快照同步、限额、流量与状态上报
- 协议适配层：当前仅实现 AnyTLS，便于后续扩展其他协议

## 目录概览

- [cmd/server](cmd/server)：服务端入口、启动流程、节点运行时、日志与协议接入
- [internal/config](internal/config)：TOML 配置解析
- [internal/node/state](internal/node/state)：运行时快照、设备跟踪、TCP 连接计数、流量统计
- [internal/ppanel](internal/ppanel)：PPanel API 客户端与用户快照落盘
- [proxy/session](proxy/session)：AnyTLS 会话、多路复用流与帧处理
- [docs](docs)：协议、FAQ、发布和 URI 文档

## 配置文件

示例配置见 [example/node.toml](example/node.toml)。

当前使用如下 TOML 结构：

```toml
[Panel]
webapi_url = "https://api.ppanel.dev"
webapi_key = "1234567890"
node_id = 1

[TLS]
cert_file = "/etc/anytls/cert.pem"
key_file = "/etc/anytls/key.pem"

[Config]
log_level = "info"
log_file_dir = "/etc/anytls/log"
log_file_retention_days = 7
timezone = "UTC+8"

[Network]
tcp_timeout = 60
udp_timeout = 2
tcp_limit = 0
```

配置说明：

- `Panel`：PPanel 地址、密钥和节点 ID
- `TLS`：服务端证书和私钥路径
- `Config.log_file_dir`：日志目录，可选
- `Config.log_file_retention_days`：日志最大保留天数，`0` 表示不清理
- `Network.tcp_timeout`：TCP 空闲超时，单位分钟
- `Network.udp_timeout`：UDP 空闲超时，单位分钟
- `Network.tcp_limit`：每名用户当前入站 TCP 连接数上限，`0` 表示不限制

用户列表会保存为配置文件同目录下的 `users.json`。

## 本地运行

构建：

```bash
go build -o anytls-server ./cmd/server
```

启动：

```bash
./anytls-server -c ./node.toml
```

测试：

```bash
go test ./...
go test -race ./...
```

## 日志

- 支持 `debug`、`info`、`warn`、`error` 四个等级
- 文件日志按天写入 `anytls-server-YYYY-MM-DD.log`
- `ANYTLS_DEBUG_VERBOSE=1` 可开启额外诊断日志
- PPanel API 请求摘要会在调试日志中输出

## Docker

仓库包含 [Dockerfile](Dockerfile)、[example/compose.yaml](example/compose.yaml) 和 [docker-bake.hcl](docker-bake.hcl)。

常见用法：

```bash
cp example/node.toml node.toml
mkdir -p log
docker compose -f example/compose.yaml up -d
```

如果本地访问 Docker Hub 不稳定，可以在构建时覆盖基础镜像源前缀：

```bash
docker buildx bake image-local --set '*.args.BASE_IMAGE_PREFIX=<your-mirror>/library'
```

如果需要清理本地 buildx 缓存，可以执行：

```bash
rm -rf .buildx-cache
```

## 相关文档

- [docs/protocol.md](docs/protocol.md)
- [docs/faq.md](docs/faq.md)
- [docs/uri_scheme.md](docs/uri_scheme.md)
- [docs/release.md](docs/release.md)
- [docs/release-checklist.md](docs/release-checklist.md)