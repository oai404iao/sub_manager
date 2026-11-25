package protocol

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/oai404iao/sub_manager/internal/model"
)

const defaultHysteria2Port = 443

func NormalizeHysteria2(node *model.Node) error {
	if node.Protocol != "hysteria2" {
		return nil
	}
	node.TLS, node.UDP = true, true
	if options := node.Hysteria2; options != nil && options.Ports != "" {
		first, ports, err := parseHysteria2Ports(options.Ports)
		if err != nil {
			return err
		}
		node.Port, options.Ports = first, ports
	}
	return nil
}

func parseHysteria2URI(raw string) (model.Node, error) {
	scheme, rest, _ := strings.Cut(raw, "://")
	end := strings.IndexAny(rest, "/?#")
	if end < 0 {
		end = len(rest)
	}
	authority, suffix := rest[:end], rest[end:]
	start := strings.LastIndexByte(authority, '@') + 1
	hostPort := authority[start:]
	port := defaultHysteria2Port
	var ports string
	if sep := strings.LastIndexByte(hostPort, ':'); sep > strings.LastIndexByte(hostPort, ']') {
		host, portSpec := hostPort[:sep], hostPort[sep+1:]
		if !strings.HasPrefix(host, "[") && strings.Contains(host, ":") {
			return model.Node{}, errors.New("Hysteria 2 URI 的 IPv6 地址须使用方括号")
		}
		var err error
		port, ports, err = parseHysteria2Ports(portSpec)
		if err != nil {
			return model.Node{}, err
		}
		if ports == strconv.Itoa(port) {
			ports = ""
		}
		// net/url accepts only a single numeric port in the authority.
		authority = authority[:start] + host + ":" + strconv.Itoa(port)
	}
	parsed, err := url.Parse(scheme + "://" + authority + suffix)
	if err != nil || parsed.Host == "" || (parsed.Path != "" && parsed.EscapedPath() != "/") {
		return model.Node{}, errors.New("Hysteria 2 URI 格式无效")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return model.Node{}, errors.New("Hysteria 2 URI 参数编码无效")
	}
	for key, values := range query {
		switch key {
		case "sni", "insecure", "pinSHA256", "ech", "obfs", "obfs-password":
			if len(values) != 1 {
				return model.Node{}, errors.New("Hysteria 2 URI 参数重复")
			}
		default:
			return model.Node{}, errors.New("不支持的 Hysteria 2 URI 参数")
		}
	}
	if insecure := query.Get("insecure"); query.Has("insecure") && insecure != "0" && insecure != "1" {
		return model.Node{}, errors.New("Hysteria 2 URI 的 insecure 必须为 0 或 1")
	}
	var auth string
	if parsed.User != nil {
		auth = parsed.User.Username()
		if password, ok := parsed.User.Password(); ok {
			auth += ":" + password
		}
	}
	node := model.Node{
		Name: defaultName(parsed.Fragment, parsed.Hostname()), Protocol: "hysteria2",
		Server: parsed.Hostname(), Port: port, Password: auth, TLS: true, UDP: true,
		SNI: query.Get("sni"), AllowInsecure: query.Get("insecure") == "1",
		PinnedPeerCert: query.Get("pinSHA256"), ECHConfigList: query.Get("ech"),
	}
	options := model.Hysteria2Options{Ports: ports, Obfs: query.Get("obfs"), ObfsPassword: query.Get("obfs-password")}
	if options != (model.Hysteria2Options{}) {
		node.Hysteria2 = &options
	}
	return node, Validate(node)
}

func hysteria2URI(node model.Node) (string, error) {
	if err := Validate(node); err != nil {
		return "", err
	}
	port := strconv.Itoa(node.Port)
	query := url.Values{}
	if options := node.Hysteria2; options != nil {
		if options.Ports != "" {
			_, port, _ = parseHysteria2Ports(options.Ports)
		}
		set(query, "obfs", options.Obfs)
		set(query, "obfs-password", options.ObfsPassword)
	}
	set(query, "sni", node.SNI)
	set(query, "pinSHA256", node.PinnedPeerCert)
	set(query, "ech", node.ECHConfigList)
	if node.AllowInsecure {
		query.Set("insecure", "1")
	}
	var user *url.Userinfo
	if username, password, ok := strings.Cut(node.Password, ":"); ok {
		user = url.UserPassword(username, password)
	} else if node.Password != "" {
		user = url.User(node.Password)
	}
	return (&url.URL{
		Scheme: "hysteria2", User: user, Host: net.JoinHostPort(node.Server, port), Path: "/",
		RawQuery: strings.ReplaceAll(query.Encode(), "+", "%20"), Fragment: node.Name,
	}).String(), nil
}

