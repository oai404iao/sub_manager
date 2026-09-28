# VLESS / Xray-core v26.7.28

> 最后核对：2026-08-08
> 实现基线：Xray-core `v26.7.28`，commit
> `5ca6f4b7d4dc20a881d4330e498892697627ec0c`。该 tag 于
> 2026-07-28 以 pre-release 发布；截至 2026-08-08，它同时是官方仓库
> `main` 的最新 tag。配置代码是最终判定依据，文档作为解释性参考。

## 实现范围

本项目是订阅与客户端节点管理器，因此“完整 VLESS 实现”指 **Xray-core 当前
VLESS outbound/client 节点规范**，包括完整 `OutboundObject`、
`streamSettings`、分享 URI、导入、校验与导出。服务端 inbound 的
`decryption`、fallbacks、server-side REALITY 私钥等不属于订阅节点数据，
不会混入客户端节点模型；相关对象出现在导入 JSON 的非 outbound 部分时会被
忽略。

## 调研方式

本次没有继续沿用旧的客户端字段清单，而是重新核对了：

- Xray-core `infra/conf/vless.go`
- Xray-core `infra/conf/transport_internet.go`
- Xray-core `infra/conf/transport_method.go`
- Xray-core `infra/conf/transport_security.go`
- Xray-core `infra/conf/transport_sockopt.go`
- 各 transport 的 `.proto` 与实际 `Build()` 校验逻辑
- Xray 官方英文文档当前 `main`
- VLESS 分享链接讨论 #716
- VLESS Encryption PR #5067

项目中的生成结果还会使用官方 `xray run -test` 做可选集成测试。

## 与旧实现相比的重要变化

1. **VLESS Encryption 已正式实现**，不再只有 `none`。
2. `flow` 支持：
   - 空字符串
   - `xtls-rprx-vision`
   - `xtls-rprx-vision-udp443`
3. 推荐使用简化的 outbound settings，不再默认生成 `vnext/users`。
4. `streamSettings.method` 是当前字段名；仍兼容导入旧字段 `network`。
5. 当前传输方法是：
   - `raw`
   - `xhttp`
   - `mkcp`
   - `grpc`
   - `websocket`
   - `httpupgrade`
   - `hysteria`
6. `http`/`h2`/`h3` 与旧 `quic` transport 已从当前配置中移除。
7. `allowInsecure` 已由 Xray-core 移除。必须改用
   `pinnedPeerCertSha256` 和/或 `verifyPeerCertByName`。
8. REALITY 客户端当前字段名是 `password`；`publicKey` 仅作为旧别名导入。
9. REALITY 增加 `mldsa65Verify`，用于 ML-DSA-65 后量子签名验证。
10. mKCP 的旧 `header` 与 `seed` 已移除，应改用 FinalMask。

## 当前推荐 OutboundObject

```json
{
  "tag": "vless-out",
  "protocol": "vless",
  "sendThrough": "0.0.0.0",
  "targetStrategy": "AsIs",
  "settings": {
    "address": "example.com",
    "port": 443,
    "id": "5783a3e7-e373-51cd-8642-c83782b807c5",
    "encryption": "none",
    "flow": "xtls-rprx-vision",
    "level": 0,
    "email": "",
    "reverse": {}
  },
  "streamSettings": {
    "method": "raw",
    "security": "reality",
    "rawSettings": {},
    "realitySettings": {
      "serverName": "example.com",
      "fingerprint": "chrome",
      "password": "",
      "shortId": "",
      "mldsa65Verify": "",
      "spiderX": "/"
    },
    "sockopt": {},
    "finalmask": {}
  },
  "proxySettings": {},
  "mux": {
    "enabled": false,
    "concurrency": 8,
    "xudpConcurrency": 16,
    "xudpProxyUDP443": "reject"
  }
}
```

系统导入旧 `vnext/users` 格式，但导出统一转换为当前简化格式。Xray-core
v26.7.28 本身也要求每个 VLESS outbound 只能包含一个 endpoint 和一个 user；
多个服务器应拆成多个 outbound，再使用 balancer。

