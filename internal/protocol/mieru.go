package protocol

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oai404iao/sub_manager/internal/model"
)

const (
	mieruMinMTU = 1280
	mieruMaxMTU = 1500
)

func NormalizeMieru(node *model.Node) error {
	if node.Protocol != "mieru" {
		return nil
	}
	if node.Mieru == nil || len(node.Mieru.Bindings) == 0 {
		return errors.New("至少填写一个 Mieru 端口与传输方式")
	}
	for index := range node.Mieru.Bindings {
		binding := &node.Mieru.Bindings[index]
		first, spec, err := mieruPort(binding.Port)
		if err != nil {
			return fmt.Errorf("Mieru 端口绑定 %d: %w", index+1, err)
		}
		binding.Port = spec
		if index == 0 {
			node.Port = first
		}
	}
	return nil
}

func parseMieruURI(parsed *url.URL) (model.Node, error) {
	host, err := mieruAuthority(parsed.Hostname())
	if err != nil || parsed.Host != host || (parsed.Path != "" && parsed.EscapedPath() != "/") {
		return model.Node{}, errors.New("Mieru URI 须使用服务器地址，端口须写在 port 参数中")
	}
	if parsed.User == nil {
		return model.Node{}, errors.New("Mieru URI 缺少用户名和密码")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return model.Node{}, errors.New("Mieru URI 参数编码无效")
	}
	for key, values := range query {
		switch key {
		case "port", "protocol":
		case "profile", "mtu", "multiplexing", "handshake-mode", "traffic-pattern":
			if len(values) != 1 {
				return model.Node{}, errors.New("Mieru URI 有重复的单值参数")
			}
		default:
			return model.Node{}, errors.New("不支持的 Mieru URI 参数")
		}
	}
	ports, transports := query["port"], query["protocol"]
	if len(ports) != len(transports) || len(ports) == 0 {
		return model.Node{}, errors.New("Mieru URI 至少需要一对 port 与 protocol，并且数量须相同")
	}
	options := model.MieruOptions{
		Profile: query.Get("profile"), Multiplexing: query.Get("multiplexing"),
		HandshakeMode: query.Get("handshake-mode"), TrafficPattern: query.Get("traffic-pattern"),
		Bindings: make([]model.MieruBinding, len(ports)),
	}
	if query.Has("mtu") {
		options.MTU, err = strconv.Atoi(query.Get("mtu"))
		if err != nil {
			return model.Node{}, mieruMTUError()
		}
	}
	for index, port := range ports {
		options.Bindings[index] = model.MieruBinding{Port: port, Transport: transports[index]}
	}
	password, _ := parsed.User.Password()
	node := model.Node{
		Name: defaultName(parsed.Fragment, options.Profile), Protocol: "mieru", Server: parsed.Hostname(),
		Username: parsed.User.Username(), Password: password, Mieru: &options, UDP: true,
	}
	if err := NormalizeMieru(&node); err != nil {
		return model.Node{}, err
	}
	return node, Validate(node)
}

func mieruURI(node model.Node) (string, error) {
	if err := Validate(node); err != nil {
		return "", err
	}
	if !node.UDP {
		return "", errors.New("Mieru URI 无法保留禁用 UDP 转发的策略；请使用原生客户端配置")
	}
	options := node.Mieru
	query := url.Values{"profile": {options.Profile}}
	for _, binding := range options.Bindings {
		_, port, _ := mieruPort(binding.Port)
		query.Add("port", port)
		query.Add("protocol", binding.Transport)
	}
	if options.MTU != 0 {
		query.Set("mtu", strconv.Itoa(options.MTU))
	}
	set(query, "multiplexing", options.Multiplexing)
	set(query, "handshake-mode", options.HandshakeMode)
	set(query, "traffic-pattern", options.TrafficPattern)
	host, _ := mieruAuthority(node.Server)
	return (&url.URL{
		Scheme: "mierus", User: url.UserPassword(node.Username, node.Password), Host: host,
		RawQuery: query.Encode(), Fragment: node.Name,
	}).String(), nil
}

