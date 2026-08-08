# 架构与目录设计

## 技术决策

- **前端**：TypeScript、React、Vite、Tailwind CSS v4、shadcn/ui 官方组件。
- **后端**：Go 标准 `net/http`，减少框架绑定；SQLite 作为首期存储。
- **发布**：Vite 输出到 `internal/webassets/dist`，由 `go:embed` 打入二进制。
- **数据模型**：节点与分组采用多对多关系；订阅绑定一个自动创建或指定的分组。
- **安全**：bcrypt 密码、随机 Session、HttpOnly/SameSite Cookie、HMAC 分享签名、
  订阅抓取的私网地址拦截、响应体大小限制。

## 目录

```text
.
├── cmd/server/                 # 程序入口
├── internal/
│   ├── auth/                   # 密码与 Session
│   ├── config/                 # 环境变量配置
│   ├── httpapi/                # 路由、JSON API、公开分享
│   ├── model/                  # 领域模型
│   ├── protocol/               # VLESS/SOCKS URI、订阅、Mihomo YAML 解析/导出
│   ├── share/                  # HMAC 签名
│   ├── store/                  # SQLite schema 与查询
│   └── webassets/              # go:embed 前端产物
├── web/
│   └── src/
│       ├── components/ui/      # shadcn 官方源码组件
│       ├── components/         # 业务组件
│       └── lib/                # API client 与类型
└── docs/
    ├── protocols/              # 协议配置项与兼容性说明
    └── implementation-plan.md
```

## 核心数据模型

- `users`：管理员账号。
- `sessions`：Session token 摘要、用户、过期时间。
- `groups`：用户维护的节点分组。
- `nodes`：规范化节点字段和原始扩展 JSON。
- `node_groups`：节点与分组多对多关系。
- `subscriptions`：上游 URL、目标分组、刷新状态。

## API 边界

- `/api/auth/*`：登录、登出、当前用户。
- `/api/state`：管理页一次性加载节点、分组、订阅与统计。
- `/api/nodes`、`/api/groups`、`/api/subscriptions`：CRUD/导入。
- `/api/shares`：生成已签名 URL 与二维码地址。
- `/s?...`：无需登录、验证签名后输出订阅或节点内容。

后续可在不改变前端领域模型的情况下，将 SQLite Store 替换为 PostgreSQL，
或增加多用户 `owner_id`。
