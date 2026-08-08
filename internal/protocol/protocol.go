package protocol

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/oai404iao/sub_manager/internal/model"
)

func ParseText(input string) ([]model.Node, error) {
	input = strings.TrimSpace(strings.TrimPrefix(input, "\uFEFF"))
	if input == "" {
		return nil, errors.New("empty import content")
	}

	candidates := []string{input}
	if decoded, ok := decodeSubscription(input); ok && decoded != input {
		candidates = append(candidates, decoded)
	}

	var parseErrors []string
	for _, content := range candidates {
		content = strings.TrimSpace(strings.TrimPrefix(content, "\uFEFF"))
		if nodes, recognized, err := ParseXrayJSON(content); recognized {
			return nodes, err
		}
		if nodes, recognized, err := parseYAML(content); recognized {
			return nodes, err
		}
		nodes, errorsForContent := parseURIContent(content)
		if len(nodes) > 0 {
			return nodes, nil
		}
		for _, message := range errorsForContent {
			if !containsString(parseErrors, message) {
				parseErrors = append(parseErrors, message)
			}
		}
	}
	if len(parseErrors) > 0 {
		return nil, fmt.Errorf("no supported nodes found: %s", strings.Join(parseErrors, "; "))
	}
	return nil, errors.New("no supported VLESS or SOCKS5 nodes found")
}