## VLESS settings

| 字段 | 当前规则 |
|---|---|
| `address` | 必填；域名、IPv4 或 IPv6 |
| `port` | 必填；1–65535 |
| `id` | UUID，或 UTF-8 长度不超过 30 字节的自定义字符串 |
| `encryption` | 必填；关闭时必须显式写 `none` |
| `flow` | 空、`xtls-rprx-vision`、`xtls-rprx-vision-udp443` |
| `level` | 可选，本地 policy level，默认 0 |
| `email` | 可选 |
| `reverse` | 可选，VLESS minimalist reverse proxy |

自定义 `id` 使用 Xray 的规则映射为 UUIDv5：以 16 个零字节作为 namespace，
SHA-1 后设置 UUID version 5 与 RFC 4122 variant。分享 URI 输出实际映射后的
UUID，Xray JSON 中保留用户输入的原始 `id`。

## VLESS Encryption

客户端 `encryption` 格式：

```text
mlkem768x25519plus.<appearance>.<session>.<padding...>.<auth>[.<relay-auth>...]
```

### 固定块

| 块 | 当前值 |
|---|---|
| handshake | `mlkem768x25519plus` |
| appearance | `native` / `xorpub` / `random` |
| session | `0rtt` / `1rtt` |

### padding

padding 与 delay 交替出现，格式都是：

```text
probability-min-max
```

- probability：0–100
- 第一段 padding 必须是 100% 且最小长度至少 35
- 所有 padding 最大长度之和不能超过 65553
- 未填写 padding 时，Xray 使用内置默认值

### authentication

- X25519：Base64URL Raw 解码后 32 字节
- ML-KEM-768：Base64URL Raw 解码后 1184 字节
- 支持连续多个认证参数作为 relay chain

推荐直接使用官方命令生成：

```bash
xray vlessenc
```

## Flow 与安全组合

### `xtls-rprx-vision`

- TCP + TLS/REALITY：可以尝试底层 Splice。
- 开启 VLESS Encryption：不限制底层 transport；非 TCP 时可穿透
  Encryption 以减少额外开销。
- 默认拦截 UDP/443，促使浏览器回退 TCP HTTPS。

### `xtls-rprx-vision-udp443`

与 Vision 相同，但不拦截 UDP/443。

### 公开地址安全限制

连接公开服务端时，以下三者至少需要一个：

- `streamSettings.security: "tls"`
- `streamSettings.security: "reality"`
- 启用 VLESS Encryption

`security: none + encryption: none` 只允许可信私网地址。

## StreamSettings

### 传输兼容性

| method | none | TLS | REALITY |
|---|---:|---:|---:|
| `raw` | 支持 | 支持 | 支持 |
| `xhttp` | 支持 | 支持 | 支持 |
| `grpc` | 支持 | 支持 | 支持 |
| `websocket` | 支持 | 支持 | 不支持 |
| `httpupgrade` | 支持 | 支持 | 不支持 |
| `mkcp` | 支持 | 支持 | 不支持 |
| `hysteria` | 不支持 | 必须 | 不支持 |

### RAW

```json
{
  "rawSettings": {
    "header": {
      "type": "none"
    }
  }
}
```

`header.type` 可为 `none` 或 `http`。HTTP header 模式还支持完整 request /
response version、method、path、status、reason 与多值 headers。

### XHTTP

代码中的当前字段全集：

- `host`
- `path`
- `mode`: `auto` / `packet-up` / `stream-up` / `stream-one`
- `headers`
- `xPaddingBytes`
- `xPaddingObfsMode`
- `xPaddingKey`
- `xPaddingHeader`
- `xPaddingPlacement`: `cookie` / `header` / `query` / `queryInHeader`
- `xPaddingMethod`: `repeat-x` / `tokenish`
- `uplinkHTTPMethod`
- `sessionIDPlacement`
- `sessionIDKey`
- `sessionIDTable`
- `sessionIDLength`
- `seqPlacement`
- `seqKey`
- `uplinkDataPlacement`
- `uplinkDataKey`
- `uplinkChunkSize`
- `noGRPCHeader`
- `noSSEHeader`
- `scMaxEachPostBytes`
- `scMinPostsIntervalMs`
- `scMaxBufferedPosts`
- `scStreamUpServerSecs`
- `serverMaxHeaderBytes`
- `xmux`
- `downloadSettings`
- `extra`

