# 发布指南

本文档说明 `anytls-go` 当前的 CI 发布链路、版本发布流程，以及在发布前建议执行的本地检查。

## 当前发布能力

仓库当前有两条发布链路：

1. Docker / GHCR 发布
说明：由 [docker-publish.yml](../.github/workflows/docker-publish.yml) 负责，使用 buildx 构建 `linux/amd64` 和 `linux/arm64` 多平台镜像，并推送到 `ghcr.io/0xTunnel/anytls-ppanel`。

2. 二进制构建
说明：由 [.goreleaser.yaml](../.goreleaser.yaml) 定义，并由 [release-binaries.yml](../.github/workflows/release-binaries.yml) 在 tag 发布时自动生成 GitHub Release 归档。

## CI 触发规则

当前 workflow 触发规则如下：

1. `push` 到 `main`
结果：运行 `go test ./...`，然后构建并推送多平台镜像。
标签：`latest`、短 SHA。

2. `push` 标签 `v*`
结果：运行 `go test ./...`，然后构建并推送多平台镜像。
标签：版本标签本身，例如 `v1.0.0`，同时如果默认分支策略满足，也会保留 `latest`。

3. `pull_request` 到 `main`
结果：只运行测试和镜像构建校验，不推送到 GHCR。

4. `workflow_dispatch`
结果：可手动触发同一条发布工作流。

5. `release-binaries.yml`
结果：在 `v*` tag push 或手动指定 tag 时，执行 goreleaser 并发布 GitHub Release 二进制归档。

## Docker 镜像发布约定

默认镜像仓库：

```text
ghcr.io/0xTunnel/anytls-ppanel
```

建议按以下方式理解镜像标签：

1. `latest`
用途：主分支最新可部署镜像。
来源：`main` 分支 push。

2. `sha-...` 或短 SHA 标签
用途：精确追踪某次提交对应的镜像。
来源：`main` 分支或 tag 发布流程。

3. `v1.0.0` 这类版本标签
用途：正式版本发布。
来源：推送符合 `v*` 模式的 git tag。

## 推荐发布流程

### 日常滚动发布

适用场景：
代码已经合入 `main`，希望服务器拉取最新镜像部署。

步骤：

1. 在本地确认测试通过

```bash
go test ./...
docker buildx bake image-local
```

2. 推送到 `main`

```bash
git push origin main
```

3. 等待 GitHub Actions 完成
检查 [docker-publish.yml](../.github/workflows/docker-publish.yml) 对应运行是否成功。

4. 服务器部署最新镜像

```bash
export ANYTLS_IMAGE=ghcr.io/0xTunnel/anytls-ppanel:latest
docker compose pull
docker compose up -d
```

### 正式版本发布

适用场景：
需要发布一个可回溯的版本号，例如 `v1.0.0`。

步骤：

1. 在本地确认工作区干净，测试通过

```bash
go test ./...
docker buildx bake image-local
git status --short
```

2. 创建并推送版本标签

```bash
git tag v1.0.0
git push origin v1.0.0
```

3. 等待 GitHub Actions 完成
成功后，GHCR 中应出现：

```text
ghcr.io/0xTunnel/anytls-ppanel:v1.0.0
```

4. 服务器固定部署该版本

```bash
export ANYTLS_IMAGE=ghcr.io/0xTunnel/anytls-ppanel:v1.0.0
docker compose pull
docker compose up -d
```

5. 如需二进制归档，同时检查 [release-binaries.yml](../.github/workflows/release-binaries.yml) 是否已生成对应 GitHub Release。

## GHCR 权限与可见性

安全提示：

1. 服务器侧 token 建议使用 GitHub Personal Access Token
2. 最小权限：拉取镜像使用 `read:packages`，手动推送镜像使用 `write:packages`
3. 不要将 token 提交到代码库，也不要直接写死到脚本文件中
4. 建议通过环境变量、CI Secret 或密钥管理工具保存 token
5. 如果 token 泄露，应立即在 GitHub 后台撤销并重新生成

如果镜像包保持私有：

1. CI 推送依赖 GitHub Actions 的 `GITHUB_TOKEN`
2. 服务器拉取前需要先登录 GHCR

