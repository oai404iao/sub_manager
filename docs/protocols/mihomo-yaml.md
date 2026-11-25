# Mihomo YAML 分享和导出

在节点列表勾选节点后可复制 Mihomo YAML；在单节点或分组的「新建分享」中，
可选择 Mihomo YAML 生成签名 URL。两者都只输出 Mihomo
[代理集合](https://wiki.metacubex.one/config/proxy-providers/content/)的顶层
`proxies` 列表，不包含 `proxy-groups`、`rules`、DNS 或其他客户端设置。
现有 Base64 和 URI 格式不变。

- **VLESS**：支持可等价表示的 RAW/TCP、WebSocket、TLS gRPC 和 XHTTP，
  以及相应的 TLS/REALITY 配置。XHTTP 的显式 XMUX 映射为 `xhttp-opts.reuse-settings`；
  没有显式有效参数时不填造默认值，两个内核的默认连接池行为可能不同。
- **SOCKS5**：支持认证、UDP 策略及 Mihomo 能表达的 TLS；不转换自定义 SNI。
- **Hysteria 2**：支持认证、SNI、TLS 验证开关、静态 ECH、混淆和端口列表；
  `pinSHA256` 与 Mihomo 证书指纹验证规则不同，不做自动转换。
- **Mieru**：一个节点含多组端口绑定时，为每组生成一条 Mihomo 代理，保留
  TCP/UDP 传输及 UDP 转发策略。Profile 是客户端配置名，不进入 Mihomo 节点；
  自定义 MTU 无等价字段，需使用支持它的 Mieru 客户端。

Mihomo YAML 能直接保留 VLESS、SOCKS5 和 Mieru 的 UDP 转发策略。遇到不能
等价写入 Mihomo 的配置，整次分享或导出会报错并指出对应节点，不会静默丢弃。
