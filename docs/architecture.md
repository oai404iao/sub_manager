# 架构与目录设计

## 技术决策

- **前端**：TypeScript、React、Vite、Tailwind CSS v4、shadcn/ui 官方组件。
- **后端**：Go 标准 `net/http`，减少框架绑定；SQLite 作为首期存储。
- **发布**：Vite 输出到 `internal/webassets/dist`，由 `go:embed` 打入二进制。
- **数据模型**：节点与分组采用多对多关系；分组之间采用无循环的多层包含关系；
  订阅绑定一个自动创建或指定的分组。
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
│   ├── protocol/               # URI、Xray JSON、订阅、Mihomo YAML 解析/导出
│   ├── share/                  # HMAC 签名
│   ├── store/                  # SQLite schema 与查询
│   ├── version/                # 构建版本、提交与构建时间
│   └── webassets/              # go:embed 前端产物
├── web/
│   ├── public/brand/           # Logo 与静态品牌资产
│   └── src/
│       ├── components/ui/      # shadcn 官方源码组件
│       ├── components/         # 业务组件与品牌组件
│       └── lib/                # API client 与类型
├── scripts/                    # 版本更新、发布包与标签发布
├── .github/workflows/          # CI、GitHub Release 与 GHCR 发布
├── Dockerfile                  # Node/Go 多阶段构建与非 root 运行镜像
├── compose.example.yaml        # 持久卷和安全选项部署示例
└── docs/
    ├── brand.md                # 品牌设计与 Logo 使用规范
    ├── protocols/              # 协议配置项与兼容性说明
    └── implementation-plan.md
```

## 核心数据模型

- `users`：管理员账号。
- `sessions`：Session token 摘要、用户、过期时间。
- `groups`：用户维护的节点分组。
- `group_children`：分组的直接下级关系；一个分组可被多个上级分组复用，但禁止
  包含自身或形成循环。
- `nodes`：规范化节点字段和原始扩展 JSON；VLESS 节点同时保存完整
  `xray_outbound`，用于无损往返当前 Xray 配置。
- `node_groups`：节点与分组多对多关系。
- `subscriptions`：上游 URL、目标分组、刷新状态。
- `shares`：节点/分组分享历史、目标名称快照、签名 URL 与可选到期时间。

## API 边界

- `/api/auth/*`：登录、登出、当前用户。
- `/api/state`：管理页一次性加载节点、分组、订阅、分享历史与统计。
- `/api/nodes`、`/api/subscriptions`：节点与订阅 CRUD；订阅刷新时原子替换其托管节点。
- `/api/groups`：分组创建、编辑、删除与下级分组组织。
- `DELETE /api/nodes`：在事务中批量删除所选节点。
- `PATCH /api/nodes/groups`：批量添加、移出或替换手工节点的直接分组；订阅托管节点
  的分组仍由订阅目标分组控制。
- `POST /api/nodes/export`：按请求顺序导出多行 URI 或标准 Base64 订阅内容。
- `/api/nodes/import`：解析 Xray/Mihomo YAML、JSON、URI 和 Base64 内容。
- `/api/nodes/{id}/xray`：导出完整 Xray VLESS OutboundObject。
- `/api/shares`：生成并记录单一签名订阅 URL；支持限时与永久分享。分组仅返回
  URL 二维码，单节点额外返回节点 URI 二维码。分组分享通过递归查询聚合并去重
  当前分组及全部下级分组的节点。
- `/api/version`：无需鉴权的构建版本信息。
- `/healthz`：无需鉴权的容器健康检查。
- `/s?...`：无需登录、验证签名后输出订阅或节点内容。

后续可在不改变前端领域模型的情况下，将 SQLite Store 替换为 PostgreSQL，
或增加多用户 `owner_id`。

## 构建与发布

前端由 Vite 输出到 `internal/webassets/dist`，Go 使用 `go:embed all:dist`
打入服务器二进制。生产构建通过链接参数写入 `VERSION`、Git 提交和 UTC
构建时间。

Docker 构建分为 Node 前端、Go 静态二进制和 Alpine 运行时三层。运行时
使用固定非 root UID、只持久化 `/data`，首个发布流水线生成
`linux/amd64` 镜像。