```bash
echo "$GITHUB_TOKEN" | docker login ghcr.io -u 0xTunnel --password-stdin
```

如果希望服务器无需登录即可拉取：

1. 打开 GitHub 上 `0xTunnel` 的 Packages 页面
2. 进入 `anytls-ppanel` 包详情页
3. 进入设置
4. 将包可见性改为 `Public`

## 本地发版前检查清单

建议在推送 `main` 或打 tag 之前完成以下检查：

1. 配置解析测试通过

```bash
go test ./internal/config/...
```

2. 全量测试通过

```bash
go test ./...
```

3. 本地 buildx 构建通过

```bash
docker buildx bake image-local
```

4. compose 配置可解析

```bash
docker compose config
```

5. 若发布正式版本，确认 tag 符合 `vX.Y.Z` 约定

版本号建议遵循语义化版本（SemVer）：

1. `X`：不兼容变更
2. `Y`：向后兼容的新功能
3. `Z`：向后兼容的问题修复

示例：`v1.2.3`

## 二进制发布

### 本地快照构建

如果需要生成跨平台二进制而不依赖容器镜像，可以手动运行：

```bash
goreleaser build --snapshot --clean
```

### 自动发布 GitHub Release

仓库已提供 [release-binaries.yml](../.github/workflows/release-binaries.yml)。

自动触发方式：

1. 推送 `v*` 标签，例如 `v1.0.0`
2. 手动触发 workflow_dispatch，并填写一个已存在的版本标签

手动触发适用场景：

1. 需要重新发布已有 tag 对应的二进制归档
2. 需要在 GitHub Actions 页面手工重跑 release 流程

注意：手动触发时填写的 tag 必须已存在，并且符合 `vX.Y.Z` 格式。

补充说明：如果该 tag 对应的 GitHub Release 已存在，手动重跑会更新同一版本下的 release 资产，而不是创建一个新版本。重跑前应先确认当前 Release 页面中的资产是否允许被刷新。

手动触发步骤：

1. 打开仓库的 Actions 页面
2. 选择 `release-binaries` workflow
3. 点击 `Run workflow`
4. 在 `tag` 输入框填写已存在的版本标签，例如 `v1.0.0`
5. 确认后启动 workflow，并观察 `Validate manual tag input` 与后续 checkout 步骤是否通过

### 手动本地发布 GitHub Release

如果需要绕过 GitHub Actions，在本地直接发布 GitHub Release，可执行：

```bash
export GITHUB_TOKEN=your_github_token
goreleaser release --clean
```

建议先确认：

1. 当前提交已经通过 `go test ./...`
2. 目标 tag 已创建并推送
3. `GITHUB_TOKEN` 拥有创建 release 所需权限

## CI 失败排查

如果 GitHub Actions 发布失败，建议按以下顺序排查：

1. 打开对应 workflow run，查看失败 step 的日志
2. 本地复现 Go 测试

```bash
go test ./...
```

3. 本地复现 Docker 构建

```bash
docker buildx bake image-local
```

4. 检查 GHCR 权限是否正常
5. 如果是网络或拉取超时，可在 Actions 页面重试任务

如果是二进制 release 失败，还应额外检查：

1. git tag 是否已推送到远端
2. `GITHUB_TOKEN` 是否具备 `contents: write`
3. `.goreleaser.yaml` 中的目标平台配置是否正确

## 回滚建议

如果新版本镜像上线后需要回滚，优先使用已发布的版本标签，而不是回退到一个不明确的 `latest`。

回滚建议：

1. 先确认可用的历史版本标签
2. 使用明确版本号替换 `ANYTLS_IMAGE`
3. 拉取并重启容器
4. 通过日志和运行状态验证回滚结果

示例：

```bash
export ANYTLS_IMAGE=ghcr.io/0xTunnel/anytls-ppanel:v1.0.0
docker compose pull
docker compose up -d
```

回滚后建议至少检查：

1. `docker compose ps`
2. `docker compose logs --tail=100`
3. 面板侧节点是否恢复在线

如需一份可直接填写的运维检查模板，可参考 [release-checklist.md](./release-checklist.md)。