`extra` 会被 Xray 反序列化为另一份 XHTTP 配置，外层 `host/path/mode` 覆盖
其中对应值。系统完整保留这些字段；常用字段使用表单，其余通过完整 Xray JSON
编辑器管理。

### mKCP

以当前代码而不是旧文档示例为准：

- `mtu`
- `tti`
- `uplinkCapacity`
- `downlinkCapacity`
- `cwndMultiplier`
- `maxSendingWindow`

`header` 和 `seed` 已移除。

### gRPC

- `authority`
- `serviceName`
- `multiMode`
- `idle_timeout`
- `health_check_timeout`
- `permit_without_stream`
- `initial_windows_size`
- `user_agent`

### WebSocket

- `host`
- `path`，支持 `?ed=<threshold>` Early Data，最大建议值/校验上限 8192
- `headers`
- `heartbeatPeriod`
- `acceptProxyProtocol` 仅 inbound

### HTTPUpgrade

- `host`
- `path`，支持 `?ed=<threshold>`
- `headers`
- `acceptProxyProtocol` 仅 inbound

### Hysteria transport

- `version` 必须为 2
- `auth`
- `udpIdleTimeout`
- `masquerade`

必须配合 TLS。QUIC 拥塞控制、UDP hopping 与窗口参数现在位于
`finalmask.quicParams`。

## TLS 客户端字段

系统完整保留以下当前字段：

- `serverName`
- `verifyPeerCertByName`
- `alpn`
- `minVersion`
- `maxVersion`
- `cipherSuites`
- `certificates`
- `disableSystemRoot`
- `enableSessionResumption`
- `fingerprint`
- `pinnedPeerCertSha256`
- `curvePreferences`
- `masterKeyLog`
- `echConfigList`
- `echSockopt`

`allowInsecure: true` 在 v26.7.28 会直接触发 removed-feature 错误，本系统同样
拒绝保存。

## REALITY 客户端字段

| 字段 | 规则 |
|---|---|
| `serverName` | 服务端允许的 serverName |
| `fingerprint` | 必填；REALITY 不允许 `unsafe`/`hellogolang` |
| `password` | 必填；Base64URL Raw 解码后 32 字节 |
| `shortId` | 0–16 个十六进制字符，字符数必须为偶数 |
| `mldsa65Verify` | 可选；Base64URL Raw 解码后 1952 字节 |
| `spiderX` | 可选；必须以 `/` 开头，默认 `/` |

## Outbound 外层字段

除了 VLESS settings，系统的 `xray_outbound` 会无损保存：

- `tag`
- `sendThrough`
- `targetStrategy`
- `proxySettings`
- `mux`
- `streamSettings`
- 未识别的未来字段

因此从 Xray JSON 导入后，即使某字段没有单独的表单控件，也不会在保存和再次
导出时丢失。

## 分享 URI

系统按 Xray 分享链接讨论 #716 实现当前相关参数：

| 参数 | 映射 |
|---|---|
| userinfo | 映射后的 UUID |
| `encryption` | VLESS Encryption 完整字符串 |
| `flow` | Flow |
| `type` | `tcp`/`xhttp`/`kcp`/`grpc`/`ws`/`httpupgrade`/`hysteria` |
| `security` | `none`/`tls`/`reality` |
| `sni` | TLS/REALITY serverName |
| `alpn` | TLS ALPN |
| `fp` | TLS/REALITY fingerprint |
| `ech` | TLS echConfigList |
| `pcs` | pinnedPeerCertSha256 |
| `vcn` | verifyPeerCertByName |
| `pbk` | REALITY password |
| `sid` | REALITY shortId |
| `pqv` | REALITY mldsa65Verify |
| `spx` | REALITY spiderX |
| `host` / `path` | HTTP 类 transport |
| `mode` | XHTTP mode 或 gRPC multi |
| `serviceName` / `authority` | gRPC |
| `mtu` / `tti` | mKCP |
| `extra` | Base64URL Raw 编码的 XHTTP extra JSON |
| `fm` | Base64URL Raw 编码的 FinalMask JSON |

