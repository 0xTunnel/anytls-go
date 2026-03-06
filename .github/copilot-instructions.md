# 项目指南

## 代码风格
- 所有 Go 代码修改都必须使用 `gofmt`/`goimports` 格式化。
- 返回错误时请带上下文包装（`fmt.Errorf("...: %w", err)`）。
- 保持接口小而精，并尽量定义在使用方附近。
- 明确保证并发安全：`proxy/session` 中的共享 map/状态必须继续受现有锁保护。
- 保持现有 `logrus` 日志风格，较噪声的诊断信息放在环境变量开关后。

## 架构
- `cmd/server` 是可执行入口，负责参数与启动流程。
- 服务端只能以 PPanel v1 节点模式启动；启动参数入口为 TOML 配置文件。
- 当前配置结构为：`[Panel] webapi_url/webapi_key/node_id`、`[TLS] cert_file/key_file`、`[Config] log_level/log_file_dir`。
- 核心协议逻辑位于 `proxy/session`：
  - `frame.go`：帧格式与命令常量。
  - `session.go`：会话生命周期、控制/数据帧处理、设置握手。
  - `stream.go`：单个会话上的多路复用流实现。
- `proxy/padding` 定义包长/填充行为与填充方案更新机制。
- `proxy/pipe/` 提供带 deadline 的 pipe 基元（`deadline.go`、`io_pipe.go`），供 stream 使用。
- `util/deadline.go` 提供通用的 deadline watcher 工具。
- `docs/protocol.md` 是协议行为的唯一事实来源，代码必须与其保持一致。

## 构建与测试
- 构建二进制：
  - `go build -o anytls-server ./cmd/server`
- 发布构建（跨平台）：
  - `goreleaser build --snapshot --clean`
- 测试：
  - 当前状态：仓库已包含配置加载与节点状态相关测试。
  - `go test ./...`（健全性检查）
  - `go test -race ./...`（竞态检查）
- 本地快速验证：
  - 服务端：`./anytls-server -c ./node.toml`

## 约定
- 优先保证协议兼容性：如需调整帧语义或命令顺序，必须同步更新 `docs/protocol.md`。
- 协议要求客户端在认证后必须立刻发送 `cmdSettings`，服务端必须验证该行为（见 `docs/protocol.md`）。
- 不要改造 AnyTLS 线协议；仓库需要兼容只认原版 AnyTLS 协议的第三方客户端。
- 排障时使用并记录现有调试环境变量：
  - `TLS_KEY_LOG`（TLS key log 输出文件路径）

## 参考
- `readme.md`
- `docs/protocol.md`
- `docs/faq.md`
- `docs/uri_scheme.md`
