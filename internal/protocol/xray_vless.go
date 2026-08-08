package protocol

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"submanager/internal/model"
)

const (
	XrayCoreVersion = "v26.7.28"
	XrayCoreCommit  = "5ca6f4b7d4dc20a881d4330e498892697627ec0c"
)

var currentTransports = map[string]string{
	"":            "raw",
	"raw":         "raw",
	"tcp":         "raw",
	"xhttp":       "xhttp",
	"splithttp":   "xhttp",
	"mkcp":        "mkcp",
	"kcp":         "mkcp",
	"grpc":        "grpc",
	"websocket":   "websocket",
	"ws":          "websocket",
	"httpupgrade": "httpupgrade",
	"hysteria":    "hysteria",
}

var removedTransports = map[string]string{
	"http": "XHTTP stream-one",
	"h2":   "XHTTP stream-one",
	"h3":   "XHTTP stream-one",
	"quic": "XHTTP stream-one H3",
}

var xrayPrivatePrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/3"),
	netip.MustParsePrefix("::/127"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

var xrayPrivateDomains = []string{
	"lan", "localdomain", "example", "invalid", "localhost", "test", "local",
	"home.arpa", "internal",
}

var commonFingerprints = map[string]bool{
	"": true, "chrome": true, "firefox": true, "safari": true, "ios": true,
	"android": true, "edge": true, "360": true, "qq": true, "random": true,
	"randomized": true, "randomizednoalpn": true, "unsafe": true,
	"hellofirefox_120": true, "hellofirefox_148": true,
	"hellochrome_120": true, "hellochrome_131": true, "hellochrome_133": true,
	"helloios_13": true, "helloios_14": true, "helloedge_106": true,
	"hellosafari_26_3": true, "hello360_11_0": true, "helloqq_11_1": true,
	"hellogolang": true, "hellorandomized": true, "hellorandomizedalpn": true,
	"hellorandomizednoalpn": true, "hellofirefox_auto": true,
	"hellofirefox_55": true, "hellofirefox_56": true, "hellofirefox_63": true,
	"hellofirefox_65": true, "hellofirefox_99": true, "hellofirefox_102": true,
	"hellofirefox_105": true, "hellochrome_auto": true, "hellochrome_58": true,
	"hellochrome_62": true, "hellochrome_70": true, "hellochrome_72": true,
	"hellochrome_83": true, "hellochrome_87": true, "hellochrome_96": true,
	"hellochrome_100": true, "hellochrome_102": true,
	"hellochrome_106_shuffle": true, "helloios_auto": true,
	"helloios_11_1": true, "helloios_12_1": true,
	"helloandroid_11_okhttp": true, "helloedge_85": true,
	"helloedge_auto": true, "hellosafari_16_0": true,
	"hellosafari_auto": true, "hello360_auto": true, "hello360_7_5": true,
	"helloqq_auto": true, "hellochrome_100_psk": true,
	"hellochrome_112_psk_shuf": true, "hellochrome_114_padding_psk_shuf": true,
	"hellochrome_115_pq": true, "hellochrome_115_pq_psk": true,
	"hellochrome_120_pq": true,
}

// ParseXrayJSON accepts a full Xray configuration, a single outbound object,
// or an array of outbound objects. It returns recognized VLESS outbounds.
func ParseXrayJSON(input string) ([]model.Node, bool, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil, false, nil
	}
	var document any
	if err := json.Unmarshal([]byte(trimmed), &document); err != nil {
		return nil, false, nil
	}

	var outbounds []map[string]any
	switch value := document.(type) {
	case map[string]any:
		if items, ok := value["outbounds"].([]any); ok {
			for _, item := range items {
				if outbound, ok := item.(map[string]any); ok {
					outbounds = append(outbounds, outbound)
				}
			}
		} else if _, ok := value["protocol"]; ok {
			outbounds = append(outbounds, value)
		} else {
			return nil, true, errors.New("Xray JSON does not contain an outbound or outbounds array")
		}
	case []any:
		for _, item := range value {
			if outbound, ok := item.(map[string]any); ok {
				outbounds = append(outbounds, outbound)
			}
		}
	default:
		return nil, true, errors.New("Xray JSON root must be an object or array")
	}

	nodes := make([]model.Node, 0, len(outbounds))
	for index, outbound := range outbounds {
		if !strings.EqualFold(stringFromMap(outbound, "protocol"), "vless") {
			continue
		}
		node, err := NodeFromXrayOutbound(outbound)
		if err != nil {
			return nil, true, fmt.Errorf("Xray VLESS outbound %d: %w", index+1, err)
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, true, errors.New("Xray JSON does not contain a VLESS outbound")
	}
	return nodes, true, nil
}

func NodeFromXrayOutbound(outbound map[string]any) (model.Node, error) {
	if !strings.EqualFold(stringFromMap(outbound, "protocol"), "vless") {
		return model.Node{}, errors.New(`outbound "protocol" must be "vless"`)
	}
	settings, ok := mapFromMap(outbound, "settings")
	if !ok {
		return model.Node{}, errors.New(`outbound "settings" must be an object`)
	}

	var address, id, encryption, flow, email string
	var port int
	var level int
	var reverse any
	if _, simplified := settings["address"]; simplified {
		address = stringFromMap(settings, "address")
		port = intFromMap(settings, "port")
		id = stringFromMap(settings, "id")
		encryption = stringFromMap(settings, "encryption")
		flow = stringFromMap(settings, "flow")
		email = stringFromMap(settings, "email")
		level = intFromMap(settings, "level")
		reverse = settings["reverse"]
	} else {
		vnext, ok := sliceFromMap(settings, "vnext")
		if !ok || len(vnext) != 1 {
			return model.Node{}, errors.New(`legacy settings.vnext must contain exactly one endpoint`)
		}
		endpoint, ok := vnext[0].(map[string]any)
		if !ok {
			return model.Node{}, errors.New(`settings.vnext[0] must be an object`)
		}
		users, ok := sliceFromMap(endpoint, "users")
		if !ok || len(users) != 1 {
			return model.Node{}, errors.New(`legacy settings.vnext[0].users must contain exactly one user`)
		}
		user, ok := users[0].(map[string]any)
		if !ok {
			return model.Node{}, errors.New(`settings.vnext[0].users[0] must be an object`)
		}
		address = stringFromMap(endpoint, "address")
		port = intFromMap(endpoint, "port")
		id = stringFromMap(user, "id")
		encryption = stringFromMap(user, "encryption")
		flow = stringFromMap(user, "flow")
		email = stringFromMap(user, "email")
		level = intFromMap(user, "level")
	}

	node := model.Node{
		Name:         valueOr(stringFromMap(outbound, "tag"), address),
		Protocol:     "vless",
		Server:       address,
		Port:         port,
		UUID:         id,
		Encryption:   encryption,
		Flow:         flow,
		Network:      "raw",
		Security:     "none",
		UDP:          true,
		Extra:        map[string]string{},
		XrayOutbound: cloneMap(outbound),
	}
	if node.Encryption == "" {
		return model.Node{}, errors.New(`VLESS settings.encryption is required; use "none" to disable`)
	}
	if stream, ok := mapFromMap(outbound, "streamSettings"); ok {
		applyStreamSettings(&node, stream)
	}

	// Keep current simplified VLESS settings in the canonical outbound while
	// preserving every unrelated/advanced field.
	canonical, err := buildXrayOutbound(node)
	if err != nil {
		return model.Node{}, err
	}
	canonicalSettings, _ := mapFromMap(canonical, "settings")
	if email != "" {
		canonicalSettings["email"] = email
	}
	if level != 0 {
		canonicalSettings["level"] = level
	}
	if reverse != nil {
		canonicalSettings["reverse"] = reverse
	}
	node.XrayOutbound = canonical

	if err := Validate(node); err != nil {
		return model.Node{}, err
	}
	return node, nil
}

// ApplyXrayOutbound makes the exact Xray outbound object authoritative while
// preserving database metadata and group membership on node.
func ApplyXrayOutbound(node *model.Node) error {
	if node == nil || node.Protocol != "vless" || node.XrayOutbound == nil {
		return nil
	}
	parsed, err := NodeFromXrayOutbound(node.XrayOutbound)
	if err != nil {
		return err
	}
	parsed.ID = node.ID
	parsed.GroupIDs = node.GroupIDs
	parsed.SubscriptionID = node.SubscriptionID
	parsed.CreatedAt = node.CreatedAt
	parsed.UpdatedAt = node.UpdatedAt
	if node.Name != "" && stringFromMap(node.XrayOutbound, "tag") == "" {
		parsed.Name = node.Name
	}
	*node = parsed
	return nil
}

func EnsureXrayOutbound(node *model.Node) error {
	if node == nil || node.Protocol != "vless" {
		return nil
	}
	outbound, err := buildXrayOutbound(*node)
	if err != nil {
		return err
	}
	node.XrayOutbound = outbound
	return nil
}

func XrayOutboundJSON(node model.Node) ([]byte, error) {
	outbound, err := buildXrayOutbound(node)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(outbound, "", "  ")
}

func buildXrayOutbound(node model.Node) (map[string]any, error) {
	if node.Protocol != "vless" {
		return nil, errors.New("Xray export is currently available only for VLESS nodes")
	}
	if err := validateVLESS(node); err != nil {
		return nil, err
	}
	outbound := cloneMap(node.XrayOutbound)
	if outbound == nil {
		outbound = map[string]any{}
	}
	outbound["protocol"] = "vless"
	if _, exists := outbound["tag"]; !exists && node.Name != "" {
		outbound["tag"] = node.Name
	}

	settings, _ := mapFromMap(outbound, "settings")
	if settings == nil {
		settings = map[string]any{}
	}
	delete(settings, "vnext")
	settings["address"] = node.Server
	settings["port"] = node.Port
	settings["id"] = node.UUID
	settings["encryption"] = valueOr(node.Encryption, "none")
	if node.Flow != "" {
		settings["flow"] = node.Flow
	} else {
		delete(settings, "flow")
	}
	outbound["settings"] = settings

	stream, _ := mapFromMap(outbound, "streamSettings")
	if stream == nil {
		stream = map[string]any{}
	}
	method, err := canonicalTransport(node.Network)
	if err != nil {
		return nil, err
	}
	stream["method"] = method
	delete(stream, "network")
	security := strings.ToLower(valueOr(node.Security, "none"))
	stream["security"] = security

	switch method {
	case "raw":
		raw := ensureChildMap(stream, "rawSettings")
		if node.HeaderType != "" {
			header := ensureChildMap(raw, "header")
			header["type"] = node.HeaderType
		}
	case "xhttp":
		xhttp := ensureChildMap(stream, "xhttpSettings")
		setOrDelete(xhttp, "host", node.Host)
		setOrDelete(xhttp, "path", node.Path)
		setOrDelete(xhttp, "mode", node.XHTTPMode)
		if node.XHTTPExtra != nil {
			xhttp["extra"] = node.XHTTPExtra
		}
	case "mkcp":
		kcp := ensureChildMap(stream, "kcpSettings")
		if node.KCPMTU != 0 {
			kcp["mtu"] = node.KCPMTU
		}
		if node.KCPTTI != 0 {
			kcp["tti"] = node.KCPTTI
		}
	case "websocket":
		ws := ensureChildMap(stream, "wsSettings")
		setOrDelete(ws, "host", node.Host)
		setOrDelete(ws, "path", node.Path)
	case "httpupgrade":
		upgrade := ensureChildMap(stream, "httpupgradeSettings")
		setOrDelete(upgrade, "host", node.Host)
		setOrDelete(upgrade, "path", node.Path)
	case "grpc":
		grpc := ensureChildMap(stream, "grpcSettings")
		setOrDelete(grpc, "serviceName", node.ServiceName)
		setOrDelete(grpc, "authority", node.Authority)
		if node.GRPCMultiMode {
			grpc["multiMode"] = true
		}
	}

	switch security {
	case "tls":
		tls := ensureChildMap(stream, "tlsSettings")
		setOrDelete(tls, "serverName", node.SNI)
		setOrDelete(tls, "fingerprint", node.Fingerprint)
		if len(node.ALPN) > 0 {
			tls["alpn"] = stringSliceAny(node.ALPN)
		} else {
			delete(tls, "alpn")
		}
		setOrDelete(tls, "echConfigList", node.ECHConfigList)
		setOrDelete(tls, "pinnedPeerCertSha256", node.PinnedPeerCert)
		setOrDelete(tls, "verifyPeerCertByName", node.VerifyPeerName)
		// Xray-core v26.7.28 removed allowInsecure.
		delete(tls, "allowInsecure")
	case "reality":
		reality := ensureChildMap(stream, "realitySettings")
		setOrDelete(reality, "serverName", node.SNI)
		setOrDelete(reality, "fingerprint", node.Fingerprint)
		setOrDelete(reality, "password", node.PublicKey)
		delete(reality, "publicKey")
		setOrDelete(reality, "shortId", node.ShortID)
		setOrDelete(reality, "spiderX", node.SpiderX)
		setOrDelete(reality, "mldsa65Verify", node.MLDSA65Verify)
	}
	if node.FinalMask != nil {
		stream["finalmask"] = node.FinalMask
	}
	outbound["streamSettings"] = stream
	candidate := node
	candidate.XrayOutbound = outbound
	if err := validateVLESS(candidate); err != nil {
		return nil, err
	}
	return outbound, nil
}

func applyStreamSettings(node *model.Node, stream map[string]any) {
	method := stringFromMap(stream, "method")
	if method == "" {
		method = stringFromMap(stream, "network")
	}
	if canonical, err := canonicalTransport(method); err == nil {
		node.Network = canonical
	} else {
		node.Network = strings.ToLower(method)
	}
	node.Security = strings.ToLower(valueOr(stringFromMap(stream, "security"), "none"))

	switch node.Network {
	case "raw":
		settings, _ := firstMap(stream, "rawSettings", "tcpSettings")
		if header, ok := mapFromMap(settings, "header"); ok {
			node.HeaderType = stringFromMap(header, "type")
		}
	case "xhttp":
		settings, _ := firstMap(stream, "xhttpSettings", "splithttpSettings")
		node.Host = stringFromMap(settings, "host")
		node.Path = stringFromMap(settings, "path")
		node.XHTTPMode = stringFromMap(settings, "mode")
		if extra, ok := settings["extra"].(map[string]any); ok {
			node.XHTTPExtra = cloneMap(extra)
		}
	case "mkcp":
		settings, _ := mapFromMap(stream, "kcpSettings")
		node.KCPMTU = intFromMap(settings, "mtu")
		node.KCPTTI = intFromMap(settings, "tti")
	case "websocket":
		settings, _ := mapFromMap(stream, "wsSettings")
		node.Host = stringFromMap(settings, "host")
		node.Path = stringFromMap(settings, "path")
		if node.Host == "" {
			if headers, ok := mapFromMap(settings, "headers"); ok {
				node.Host = headerValue(headers, "host")
			}
		}
	case "httpupgrade":
		settings, _ := mapFromMap(stream, "httpupgradeSettings")
		node.Host = stringFromMap(settings, "host")
		node.Path = stringFromMap(settings, "path")
	case "grpc":
		settings, _ := mapFromMap(stream, "grpcSettings")
		node.ServiceName = stringFromMap(settings, "serviceName")
		node.Authority = stringFromMap(settings, "authority")
		node.GRPCMultiMode = boolFromMap(settings, "multiMode")
	}

	switch node.Security {
	case "tls":
		settings, _ := mapFromMap(stream, "tlsSettings")
		node.SNI = stringFromMap(settings, "serverName")
		node.Fingerprint = strings.ToLower(stringFromMap(settings, "fingerprint"))
		node.ALPN = stringSliceFromMap(settings, "alpn")
		node.AllowInsecure = boolFromMap(settings, "allowInsecure")
		node.ECHConfigList = stringFromMap(settings, "echConfigList")
		node.PinnedPeerCert = stringFromMap(settings, "pinnedPeerCertSha256")
		node.VerifyPeerName = stringFromMap(settings, "verifyPeerCertByName")
	case "reality":
		settings, _ := mapFromMap(stream, "realitySettings")
		node.SNI = stringFromMap(settings, "serverName")
		node.Fingerprint = strings.ToLower(stringFromMap(settings, "fingerprint"))
		node.PublicKey = valueOr(stringFromMap(settings, "password"), stringFromMap(settings, "publicKey"))
		node.ShortID = stringFromMap(settings, "shortId")
		node.SpiderX = stringFromMap(settings, "spiderX")
		node.MLDSA65Verify = stringFromMap(settings, "mldsa65Verify")
	}
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		node.FinalMask = cloneMap(finalmask)
	}
}

