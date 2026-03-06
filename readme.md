# AnyTLS

一个试图缓解 嵌套的TLS握手指纹(TLS in TLS) 问题的代理协议。`anytls-go` 当前提供 AnyTLS 服务端实现。

- 灵活的分包和填充策略
- 连接复用，降低代理延迟
- 简洁的配置

[用户常见问题](./docs/faq.md)

[协议文档](./docs/protocol.md)

[URI 格式](./docs/uri_scheme.md)

[发布指南](./docs/release.md)

[发布检查模板](./docs/release-checklist.md)

## 启动方式

当前服务端只支持基于 PPanel v1 节点模式启动，并通过 TOML 配置文件加载节点接入参数。

### 示例配置文件

完整示例见仓库根目录的 [node.example.toml](node.example.toml)。

配置文件扩展名必须为 `.toml`，并使用如下结构：

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
timezone = "UTC+8"

[Network]
tcp_timeout = 60
udp_timeout = 2
```

其中 `Config.log_file_dir` 为可选项；设置后，服务端会同时输出到标准输出和该目录下的 `anytls-server.log`。`/v1/server/user` 拉取到的用户列表也会以格式化 JSON 形式保存为同目录下的 `ppanel-users.json`；如果未设置 `Config.log_file_dir`，则默认保存到节点配置文件所在目录。`Config.timezone` 也为可选项，默认使用 `UTC+8`，可填写 `UTC+8` 这类 UTC 偏移格式，或 `Asia/Shanghai` 这类 IANA 时区名称。

`Network` 配置组用于声明空闲连接超时，单位均为分钟：

- `Network.tcp_timeout`：空闲 TCP 连接超时时间，默认 `60`，设置值必须大于 `0`
- `Network.udp_timeout`：空闲 UDP 连接超时时间，默认 `2`，设置值必须大于 `0`

### 日志说明

当前服务端日志统一使用 `debug`、`info`、`warn`、`error` 四个等级：

- `debug`：联调用细节，例如 fallback、目标地址读取失败等。高频成功路径日志默认会收敛，避免长期运行时刷屏。
- `info`：状态变化和里程碑事件，例如服务启动、监听成功、节点快照同步成功、正常停机等。
- `warn`：请求级异常但服务仍继续，例如设备数超限、客户端未先发 settings、出站拨号失败、收到客户端 alert 等。
- `error`：服务级失败或明显异常，例如快照为空、同步/上报失败、不可恢复的监听错误、panic 恢复等。

日志会尽量带上这些字段，便于服务器筛查：

- `component`：日志来源模块，例如 `server`、`node`、`runtime`、`inbound`、`outbound`、`session`
- `event`：固定事件名，例如 `startup`、`sync_snapshot`、`device_reject`、`dial_failed`
- `user_id`、`remote_ip`、`target`：按场景补充的排障字段

服务器联调阶段建议将 `log_level` 设为 `debug`；稳定运行后建议切回 `info`。

如需临时查看高频诊断日志，可额外设置环境变量 `ANYTLS_DEBUG_VERBOSE=1`。启用后会恢复这类 `debug` 日志，例如版本协商、padding 更新、周期性状态上报和未变化的快照同步。

### 示例启动

```
./anytls-server -c ./node.toml
```

## Docker 部署

### 构建镜像

```
docker build -t anytls-ppanel:latest .
```

### 使用 buildx 构建多平台镜像

仓库根目录已提供 [docker-bake.hcl](docker-bake.hcl)。如果你主要部署到 Linux 服务器，建议统一使用 buildx：

单平台本地加载到当前 Docker：

```
docker buildx bake image-local
```

构建 Linux 多平台镜像并推送到仓库：

```
docker buildx bake image-multiarch \
	--set image-multiarch.tags=ghcr.io/0xTunnel/anytls-ppanel:latest \
	--push
