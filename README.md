<p align="center">
  <img src="web/public/brand/logo-mark.svg" width="88" height="88" alt="Sub Manager Logo">
</p>

<h1 align="center">Sub Manager</h1>

<p align="center">
  面向 Xray 与 Mihomo 的轻量、自托管订阅与节点管理面板。
  <br>
  将导入、标准化、分组、刷新与安全分享整合进一个 Go 单文件程序。
</p>

<p align="center">
  <a href="https://github.com/oai404iao/sub_manager/actions/workflows/ci.yml"><img src="https://github.com/oai404iao/sub_manager/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/oai404iao/sub_manager/actions/workflows/release.yml"><img src="https://github.com/oai404iao/sub_manager/actions/workflows/release.yml/badge.svg" alt="Release"></a>
  <a href="https://github.com/oai404iao/sub_manager/releases"><img src="https://img.shields.io/github/v/release/oai404iao/sub_manager?display_name=tag&sort=semver" alt="GitHub Release"></a>
  <a href="https://github.com/oai404iao/sub_manager/pkgs/container/sub_manager"><img src="https://img.shields.io/badge/GHCR-sub__manager-2496ED?logo=docker&logoColor=white" alt="GHCR Image"></a>
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26">
  <img src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=111827" alt="React 19">
</p>

<p align="center">
  <a href="#功能亮点">功能亮点</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#docker-部署">Docker</a> ·
  <a href="#配置">配置</a> ·
  <a href="#开发与验证">开发</a> ·
  <a href="#项目文档">文档</a>
</p>

> [!IMPORTANT]
> Sub Manager 当前定位为单管理员、自托管管理面板。请只导入和分发你有权使用的节点与订阅，并在公网部署前完成安全配置。

## 功能亮点

| 能力 | 说明 |
| --- | --- |
| 订阅与节点管理 | 节点、分组和订阅的创建、编辑、删除、筛选与刷新；支持节点批量删除、改组及 URI/Base64 复制，托管节点改组时整条订阅同步迁移 |
| 多层分组 | 分组可递归包含下级分组；筛选和分享时聚合并去重全部下级节点 |
| 多格式导入 | VLESS / SOCKS5 URI、Xray JSON/YAML、Mihomo YAML、标准与 URL-safe Base64、data URI |
| Xray 无损往返 | 保存完整 VLESS `OutboundObject`，支持高级 JSON 编辑与单节点下载 |
| 原子订阅刷新 | 刷新时在事务内替换订阅托管节点，避免出现半更新状态 |
| 安全分享 | 可限时或永久的 HMAC-SHA256 单一分享 URL、可撤销和删除的分享历史，以及按对象区分的二维码 |
| 轻量部署 | React 前端通过 `go:embed` 打入 Go 二进制；同时提供 Docker / Compose |
| 基础安全 | bcrypt 密码、Session 摘要存储、HttpOnly/SameSite Cookie、订阅请求 SSRF 防护 |

### 协议与格式

| 格式 | 导入 | 导出 / 分享 | 备注 |
| --- | :---: | :---: | --- |
| VLESS URI | ✅ | ✅ | 支持当前传输层、TLS、REALITY、XHTTP 与 VLESS Encryption 字段 |
| SOCKS5 URI | ✅ | ✅ | 支持用户名/密码、UDP 与 TLS 扩展参数 |
| Xray JSON | ✅ | ✅ | 导入完整配置、单个 outbound 或 outbound 数组；导出单个 VLESS outbound |
| Xray YAML | ✅ | — | 作为 Xray 配置导入并规范化 |
| Mihomo YAML | ✅ | — | 当前支持 VLESS 与 SOCKS5 proxy 导入 |
| Base64 订阅 | ✅ | ✅ | 支持标准、Raw、URL-safe 与换行内容 |
| Base64 data URI | ✅ | — | 支持带 `;base64` 标记的 data URI |

VLESS 实现基线为 **Xray-core v26.7.28**。字段范围、兼容规则与上游依据见
[VLESS 协议文档](docs/protocols/vless.md)。

## 快速开始

### 环境要求

- Go `1.26`
- Node.js `24`
- npm