### Mihomo 专用扩展

`support-x25519mlkem768=true` **不是 Xray VLESS 分享链接提案的标准参数**，
而是 Mihomo 识别的 REALITY 分享 URI 扩展。节点编辑器中的「Mihomo
X25519-MLKEM768」选项开启时，仅对 `security=reality` 的 URI 添加该参数；
关闭时省略。导入 Mihomo YAML 的
`reality-opts.support-x25519mlkem768: true` 也会保留到分享 URI。示例：

```text
vless://<uuid>@example.com:443?security=reality&pbk=<password>&fp=chrome&support-x25519mlkem768=true#example
```

此选项保存在节点的 URI 扩展参数中，**不会写入 Xray
`streamSettings.realitySettings`**；其他客户端可能忽略它。Mihomo 的开关
只决定是否保留指纹提供的 X25519-MLKEM768 能力，不能保证所选指纹一定提供
对应的 TLS key share。它不同于 `pqv`（REALITY ML-DSA-65 签名公钥）和
`encryption=mlkem768x25519plus...`（VLESS Encryption）。

URI 无法天然表达 Xray JSON 的全部字段，所以**完整无损导入/导出应使用 Xray
JSON**；分享 URI 用于客户端通用订阅兼容。

## 项目实现覆盖

- VLESS URI 解析和生成
- Base64 订阅
- Xray 完整配置、单个 outbound、outbound 数组导入
- 旧 `vnext/users` 导入
- 当前简化 settings 导出
- VLESS Encryption 结构和 padding 校验
- 自定义 ID 到 UUIDv5 映射
- 当前 flow 校验
- transport/security 兼容校验
- REALITY password/shortId/mldsa65Verify/spiderX 校验
- TLS certificate pin 校验
- removed transport / allowInsecure / mKCP legacy 字段拒绝
- 完整 `xray_outbound` JSON 无损存储
- Xray JSON 下载 API
- 使用官方 v26.7.28 二进制的可选集成测试

## 一手资料

- Xray-core tag：<https://github.com/XTLS/Xray-core/tree/v26.7.28>
- VLESS 配置代码：<https://github.com/XTLS/Xray-core/blob/v26.7.28/infra/conf/vless.go>
- Transport 配置代码：<https://github.com/XTLS/Xray-core/blob/v26.7.28/infra/conf/transport_internet.go>
- Transport method 代码：<https://github.com/XTLS/Xray-core/blob/v26.7.28/infra/conf/transport_method.go>
- TLS / REALITY 配置代码：<https://github.com/XTLS/Xray-core/blob/v26.7.28/infra/conf/transport_security.go>
- 官方 VLESS outbound 文档：<https://xtls.github.io/en/config/outbounds/vless.html>
- 官方 transport 文档：<https://xtls.github.io/en/config/transport.html>
- VLESS 分享链接标准：<https://github.com/XTLS/Xray-core/discussions/716>
- Mihomo REALITY 配置说明：<https://github.com/MetaCubeX/Meta-Docs/blob/main/docs/config/proxies/tls.en.md>
- Mihomo VLESS URI 扩展解析：<https://github.com/MetaCubeX/mihomo/blob/Alpha/common/convert/v.go>
- Mihomo REALITY 握手实现：<https://github.com/MetaCubeX/mihomo/blob/Alpha/component/tls/reality.go>
- VLESS Encryption PR：<https://github.com/XTLS/Xray-core/pull/5067>
