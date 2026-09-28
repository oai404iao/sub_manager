# AGENTS.md — Sub Manager

> 面向在本仓库中工作的 AI coding agents。修改代码前先阅读本文件；除非特别说明，命令均从仓库根目录执行。

## 项目定位

Sub Manager 是一个单管理员、自托管的 Xray / Mihomo 订阅与节点管理系统。
后端使用 Go 标准 `net/http` 和 SQLite，前端使用 React、TypeScript、Vite、
Tailwind CSS v4 与 shadcn/ui（Base UI）。生产构建将 Vite 产物通过
`go:embed` 打入单个 Go 二进制。

当前支持 VLESS 与 SOCKS5，核心能力包括多格式导入、节点/分组/订阅 CRUD、
订阅原子刷新、完整 Xray VLESS outbound 无损保存，以及可限时或永久的 HMAC
分享与历史记录。
用户界面、API 错误和主要文档使用中文；代码标识符使用英文。

## Repository Layout

```text
sub_manager/
|-- cmd/server/main.go             # 进程入口、版本参数、HTTP server 与优雅退出
|-- internal/
|   |-- auth/                      # bcrypt 密码、随机 Session token 与 SHA-256 摘要
|   |-- config/                    # SUBMAN_* 环境变量及开发默认值
|   |-- httpapi/                   # 路由、鉴权、CRUD、订阅抓取、分享与 SPA 托管
|   |-- model/model.go             # 后端领域模型与 JSON contract
|   |-- protocol/
|   |   |-- protocol.go            # URI/Base64/YAML/Mihomo 导入与 URI 导出
|   |   |-- xray_vless.go          # Xray VLESS JSON、规范化、校验与无损往返
|   |   `-- testdata/              # 协议导入测试样本
|   |-- share/                     # HMAC-SHA256 签名
|   |-- store/                     # SQLite schema、查询与事务
|   |-- version/                   # ldflags 注入的版本信息
|   `-- webassets/                 # go:embed 声明与 Vite 生产输出
|-- web/
|   |-- components.json            # shadcn Base Nova / Base UI 项目配置
|   |-- src/App.tsx                # 鉴权检查与顶层页面切换
|   |-- src/components/dashboard.tsx # 主要管理界面和领域表单
|   |-- src/components/ui/         # shadcn 组件源码
|   |-- src/lib/api.ts             # 前端 API client 与后端模型镜像
|   |-- src/index.css              # Tailwind v4、主题 token 与全局样式
|   `-- vite.config.ts             # API proxy 与嵌入式产物输出目录
|-- docs/
|   |-- architecture.md            # 架构和数据模型
|   |-- protocols/                 # 协议兼容性与上游依据
|   `-- brand.md                   # Logo 与品牌资产约束
|-- scripts/                       # 版本同步、发布包和 Git tag 发布
|-- .github/workflows/             # CI、容器 smoke test 与 release
|-- Makefile                       # 本地与 CI 的权威命令入口
|-- Dockerfile                     # Node/Go/Alpine 多阶段镜像
|-- compose.example.yaml           # 推荐部署示例
`-- VERSION                        # 当前 SemVer
```

## Workspace / Package Rules

- 根目录是 Go module：`github.com/oai404iao/sub_manager`。
- `web/` 是独立 pnpm package；包管理器是 pnpm `12.4.1`，锁文件是
  `web/pnpm-lock.yaml`。依赖命令使用 `pnpm -C web ...`。
- CI 与 Docker 使用 Go `1.26`、Node.js `24`。不要无依据降低或提高版本。
- 使用 `Makefile` 中的范围化 Go 命令。不要将 `go test ./...` 作为权威命令；
  安装前端依赖后，`web/node_modules` 内可能出现可被 Go 发现的第三方目录。
- 应用只读取进程环境变量，不会自动解析 `.env`。Compose 会按 Docker Compose
  规则读取根目录 `.env`。
- `bin/`、`data/`、`dist/`、`.env`、`web/node_modules/` 是本地产物，不应提交。
- Vite 的生产输出目录是 `internal/webassets/dist`。不要手改该目录；通过
  `pnpm -C web run build` 或 `make build` 生成。`index.html` 是被跟踪的
  生成文件，前端变更后应重建并检查其 diff；哈希资源由构建时重新生成。
- `VERSION` 与 `web/package.json` 的版本必须同步。`web/pnpm-lock.yaml`
  锁定依赖而不记录项目版本。使用 `./scripts/set-version.sh X.Y.Z`，
  不要只改其中一个版本文件。

## Build, Test & Dev Commands

```bash
# 首次安装前端依赖
pnpm -C web install --frozen-lockfile

