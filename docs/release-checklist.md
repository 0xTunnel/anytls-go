# 发布检查模板

本文档提供一份偏运维视角的发布检查模板，可在每次上线前复制一份作为实际记录。

## 使用方式

建议在每次发布前复制下面的模板，按本次版本填写：

```text
发布版本：vX.Y.Z
发布时间：YYYY-MM-DD HH:MM
发布负责人：
变更负责人：
部署目标：生产 / 预发 / 测试
镜像标签：ghcr.io/0xTunnel/anytls-ppanel:vX.Y.Z
```

## 发布前检查

- [ ] 已确认发布版本号，例如 `v1.0.0`
- [ ] 已确认 `main` 分支代码已合并且工作区干净
- [ ] 已执行 `go test ./...`
- [ ] 已执行 `docker buildx bake image-local`
- [ ] 已执行 `docker compose config`
- [ ] 已确认 `node.toml`、`cert.pem`、`key.pem`、`log/` 已就位
- [ ] 已确认目标服务器可以访问 GHCR
- [ ] 若 GHCR 包为私有，已完成 `docker login ghcr.io`
- [ ] 已确认回滚目标版本存在且可拉取

## 变更说明模板

建议至少记录以下内容：

### 本次变更

- 变更摘要：
- 影响范围：
- 是否涉及协议兼容性：是 / 否
- 是否涉及配置格式变更：是 / 否
- 是否需要人工迁移：是 / 否

### 重点风险

- 风险 1：
- 风险 2：
- 风险缓解措施：

## 发布步骤记录

### 场景 A：新版本发布

- [ ] 创建并推送版本标签

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

- [ ] 等待 GitHub Actions 自动触发发布流程

### 场景 B：手动重发已有版本

- [ ] 已确认远端存在目标 tag

```bash
git ls-remote --tags origin | grep vX.Y.Z
```

- [ ] 已在 GitHub Actions 页面手动触发 `release-binaries` workflow
- [ ] 已填写正确 tag，并确认允许更新已有 GitHub Release 资产

### 共同步骤

- [ ] 等待 GitHub Actions 中 [release-binaries.yml](../.github/workflows/release-binaries.yml) 完成
- [ ] 等待 GitHub Actions 中 [docker-publish.yml](../.github/workflows/docker-publish.yml) 完成
- [ ] 如为手动重跑，已确认同名 GitHub Release 资产允许被覆盖更新
- [ ] 确认 GHCR 中已出现目标镜像标签
- [ ] 服务器执行 `docker compose pull`
- [ ] 服务器执行 `docker compose up -d`

## 发布后验证

- [ ] GitHub Release 页面已生成目标版本
- [ ] GitHub Release 归档文件可正常下载
- [ ] `docker compose ps` 状态正常
- [ ] `docker compose logs --tail=100` 无明显错误
- [ ] 面板侧节点显示在线
- [ ] 节点拉取配置正常
- [ ] 新连接握手正常
- [ ] 流量上报正常
- [ ] 日志文件输出正常

## 回滚预案

### 回滚目标

- 上一个稳定版本：
- 回滚镜像：`ghcr.io/0xTunnel/anytls-ppanel:vA.B.C`

### 回滚命令

```bash
export ANYTLS_IMAGE=ghcr.io/0xTunnel/anytls-ppanel:vA.B.C
docker compose pull
docker compose up -d
```

### 回滚后验证

- [ ] `docker compose ps` 正常
- [ ] `docker compose logs --tail=100` 正常
- [ ] 面板侧节点恢复在线
- [ ] 客户端连接恢复正常

## 发布结论

- 发布结果：成功 / 回滚 / 终止
- 实际完成时间：
- 备注：