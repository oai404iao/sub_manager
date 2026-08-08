# Sub Manager

面向 Xray 与 Mihomo 的轻量订阅、节点和分组管理系统。当前首期支持
VLESS 与 SOCKS5，React 前端会被打包并嵌入 Go 单文件程序。

[![CI](https://github.com/oai404iao/sub_manager/actions/workflows/ci.yml/badge.svg)](https://github.com/oai404iao/sub_manager/actions/workflows/ci.yml)
[![Release](https://github.com/oai404iao/sub_manager/actions/workflows/release.yml/badge.svg)](https://github.com/oai404iao/sub_manager/actions/workflows/release.yml)

## 当前实现

- Cookie Session 登录鉴权（密码使用 bcrypt，Session 仅保存 SHA-256 摘要）
- SQLite 持久化
- VLESS / SOCKS5 URI、Xray/Mihomo YAML、JSON 与多种 Base64 订阅导入
- 对齐 Xray-core v26.7.28 的 VLESS Encryption、当前 transports、
  TLS/REALITY、XHTTP 分享参数与 UUIDv5 映射
- Xray VLESS OutboundObject 无损保存、完整 JSON 编辑与下载
- 节点与订阅增删改、分组管理和订阅原子刷新 API
- 带过期时间的 HMAC-SHA256 分享 URL
- “订阅 URL”与“完整节点内容”两类二维码
- shadcn/ui（Base UI）+ TypeScript + React + Vite 管理界面
- 前端静态资源通过 `go:embed` 进入 Go 二进制

## 快速启动

```bash
cp .env.example .env
npm --prefix web ci
make build
./bin/sub-manager
```

默认监听 `0.0.0.0:8080`，本机可通过 `http://127.0.0.1:8080` 访问。首次启动若未设置环境变量，会创建
`admin / admin123`，仅适合本机开发；部署前必须更换。

开发时可分别启动：

```bash
go run ./cmd/server
npm --prefix web run dev
```

Vite 会将 `/api`、`/s` 代理到 `127.0.0.1:8080`。

查看构建版本：

```bash
./bin/sub-manager --version
curl http://127.0.0.1:8080/api/version
```

`GET /healthz` 是无需鉴权的容器健康检查端点。

## Docker

发布镜像位于 `ghcr.io/oai404iao/sub_manager`。运行固定版本：

```bash
docker run -d \
  --name sub-manager \
  --restart unless-stopped \
  -p 8080:8080 \
  -v sub-manager-data:/data \
  -e SUBMAN_ADMIN_PASSWORD='replace-with-a-strong-password' \
  -e SUBMAN_SIGNING_KEY="$(openssl rand -hex 32)" \
  -e SUBMAN_BASE_URL='http://127.0.0.1:8080' \
  ghcr.io/oai404iao/sub_manager:0.1.1
```

镜像默认以非 root 用户运行，监听 `0.0.0.0:8080`，数据库写入
`/data/sub-manager.db`。

本地构建：

```bash
make docker-build
```

Compose 示例：

```bash
cp .env.example .env
# 修改 .env 中的管理员密码、签名密钥和公开访问地址
docker compose -f compose.example.yaml up -d
docker compose -f compose.example.yaml ps
```

如需固定镜像版本，可在 `.env` 中添加：

```dotenv
SUBMAN_IMAGE=ghcr.io/oai404iao/sub_manager:0.1.1
```

## 版本与发布

项目采用 SemVer。`VERSION`、`web/package.json` 和
`web/package-lock.json` 必须保持一致。

准备下一个版本：

```bash
./scripts/set-version.sh 0.2.0
```

发布脚本只允许从已同步且干净的 `main` 分支运行。它会执行完整检查、
构建 Linux amd64 发布包、创建带注释的 `vX.Y.Z` 标签并推送：

```bash
./scripts/release.sh 0.2.0
```

标签推送后，GitHub Actions 会：

1. 重新执行 Go、TypeScript、ESLint 和生产构建检查；
2. 创建 Linux amd64 压缩包和 `SHA256SUMS`；
3. 发布 GHCR 镜像的完整版本、主次版本、主版本、`latest` 和提交标签；
4. 生成镜像 SBOM、构建来源证明和 GitHub Release。

日常提交与 Pull Request 由 `.github/workflows/ci.yml` 校验；版本标签由
`.github/workflows/release.yml` 发布。

## 文档

- [架构与目录设计](docs/architecture.md)
- [VLESS 可配置项](docs/protocols/vless.md)
- [SOCKS5 可配置项](docs/protocols/socks.md)
- [实现计划](docs/implementation-plan.md)

## 使用官方 Xray 校验生成配置

安装 Xray-core v26.7.28 后运行：

```bash
XRAY_BIN=/path/to/xray make test-xray
```