func validateMieru(node model.Node) error {
	if _, err := mieruAuthority(node.Server); err != nil {
		return err
	}
	if node.TLS || node.AllowInsecure || node.SNI != "" || len(node.Extra) > 0 || node.XrayOutbound != nil {
		return errors.New("Mieru 节点不支持 TLS、Xray 配置或 URI 扩展字段")
	}
	if node.Username == "" || node.Password == "" {
		return errors.New("Mieru 用户名和密码均不能为空")
	}
	if len(node.Username) > 64 || len(node.Password) > 64 {
		return errors.New("Mieru 用户名和密码各不能超过 64 字节")
	}
	options := node.Mieru
	if options == nil || strings.TrimSpace(options.Profile) == "" {
		return errors.New("Mieru profile 名称不能为空")
	}
	for _, text := range []string{node.Name, node.Server, node.Username, node.Password, options.Profile} {
		if !utf8.ValidString(text) {
			return errors.New("Mieru 名称、服务器和认证信息必须为有效的 UTF-8")
		}
	}
	if len(options.Bindings) == 0 {
		return errors.New("至少填写一个 Mieru 端口与传输方式")
	}
	for index, binding := range options.Bindings {
		if _, _, err := mieruPort(binding.Port); err != nil {
			return fmt.Errorf("Mieru 端口绑定 %d: %w", index+1, err)
		}
		if binding.Transport != "TCP" && binding.Transport != "UDP" {
			return fmt.Errorf("Mieru 端口绑定 %d 的传输方式必须是 TCP 或 UDP", index+1)
		}
	}
	if options.MTU != 0 && (options.MTU < mieruMinMTU || options.MTU > mieruMaxMTU) {
		return mieruMTUError()
	}
	switch options.Multiplexing {
	case "", "MULTIPLEXING_DEFAULT", "MULTIPLEXING_OFF", "MULTIPLEXING_LOW", "MULTIPLEXING_MIDDLE", "MULTIPLEXING_HIGH":
	default:
		return errors.New("无效的 Mieru 多路复用级别")
	}
	switch options.HandshakeMode {
	case "", "HANDSHAKE_DEFAULT", "HANDSHAKE_STANDARD", "HANDSHAKE_NO_WAIT":
	default:
		return errors.New("无效的 Mieru 握手模式")
	}
	if options.TrafficPattern != "" {
		if _, err := base64.StdEncoding.DecodeString(options.TrafficPattern); err != nil {
			return errors.New("Mieru traffic-pattern 必须是标准 Base64")
		}
	}
	return nil
}

func mieruMTUError() error {
	return fmt.Errorf("Mieru MTU 必须是 %d–%d 的整数", mieruMinMTU, mieruMaxMTU)
}

func mieruAuthority(server string) (string, error) {
	invalid := errors.New("Mieru 服务器须为不带端口的域名或 IP 地址")
	if strings.Contains(server, ":") {
		if addr, err := netip.ParseAddr(server); err == nil && addr.Is6() && addr.Zone() == "" {
			return "[" + server + "]", nil
		}
		return "", invalid
	}
	if server == "" || strings.ContainsAny(server, "[]") {
		return "", invalid
	}
	parsed, err := url.Parse("mierus://" + server)
	if err != nil || parsed.Host != server || parsed.User != nil {
		return "", invalid
	}
	return server, nil
}

func mieruPort(spec string) (int, string, error) {
	invalid := errors.New("端口必须为 1–65535 的整数或起止范围，例如 443 或 5000-5010")
	start, end, isRange := strings.Cut(strings.TrimSpace(spec), "-")
	first, err := strconv.ParseUint(start, 10, 16)
	if err != nil || first == 0 {
		return 0, "", invalid
	}
	port := strconv.FormatUint(first, 10)
	if isRange {
		last, err := strconv.ParseUint(end, 10, 16)
		if err != nil || last < first {
			return 0, "", invalid
		}
		port += "-" + strconv.FormatUint(last, 10)
	}
	return int(first), port, nil
}

func parseMihomoMieru(proxy map[string]any) (model.Node, error) {
	var p struct {
		Type           string `json:"type"`
		Name           string `json:"name"`
		Server         string `json:"server"`
		Port           int    `json:"port"`
		PortRange      string `json:"port-range"`
		Transport      string `json:"transport"`
		UDP            bool   `json:"udp"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		Multiplexing   string `json:"multiplexing"`
		HandshakeMode  string `json:"handshake-mode"`
		TrafficPattern string `json:"traffic-pattern"`
	}
	if err := decodeMihomo(proxy, &p); err != nil {
		return model.Node{}, err
	}
	if p.Port != 0 && p.PortRange != "" {
		return model.Node{}, errors.New("Mihomo Mieru 的 port 与 port-range 不能同时填写")
	}
	var port string
	if p.PortRange != "" {
		if !strings.Contains(p.PortRange, "-") {
			return model.Node{}, errors.New("Mihomo Mieru 的 port-range 必须为起止范围")
		}
		port = p.PortRange
	} else {
		port = strconv.Itoa(p.Port)
	}
	node := model.Node{
		Name: p.Name, Protocol: "mieru", Server: p.Server, Username: p.Username, Password: p.Password, UDP: p.UDP,
		Mieru: &model.MieruOptions{
			Profile: p.Name, Bindings: []model.MieruBinding{{Port: port, Transport: p.Transport}},
			Multiplexing: p.Multiplexing, HandshakeMode: p.HandshakeMode, TrafficPattern: p.TrafficPattern,
		},
	}
	if err := NormalizeMieru(&node); err != nil {
		return model.Node{}, err
	}
	return node, Validate(node)
}
