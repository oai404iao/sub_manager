# VLESS 可配置项

> 调研日期：2026-08-08。VLESS 分享 URI 并非 IETF 标准；不同客户端对查询参数
> 的支持有差异。系统保留未知参数，导出时优先使用业界通行字段。

## 规范化字段

| 分类 | 字段 | 类型/常见值 | Xray | Mihomo | 说明 |
|---|---|---|---|---|---|
| 基础 | `name` | string | tag（外层） | `name` | 展示名，URI 中在 `#fragment` |
| 基础 | `server` | hostname/IP | `address` | `server` | 服务端 |
| 基础 | `port` | 1-65535 | `port` | `port` | 端口 |
| 认证 | `uuid` | UUID/string | `id` | `uuid` | VLESS 用户 ID |
| 认证 | `encryption` | `none` | `encryption` | 通常隐含 | VLESS 通常固定为 `none` |
| 认证 | `flow` | 空、`xtls-rprx-vision` | `flow` | `flow` | Vision 流控 |
| 传输 | `network` | `tcp`,`ws`,`grpc`,`httpupgrade`,`xhttp` | `streamSettings.network` | `network` | URI 常用参数名 `type` |
| TLS | `security` | `none`,`tls`,`reality` | `streamSettings.security` | `tls` + `reality-opts` | URI 参数 |
| TLS | `sni` | hostname | `tlsSettings.serverName` / `realitySettings.serverName` | `servername` | TLS SNI |
| TLS | `alpn` | list | `tlsSettings.alpn` | `alpn` | 例如 `h2,http/1.1` |
| TLS | `fingerprint` | `chrome`,`firefox`,`safari`,`random` 等 | `fingerprint` | `client-fingerprint` | uTLS 指纹，URI 常用 `fp` |
| TLS | `allow_insecure` | bool | `allowInsecure` | `skip-cert-verify` | 不校验证书，默认 false |
| REALITY | `public_key` | string | `password`（新配置语义）/兼容 `publicKey` | `reality-opts.public-key` | URI 常用 `pbk` |
| REALITY | `short_id` | hex string | `shortId` | `reality-opts.short-id` | URI 常用 `sid` |
| REALITY | `spider_x` | path | `spiderX` | 部分版本不使用 | URI 常用 `spx` |
| WS | `host` | hostname | `wsSettings.headers.Host` | `ws-opts.headers.Host` | HTTP Host |
| WS | `path` | path | `wsSettings.path` | `ws-opts.path` | WebSocket path |
| gRPC | `service_name` | string | `grpcSettings.serviceName` | `grpc-opts.grpc-service-name` | URI 参数 `serviceName` |
| gRPC | `authority` | string | `grpcSettings.authority` | `grpc-opts.grpc-service-name` 之外的实现相关项 | 可选 |
| TCP | `header_type` | `none`,`http` | `tcpSettings.header.type` | 部分支持 | URI 参数 `headerType` |
| 通用 | `udp` | bool | 由路由/出站能力决定 | `udp` | Mihomo 开关 |
| 通用 | `packet_encoding` | `xudp`,`packet` 等 | `packetEncoding` | 实现相关 | 首期放扩展字段 |

## 分享 URI

```text
vless://UUID@HOST:PORT
  ?encryption=none
  &flow=xtls-rprx-vision
  &security=reality
  &sni=example.com
  &fp=chrome
  &pbk=PUBLIC_KEY
  &sid=SHORT_ID
  &type=tcp
  #NAME
```

首期解析并可视化编辑：`uuid/server/port/encryption/flow/network/security/sni/alpn/
fingerprint/allow_insecure/public_key/short_id/spider_x/host/path/service_name/
authority/header_type/udp`。未知查询参数进入 `extra`，避免导入后丢失。

## 校验规则

- `server` 非空，`port` 在 1–65535。
- `uuid` 非空；界面允许非标准 ID 以兼容部分实现，但给出警告。
- `security=reality` 时至少需要 `sni` 与 `public_key`。
- `network=ws/httpupgrade/xhttp` 时保留 `path`、`host`。
- `network=grpc` 时保留 `service_name`。
- `allow_insecure` 默认关闭。

## 一手资料

- Xray VLESS outbound：<https://xtls.github.io/en/config/outbounds/vless.html>
- Xray transport：<https://xtls.github.io/en/config/transports/>
- Mihomo VLESS：<https://wiki.metacubex.one/en/config/proxies/vless/>
- Xray-core VLESS URI discussion：<https://github.com/XTLS/Xray-core/discussions/716>
