# Sub Manager

面向 Xray 与 Mihomo 的轻量订阅、节点和分组管理系统。当前首期支持
VLESS 与 SOCKS5，React 前端会被打包并嵌入 Go 单文件程序。

## 当前实现

- Cookie Session 登录鉴权（密码使用 bcrypt，Session 仅保存 SHA-256 摘要）
- SQLite 持久化
- VLESS / SOCKS5 分享链接导入、Base64 订阅导入、Mihomo YAML 导入
- 节点、分组、订阅管理 API
- 带过期时间的 HMAC-SHA256 分享 URL
- “订阅 URL”与“完整节点内容”两类二维码
- shadcn/ui（Base UI）+ TypeScript + React + Vite 管理界面
- 前端静态资源通过 `go:embed` 进入 Go 二进制

## 快速启动

```bash
cp .env.example .env
npm --prefix web install
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

## 文档

- [架构与目录设计](docs/architecture.md)
- [VLESS 可配置项](docs/protocols/vless.md)
- [SOCKS5 可配置项](docs/protocols/socks.md)
- [实现计划](docs/implementation-plan.md)
