package protocol

import (
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/oai404iao/sub_manager/internal/model"
)

func mihomoSOCKSProxy(node model.Node, proxy map[string]any) error {
	switch {
	case len(node.Extra) > 0 || node.XrayOutbound != nil:
		return errors.New("Mihomo SOCKS5 无法保留扩展参数或 Xray 配置")
	case node.Username == "" && node.Password != "":
		return errors.New("Mihomo SOCKS5 仅在填写用户名时使用密码")
	case len(node.Username) > 255 || len(node.Password) > 255:
		return errors.New("SOCKS5 用户名和密码各不能超过 255 字节")
	case !node.TLS && (node.SNI != "" || node.AllowInsecure):
		return errors.New("SOCKS5 的 SNI 和跳过证书验证需要启用 TLS")
	case node.SNI != "" && node.SNI != node.Server:
		return errors.New("Mihomo SOCKS5 使用服务器地址作为 TLS 服务器名，无法设置不同的 SNI")
	case node.Fingerprint != "" || node.VerifyPeerName != "" || node.PinnedPeerCert != "" || node.ECHConfigList != "" || len(node.ALPN) > 0:
		return errors.New("Mihomo SOCKS5 无法保留节点的额外 TLS 设置")
	}
	setOrDelete(proxy, "username", node.Username)
	setOrDelete(proxy, "password", node.Password)
	proxy["udp"] = node.UDP
	if node.TLS {
		proxy["tls"] = true
		proxy["skip-cert-verify"] = node.AllowInsecure
	}
	return nil
}

func mihomoHysteria2Proxy(node model.Node, proxy map[string]any) error {
	switch {
	case node.PinnedPeerCert != "":
		return errors.New("Hysteria 2 的 pinSHA256 无法等价转换为 Mihomo fingerprint：两者的叶证书匹配和常规 TLS 验证规则不同")
	case len(node.Extra) > 0 || node.XrayOutbound != nil:
		return errors.New("Mihomo Hysteria 2 无法保留扩展参数或 Xray 配置")
	case node.Username != "":
		return errors.New("Hysteria 2 的认证信息须完整填写在密码字段中")
	case node.Fingerprint != "" || node.VerifyPeerName != "" || len(node.ALPN) > 0:
		return errors.New("Mihomo Hysteria 2 无法保留节点的额外 TLS 设置")
	}
	setOrDelete(proxy, "password", node.Password)
	setOrDelete(proxy, "sni", node.SNI)
	proxy["skip-cert-verify"] = node.AllowInsecure
	if node.ECHConfigList != "" {
		proxy["ech-opts"] = map[string]any{"enable": true, "config": node.ECHConfigList}
	}
	if options := node.Hysteria2; options != nil {
		if options.Ports != "" {
			first, ports, _ := parseHysteria2Ports(options.Ports)
			// Mihomo's unsigned range parser accepts at most 28 entries.
			if strings.Count(ports, ",") >= 28 {
				return errors.New("Mihomo Hysteria 2 的 ports 最多支持 28 项（单个端口或范围）")
			}
			proxy["port"], proxy["ports"] = first, ports
		}
		setOrDelete(proxy, "obfs", options.Obfs)
		setOrDelete(proxy, "obfs-password", options.ObfsPassword)
	}
	return nil
}

func mihomoMieruProxies(node model.Node, proxy map[string]any) ([]map[string]any, error) {
	options := node.Mieru
	if options.MTU != 0 && options.MTU != 1400 {
		return nil, errors.New("Mihomo Mieru 无法保留自定义 MTU；请使用支持该设置的 Mieru 客户端")
	}
	proxy["username"], proxy["password"], proxy["udp"] = node.Username, node.Password, node.UDP
	setOrDelete(proxy, "multiplexing", options.Multiplexing)
	setOrDelete(proxy, "handshake-mode", options.HandshakeMode)
	setOrDelete(proxy, "traffic-pattern", options.TrafficPattern)
	proxies := make([]map[string]any, 0, len(options.Bindings))
	for _, binding := range options.Bindings {
		first, port, _ := mieruPort(binding.Port)
		endpoint := maps.Clone(proxy)
		if len(options.Bindings) > 1 {
			endpoint["name"] = fmt.Sprintf("%s:%s/%s", proxy["name"], port, binding.Transport)
		}
		endpoint["transport"] = binding.Transport
		if strings.Contains(port, "-") {
			delete(endpoint, "port")
			endpoint["port-range"] = port
		} else {
			endpoint["port"] = first
		}
		proxies = append(proxies, endpoint)
	}
	return proxies, nil
}