```

如果你不使用 bake，也可以直接执行：

```
docker buildx build \
	--platform linux/amd64,linux/arm64 \
	-t ghcr.io/0xTunnel/anytls-ppanel:latest \
	--push \
	.
```

如需显式指定目标平台，可使用：

```
docker build --platform linux/amd64 -t anytls-ppanel:latest .
```

### 容器启动

当前仓库已提供 [compose.yaml](compose.yaml)。镜像默认启动命令为 `-c /etc/anytls/node.toml`，并使用 host 网络。

默认镜像地址使用 GHCR 形式：`ghcr.io/0xTunnel/anytls-ppanel:latest`。实际部署时，如需覆盖标签，可通过环境变量指定：

```
export ANYTLS_IMAGE=ghcr.io/0xTunnel/anytls-ppanel:latest
```

compose 当前采用更保守的挂载方式：

1. `node.toml` 只读挂载到容器内
2. `cert.pem` 和 `key.pem` 只读挂载到容器内
3. `./log` 单独挂载为日志目录
4. 容器日志启用 `json-file` 轮转，单文件 `10m`，保留 `3` 份

镜像内已预创建 `/etc/anytls/log`，并使用 `SIGTERM` 作为停止信号，适合直接交给 Docker 或 systemd 驱动的 compose 进行启停管理。

使用方式：

```
cp node.example.toml node.toml
mkdir -p log
docker compose up -d
```

请确保当前目录下存在：

1. `node.toml`
2. `cert.pem`
3. `key.pem`
4. `log/`

并且 `node.toml` 中的路径保持与容器内路径一致：

```toml
[TLS]
cert_file = "/etc/anytls/cert.pem"
key_file = "/etc/anytls/key.pem"

[Config]
log_file_dir = "/etc/anytls/log"
```

由于 compose 使用 `network_mode: host`，无需再单独配置 `ports` 映射。

如果服务器需要拉取私有 GHCR 镜像，请先登录：

```
echo "$GITHUB_TOKEN" | docker login ghcr.io -u 0xTunnel --password-stdin
```

如果你希望服务器无需登录即可直接拉取，建议将 GHCR 包设为公开：

1. 打开 GitHub 上 0xTunnel 账号或组织的 Packages 页面
2. 进入 `anytls-ppanel` 包详情
3. 打开包设置页
4. 将包可见性改为 `Public`

公开后，服务器可直接执行 `docker compose up -d` 拉取镜像，无需额外 `docker login`。

### GitHub Actions 发布到 GHCR

仓库已提供 [docker-publish.yml](.github/workflows/docker-publish.yml)。它会：

1. 在 `main` 分支 push 时运行测试并发布多平台镜像
2. 在 `v*` 标签 push 时发布对应 tag 镜像
3. 在 PR 中只做测试和构建校验，不推送镜像

发布约定建议如下：

1. 日常滚动发布：push 到 `main`，生成 `latest` 和短 SHA 标签
2. 版本发布：打 `v1.0.0` 这类标签，生成 `ghcr.io/0xTunnel/anytls-ppanel:v1.0.0`

示例：

```
git tag v1.0.0
git push origin v1.0.0
```

Workflow 默认会把镜像发布到：

```
ghcr.io/0xTunnel/anytls-ppanel
```

完整的 CI 发布说明、版本发布步骤和发版检查清单见 [docs/release.md](./docs/release.md)。

仓库根目录提供了 [node.example.toml](node.example.toml) 作为容器和宿主机部署的共用模板。

如需客户端，请使用已支持 AnyTLS 的第三方实现，例如 sing-box、mihomo 或 Shadowrocket。

### sing-box

https://github.com/SagerNet/sing-box

它包含了 anytls 协议的服务器和客户端实现。

### mihomo

https://github.com/MetaCubeX/mihomo

它包含了 anytls 协议的服务器和客户端实现。

### Shadowrocket

Shadowrocket 2.2.65+ 实现了 anytls 协议客户端。