func validateVLESS(node model.Node) error {
	if _, err := VLESSUUID(node.UUID); err != nil {
		return err
	}
	switch node.Flow {
	case "", "xtls-rprx-vision", "xtls-rprx-vision-udp443":
	default:
		return fmt.Errorf("unsupported VLESS flow %q in Xray-core %s", node.Flow, XrayCoreVersion)
	}
	if err := ValidateVLESSEncryption(valueOr(node.Encryption, "none")); err != nil {
		return err
	}
	method, err := canonicalTransport(node.Network)
	if err != nil {
		return err
	}
	security := strings.ToLower(valueOr(node.Security, "none"))
	switch security {
	case "none", "tls", "reality":
	default:
		return fmt.Errorf("unsupported Xray transport security %q", security)
	}
	if security == "reality" && method != "raw" && method != "xhttp" && method != "grpc" {
		return errors.New("REALITY supports only RAW, XHTTP, and gRPC")
	}
	if method == "hysteria" && security != "tls" {
		return errors.New("Hysteria transport requires TLS")
	}
	if node.AllowInsecure {
		return errors.New(`Xray-core v26.7.28 removed "allowInsecure"; use pinnedPeerCertSha256 and verifyPeerCertByName`)
	}
	if security == "none" && valueOr(node.Encryption, "none") == "none" && requiresTransportSecurity(node.Server) {
		return errors.New("VLESS to a public server requires TLS, REALITY, or VLESS Encryption")
	}
	var stream map[string]any
	if node.XrayOutbound != nil {
		stream, _ = mapFromMap(node.XrayOutbound, "streamSettings")
	}
	if security == "tls" {
		tlsSettings, _ := mapFromMap(stream, "tlsSettings")
		if tlsSettings == nil {
			tlsSettings = map[string]any{
				"fingerprint":          node.Fingerprint,
				"pinnedPeerCertSha256": node.PinnedPeerCert,
			}
		}
		if boolFromMap(tlsSettings, "allowInsecure") {
			return errors.New(`Xray-core v26.7.28 removed tlsSettings.allowInsecure`)
		}
		if err := validatePinnedCerts(valueOr(stringFromMap(tlsSettings, "pinnedPeerCertSha256"), node.PinnedPeerCert)); err != nil {
			return err
		}
		fingerprint := strings.ToLower(valueOr(stringFromMap(tlsSettings, "fingerprint"), node.Fingerprint))
		if !commonFingerprints[fingerprint] {
			return fmt.Errorf("unknown TLS fingerprint %q", fingerprint)
		}
	}
	if security == "reality" {
		reality, _ := mapFromMap(stream, "realitySettings")
		if reality == nil {
			reality = map[string]any{
				"serverName":    node.SNI,
				"fingerprint":   node.Fingerprint,
				"password":      node.PublicKey,
				"shortId":       node.ShortID,
				"mldsa65Verify": node.MLDSA65Verify,
				"spiderX":       node.SpiderX,
			}
		}
		if err := validateRealityClient(reality); err != nil {
			return err
		}
	}
	if err := validateTransportSettings(method, stream); err != nil {
		return err
	}
	if node.XrayOutbound != nil {
		if err := validateMux(outboundMap(node.XrayOutbound, "mux")); err != nil {
			return err
		}
	}
	return nil
}