func validateHysteria2(node model.Node) error {
	if strings.Contains(node.Server, ":") {
		addr, err := netip.ParseAddr(node.Server)
		if err != nil || !addr.Is6() || addr.Zone() != "" {
			return errors.New("Hysteria 2 服务器地址无效，请填写域名或不带端口的 IP 地址")
		}
	} else if strings.ContainsAny(node.Server, "/\\?#@[]%") || strings.ContainsFunc(node.Server, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) {
		return errors.New("Hysteria 2 服务器地址无效，请填写域名或 IP 地址")
	}
	if pin := node.PinnedPeerCert; pin != "" {
		fingerprint, err := hex.DecodeString(strings.ReplaceAll(pin, ":", ""))
		if err != nil || len(fingerprint) != 32 {
			return errors.New("Hysteria 2 的 pinSHA256 必须是 32 字节的十六进制证书指纹")
		}
	}
	if node.ECHConfigList != "" {
		if _, err := base64.StdEncoding.DecodeString(node.ECHConfigList); err != nil {
			return errors.New("Hysteria 2 的 ECH 配置必须是标准 Base64")
		}
	}
	if options := node.Hysteria2; options != nil {
		if options.Ports != "" {
			if _, _, err := parseHysteria2Ports(options.Ports); err != nil {
				return err
			}
		}
		switch options.Obfs {
		case "":
			if options.ObfsPassword != "" {
				return errors.New("Hysteria 2 使用混淆密码时必须选择混淆类型")
			}
		case "salamander", "gecko":
			if options.ObfsPassword == "" {
				return errors.New("Hysteria 2 启用混淆时必须填写混淆密码")
			}
		default:
			return errors.New("Hysteria 2 仅支持 Salamander 或 Gecko 混淆")
		}
	}
	return nil
}

// parseHysteria2Ports validates the list without expanding ranges. The first
// port is used for the single-port node summary; the full list goes in the URI.
func parseHysteria2Ports(spec string) (int, string, error) {
	var first int
	var normalized []string
	invalid := errors.New("Hysteria 2 端口须为 1–65535，支持用逗号分隔端口及起止范围")
	for item := range strings.SplitSeq(spec, ",") {
		start, end, isRange := strings.Cut(strings.TrimSpace(item), "-")
		startPort, err := strconv.ParseUint(start, 10, 16)
		if err != nil || startPort == 0 {
			return 0, "", invalid
		}
		if first == 0 {
			first = int(startPort)
		}
		segment := strconv.FormatUint(startPort, 10)
		if isRange {
			endPort, err := strconv.ParseUint(end, 10, 16)
			if err != nil || endPort < startPort {
				return 0, "", invalid
			}
			segment += "-" + strconv.FormatUint(endPort, 10)
		}
		normalized = append(normalized, segment)
	}
	return first, strings.Join(normalized, ","), nil
}

func parseMihomoHysteria2(proxy map[string]any) (model.Node, error) {
	var p struct {
		Type           string `json:"type"`
		Name           string `json:"name"`
		Server         string `json:"server"`
		Port           int    `json:"port"`
		Ports          string `json:"ports"`
		Password       string `json:"password"`
		Obfs           string `json:"obfs"`
		ObfsPassword   string `json:"obfs-password"`
		SNI            string `json:"sni"`
		SkipCertVerify bool   `json:"skip-cert-verify"`
		ECH            *struct {
			Enable bool   `json:"enable"`
			Config string `json:"config"`
		} `json:"ech-opts"`
	}
	if err := decodeMihomo(proxy, &p); err != nil {
		return model.Node{}, err
	}
	node := model.Node{
		Name: p.Name, Protocol: "hysteria2", Server: p.Server, Port: p.Port,
		Password: p.Password, SNI: p.SNI, AllowInsecure: p.SkipCertVerify,
	}
	if p.ECH != nil && p.ECH.Enable {
		if p.ECH.Config == "" {
			return model.Node{}, errors.New("Hysteria 2 URI 无法表达 Mihomo 的动态 ECH，请填写 ech-opts.config")
		}
		node.ECHConfigList = p.ECH.Config
	}
	options := model.Hysteria2Options{Ports: p.Ports, Obfs: p.Obfs, ObfsPassword: p.ObfsPassword}
	if options != (model.Hysteria2Options{}) {
		node.Hysteria2 = &options
	}
	if err := NormalizeHysteria2(&node); err != nil {
		return model.Node{}, err
	}
	return node, Validate(node)
}
