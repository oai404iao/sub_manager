# Changelog

本项目遵循 [Semantic Versioning](https://semver.org/)。

## 0.1.1 - 2026-08-08

- 修复超长 URI、Base64 和密钥内容导致文本框宽度溢出的问题。
- 扩展 YAML、URI、标准/URL-safe Base64 与 data URI 导入解析。
- 增加订阅编辑接口和界面，刷新时原子替换上一次同步的节点。
- 补充节点显式编辑入口，节点与订阅均支持新增、编辑和删除。

## 0.1.0 - 2026-08-08

首个公开版本：

- 提供登录鉴权、SQLite 持久化以及节点、分组和订阅管理。
- 支持 VLESS、SOCKS5、Mihomo YAML、Base64 订阅和 Xray JSON 导入。
- 对齐 Xray-core v26.7.28 的 VLESS 出站配置、传输层和分享链接。
- 支持带签名的分享 URL，以及订阅 URL/完整节点二维码。
- 提供嵌入 Go 二进制的 React + shadcn/ui 管理界面。
- 提供 Linux amd64 二进制、Docker 镜像和自动化发布流程。