func ValidateVLESSEncryption(value string) error {
	if value == "none" {
		return nil
	}
	blocks := strings.Split(value, ".")
	if len(blocks) < 4 || blocks[0] != "mlkem768x25519plus" {
		return fmt.Errorf("unsupported VLESS encryption %q", value)
	}
	switch blocks[1] {
	case "native", "xorpub", "random":
	default:
		return fmt.Errorf("unsupported VLESS encryption traffic appearance %q", blocks[1])
	}
	switch blocks[2] {
	case "0rtt", "1rtt":
	default:
		return fmt.Errorf("unsupported VLESS encryption session mode %q", blocks[2])
	}

	authIndex := -1
	for index := 3; index < len(blocks); index++ {
		if len(blocks[index]) < 20 {
			if authIndex >= 0 {
				return errors.New("VLESS encryption padding must precede all authentication parameters")
			}
			continue
		}
		decoded, err := base64.RawURLEncoding.DecodeString(blocks[index])
		if err == nil && (len(decoded) == 32 || len(decoded) == 1184) {
			authIndex = index
			break
		}
		return fmt.Errorf("invalid VLESS encryption authentication parameter %q", blocks[index])
	}
	if authIndex < 0 {
		return errors.New("VLESS encryption requires a 32-byte X25519 or 1184-byte ML-KEM-768 authentication parameter")
	}
	for _, block := range blocks[authIndex:] {
		decoded, err := base64.RawURLEncoding.DecodeString(block)
		if err != nil || (len(decoded) != 32 && len(decoded) != 1184) {
			return errors.New("VLESS encryption authentication and relay parameters must be 32-byte X25519 or 1184-byte ML-KEM-768 base64url values")
		}
	}
	padding := blocks[3:authIndex]
	if len(padding) > 0 {
		if err := validateEncryptionPadding(padding); err != nil {
			return err
		}
	}
	return nil
}