```bash
git clone https://github.com/oai404iao/sub_manager.git
cd sub_manager

npm --prefix web ci
make build
./bin/sub-manager
```

默认监听 `0.0.0.0:8080`，本机访问：

```text
http://127.0.0.1:8080
```

首次使用空数据库时，开发默认账号为：

```text
用户名：admin
密码：admin123
```

> [!WARNING]
> 默认账号和默认签名密钥仅用于本机开发。公网部署前必须设置强密码、随机签名密钥和正确的公开访问地址。

### 使用环境变量

程序读取系统环境变量，**不会自动加载 `.env` 文件**。在 Bash 中可以这样启动：

```bash
cp .env.example .env
# 编辑 .env，至少修改管理员密码、签名密钥和公开访问地址

set -a
source .env
set +a

./bin/sub-manager
```

### 前后端分离开发

分别在两个终端运行：

```bash
# Terminal 1: Go API
make dev

# Terminal 2: Vite
npm --prefix web run dev
```

Vite 会将 `/api` 与 `/s` 代理到 `127.0.0.1:8080`。

## Docker 部署

### Docker Compose

```bash
cp .env.example .env
# 修改 .env 中的 SUBMAN_ADMIN_PASSWORD、SUBMAN_SIGNING_KEY、
# SUBMAN_BASE_URL；HTTPS 部署还应启用 SUBMAN_SECURE_COOKIE

docker compose -f compose.example.yaml up -d
docker compose -f compose.example.yaml ps
```

查看日志：

```bash
docker compose -f compose.example.yaml logs -f
```

Compose 默认使用 `ghcr.io/oai404iao/sub_manager:latest`。如需固定版本，可在
`.env` 中增加：

```dotenv
SUBMAN_IMAGE=ghcr.io/oai404iao/sub_manager:0.1.2
```

### Docker CLI

以下示例假设服务位于 HTTPS 反向代理之后，因此只将容器端口绑定到本机，并启用
Secure Cookie。若直接通过 HTTP 访问，请使用实际的 `http://` 地址并将
`SUBMAN_SECURE_COOKIE` 设为 `false`。

```bash
docker run -d \
  --name sub-manager \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v sub-manager-data:/data \
  -e SUBMAN_ADMIN_PASSWORD='replace-with-a-strong-password' \
  -e SUBMAN_SIGNING_KEY="$(openssl rand -hex 32)" \
  -e SUBMAN_BASE_URL='https://sub.example.com' \
  -e SUBMAN_SECURE_COOKIE='true' \
  ghcr.io/oai404iao/sub_manager:0.1.2
```

镜像默认以非 root 用户运行，数据库保存在 `/data/sub-manager.db`，并通过
`GET /healthz` 执行容器健康检查。

本地构建镜像：

```bash
make docker-build
```

## 配置

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `SUBMAN_ADDR` | `0.0.0.0:8080` | HTTP 监听地址 |
| `SUBMAN_DB` | `data/sub-manager.db` | SQLite 数据库路径 |
| `SUBMAN_ADMIN_USER` | `admin` | 空数据库首次启动时创建的管理员用户名 |
| `SUBMAN_ADMIN_PASSWORD` | `admin123` | 空数据库首次启动时创建的管理员密码 |
| `SUBMAN_BASE_URL` | `http://127.0.0.1:8080` | 生成公开分享链接时使用的外部访问地址 |
| `SUBMAN_SIGNING_KEY` | 开发用默认值 | HMAC 分享签名密钥；生产环境应使用至少 32 字节随机值 |
| `SUBMAN_SECURE_COOKIE` | `false` | HTTPS 部署时设为 `true` |

`SUBMAN_ADMIN_USER` 与 `SUBMAN_ADMIN_PASSWORD` 只用于**空数据库的首次管理员初始化**；
已有数据库中的账号不会因修改环境变量而自动更新。修改 `SUBMAN_SIGNING_KEY`
会使之前生成的分享链接失效。

## 使用流程

1. 登录后创建分组；可选择已有分组作为下级分组，组成无循环的多层结构。添加订阅时
   也可以让系统自动创建目标分组。
