package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/oai404iao/sub_manager/internal/model"
)

// MihomoYAML renders a proxy-provider document, not a full Mihomo configuration.
func MihomoYAML(nodes []model.Node) (string, error) {
	proxies := make([]map[string]any, 0, len(nodes))
	names := make(map[string]bool)
	for _, node := range nodes {
		items, err := mihomoProxies(node)
		if err != nil {
			return "", fmt.Errorf("节点 %q: %w", node.Name, err)
		}
		for _, proxy := range items {
			base, _ := proxy["name"].(string)
			name := base
			for suffix := 2; names[name]; suffix++ {
				name = fmt.Sprintf("%s (%d)", base, suffix)
			}
			proxy["name"], names[name] = name, true
			proxies = append(proxies, proxy)
		}
	}
	data, err := yaml.Marshal(map[string]any{"proxies": proxies})
	return string(data), err
}

func mihomoProxies(node model.Node) ([]map[string]any, error) {
	if err := ApplyXrayOutbound(&node); err != nil {
		return nil, err
	}
	if err := Validate(node); err != nil {
		return nil, err
	}
	proxy := map[string]any{"name": node.Name, "type": node.Protocol, "server": node.Server, "port": node.Port}
	var err error
	switch node.Protocol {
	case "vless":
		err = mihomoVLESSProxy(node, proxy)
	case "socks5":
		err = mihomoSOCKSProxy(node, proxy)
	case "hysteria2":
		err = mihomoHysteria2Proxy(node, proxy)
	case "mieru":
		return mihomoMieruProxies(node, proxy)
	}
	return []map[string]any{proxy}, err
}

func mihomoVLESSProxy(node model.Node, proxy map[string]any) error {
	outbound, err := buildXrayOutbound(node)
	if err != nil {
		return err
	}
	if err := mihomoFields(outbound, "outbound", "protocol tag settings streamSettings"); err != nil {
		return err
	}
	if err := mihomoFields(outboundMap(outbound, "settings"), "settings", "address port id encryption flow"); err != nil {
		return err
	}
	stream := outboundMap(outbound, "streamSettings")
	if err := mihomoFields(stream, "streamSettings", "method security rawSettings tcpSettings wsSettings grpcSettings xhttpSettings splithttpSettings tlsSettings realitySettings"); err != nil {
		return err
	}
	id, err := VLESSUUID(node.UUID)
	if err != nil {
		return err
	}
	proxy["uuid"], proxy["udp"] = id, node.UDP
	if node.Encryption != "none" {
		setOrDelete(proxy, "encryption", node.Encryption)
	}
	if node.Flow == "xtls-rprx-vision-udp443" {
		return errors.New("Mihomo YAML 无法保留 VLESS 的 xtls-rprx-vision-udp443 行为")
	}
	setOrDelete(proxy, "flow", node.Flow)
	for key, value := range node.Extra {
		if key != "support-x25519mlkem768" || node.Security != "reality" || value != "true" {
			return fmt.Errorf("Mihomo YAML 无法无损表达 VLESS 扩展 %q", key)
		}
	}

	method := stringFromMap(stream, "method")
	switch method {
	case "raw":
		settings, _ := firstMap(stream, "rawSettings", "tcpSettings")
		header := outboundMap(settings, "header")
		if err := mihomoFields(settings, "rawSettings", "header"); err != nil {
			return err
		}
		if err := mihomoFields(header, "rawSettings.header", "type"); err != nil {
			return err
		}
		if node.HeaderType != "" && node.HeaderType != "none" {
			return errors.New("Mihomo YAML 无法无损表达 Xray RAW header")
		}
		proxy["network"] = "tcp"
	case "websocket":
		ws, err := mihomoVLESSWS(node, outboundMap(stream, "wsSettings"))
		if err != nil {
			return err
		}
		proxy["network"], proxy["ws-opts"] = "ws", ws
	case "grpc":
		settings := outboundMap(stream, "grpcSettings")
		if err := mihomoFields(settings, "grpcSettings", "serviceName authority multiMode"); err != nil {
			return err
		}
		if node.Authority != "" || node.GRPCMultiMode {
			return errors.New("Mihomo YAML 无法无损表达 gRPC authority 或 multiMode")
		}
		if node.ServiceName == "" || url.PathEscape(node.ServiceName) != node.ServiceName {
			return errors.New("Mihomo YAML 无法无损保留空的或需要路径转义的 gRPC serviceName")
		}
		_, ipErr := netip.ParseAddr(node.Server)
		if node.Security != "tls" || (ipErr == nil && node.SNI == "") {
			return errors.New("Mihomo YAML 无法无损保留该 gRPC 节点的请求 authority；需要 TLS，并使用域名或明确 SNI")
		}
		proxy["network"], proxy["grpc-opts"] = "grpc", map[string]any{"grpc-service-name": node.ServiceName}
	case "xhttp":
		settings, _ := firstMap(stream, "xhttpSettings", "splithttpSettings")
		opts, err := mihomoVLESSXHTTP(settings)
		if err != nil {
			return err
		}
		proxy["network"], proxy["xhttp-opts"] = "xhttp", opts
		if node.Security == "none" {
			proxy["alpn"] = []string{"http/1.1"}
		}
	default:
		return fmt.Errorf("Mihomo YAML 不支持此 VLESS 传输方式 %q", method)
	}

	return mihomoVLESSTLS(node, stream, proxy, method)
}