func validateEncryptionPadding(blocks []string) error {
	maxLength := 0
	for index, block := range blocks {
		parts := strings.Split(block, "-")
		if len(parts) < 3 {
			return fmt.Errorf("invalid VLESS encryption padding block %q", block)
		}
		values := [3]int{}
		for i := range values {
			value, err := strconv.Atoi(parts[i])
			if err != nil {
				return fmt.Errorf("invalid VLESS encryption padding block %q", block)
			}
			values[i] = value
		}
		if values[0] < 0 || values[0] > 100 || values[1] < 0 || values[2] < values[1] {
			return fmt.Errorf("invalid VLESS encryption padding range %q", block)
		}
		if index == 0 && (values[0] < 100 || values[1] < 35 || values[2] < 35) {
			return errors.New("first VLESS encryption padding block must have 100% probability and a minimum length of 35")
		}
		if index%2 == 0 {
			maxLength += values[2]
		}
	}
	if maxLength > 65553 {
		return errors.New("total VLESS encryption padding length must not exceed 65553")
	}
	return nil
}

// VLESSUUID follows Xray-core common/uuid.ParseString: custom strings of up
// to 30 bytes are mapped to a UUIDv5 with an all-zero namespace.
func VLESSUUID(id string) (string, error) {
	text := []byte(id)
	if len(text) < 32 || len(text) > 36 {
		if len(text) == 0 || len(text) > 30 {
			return "", errors.New("VLESS ID must be a UUID or a non-empty custom string no longer than 30 bytes")
		}
		sum := sha1.Sum(append(make([]byte, 16), text...))
		value := append([]byte(nil), sum[:16]...)
		value[6] = (value[6] & 0x0f) | 0x50
		value[8] = (value[8] & 0x3f) | 0x80
		return formatUUID(value), nil
	}

	value := make([]byte, 16)
	offset := 0
	for _, group := range []int{8, 4, 4, 4, 12} {
		if len(text) > 0 && text[0] == '-' {
			text = text[1:]
		}
		if len(text) < group {
			return "", errors.New("invalid VLESS UUID")
		}
		if _, err := hex.Decode(value[offset:offset+group/2], text[:group]); err != nil {
			return "", errors.New("invalid VLESS UUID")
		}
		text = text[group:]
		offset += group / 2
	}
	if len(text) != 0 {
		return "", errors.New("invalid VLESS UUID")
	}
	return formatUUID(value), nil
}