2. 粘贴节点 URI、Base64 订阅、Xray 配置或 Mihomo YAML 批量导入。
3. 在节点编辑器中维护常用字段，必要时切换到完整 Xray JSON。
4. 在节点列表勾选一个或多个节点，可批量添加、移出或替换分组，复制多行 URI
   或标准 Base64 订阅内容，也可批量删除。所选节点包含订阅托管节点时，会同步
   修改订阅目标分组，并迁移该订阅的全部节点。
5. 为节点或分组生成限时或永久的订阅分享 URL，并可从对应对象的分享弹窗查看
   历史记录、撤销分享或删除记录。分组分享会递归聚合并去重所有下级分组节点；
   分组提供分享 URL 二维码，单节点还会额外提供节点 URI 二维码。
6. 通过订阅列表手动刷新上游内容；系统会原子替换该订阅托管的节点。

## 架构

```text
Browser
  ├─ Development: Vite dev server
  └─ Production: embedded React assets
                    │
                    ▼
              Go net/http API
          ┌─────────┼──────────┐
          ▼         ▼          ▼
       Session   Protocol    Signed share
        auth     parsers      endpoints
          │         │          │
          └─────────┴──────────┘
                    │
                    ▼
               SQLite store
```

- 前端：TypeScript、React、Vite、Tailwind CSS v4、shadcn/ui（Base UI）
- 后端：Go 标准 `net/http`
- 存储：SQLite（WAL）
- 发布：Vite 输出到 `internal/webassets/dist`，由 `go:embed` 嵌入二进制

更多细节见 [架构与目录设计](docs/architecture.md)。

## 运行状态与版本

无需登录的运维端点：

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/api/version
./bin/sub-manager --version
```

## 开发与验证

| 命令 | 用途 |
| --- | --- |
| `make dev` | 启动 Go API |
| `npm --prefix web run dev` | 启动 Vite 开发服务器 |
| `make build` | 构建前端并生成 `bin/sub-manager` |
| `make test` | 运行 Go 测试和 TypeScript 类型检查 |
| `make vet` | 运行 `go vet` |
| `make lint` | 运行 ESLint |
| `make fmt` | 格式化 Go、TypeScript 与 TSX |
| `make ci` | 执行与 GitHub Actions 对齐的完整检查 |
| `make docker-build` | 本地构建容器镜像 |

使用官方 Xray-core v26.7.28 校验生成配置：

```bash
XRAY_BIN=/path/to/xray make test-xray
```

## 版本与发布

项目采用 SemVer。以下文件的版本必须保持一致：

- `VERSION`
- `web/package.json`
- `web/package-lock.json`

准备新版本：

```bash
./scripts/set-version.sh 0.2.0
```

从已同步且工作区干净的 `main` 分支发布：

```bash
./scripts/release.sh 0.2.0
```

推送 `vX.Y.Z` 标签后，GitHub Actions 会完成校验、Linux amd64 发布包、
GHCR 镜像、SBOM、构建来源证明与 GitHub Release。

## 安全建议

- 不要在公网继续使用默认管理员密码或默认签名密钥。
- HTTPS 部署时设置 `SUBMAN_SECURE_COOKIE=true`。
- 将 `SUBMAN_BASE_URL` 设置为用户实际访问的 HTTPS 地址。
- 停止应用后备份 SQLite 数据库，或使用支持 WAL 的 SQLite 在线备份方式。
- 节点凭据会提供给已登录的管理前端；请保护管理员账号和数据库文件。
- 订阅抓取会阻止私网、回环、链路本地和保留地址，但仍应只添加可信上游。

## 项目文档

- [架构与目录设计](docs/architecture.md)
- [VLESS 可配置项](docs/protocols/vless.md)
- [SOCKS5 可配置项](docs/protocols/socks.md)
- [品牌与 Logo 规范](docs/brand.md)
- [实现计划](docs/implementation-plan.md)
- [更新记录](CHANGELOG.md)

## 参与开发

欢迎通过 Issue 与 Pull Request 改进项目。提交前请至少执行：

```bash
npm --prefix web ci
make ci
```

涉及协议兼容性的修改还应补充 `internal/protocol` 测试，并在可用时运行官方
Xray 校验。
