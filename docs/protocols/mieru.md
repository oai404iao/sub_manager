# Mieru

Sub Manager 支持通过节点表单或 Mihomo `type: mieru` 配置添加 Mieru 节点，
也可导入、导出和分享 [官方简单链接](https://github.com/enfein/mieru/blob/main/docs/client-install.zh_CN.md#简单分享链接)
`mierus://`。例如：

```text
mierus://test-user:test-password@example.com?profile=default&port=443&protocol=TCP&port=5000-5010&protocol=UDP#Demo
```

一条链接有一个服务器及一个 `profile`，可包含多个端口或端口范围。
`port` 和 `protocol` 按出现顺序配对，避免丢失每组端口的 TCP/UDP 选择。
表单中的「名称」用于面板显示；`profile` 是 Mieru 客户端配置名。使用原生客户端
导入同名链接可能覆盖已有配置，请按需换用不同的 `profile`。可选参数为
`mtu`（1280–1500，默认 1400）、`multiplexing`、`handshake-mode` 和
`traffic-pattern`；后者按标准 Base64 字符串保留，其 protobuf 内容由客户端解释。

## 兼容边界

- 导入 [Mihomo YAML/JSON](https://wiki.metacubex.one/config/proxies/mieru/) 时，
  `port` 与 `port-range` 只能填写一项。`transport` 指连接服务器使用 TCP
  还是 UDP；`udp` 是转发 UDP 流量的开关，两者独立。Mihomo 原生配置未写 `udp`
  时保留其关闭行为；简单链接不能表达关闭策略，可以改用 [Mihomo YAML](mihomo-yaml.md) 分享。
- [Mihomo 的 URI 导入器](https://github.com/MetaCubeX/mihomo/blob/Alpha/common/convert/converter.go)
  会按端口绑定拆为多个节点，且不读取 `mtu`。需要保留自定义 MTU 时，应使用支持它的 Mieru 客户端。
- 不支持 `mieru://` 完整客户端配置、Xray JSON 或 Mieru 服务端配置。若已有
  `mieru://`，可由官方客户端导入后运行 `mieru export config simple`，再复制
  生成的节点链接；`mierus://` 不包含客户端的本地监听等设置。