func formatUUID(value []byte) string {
	encoded := hex.EncodeToString(value)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func validateRealityClient(settings map[string]any) error {
	if settings == nil {
		return errors.New("REALITY requires realitySettings")
	}
	fingerprint := strings.ToLower(stringFromMap(settings, "fingerprint"))
	if fingerprint == "" || fingerprint == "unsafe" || fingerprint == "hellogolang" {
		return errors.New("REALITY requires a supported uTLS fingerprint")
	}
	if !commonFingerprints[fingerprint] {
		return fmt.Errorf("unknown REALITY fingerprint %q", fingerprint)
	}
	password := valueOr(stringFromMap(settings, "password"), stringFromMap(settings, "publicKey"))
	decoded, err := base64.RawURLEncoding.DecodeString(password)
	if err != nil || len(decoded) != 32 {
		return errors.New("REALITY password must be a 32-byte base64url value")
	}
	shortID := stringFromMap(settings, "shortId")
	if len(shortID) > 16 || len(shortID)%2 != 0 {
		return errors.New("REALITY shortId must contain an even number of hexadecimal characters, at most 16")
	}
	if _, err := hex.DecodeString(shortID); err != nil {
		return errors.New("REALITY shortId must be hexadecimal")
	}
	if verify := stringFromMap(settings, "mldsa65Verify"); verify != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(verify)
		if err != nil || len(decoded) != 1952 {
			return errors.New("REALITY mldsa65Verify must be a 1952-byte base64url value")
		}
	}
	if spider := stringFromMap(settings, "spiderX"); spider != "" && !strings.HasPrefix(spider, "/") {
		return errors.New("REALITY spiderX must start with /")
	}
	return nil
}