func parseURIContent(content string) ([]model.Node, []string) {
	var nodes []model.Node
	var parseErrors []string
	content = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(content)
	for _, rawLine := range strings.Split(content, "\n") {
		line := normalizeImportLine(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := ParseURI(line)
		if err != nil {
			parseErrors = append(parseErrors, err.Error())
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, parseErrors
}

func normalizeImportLine(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "\uFEFF"))
	if strings.HasPrefix(line, "- ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
	}
	if len(line) >= 2 {
		switch {
		case line[0] == '"' && line[len(line)-1] == '"':
			if unquoted, err := strconv.Unquote(line); err == nil {
				line = unquoted
			}
		case line[0] == '\'' && line[len(line)-1] == '\'':
			line = strings.ReplaceAll(line[1:len(line)-1], "''", "'")
		}
	}
	return strings.TrimSpace(line)
}

func ParseURI(raw string) (model.Node, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return model.Node{}, err
	}
	switch strings.ToLower(parsed.Scheme) {
	case "vless":
		return parseVLESS(parsed)
	case "socks", "socks5":
		return parseSOCKS(parsed)
	default:
		return model.Node{}, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
}

func URI(node model.Node) (string, error) {
	switch node.Protocol {
	case "vless":
		return vlessURI(node)
	case "socks5":
		return socksURI(node)
	default:
		return "", fmt.Errorf("unsupported protocol %q", node.Protocol)
	}
}

func Subscription(nodes []model.Node) (string, error) {
	lines := make([]string, 0, len(nodes))
	for _, node := range nodes {
		line, err := URI(node)
		if err != nil {
			return "", err
		}
		lines = append(lines, line)
	}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n"))), nil
}

func FullContent(nodes []model.Node) (string, error) {
	lines := make([]string, 0, len(nodes))
	for _, node := range nodes {
		line, err := URI(node)
		if err != nil {
			return "", err
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}

func parseVLESS(parsed *url.URL) (model.Node, error) {
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return model.Node{}, errors.New("VLESS port is invalid")
	}
	id := ""
	if parsed.User != nil {
		id = parsed.User.Username()
	}
	if id == "" || parsed.Hostname() == "" {
		return model.Node{}, errors.New("VLESS UUID and server are required")
	}
	query := parsed.Query()
	known := map[string]bool{
		"encryption": true, "flow": true, "type": true, "security": true,
		"sni": true, "alpn": true, "fp": true, "allowInsecure": true,
		"pbk": true, "sid": true, "spx": true, "host": true, "path": true,
		"serviceName": true, "authority": true, "headerType": true, "udp": true,
		"mode": true, "mtu": true, "tti": true, "extra": true, "fm": true,
		"ech": true, "pcs": true, "vcn": true, "pqv": true,
	}
	network, networkErr := canonicalTransport(valueOr(query.Get("type"), "tcp"))
	if networkErr != nil {
		return model.Node{}, networkErr
	}
	node := model.Node{
		Name:          defaultName(parsed.Fragment, parsed.Hostname()),
		Protocol:      "vless",
		Server:        parsed.Hostname(),
		Port:          port,
		UUID:          id,
		Encryption:    valueOr(query.Get("encryption"), "none"),
		Flow:          query.Get("flow"),
		Network:       network,
		Security:      valueOr(query.Get("security"), "none"),
		SNI:           query.Get("sni"),
		ALPN:          splitComma(query.Get("alpn")),
		Fingerprint:   query.Get("fp"),
		AllowInsecure: parseBool(query.Get("allowInsecure")),
		PublicKey:     query.Get("pbk"),
		ShortID:       query.Get("sid"),
		SpiderX:       query.Get("spx"),
		Host:          query.Get("host"),
		Path:          query.Get("path"),
		ServiceName:   query.Get("serviceName"),
		Authority:     query.Get("authority"),
		HeaderType:    query.Get("headerType"),
		UDP:           query.Get("udp") == "" || parseBool(query.Get("udp")),
		Extra:         extras(query, known),
	}
	if err := EnsureXrayOutbound(&node); err != nil {
		return model.Node{}, err
	}
	stream, _ := mapFromMap(node.XrayOutbound, "streamSettings")
	switch network {
	case "xhttp":
		settings := ensureChildMap(stream, "xhttpSettings")
		setOrDelete(settings, "mode", query.Get("mode"))
		if encoded := query.Get("extra"); encoded != "" {
			value, err := decodeBase64JSON(encoded)
			if err != nil {
				return model.Node{}, fmt.Errorf("invalid XHTTP extra: %w", err)
			}
			settings["extra"] = value
		}
	case "mkcp":
		settings := ensureChildMap(stream, "kcpSettings")
		if value := query.Get("mtu"); value != "" {
			number, err := strconv.Atoi(value)
			if err != nil {
				return model.Node{}, errors.New("invalid mKCP mtu")
			}
			settings["mtu"] = number
		}
		if value := query.Get("tti"); value != "" {
			number, err := strconv.Atoi(value)
			if err != nil {
				return model.Node{}, errors.New("invalid mKCP tti")
			}
			settings["tti"] = number
		}
	case "grpc":
		settings := ensureChildMap(stream, "grpcSettings")
		if query.Get("mode") == "multi" {
			settings["multiMode"] = true
		}
	}
	switch node.Security {
	case "tls":
		settings := ensureChildMap(stream, "tlsSettings")
		setOrDelete(settings, "echConfigList", query.Get("ech"))
		setOrDelete(settings, "pinnedPeerCertSha256", query.Get("pcs"))
		setOrDelete(settings, "verifyPeerCertByName", query.Get("vcn"))
		if node.AllowInsecure {
			settings["allowInsecure"] = true
		}
	case "reality":
		settings := ensureChildMap(stream, "realitySettings")
		setOrDelete(settings, "mldsa65Verify", query.Get("pqv"))
	}
	if encoded := query.Get("fm"); encoded != "" {
		value, err := decodeBase64JSON(encoded)
		if err != nil {
			return model.Node{}, fmt.Errorf("invalid FinalMask: %w", err)
		}
		stream["finalmask"] = value
	}
	node.XrayOutbound["streamSettings"] = stream
	applyStreamSettings(&node, stream)
	return node, Validate(node)
}

func parseSOCKS(parsed *url.URL) (model.Node, error) {
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return model.Node{}, errors.New("SOCKS5 port is invalid")
	}
	query := parsed.Query()
	username, password := "", ""
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	known := map[string]bool{
		"udp": true, "tls": true, "sni": true, "allowInsecure": true,
	}
	node := model.Node{
		Name:          defaultName(parsed.Fragment, parsed.Hostname()),
		Protocol:      "socks5",
		Server:        parsed.Hostname(),
		Port:          port,
		Username:      username,
		Password:      password,
		UDP:           parseBool(query.Get("udp")),
		TLS:           parseBool(query.Get("tls")),
		SNI:           query.Get("sni"),
		AllowInsecure: parseBool(query.Get("allowInsecure")),
		Extra:         extras(query, known),
	}
	return node, Validate(node)
}

func vlessURI(node model.Node) (string, error) {
	outbound, err := buildXrayOutbound(node)
	if err != nil {
		return "", err
	}
	stream, _ := mapFromMap(outbound, "streamSettings")
	method, err := canonicalTransport(stringFromMap(stream, "method"))
	if err != nil {
		return "", err
	}
	security := strings.ToLower(valueOr(stringFromMap(stream, "security"), "none"))
	query := url.Values{}
	query.Set("encryption", valueOr(node.Encryption, "none"))
	set(query, "flow", node.Flow)
	query.Set("type", uriTransport(method))
	query.Set("security", security)

	switch method {
	case "raw":
		settings, _ := firstMap(stream, "rawSettings", "tcpSettings")
		if header, ok := mapFromMap(settings, "header"); ok {
			set(query, "headerType", stringFromMap(header, "type"))
		}
	case "xhttp":
		settings, _ := firstMap(stream, "xhttpSettings", "splithttpSettings")
		set(query, "host", stringFromMap(settings, "host"))
		set(query, "path", stringFromMap(settings, "path"))
		set(query, "mode", stringFromMap(settings, "mode"))
		if extra, ok := settings["extra"]; ok && extra != nil {
			encoded, err := encodeBase64JSON(extra)
			if err != nil {
				return "", err
			}
			set(query, "extra", encoded)
		}
	case "mkcp":
		settings, _ := mapFromMap(stream, "kcpSettings")
		if value := intFromMap(settings, "mtu"); value != 0 {
			query.Set("mtu", strconv.Itoa(value))
		}
		if value := intFromMap(settings, "tti"); value != 0 {
			query.Set("tti", strconv.Itoa(value))
		}
	case "grpc":
		settings, _ := mapFromMap(stream, "grpcSettings")
		set(query, "serviceName", stringFromMap(settings, "serviceName"))
		set(query, "authority", stringFromMap(settings, "authority"))
		if boolFromMap(settings, "multiMode") {
			query.Set("mode", "multi")
		}
	case "websocket":
		settings, _ := mapFromMap(stream, "wsSettings")
		set(query, "host", stringFromMap(settings, "host"))
		set(query, "path", stringFromMap(settings, "path"))
	case "httpupgrade":
		settings, _ := mapFromMap(stream, "httpupgradeSettings")
		set(query, "host", stringFromMap(settings, "host"))
		set(query, "path", stringFromMap(settings, "path"))
	}

	switch security {
	case "tls":
		settings, _ := mapFromMap(stream, "tlsSettings")
		set(query, "sni", stringFromMap(settings, "serverName"))
		set(query, "fp", stringFromMap(settings, "fingerprint"))
		if alpn := stringSliceFromMap(settings, "alpn"); len(alpn) > 0 {
			query.Set("alpn", strings.Join(alpn, ","))
		}
		set(query, "ech", stringFromMap(settings, "echConfigList"))
		set(query, "pcs", stringFromMap(settings, "pinnedPeerCertSha256"))
		set(query, "vcn", stringFromMap(settings, "verifyPeerCertByName"))
	case "reality":
		settings, _ := mapFromMap(stream, "realitySettings")
		set(query, "sni", stringFromMap(settings, "serverName"))
		set(query, "fp", stringFromMap(settings, "fingerprint"))
		set(query, "pbk", valueOr(stringFromMap(settings, "password"), stringFromMap(settings, "publicKey")))
		set(query, "sid", stringFromMap(settings, "shortId"))
		set(query, "pqv", stringFromMap(settings, "mldsa65Verify"))
		set(query, "spx", stringFromMap(settings, "spiderX"))
	}
	if finalmask, ok := stream["finalmask"]; ok && finalmask != nil {
		encoded, err := encodeBase64JSON(finalmask)
		if err != nil {
			return "", err
		}
		set(query, "fm", encoded)
	}
	for key, value := range node.Extra {
		if !query.Has(key) {
			query.Set(key, value)
		}
	}
	uriID, err := VLESSUUID(node.UUID)
	if err != nil {
		return "", err
	}
	return (&url.URL{
		Scheme:   "vless",
		User:     url.User(uriID),
		Host:     net.JoinHostPort(node.Server, strconv.Itoa(node.Port)),
		RawQuery: query.Encode(),
		Fragment: node.Name,
	}).String(), nil
}

func socksURI(node model.Node) (string, error) {
	if err := Validate(node); err != nil {
		return "", err
	}
	query := url.Values{}
	if node.UDP {
		query.Set("udp", "true")
	}
	if node.TLS {
		query.Set("tls", "true")
	}
	set(query, "sni", node.SNI)
	if node.AllowInsecure {
		query.Set("allowInsecure", "true")
	}
	for key, value := range node.Extra {
		if !query.Has(key) {
			query.Set(key, value)
		}
	}
	var user *url.Userinfo
	if node.Username != "" {
		user = url.UserPassword(node.Username, node.Password)
	}
	return (&url.URL{
		Scheme:   "socks5",
		User:     user,
		Host:     net.JoinHostPort(node.Server, strconv.Itoa(node.Port)),
		RawQuery: query.Encode(),
		Fragment: node.Name,
	}).String(), nil
}

func Validate(node model.Node) error {
	if strings.TrimSpace(node.Name) == "" {
		return errors.New("node name is required")
	}
	if strings.TrimSpace(node.Server) == "" {
		return errors.New("server is required")
	}
	if node.Port < 1 || node.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	switch node.Protocol {
	case "vless":
		if err := validateVLESS(node); err != nil {
			return err
		}
	case "socks5":
	default:
		return errors.New("protocol must be vless or socks5")
	}
	return nil
}

func parseYAML(input string) ([]model.Node, bool, error) {
	var document any
	if err := yaml.Unmarshal([]byte(input), &document); err != nil {
		return nil, false, nil
	}
	document = normalizeYAML(document)
	switch document.(type) {
	case map[string]any, []any:
	default:
		return nil, false, nil
	}

	if content, recognized := yamlURIContent(document); recognized {
		nodes, parseErrors := parseURIContent(content)
		if len(nodes) > 0 {
			return nodes, true, nil
		}
		if len(parseErrors) > 0 {
			return nil, true, fmt.Errorf("invalid YAML node URIs: %s", strings.Join(parseErrors, "; "))
		}
		return nil, true, errors.New("YAML node URI list is empty")
	}

	if looksLikeXrayDocument(document) {
		data, err := json.Marshal(document)
		if err != nil {
			return nil, true, fmt.Errorf("encode Xray YAML: %w", err)
		}
		nodes, recognized, err := ParseXrayJSON(string(data))
		if recognized {
			return nodes, true, err
		}
	}
	if proxies, recognized := mihomoProxyMaps(document); recognized {
		return parseMihomoProxies(proxies)
	}
	return nil, false, nil
}

func parseMihomoProxies(proxies []map[string]any) ([]model.Node, bool, error) {
	nodes := make([]model.Node, 0, len(proxies))
	var validationErrors []string
	for _, proxy := range proxies {
		kind := lowerString(proxy["type"])
		switch kind {
		case "vless":
			node := model.Node{
				Name:          stringValue(proxy["name"]),
				Protocol:      "vless",
				Server:        stringValue(proxy["server"]),
				Port:          intValue(proxy["port"]),
				UUID:          stringValue(proxy["uuid"]),
				Flow:          stringValue(proxy["flow"]),
				Encryption:    "none",
				Network:       valueOr(stringValue(proxy["network"]), "raw"),
				Security:      "none",
				SNI:           stringValue(proxy["servername"]),
				Fingerprint:   stringValue(proxy["client-fingerprint"]),
				AllowInsecure: boolValue(proxy["skip-cert-verify"]),
				UDP:           boolDefault(proxy["udp"], true),
				Extra:         map[string]string{},
			}
			if boolValue(proxy["tls"]) {
				node.Security = "tls"
			}
			if reality, ok := proxy["reality-opts"].(map[string]any); ok {
				node.Security = "reality"
				node.PublicKey = stringValue(reality["public-key"])
				node.ShortID = stringValue(reality["short-id"])
			}
			if options, ok := proxy["ws-opts"].(map[string]any); ok {
				node.Path = stringValue(options["path"])
				if headers, ok := options["headers"].(map[string]any); ok {
					node.Host = stringValue(headers["Host"])
					if node.Host == "" {
						node.Host = stringValue(headers["host"])
					}
				}
			}
			if options, ok := proxy["grpc-opts"].(map[string]any); ok {
				node.ServiceName = stringValue(options["grpc-service-name"])
			}
			if canonical, err := canonicalTransport(node.Network); err == nil {
				node.Network = canonical
			}
			if err := EnsureXrayOutbound(&node); err != nil {
				validationErrors = append(validationErrors, node.Name+": "+err.Error())
				continue
			}
			if err := Validate(node); err != nil {
				validationErrors = append(validationErrors, node.Name+": "+err.Error())
				continue
			}
			nodes = append(nodes, node)
		case "socks5", "socks":
			node := model.Node{
				Name:          stringValue(proxy["name"]),
				Protocol:      "socks5",
				Server:        stringValue(proxy["server"]),
				Port:          intValue(proxy["port"]),
				Username:      stringValue(proxy["username"]),
				Password:      stringValue(proxy["password"]),
				UDP:           boolValue(proxy["udp"]),
				TLS:           boolValue(proxy["tls"]),
				SNI:           stringValue(proxy["servername"]),
				AllowInsecure: boolValue(proxy["skip-cert-verify"]),
				Extra:         map[string]string{},
			}
			if err := Validate(node); err != nil {
				validationErrors = append(validationErrors, node.Name+": "+err.Error())
				continue
			}
			nodes = append(nodes, node)
		}
	}
	if len(validationErrors) > 0 {
		return nil, true, fmt.Errorf("invalid Mihomo nodes: %s", strings.Join(validationErrors, "; "))
	}
	if len(nodes) == 0 {
		return nil, true, errors.New("Mihomo YAML does not contain supported VLESS or SOCKS5 proxies")
	}
	return nodes, true, nil
}

func normalizeYAML(value any) any {
	switch value := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, item := range value {
			result[key] = normalizeYAML(item)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(value))
		for key, item := range value {
			result[fmt.Sprint(key)] = normalizeYAML(item)
		}
		return result
	case []any:
		result := make([]any, len(value))
		for index, item := range value {
			result[index] = normalizeYAML(item)
		}
		return result
	default:
		return value
	}
}