func mihomoVLESSTLS(node model.Node, stream, proxy map[string]any, method string) error {
	if node.Security == "none" || node.Security == "" {
		return nil
	}
	if strings.EqualFold(node.SNI, "frommitm") {
		return errors.New("Mihomo YAML 无法保留动态 TLS SNI frommitm")
	}
	for _, protocol := range node.ALPN {
		if strings.EqualFold(protocol, "frommitm") {
			return errors.New("Mihomo YAML 无法保留动态 TLS ALPN frommitm")
		}
	}
	proxy["tls"] = true
	proxy["servername"] = valueOr(node.SNI, node.Server)
	fingerprint := valueOr(node.Fingerprint, "chrome")
	switch fingerprint {
	case "unsafe": // Xray uses Go TLS; Mihomo omits client-fingerprint for Go TLS.
	case "chrome", "firefox", "safari", "ios", "android", "edge", "360", "qq", "random", "randomized":
		proxy["client-fingerprint"] = fingerprint
	default:
		return errors.New("Mihomo YAML 不支持此 Xray TLS 客户端指纹")
	}
	if node.Security == "reality" {
		if err := mihomoFields(outboundMap(stream, "realitySettings"), "realitySettings", "serverName fingerprint password publicKey shortId"); err != nil {
			return err
		}
		opts := map[string]any{"public-key": node.PublicKey}
		setOrDelete(opts, "short-id", node.ShortID)
		if node.Extra["support-x25519mlkem768"] == "true" {
			opts["support-x25519mlkem768"] = true
		}
		proxy["reality-opts"] = opts
		return nil
	}
	if err := mihomoFields(outboundMap(stream, "tlsSettings"), "tlsSettings", "serverName fingerprint alpn"); err != nil {
		return err
	}
	if method == "xhttp" {
		switch {
		case len(node.ALPN) == 0:
			proxy["alpn"] = []string{"h2"}
		case len(node.ALPN) == 1 && (node.ALPN[0] == "h2" || node.ALPN[0] == "h3" || node.ALPN[0] == "http/1.1"):
			proxy["alpn"] = node.ALPN
		default:
			return errors.New("Mihomo XHTTP 需要明确的单一 ALPN：h2、h3 或 http/1.1")
		}
	} else if method == "raw" {
		alpn := node.ALPN
		if fingerprint != "unsafe" && len(alpn) != 0 && (len(alpn) != 2 || alpn[0] != "h2" || alpn[1] != "http/1.1") {
			return errors.New("Mihomo YAML 无法无损保留带 uTLS 指纹的 RAW 自定义 ALPN")
		}
		if len(alpn) == 0 {
			alpn = []string{"h2", "http/1.1"}
		}
		proxy["alpn"] = alpn
	} else if len(node.ALPN) != 0 {
		alpn := "http/1.1"
		if method == "grpc" {
			alpn = "h2"
		}
		if len(node.ALPN) != 1 || node.ALPN[0] != alpn {
			return fmt.Errorf("Mihomo %s 固定使用 ALPN %s，无法保留此配置", method, alpn)
		}
	}
	return nil
}

func mihomoVLESSWS(node model.Node, settings map[string]any) (map[string]any, error) {
	if err := mihomoFields(settings, "wsSettings", "path host headers"); err != nil {
		return nil, err
	}
	headers, err := mihomoHeaders(settings["headers"])
	if err != nil {
		return nil, err
	}
	for key := range headers {
		if strings.EqualFold(key, "host") {
			delete(headers, key)
		}
	}
	host := valueOr(node.Host, node.SNI)
	if host == "" {
		host = node.Server
		if ip, err := netip.ParseAddr(host); err == nil && ip.Is6() {
			host = "[" + host + "]"
		}
	}
	headers["Host"] = host
	ws := map[string]any{"headers": headers}
	path := node.Path
	parsed, err := url.Parse(path)
	if err != nil || parsed.EscapedPath() != (&url.URL{Path: parsed.Path}).EscapedPath() {
		return nil, errors.New("Mihomo YAML 无法保留此 WebSocket 路径转义")
	}
	if parsed.Query().Get("ed") != "" {
		query := parsed.Query()
		if early, _ := strconv.Atoi(query.Get("ed")); early > 0 {
			if early > 8192 {
				return nil, errors.New("WebSocket early-data 不得超过 8192 字节")
			}
			ws["max-early-data"] = early
			ws["early-data-header-name"] = "Sec-WebSocket-Protocol"
		}
		query.Del("ed")
		parsed.RawQuery = query.Encode()
		path = parsed.String()
	}
	setOrDelete(ws, "path", path)
	return ws, nil
}