func validatePinnedCerts(value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(strings.ReplaceAll(item, ":", ""))
		if item == "" {
			continue
		}
		decoded, err := hex.DecodeString(item)
		if err != nil || len(decoded) != 32 {
			return errors.New("pinnedPeerCertSha256 values must be 32-byte hexadecimal SHA-256 hashes")
		}
	}
	return nil
}

func validateTransportSettings(method string, stream map[string]any) error {
	switch method {
	case "xhttp":
		settings, _ := firstMap(stream, "xhttpSettings", "splithttpSettings")
		switch mode := stringFromMap(settings, "mode"); mode {
		case "", "auto", "packet-up", "stream-up", "stream-one":
		default:
			return fmt.Errorf("unsupported XHTTP mode %q", mode)
		}
		if headers, ok := mapFromMap(settings, "headers"); ok && headerValue(headers, "host") != "" {
			return errors.New(`XHTTP headers cannot contain "host"; use xhttpSettings.host`)
		}
	case "websocket":
		settings, _ := mapFromMap(stream, "wsSettings")
		if ed := earlyDataFromPath(stringFromMap(settings, "path")); ed > 8192 {
			return errors.New("WebSocket early data threshold must not exceed 8192")
		}
	case "httpupgrade":
		settings, _ := mapFromMap(stream, "httpupgradeSettings")
		if headers, ok := mapFromMap(settings, "headers"); ok && headerValue(headers, "host") != "" {
			return errors.New(`HTTPUpgrade headers cannot contain "host"; use httpupgradeSettings.host`)
		}
	case "mkcp":
		settings, _ := mapFromMap(stream, "kcpSettings")
		if _, exists := settings["header"]; exists {
			return errors.New("Xray-core v26.7.28 removed mKCP header; use FinalMask")
		}
		if _, exists := settings["seed"]; exists {
			return errors.New("Xray-core v26.7.28 removed mKCP seed; use FinalMask")
		}
		if mtu := intFromMap(settings, "mtu"); mtu != 0 && mtu < 21 {
			return errors.New("mKCP mtu must be at least 21")
		}
		if tti := intFromMap(settings, "tti"); tti != 0 && (tti < 10 || tti > 1000) {
			return errors.New("mKCP tti must be between 10 and 1000")
		}
	case "hysteria":
		settings, _ := mapFromMap(stream, "hysteriaSettings")
		if version := intFromMap(settings, "version"); version != 2 {
			return errors.New("Hysteria transport version must be 2")
		}
	}
	return nil
}