func yamlURIContent(document any) (string, bool) {
	var values []string
	switch document := document.(type) {
	case []any:
		values = appendYAMLStrings(values, document)
	case map[string]any:
		for _, key := range []string{"nodes", "uris", "links", "proxies"} {
			value, ok := document[key]
			if !ok {
				continue
			}
			switch value := value.(type) {
			case string:
				values = append(values, value)
			case []any:
				values = appendYAMLStrings(values, value)
			case map[string]any:
				for _, item := range value {
					if text, ok := item.(string); ok {
						values = append(values, text)
					}
				}
			}
		}
	}
	if len(values) == 0 {
		return "", false
	}
	return strings.Join(values, "\n"), true
}

func appendYAMLStrings(target []string, values []any) []string {
	for _, value := range values {
		if text, ok := value.(string); ok {
			target = append(target, text)
		}
	}
	return target
}

func mihomoProxyMaps(document any) ([]map[string]any, bool) {
	var values []any
	containerRecognized := false
	switch document := document.(type) {
	case map[string]any:
		if proxyValues, ok := document["proxies"].([]any); ok {
			values = proxyValues
			containerRecognized = true
		} else if _, hasType := document["type"]; hasType {
			values = []any{document}
			containerRecognized = true
		} else {
			return nil, false
		}
	case []any:
		values = document
	default:
		return nil, false
	}

	proxies := make([]map[string]any, 0, len(values))
	recognized := false
	for _, value := range values {
		proxy, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if _, hasType := proxy["type"]; hasType {
			recognized = true
		}
		proxies = append(proxies, proxy)
	}
	if len(proxies) == 0 {
		return nil, containerRecognized
	}
	return proxies, containerRecognized || recognized
}