# 后端开发（使用版本 ldflags）
make dev

# 前端开发；/api 与 /s 代理到 127.0.0.1:8080
pnpm -C web run dev

# 生产构建：先构建前端，再生成 bin/sub-manager
make build

# 常规验证
make test       # Go tests + TypeScript typecheck
make vet        # go vet
make lint       # ESLint
make fmt-check  # Go 格式检查
make ci         # fmt-check + vet + test + lint + frontend build

# 自动格式化
make fmt

# Docker
make docker-build

# 可选上游集成测试；必须是 Xray-core v26.7.28
XRAY_BIN=/path/to/xray make test-xray
```

提交代码前优先运行 `make ci`。只改 Markdown 时可以不运行构建，但仍需检查链接、
路径、命令和版本描述是否与仓库一致。

## Architecture

### 管理请求

```text
React SPA
  -> fetch("/api/...", same-origin cookie)
  -> internal/httpapi.Server.Handler
  -> requireAuth middleware
  -> handler validation / protocol normalization
  -> internal/store
  -> SQLite
  -> /api/state reloads the complete dashboard state, including share history
```

`/healthz`、`/api/version`、`/api/auth/login` 和 `/s` 是显式公开端点。其他
`/api/*` 路由注册在内部 mux，并统一经过 `requireAuth`。

### 订阅刷新

```text
Subscription URL
  -> safeHTTPClient (scheme/redirect/IP/timeout limits)
  -> protocol.ParseText
  -> Store.ReplaceSubscriptionNodes transaction
  -> delete old managed nodes
  -> insert all parsed nodes into the target group
```

刷新成功前不要先删除旧节点；`ReplaceSubscriptionNodes` 的事务语义是功能约束。

### VLESS 无损往返

```text
Xray JSON / form payload
  -> ApplyXrayOutbound
  -> normalized model.Node fields
  -> EnsureXrayOutbound + Validate
  -> config_json + selected SQL columns
  -> XrayOutboundJSON / URI / frontend editor
```

`model.Node.XrayOutbound` 保存完整 outbound，包括当前 UI 不认识的高级或未来字段。
常用表单字段和完整 JSON 是同一份配置的两个视图，不能将 outbound 简化为只剩
表单字段。

## Core Patterns

### API contract

- 路由集中在 `internal/httpapi/server.go:Handler()`。
- 新的受保护 API 路由应注册到内部 `api` mux；只有明确需要匿名访问的端点才注册
  到根 mux。
- 请求体通过 `decodeJSON` 读取：上限 1 MiB，并拒绝未知字段。
- API 错误统一为 `{"error":"..."}`；前端由 `APIError` 消费。
- 创建通常返回 `201`，更新返回 `200`，删除返回 `204`。
- 改动后端 JSON 字段时，同步更新 `web/src/lib/api.ts`，必要时更新
  `blankNode`、`mergeNodeIntoXray` 和相关表单。

### Node persistence

- `nodes` 表将名称、协议、服务器、端口作为查询列，同时把完整 `model.Node`
  保存到 `config_json`。
- `scanNode` 解码 JSON 后会用 SQL 列覆盖 ID、核心字段、订阅 ID 和时间戳。
- 节点与分组通过 `node_groups` 多对多关联。
- 订阅托管节点必须带 `SubscriptionID`，且当前只属于订阅目标分组。
- 涉及节点及关联表的更新使用事务；不要留下孤立或部分更新的数据。

### Protocol import and export

- `protocol.ParseText` 的识别顺序是：原文/可解码 Base64 candidate ->
  Xray JSON -> YAML -> URI lines。
- URI 仅支持 `vless`、`socks` 与 `socks5`；新增协议时必须同时考虑模型、解析、
  导出、验证、存储、前端与分享。
- VLESS 规范的权威实现位于 `internal/protocol/xray_vless.go`，当前固定基线为
  Xray-core `v26.7.28` / commit
  `5ca6f4b7d4dc20a881d4330e498892697627ec0c`。
- 修改传输、安全或分享参数时，同步更新
  `docs/protocols/vless.md`、协议测试和前端编辑器。
- Xray JSON 可以表达 URI 无法表达的字段；无损场景应继续以完整 JSON 为准。

### Frontend

- 页面状态由 `/api/state` 一次性加载；mutation 成功后通过 `onReload()` 重新拉取，
  不要同时维护第二套易失真的客户端缓存。
- 分享历史按 `kind + target_id` 过滤，只在对应节点或分组的分享弹窗中展示；
  不要增加全局分享历史页。
- 领域操作主要集中在 `web/src/components/dashboard.tsx`。新增 Node 字段通常要同步：
  `web/src/lib/api.ts`、`blankNode`、`mergeNodeIntoXray`、表单控件和提交 payload。
- `components.json` 指定 shadcn `base-nova`、Base UI、Lucide、`@/` alias。
  不要使用 Radix 专属 API。
- 优先复用 `web/src/components/ui/` 中已有组件。表单使用 `FieldGroup` / `Field`；
  按钮图标使用 `data-icon`；使用语义颜色 token，不写任意品牌外状态色。
- Prettier 规则：无分号、双引号、2 空格、80 列，并启用 Tailwind class 排序。
- 当前没有前端单元或 E2E 测试；最低验证是 typecheck、ESLint 和生产构建。

### Security-sensitive behavior

- Session cookie 名为 `subman_session`，有效期 7 天，`HttpOnly`、
  `SameSite=Strict`；数据库只保存 token 的 SHA-256 摘要。
- `SUBMAN_SECURE_COOKIE=true` 只适合 HTTPS。
- 上游订阅抓取必须继续经过 `safeHTTPClient`：仅 HTTP/HTTPS、最多 5 次跳转、
  20 秒 client timeout、私网/回环/链路本地/保留地址阻断、响应最多读取 8 MiB。
- 分享签名消息格式由 `shareMessage` 定义。改变字段或顺序会使已有 URL 失效。
  新分享只生成一个订阅 URL；`exp=0` 表示永久分享，旧的 `content=nodes` URL
  仍需保持可访问。
- `SUBMAN_BASE_URL` 决定分享 URL 的外部地址；不要从不可信请求头推导。
- 不要在日志、测试 fixture 或文档中加入真实节点密码、UUID、私钥或签名密钥。

## Gotchas

### 1. Go 开发服务器不会自动重建前端

`make dev` 只启动 Go。它使用当前 `internal/webassets/dist` 中的内容。开发 UI 时应
同时运行 Vite；验证生产嵌入行为时运行 `make build`。

### 2. 完整 Xray outbound 是 VLESS 的权威输入

保存节点时，`ApplyXrayOutbound` 会让 JSON 编辑器内容覆盖镜像字段，然后
`EnsureXrayOutbound` 将规范化字段写回 outbound。跳过该顺序可能丢失高级字段，
或让表单与导出 JSON 不一致。

### 3. Xray v26.7.28 明确拒绝若干旧字段

不要重新引入 `allowInsecure`、旧 `http/h2/h3/quic` transport、mKCP `header`
或 `seed`。REALITY 仅允许 RAW/XHTTP/gRPC；Hysteria 必须使用 TLS。若升级
Xray 基线，应以一手代码为依据，并成组更新常量、文档、校验与测试。

### 4. 管理员环境变量只负责首次建库

`Store.ensureAdmin` 在 `users` 表已有记录时不会更新账号或密码。修改
`SUBMAN_ADMIN_USER` / `SUBMAN_ADMIN_PASSWORD` 不会重置已有数据库中的管理员。

### 5. 订阅更新具有替换语义

创建和编辑订阅都会立即抓取上游；刷新会删除该订阅原有托管节点并在同一事务中
插入新节点；删除订阅也会删除其托管节点。不要悄悄改成追加语义。

### 6. 删除分组可能被数据库拒绝

`subscriptions.group_id` 使用 `ON DELETE RESTRICT`。仍被订阅引用的分组不能删除，
API 当前将此类错误映射为用户可读的 `400`。

### 7. 版本需要两处同步

发布 workflow 会核对 `VERSION` 和 `web/package.json`。修改依赖时还需
同步更新 `web/pnpm-lock.yaml`；发布使用 `--frozen-lockfile` 校验。

### 8. `go test ./...` 不是本项目的标准检查

前端依赖中可能含 Go 源码目录。使用 `make test` 或显式
`go test ./cmd/... ./internal/...`，与 CI 保持一致。

## Common Workflows

### 添加或修改 Node 字段

1. 更新 `internal/model/model.go` 的字段和 JSON tag。
2. 更新 URI/YAML/Xray 的解析、应用、构建与校验逻辑。
3. 判断字段只需进入 `config_json`，还是也需要新的可查询 SQL 列。
4. 同步 `web/src/lib/api.ts`、默认值、Xray merge 和编辑表单。
5. 为 URI round-trip、Xray preservation 和错误校验补测试。
6. 更新 `docs/protocols/*`，运行 `make ci`；相关时运行 `make test-xray`。

### 添加 API endpoint

1. 在 `Server.Handler()` 中决定公开或鉴权路由。
2. 使用现有 `decodeJSON`、`writeJSON`、`writeError`、`pathID` helpers。
3. 将持久化逻辑放入 `internal/store`，将协议逻辑放入 `internal/protocol`。
4. 在 `internal/httpapi/server_test.go` 添加 `httptest` 覆盖。
5. 同步前端 API type/call，并运行 `make ci`。

### 修改 SQLite schema

1. 当前 schema 直接位于 `Store.migrate()`，且只有 `CREATE ... IF NOT EXISTS`。
2. 对已有数据库的变化必须提供幂等迁移逻辑；不要假设用户会删除数据库。
3. 保持 foreign key、WAL、`busy_timeout` 和单连接设置，除非有测试支撑变更。
4. 使用 `t.TempDir()` 中的真实 SQLite 文件补充升级与 CRUD 测试。

### 修改订阅抓取

1. 保留 SSRF、scheme、redirect、timeout 和 body-size 边界。
2. 在 API 测试中注入 `server.client` 或自定义 `RoundTripper`，不要请求真实网络。
3. 保留刷新失败时旧节点不被替换的行为。
4. 覆盖状态字段 `last_status`、`last_error`、`last_synced_at`。

### 准备发布

1. 更新 `CHANGELOG.md`。
2. 运行 `./scripts/set-version.sh X.Y.Z`。
3. 运行 `pnpm -C web install --frozen-lockfile && pnpm -C web audit --audit-level=low && make ci`。
4. 确认 `main`、`origin/main`、工作区均干净且同步。
5. 运行 `./scripts/release.sh X.Y.Z`；脚本会创建并推送带注释标签。

## Testing

| 层级 | 位置 | 说明 |
| --- | --- | --- |
| Protocol | `internal/protocol/*_test.go` | URI round-trip、Base64/YAML/Xray 导入、校验、兼容性 |
| Store | `internal/store/store_test.go` | 临时 SQLite、分组关联、订阅原子替换与删除 |
| HTTP API | `internal/httpapi/server_test.go` | `httptest`、登录 Cookie、CRUD、公开运维端点 |
| Share | `internal/share/signer_test.go` | 签名与篡改拒绝 |
| Version | `internal/version/version_test.go` | ldflags fallback 与字符串格式 |
| Upstream | `make test-xray` | 可选；调用官方 Xray-core v26.7.28 `run -test` |
| Frontend | pnpm scripts | 目前仅 typecheck、ESLint、Prettier 与 Vite build |
| Container | `.github/workflows/ci.yml` | 构建镜像、检查 `--version`、等待 healthcheck |

优先写靠近变更层的测试。协议修复应加入最小回归 fixture；Store/API 测试不得依赖
用户的 `data/sub-manager.db` 或外部服务。

## Key Files Quick Reference

| What | Where |
| --- | --- |
| Process entry | `cmd/server/main.go` |
| Environment config | `internal/config/config.go`, `.env.example` |
| Route registry | `internal/httpapi/server.go:Handler` |
| API/backend model | `internal/model/model.go` |
| Frontend model mirror | `web/src/lib/api.ts` |
| Import/export entry point | `internal/protocol/protocol.go:ParseText` |
| Xray VLESS source of truth | `internal/protocol/xray_vless.go` |
| SQLite schema | `internal/store/store.go:Store.migrate` |
| Frontend domain UI | `web/src/components/dashboard.tsx` |
| Frontend output config | `web/vite.config.ts` |
| Embedded assets | `internal/webassets/embed.go` |
| Canonical checks | `Makefile`, `.github/workflows/ci.yml` |
| Release rules | `scripts/release.sh`, `.github/workflows/release.yml` |
| Protocol documentation | `docs/protocols/vless.md`, `docs/protocols/socks.md` |

## Code Style

- Go：保持 `gofmt`，使用参数化 SQL、`context.Context` 和现有错误响应 helpers；
  需要跨表一致性时使用事务。
- TypeScript/React：遵循仓库 Prettier/ESLint 配置，使用 `@/` alias 和现有
  shadcn/Base UI 组件。
- 文案：面向用户的界面和 API 错误优先使用简洁中文；协议名、字段名和标准术语
  保持上游拼写。
- 文档：命令必须可复制执行；版本、路径、端口和默认值必须从代码、Makefile、
  CI 或配置文件验证，禁止凭空补充。
