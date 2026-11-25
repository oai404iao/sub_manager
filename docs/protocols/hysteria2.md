# Hysteria 2（Hy2）

Hysteria 2 是独立节点协议，不是 VLESS 的 Hysteria 传输类型。可在节点表单
手动管理，也可导入 Mihomo `type: hysteria2` 节点、URI 或 Base64 订阅。
分享、批量导出及订阅使用 [Hysteria 2 官方 URI 格式](https://v2.hysteria.network/docs/developers/URI-Scheme/)：

- 识别 `hysteria2://` 和 `hy2://`，统一输出 `hysteria2://`。
- URI 未指定端口时使用 `443`。端口列表例如 `443,5000-5010`，填写后覆盖
  单个端口；节点列表显示端口列表的第一个端口。
- 认证信息原样保存，包括 `username:password` 形式，不拆为两个表单字段。
- TLS 必需。支持 `sni`、`insecure=0|1`、证书指纹 `pinSHA256` 和静态
  ECH 配置 `ech`。默认不跳过证书验证。
- 混淆支持 `salamander` 和 `gecko`，开启时须填写 `obfs-password`。

Mihomo YAML/JSON 支持 `server`、`port` / `ports`、`password`、`sni`、
`skip-cert-verify`、`obfs` / `obfs-password` 和 `ech-opts.enable` 启用的
  静态 `ech-opts.config`。Mihomo 的 `fingerprint` 也可以钉住证书链中的 CA，
而标准 Hy2 `pinSHA256` 仅匹配叶证书，因此不做自动转换。自定义跳跃间隔、
上下行带宽、动态 ECH 等不能等价写入标准 URI 的配置也会明确拒绝，不静默丢弃。

能用标准 URI 表达的字段不一定被每个客户端的分享链接解析器正确读取。例如，
[已核对的 Mihomo 版本](https://github.com/MetaCubeX/mihomo/blob/69c16319/common/convert/converter.go)
未读取 Hy2 的 `ech`，也不保证能导入需要百分号转义的认证信息或带端口列表的
IPv6 链接；遇到这些情况，应使用 Mihomo 原生配置。

目前不导入 Realm URI，不转换原生 Hysteria 客户端配置或 Hy2 Xray JSON。
Xray 完整 JSON 的导入、编辑与下载仍用于 VLESS。