func looksLikeXrayDocument(document any) bool {
	switch document := document.(type) {
	case map[string]any:
		_, hasOutbounds := document["outbounds"]
		_, hasProtocol := document["protocol"]
		return hasOutbounds || hasProtocol
	case []any:
		for _, value := range document {
			if outbound, ok := value.(map[string]any); ok {
				if _, hasProtocol := outbound["protocol"]; hasProtocol {
					return true
				}
			}
		}
	}
	return false
}

func decodeSubscription(input string) (string, bool) {
	payload := strings.TrimSpace(input)
	lower := strings.ToLower(payload)
	switch {
	case strings.HasPrefix(lower, "data:"):
		comma := strings.IndexByte(payload, ',')
		if comma < 0 || !strings.Contains(strings.ToLower(payload[:comma]), ";base64") {
			return input, false
		}
		payload = payload[comma+1:]
	case strings.HasPrefix(lower, "base64://"):
		payload = payload[len("base64://"):]
	case strings.Contains(payload, "://"):
		return input, false
	}
	compact := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, payload)
	encodings := []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(compact); err == nil && utf8.Valid(decoded) {
			content := strings.TrimSpace(strings.TrimPrefix(string(decoded), "\uFEFF"))
			if looksLikeImportContent(content) {
				return content, true
			}
		}
	}
	return input, false
}

func looksLikeImportContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "vless://") ||
		strings.Contains(lower, "socks://") ||
		strings.Contains(lower, "socks5://") {
		return true
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "-") {
		return true
	}
	for _, marker := range []string{"proxies:", "outbounds:", "nodes:", "uris:", "links:"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func extras(values url.Values, known map[string]bool) map[string]string {
	extra := map[string]string{}
	for key := range values {
		if !known[key] {
			extra[key] = values.Get(key)
		}
	}
	return extra
}

func set(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func splitComma(value string) []string {
	if value == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func parseBool(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func defaultName(name, server string) string {
	if name != "" {
		return name
	}
	return server
}

func valueOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func lowerString(value any) string {
	return strings.ToLower(stringValue(value))
}

func stringValue(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return ""
	}
}

func intValue(value any) int {
	switch value := value.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		result, _ := strconv.Atoi(value)
		return result
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch value := value.(type) {
	case bool:
		return value
	case string:
		return parseBool(value)
	default:
		return false
	}
}

func boolDefault(value any, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return boolValue(value)
}

func JSON(node model.Node) string {
	value, _ := json.Marshal(node)
	return string(value)
}

func uriTransport(method string) string {
	switch method {
	case "raw":
		return "tcp"
	case "mkcp":
		return "kcp"
	case "websocket":
		return "ws"
	default:
		return method
	}
}

func encodeBase64JSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeBase64JSON(value string) (any, error) {
	var data []byte
	var err error
	for _, encoding := range []*base64.Encoding{
		base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding,
	} {
		if data, err = encoding.DecodeString(value); err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
