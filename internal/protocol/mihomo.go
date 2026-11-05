package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/oai404iao/sub_manager/internal/model"
)

// Only options with an equivalent representation in the stored node are accepted.
// DisallowUnknownFields also applies to nested transport and security options.
type mihomoVLESS struct {
	Type           string   `json:"type"`
	Name           string   `json:"name"`
	Server         string   `json:"server"`
	Port           int      `json:"port"`
	UUID           string   `json:"uuid"`
	Flow           string   `json:"flow"`
	Encryption     string   `json:"encryption"`
	Network        string   `json:"network"`
	TLS            bool     `json:"tls"`
	UDP            *bool    `json:"udp"`
	ALPN           []string `json:"alpn"`
	ServerName     string   `json:"servername"`
	Fingerprint    string   `json:"client-fingerprint"`
	SkipCertVerify bool     `json:"skip-cert-verify"`
	Reality        *struct {
		PublicKey string `json:"public-key"`
		ShortID   string `json:"short-id"`
		MLKEM     bool   `json:"support-x25519mlkem768"`
	} `json:"reality-opts"`
	WS *struct {
		Path            string            `json:"path"`
		Headers         map[string]string `json:"headers"`
		EarlyData       int               `json:"max-early-data"`
		EarlyDataHeader string            `json:"early-data-header-name"`
	} `json:"ws-opts"`
	GRPC *struct {
		ServiceName string `json:"grpc-service-name"`
	} `json:"grpc-opts"`
	XHTTP *struct {
		Path            string            `json:"path"`
		Host            string            `json:"host"`
		Mode            string            `json:"mode"`
		Headers         map[string]string `json:"headers"`
		NoGRPCHeader    *bool             `json:"no-grpc-header"`
		PaddingBytes    string            `json:"x-padding-bytes"`
		MaxPostBytes    string            `json:"sc-max-each-post-bytes"`
		MinPostInterval string            `json:"sc-min-posts-interval-ms"`
	} `json:"xhttp-opts"`
}