func mihomoVLESSXHTTP(settings map[string]any) (map[string]any, error) {
	options := map[string]any{}
	for _, key := range []string{"host", "path", "mode"} {
		setOrDelete(options, key, stringFromMap(settings, key))
	}
	advanced := settings
	if value, found := settings["extra"]; found {
		var ok bool
		advanced, ok = value.(map[string]any)
		if !ok {
			return nil, errors.New("XHTTP extra 必须为 JSON 对象")
		}
	}
	for key, value := range advanced {
		switch key {
		case "host", "path", "mode": // Outer values take precedence over extra.
		case "headers":
			headers, err := mihomoHeaders(value)
			if err != nil {
				return nil, err
			}
			options[key] = headers
		case "noGRPCHeader":
			flag, ok := value.(bool)
			if !ok {
				return nil, errors.New("XHTTP noGRPCHeader 必须为布尔值")
			}
			options["no-grpc-header"] = flag
		case "xPaddingBytes", "scMaxEachPostBytes", "scMinPostsIntervalMs":
			keys := map[string]string{"xPaddingBytes": "x-padding-bytes", "scMaxEachPostBytes": "sc-max-each-post-bytes", "scMinPostsIntervalMs": "sc-min-posts-interval-ms"}
			text, _, err := mihomoExportRange(value)
			if err != nil {
				return nil, fmt.Errorf("XHTTP %s: %w", key, err)
			}
			setOrDelete(options, keys[key], text)
		case "xmux":
			reuse, err := mihomoExportXMUX(value)
			if err != nil {
				return nil, err
			}
			if len(reuse) > 0 {
				options["reuse-settings"] = reuse
			}
		default:
			return nil, fmt.Errorf("Mihomo YAML 无法无损表达 XHTTP 字段 %q", key)
		}
	}
	return options, nil
}

func mihomoExportXMUX(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	xmux, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("XHTTP xmux 必须为 JSON 对象")
	}
	reuse := make(map[string]any, len(xmux))
	nonzero := false
	for key, value := range xmux {
		var field string
		for mihomoKey, xrayKey := range mihomoXMUXFields {
			if xrayKey == key {
				field = mihomoKey
				break
			}
		}
		if field == "" {
			return nil, fmt.Errorf("Mihomo YAML 无法无损表达 XHTTP xmux.%s", key)
		}
		if value == nil {
			continue
		}
		if key == "hKeepAlivePeriod" {
			data, err := json.Marshal(value)
			var seconds int64
			if err != nil || json.Unmarshal(data, &seconds) != nil {
				return nil, errors.New("XHTTP xmux.hKeepAlivePeriod 必须为整数秒")
			}
			reuse[field] = seconds
			nonzero = nonzero || seconds != 0
			continue
		}
		text, upper, err := mihomoExportRange(value)
		if err != nil {
			return nil, fmt.Errorf("XHTTP xmux.%s: %w", key, err)
		}
		setOrDelete(reuse, field, text)
		nonzero = nonzero || upper > 0
	}
	if !nonzero {
		return nil, nil // No explicit tuning; do not turn on an unlimited reuse pool.
	}
	return reuse, nil
}

func mihomoExportRange(value any) (string, int64, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", 0, err
	}
	v, upper, err := mihomoXMUXRange(data)
	if err != nil {
		return "", 0, err
	}
	if integer, ok := v.(int32); ok {
		return strconv.FormatInt(int64(integer), 10), upper, nil
	}
	return v.(string), upper, nil
}

func mihomoFields(values map[string]any, path, fields string) error {
	for key := range values {
		if !strings.Contains(" "+fields+" ", " "+key+" ") {
			return fmt.Errorf("Mihomo YAML 无法无损表达 %s.%s", path, key)
		}
	}
	return nil
}

func mihomoHeaders(value any) (map[string]string, error) {
	headers := map[string]string{}
	if value == nil {
		return headers, nil
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("HTTP headers 必须为字符串映射")
	}
	for key, value := range raw {
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("HTTP headers 必须为字符串映射")
		}
		headers[key] = text
	}
	return headers, nil
}
