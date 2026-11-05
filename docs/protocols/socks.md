# SOCKS5 可配置项

> 调研日期：2026-08-08。协议本体参考 RFC 1928，用户名/密码认证参考 RFC 1929。
> 管理系统当前把 SOCKS 作为“上游代理节点”，不是本地监听器。

## 规范化字段

| 字段 | 类型/常见值 | Xray outbound | Mihomo `socks5` | 说明 |
|---|---|---|---|---|
| `name` | string | tag（外层） | `name` | 展示名 |
| `server` | hostname/IP | `servers[].address` | `server` | 上游地址 |
| `port` | 1-65535 | `servers[].port` | `port` | 上游端口 |
| `username` | string | `servers[].users[].user` | `username` | 可选 |
| `password` | string | `servers[].users[].pass` | `password` | 可选 |
| `version` | `5`（首期） | `version` | 类型本身为 socks5 | Xray 也可表达 4/4a；首期仅 SOCKS5 |
| `udp` | bool | `udp` | `udp` | UDP ASSOCIATE 能力 |
| `tls` | bool | 通常由其他出站/传输处理 | `tls` | Mihomo 的 TLS SOCKS |
| `sni` | hostname | TLS 传输相关 | `servername` | TLS 时使用 |
| `allow_insecure` | bool | TLS 传输相关 | `skip-cert-verify` | 默认 false |
| `ip_version` | `dual`,`ipv4`,`ipv6` 等 | 路由/DNS 层 | `ip-version` | 首期扩展字段 |

## 分享 URI

系统接受：

```text
socks5://user:password@host:1080#name
socks://user:password@host:1080#name
socks5h://user:password@host:1080#name
```

用户名和密码使用 URL percent-encoding；也接受 Base64 `username:password`
userinfo（标准/URL-safe、有无 padding）。显式 `user:password` 优先按明文解析，
解码后只按第一个冒号拆分。没有认证时省略 `userinfo@`。

`socks5h` 作为 SOCKS5 别名导入，本系统不运行代理或执行 DNS。
历史项目 query（`udp/tls/sni/allowInsecure`）仍可导入、保存和编辑，但它们
不是通用 SOCKS 分享规范。Mihomo Alpha `69c16319` 的 SOCKS URI 分支不读取
这些 query，不能用项目自我往返证明互通。

因此，仅基础无 TLS、无 UDP 和无扩展语义的节点可输出 SOCKS URI。设置了
TLS、SNI、跳过证书验证、UDP 或扩展字段时，分享/批量导出明确报错；请使用
原生客户端配置。系统不新增 SOCKS 原生配置导出，也不输出看似可用但丢失 TLS
的链接。普通用户名/密码节点的分享继续可用。

## 协议注意点

- SOCKS5 支持无认证与用户名/密码认证；用户名/密码子协商由 RFC 1929 定义。
- UDP 依赖客户端、服务端与中间网络同时支持，不能仅凭配置保证可用。
- `tls` 不是 RFC 1928 的标准字段，是客户端实现提供的“SOCKS over TLS”扩展。
- 密码属于敏感数据；API 默认返回是为了允许编辑，后续应增加字段脱敏与密钥加密。

## 一手资料

- RFC 1928：<https://www.rfc-editor.org/rfc/rfc1928>
- RFC 1929：<https://www.rfc-editor.org/rfc/rfc1929>
- Xray SOCKS outbound：<https://xtls.github.io/en/config/outbounds/socks.html>
- Mihomo SOCKS5：<https://wiki.metacubex.one/en/config/proxies/socks/>