type mihomoSOCKS struct {
	Type           string `json:"type"`
	Name           string `json:"name"`
	Server         string `json:"server"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	UDP            bool   `json:"udp"`
	TLS            bool   `json:"tls"`
	SkipCertVerify bool   `json:"skip-cert-verify"`
}

func decodeMihomo(proxy map[string]any, target any) error {
	data, err := json.Marshal(proxy)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func parseMihomoProxies(proxies []map[string]any) ([]model.Node, bool, error) {
	nodes := make([]model.Node, 0, len(proxies))
	for index, proxy := range proxies {
		node, err := parseMihomoProxy(proxy)
		if err != nil {
			return nil, true, fmt.Errorf("Mihomo 节点 %d (%s): %w", index+1, stringValue(proxy["name"]), err)
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, true, errors.New("Mihomo 配置没有节点")
	}
	return nodes, true, nil
}

func parseMihomoProxy(proxy map[string]any) (model.Node, error) {
	var node model.Node
	switch lowerString(proxy["type"]) {
	case "socks", "socks5":
		var p mihomoSOCKS
		if err := decodeMihomo(proxy, &p); err != nil {
			return node, err
		}
		node = model.Node{
			Name: p.Name, Protocol: "socks5", Server: p.Server, Port: p.Port,
			Username: p.Username, Password: p.Password, UDP: p.UDP,
			TLS: p.TLS, AllowInsecure: p.SkipCertVerify,
		}
	case "vless":
		var p mihomoVLESS
		if err := decodeMihomo(proxy, &p); err != nil {
			return node, err
		}
		node = model.Node{
			Name: p.Name, Protocol: "vless", Server: p.Server, Port: p.Port,
			UUID: p.UUID, Flow: p.Flow, Encryption: valueOr(p.Encryption, "none"),
			Network: valueOr(p.Network, "raw"), Security: "none", SNI: p.ServerName,
			Fingerprint: p.Fingerprint, AllowInsecure: p.SkipCertVerify,
			ALPN: p.ALPN, UDP: true, Extra: map[string]string{},
		}
		if p.UDP != nil {
			node.UDP = *p.UDP
		}
		if p.TLS {
			node.Security = "tls"
		}
		if p.Reality != nil {
			if !p.TLS {
				return node, errors.New("reality-opts 需要 tls: true")
			}
			if len(p.ALPN) > 0 {
				return node, errors.New("REALITY 不支持导入 alpn")
			}
			if p.Fingerprint == "" || p.Fingerprint == "none" {
				return node, errors.New("REALITY 需要 client-fingerprint")
			}
			node.Security, node.PublicKey, node.ShortID = "reality", p.Reality.PublicKey, p.Reality.ShortID
			if p.Reality.MLKEM {
				node.Extra["support-x25519mlkem768"] = "true"
			}
		}
		if !p.TLS && (p.ServerName != "" || p.Fingerprint != "" || p.SkipCertVerify || len(p.ALPN) > 0) {
			return node, errors.New("TLS 参数需要 tls: true")
		}
		// Mihomo uses Go TLS without a client fingerprint; Xray defaults to Chrome.
		if node.Security == "tls" && (p.Fingerprint == "" || p.Fingerprint == "none") {
			node.Fingerprint = "unsafe"
		}
		switch p.Network {
		case "", "tcp", "ws", "grpc", "xhttp":
		default:
			return node, fmt.Errorf("不支持的 Mihomo network %q", p.Network)
		}
		var err error
		node.Network, err = canonicalTransport(node.Network)
		if err != nil {
			return node, err
		}
		if node.Security == "tls" {
			if p.SkipCertVerify {
				return node, errors.New("Xray 不支持 allowInsecure / skip-cert-verify")
			}
			switch node.Network {
			case "websocket":
				node.ALPN = []string{"http/1.1"}
			case "grpc":
				node.ALPN = []string{"h2"}
			case "raw":
				if len(p.ALPN) == 0 {
					return node, errors.New("Mihomo RAW TLS 未配置 ALPN，无法等价转换为 Xray 默认 ALPN；请明确配置 alpn")
				}
			}
		}
		if (p.WS != nil && node.Network != "websocket") || (p.GRPC != nil && node.Network != "grpc") || (p.XHTTP != nil && node.Network != "xhttp") {
			return node, errors.New("传输选项与 network 不匹配")
		}
		if p.GRPC != nil {
			node.ServiceName = p.GRPC.ServiceName
		}
		if p.WS != nil {
			node.Path = p.WS.Path
			for key, value := range p.WS.Headers {
				if strings.EqualFold(key, "host") {
					if node.Host != "" {
						return node, errors.New("重复的 WebSocket Host")
					}
					node.Host = value
				}
			}
			if p.WS.EarlyData < 0 || p.WS.EarlyData > 8192 {
				return node, errors.New("WebSocket early-data 必须在 0..8192 范围内")
			}
			if p.WS.EarlyDataHeader != "" && !strings.EqualFold(p.WS.EarlyDataHeader, "Sec-WebSocket-Protocol") {
				return node, errors.New("不支持此 WebSocket early-data header")
			}
			if p.WS.EarlyData > 0 {
				if p.WS.EarlyDataHeader == "" {
					return node, errors.New("不支持路径后缀形式的 WebSocket early-data")
				}
				u, err := url.Parse(node.Path)
				if err != nil {
					return node, err
				}
				q := u.Query()
				if q.Has("ed") {
					return node, errors.New("重复的 WebSocket early-data 参数")
				}
				q.Set("ed", strconv.Itoa(p.WS.EarlyData))
				u.RawQuery = q.Encode()
				node.Path = u.String()
			}
			if node.Security == "tls" && node.SNI == "" && node.Host != "" {
				node.SNI = node.Host
			}
		}
		if p.XHTTP != nil {
			x := p.XHTTP
			node.Path, node.Host, node.XHTTPMode = x.Path, x.Host, x.Mode
			// XHTTP Host controls HTTP authority, not Mihomo's TLS server name.
			if node.Security == "tls" && node.SNI == "" {
				node.SNI = node.Server
			}
			node.XHTTPExtra = map[string]any{}
			if len(x.Headers) > 0 {
				headers := map[string]any{}
				for key, value := range x.Headers {
					if strings.EqualFold(key, "host") {
						return node, errors.New("XHTTP headers 不能包含 Host，请使用 host")
					}
					headers[key] = value
				}
				node.XHTTPExtra["headers"] = headers
			}
			if x.NoGRPCHeader != nil {
				node.XHTTPExtra["noGRPCHeader"] = *x.NoGRPCHeader
			}
			for key, value := range map[string]string{"xPaddingBytes": x.PaddingBytes, "scMaxEachPostBytes": x.MaxPostBytes, "scMinPostsIntervalMs": x.MinPostInterval} {
				if value != "" {
					node.XHTTPExtra[key] = value
				}
			}
		}
		if err := EnsureXrayOutbound(&node); err != nil {
			return node, err
		}
		if p.WS != nil && len(p.WS.Headers) > 0 {
			stream := ensureChildMap(node.XrayOutbound, "streamSettings")
			ws := ensureChildMap(stream, "wsSettings")
			headers := map[string]any{}
			for key, value := range p.WS.Headers {
				if !strings.EqualFold(key, "host") {
					headers[key] = value
				}
			}
			if len(headers) > 0 {
				ws["headers"] = headers
			}
		}
	default:
		return node, fmt.Errorf("不支持的 Mihomo 协议 %q", stringValue(proxy["type"]))
	}
	return node, Validate(node)
}
