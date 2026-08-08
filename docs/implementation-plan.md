# 实现计划

## Phase 1（当前骨架/MVP）

- [x] Go + React/Vite + shadcn/ui 工程骨架
- [x] SQLite schema、管理员初始化、Cookie Session
- [x] VLESS 与 SOCKS5 URI 解析/生成
- [x] Base64 文本订阅和 Mihomo YAML 解析
- [x] 节点、分组、订阅 API
- [x] HMAC 参数签名单一分享 URL、限时/永久分享与历史记录
- [x] 分组 URL 二维码、单节点 URL + URI 二维码
- [x] 基础管理界面

## Phase 2（可用性与兼容）

- [ ] 节点批量选择、拖拽入组、批量删除/导出
- [ ] 订阅定时刷新、差异预览、失效节点保留策略
- [x] VLESS 当前常用字段表单 + 完整 Xray JSON 编辑器
- [x] Xray VLESS JSON 导入与导出
- [ ] Mihomo JSON/YAML 双向导出
- [ ] 节点去重规则（协议 + 服务端 + 端口 + 身份）
- [ ] 审计日志与登录限流

## Phase 3（运行与质量）

- [ ] 节点连通性、TLS 握手、延迟探测
- [ ] 多用户/RBAC
- [ ] PostgreSQL
- [ ] OpenAPI、端到端测试、版本化迁移
- [ ] Docker 与 GitHub Actions 发布

## 首期验收

1. 可登录并创建分组。
2. 可粘贴 VLESS/SOCKS 链接，或填写订阅 URL 导入。
3. 可在界面编辑节点名称、地址、端口及协议关键参数。
4. 可按分组筛选节点。
5. 可为节点/分组生成限时或永久的签名公开 URL，并在对应分享弹窗查看历史。
6. 分组可生成订阅 URL 二维码；单节点还可生成节点 URI 二维码。
7. `make build` 生成一个可直接运行的 Go 二进制。