func validateMux(mux map[string]any) error {
	if mux == nil {
		return nil
	}
	switch value := stringFromMap(mux, "xudpProxyUDP443"); value {
	case "", "reject", "allow", "skip":
		return nil
	default:
		return fmt.Errorf("unsupported mux.xudpProxyUDP443 %q", value)
	}
}

func canonicalTransport(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if replacement, removed := removedTransports[value]; removed {
		return "", fmt.Errorf("Xray-core %s removed transport %q; use %s", XrayCoreVersion, value, replacement)
	}
	if current, ok := currentTransports[value]; ok {
		return current, nil
	}
	return "", fmt.Errorf("unsupported Xray transport %q", value)
}

func requiresTransportSecurity(server string) bool {
	host := strings.Trim(strings.ToLower(strings.TrimSpace(server)), "[]")
	if host == "" {
		return false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()
		for _, prefix := range xrayPrivatePrefixes {
			if prefix.Contains(ip) {
				return false
			}
		}
		return true
	}
	if !strings.Contains(host, ".") {
		return false
	}
	for _, suffix := range xrayPrivateDomains {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return false
		}
	}
	return true
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	var output map[string]any
	if json.Unmarshal(data, &output) != nil {
		return nil
	}
	return output
}

func ensureChildMap(parent map[string]any, key string) map[string]any {
	child, _ := mapFromMap(parent, key)
	if child == nil {
		child = map[string]any{}
		parent[key] = child
	}
	return child
}

func firstMap(parent map[string]any, keys ...string) (map[string]any, bool) {
	for _, key := range keys {
		if value, ok := mapFromMap(parent, key); ok {
			return value, true
		}
	}
	return nil, false
}

func mapFromMap(parent map[string]any, key string) (map[string]any, bool) {
	if parent == nil {
		return nil, false
	}
	value, ok := parent[key].(map[string]any)
	return value, ok
}

func sliceFromMap(parent map[string]any, key string) ([]any, bool) {
	if parent == nil {
		return nil, false
	}
	value, ok := parent[key].([]any)
	return value, ok
}

func stringFromMap(parent map[string]any, key string) string {
	if parent == nil {
		return ""
	}
	switch value := parent[key].(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	default:
		return ""
	}
}

func intFromMap(parent map[string]any, key string) int {
	if parent == nil {
		return 0
	}
	switch value := parent[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case json.Number:
		result, _ := strconv.Atoi(value.String())
		return result
	case string:
		result, _ := strconv.Atoi(value)
		return result
	default:
		return 0
	}
}

func boolFromMap(parent map[string]any, key string) bool {
	if parent == nil {
		return false
	}
	switch value := parent[key].(type) {
	case bool:
		return value
	case string:
		return parseBool(value)
	default:
		return false
	}
}

func stringSliceFromMap(parent map[string]any, key string) []string {
	values, ok := sliceFromMap(parent, key)
	if !ok {
		if value := stringFromMap(parent, key); value != "" {
			return splitComma(value)
		}
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if item, ok := value.(string); ok {
			result = append(result, item)
		}
	}
	return result
}

func stringSliceAny(input []string) []any {
	result := make([]any, len(input))
	for index, value := range input {
		result[index] = value
	}
	return result
}

func setOrDelete(parent map[string]any, key, value string) {
	if value == "" {
		delete(parent, key)
		return
	}
	parent[key] = value
}

func headerValue(headers map[string]any, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			if text, ok := value.(string); ok {
				return text
			}
		}
	}
	return ""
}

func outboundMap(parent map[string]any, key string) map[string]any {
	value, _ := mapFromMap(parent, key)
	return value
}

func earlyDataFromPath(path string) int {
	index := strings.Index(path, "?")
	if index < 0 {
		return 0
	}
	for _, part := range strings.Split(path[index+1:], "&") {
		key, value, found := strings.Cut(part, "=")
		if found && key == "ed" {
			result, _ := strconv.Atoi(value)
			return result
		}
	}
	return 0
}
