package protocol

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oai404iao/sub_manager/internal/model"
)

func TestVLESSRealityRoundTrip(t *testing.T) {
	password := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&sni=cdn.example.com&fp=chrome&pbk=" +
		password + "&sid=01&type=tcp#demo"
	node, err := ParseURI(raw)
	if err != nil {
		t.Fatal(err)
	}
	if node.Protocol != "vless" || node.PublicKey != password || node.Name != "demo" || node.Network != "raw" {
		t.Fatalf("unexpected node: %#v", node)
	}
	output, err := URI(node)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output, "vless://") || !strings.Contains(output, "type=tcp") || !strings.Contains(output, "pbk=") {
		t.Fatalf("unexpected URI: %s", output)
	}
}

func TestVLESSXHTTPLinkFields(t *testing.T) {
	extra := base64.RawURLEncoding.EncodeToString([]byte(`{"xPaddingBytes":"100-1000"}`))
	finalmask := base64.RawURLEncoding.EncodeToString([]byte(`{"tcp":[{"type":"fragment","settings":{"packets":"tlshello"}}]}`))
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&sni=example.com&fp=chrome&type=xhttp&host=cdn.example.com&path=%2Fapi&mode=stream-up&extra=" +
		extra + "&fm=" + finalmask + "#xhttp"
	node, err := ParseURI(raw)
	if err != nil {
		t.Fatal(err)
	}
	output, err := URI(node)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"type=xhttp", "mode=stream-up", "extra=", "fm="} {
		if !strings.Contains(output, expected) {
			t.Fatalf("URI %q does not contain %q", output, expected)
		}
	}
}

func TestVLESSGRPCAndKCPLinkFields(t *testing.T) {
	grpc, err := ParseURI("vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&type=grpc&serviceName=svc&authority=grpc.example.com&mode=multi#grpc")
	if err != nil {
		t.Fatal(err)
	}
	if !grpc.GRPCMultiMode || grpc.ServiceName != "svc" || grpc.Authority != "grpc.example.com" {
		t.Fatalf("unexpected gRPC node: %#v", grpc)
	}
	kcp, err := ParseURI("vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&type=kcp&mtu=1350&tti=50#kcp")
	if err != nil {
		t.Fatal(err)
	}
	if kcp.Network != "mkcp" || kcp.KCPMTU != 1350 || kcp.KCPTTI != 50 {
		t.Fatalf("unexpected mKCP node: %#v", kcp)
	}
}

func TestVLESSEncryptionAllowsNoTLSForPublicServer(t *testing.T) {
	key := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	node, err := ParseURI("vless://11111111-1111-1111-1111-111111111111@example.com:443?security=none&type=tcp&encryption=mlkem768x25519plus.native.0rtt." + key + "#encrypted")
	if err != nil {
		t.Fatal(err)
	}
	if node.Security != "none" || node.Encryption == "none" {
		t.Fatalf("unexpected encrypted node: %#v", node)
	}
}

func TestSOCKSRoundTrip(t *testing.T) {
	node, err := ParseURI("socks5://user:p%40ss@127.0.0.1:1080?udp=true#local")
	if err != nil {
		t.Fatal(err)
	}
	if node.Password != "p@ss" || !node.UDP {
		t.Fatalf("unexpected node: %#v", node)
	}
}

func TestBase64Subscription(t *testing.T) {
	line := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&sni=example.com#demo"
	nodes, err := ParseText(base64.StdEncoding.EncodeToString([]byte(line)))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes", len(nodes))
	}
}

func TestMihomoYAML(t *testing.T) {
	password := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	input := `
proxies:
  - name: reality
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: tcp
    tls: true
    servername: cdn.example.com
    client-fingerprint: chrome
    reality-opts:
      public-key: ` + password + `
      short-id: ab
`
	nodes, err := ParseText(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Security != "reality" || nodes[0].PublicKey != password {
		t.Fatalf("unexpected nodes: %#v", nodes)
	}
}

func TestMihomoAllowInsecureCannotBeSilentlyImportedAsXray(t *testing.T) {
	_, err := ParseText(`
proxies:
  - name: insecure
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    tls: true
    skip-cert-verify: true
`)
	if err == nil || !strings.Contains(err.Error(), "allowInsecure") {
		t.Fatalf("expected removed allowInsecure error, got %v", err)
	}
}

func TestXrayJSONImportPreservesAdvancedSettings(t *testing.T) {
	input := `{
  "outbounds": [{
    "tag": "vless-xhttp",
    "protocol": "vless",
    "settings": {
      "address": "example.com",
      "port": 443,
      "id": "custom-id",
      "encryption": "none",
      "flow": ""
    },
    "streamSettings": {
      "method": "xhttp",
      "security": "tls",
      "xhttpSettings": {
        "host": "cdn.example.com",
        "path": "/api",
        "mode": "stream-up",
        "xPaddingBytes": "100-1000",
        "xmux": {"maxConnections": "2-4"}
      },
      "tlsSettings": {
        "serverName": "example.com",
        "fingerprint": "chrome",
        "pinnedPeerCertSha256": "e8e2d387fdbffeb38e9c9065cf30a97ee23c0e3d32ee6f78ffae40966befccc9"
      },
      "sockopt": {
        "domainStrategy": "UseIP",
        "happyEyeballs": {"tryDelayMs": 250}
      }
    },
    "mux": {
      "enabled": true,
      "concurrency": 8,
      "xudpConcurrency": 16,
      "xudpProxyUDP443": "reject"
    }
  }]
}`
	nodes, err := ParseText(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Network != "xhttp" || nodes[0].UUID != "custom-id" {
		t.Fatalf("unexpected nodes: %#v", nodes)
	}
	output, err := XrayOutboundJSON(nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	var outbound map[string]any
	if err := json.Unmarshal(output, &outbound); err != nil {
		t.Fatal(err)
	}
	stream, _ := mapFromMap(outbound, "streamSettings")
	xhttp, _ := mapFromMap(stream, "xhttpSettings")
	if stringFromMap(xhttp, "xPaddingBytes") != "100-1000" {
		t.Fatalf("advanced XHTTP setting was lost: %s", output)
	}
	sockopt, _ := mapFromMap(stream, "sockopt")
	if stringFromMap(sockopt, "domainStrategy") != "UseIP" {
		t.Fatalf("sockopt was lost: %s", output)
	}
}

func TestLegacyXrayVNextImport(t *testing.T) {
	nodes, err := ParseText(`{
	  "protocol": "vless",
	  "tag": "legacy",
	  "settings": {
	    "vnext": [{
	      "address": "example.com",
	      "port": 443,
	      "users": [{
	        "id": "11111111-1111-1111-1111-111111111111",
	        "encryption": "none",
	        "flow": "xtls-rprx-vision",
	        "level": 1,
	        "email": "user@example.com"
	      }]
	    }]
	  },
	  "streamSettings": {
	    "network": "tcp",
	    "security": "tls",
	    "tlsSettings": {"serverName": "example.com"}
	  }
	}`)
	if err != nil {
		t.Fatal(err)
	}
	output, err := XrayOutboundJSON(nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), `"vnext"`) || !strings.Contains(string(output), `"address": "example.com"`) {
		t.Fatalf("legacy settings were not converted: %s", output)
	}
}

func TestVLESSCustomIDMapping(t *testing.T) {
	uuid, err := VLESSUUID("example")
	if err != nil {
		t.Fatal(err)
	}
	if uuid != "feb54431-301b-52bb-a6dd-e1e93e81bb9e" {
		t.Fatalf("unexpected mapped UUID: %s", uuid)
	}
}

func TestVLESSEncryption(t *testing.T) {
	x25519 := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	valid := "mlkem768x25519plus.native.0rtt.100-111-1111.75-0-111.50-0-3333." + x25519
	if err := ValidateVLESSEncryption(valid); err != nil {
		t.Fatal(err)
	}
	if err := ValidateVLESSEncryption(valid + "." + x25519); err != nil {
		t.Fatalf("relay authentication should be accepted: %v", err)
	}
	for _, invalid := range []string{
		"",
		"auto",
		"mlkem768x25519plus.native.2rtt." + x25519,
		"mlkem768x25519plus.native.0rtt.50-1-2." + x25519,
	} {
		if err := ValidateVLESSEncryption(invalid); err == nil {
			t.Fatalf("expected %q to be rejected", invalid)
		}
	}
}

func TestXrayJSONRequiresEncryption(t *testing.T) {
	_, err := ParseText(`{
	  "protocol": "vless",
	  "settings": {
	    "address": "example.com",
	    "port": 443,
	    "id": "11111111-1111-1111-1111-111111111111"
	  },
	  "streamSettings": {
	    "method": "raw",
	    "security": "tls"
	  }
	}`)
	if err == nil || !strings.Contains(err.Error(), "encryption is required") {
		t.Fatalf("expected missing encryption error, got %v", err)
	}
}

func TestLineWrappedBase64Subscription(t *testing.T) {
	line := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&sni=example.com#demo"
	encoded := base64.StdEncoding.EncodeToString([]byte(line))
	encoded = encoded[:40] + "\n" + encoded[40:]
	nodes, err := ParseText(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes", len(nodes))
	}
}

func TestRemovedXrayFeatures(t *testing.T) {
	node := model.Node{
		Name:          "legacy",
		Protocol:      "vless",
		Server:        "example.com",
		Port:          443,
		UUID:          "11111111-1111-1111-1111-111111111111",
		Encryption:    "none",
		Network:       "raw",
		Security:      "tls",
		AllowInsecure: true,
	}
	if err := Validate(node); err == nil || !strings.Contains(err.Error(), "allowInsecure") {
		t.Fatalf("expected allowInsecure removal error, got %v", err)
	}
	node.AllowInsecure = false
	node.Network = "quic"
	if err := Validate(node); err == nil || !strings.Contains(err.Error(), "removed transport") {
		t.Fatalf("expected QUIC removal error, got %v", err)
	}
	if _, err := ParseURI("vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&allowInsecure=true#legacy"); err == nil {
		t.Fatal("legacy allowInsecure link should be rejected")
	}
}

func TestRealityClientValidation(t *testing.T) {
	node := model.Node{
		Name:        "reality",
		Protocol:    "vless",
		Server:      "example.com",
		Port:        443,
		UUID:        "11111111-1111-1111-1111-111111111111",
		Encryption:  "none",
		Network:     "raw",
		Security:    "reality",
		SNI:         "example.com",
		Fingerprint: "chrome",
		PublicKey:   "invalid",
	}
	if err := Validate(node); err == nil || !strings.Contains(err.Error(), "32-byte") {
		t.Fatalf("expected REALITY password error, got %v", err)
	}
	node.PublicKey = base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	node.ShortID = "abc"
	if err := Validate(node); err == nil || !strings.Contains(err.Error(), "even number") {
		t.Fatalf("expected REALITY shortId error, got %v", err)
	}
}

func TestXrayPrivateAddressRules(t *testing.T) {
	for _, server := range []string{
		"127.0.0.1", "100.64.0.1", "192.0.2.1", "::1", "service",
		"node.home.arpa", "example.test",
	} {
		if requiresTransportSecurity(server) {
			t.Errorf("%s should use Xray's private-address exemption", server)
		}
	}
	for _, server := range []string{"1.1.1.1", "2606:4700:4700::1111", "example.com"} {
		if !requiresTransportSecurity(server) {
			t.Errorf("%s should require transport security", server)
		}
	}
}

func TestGeneratedOutboundWithOfficialXray(t *testing.T) {
	xray := os.Getenv("XRAY_BIN")
	if xray == "" {
		t.Skip("set XRAY_BIN to Xray-core v26.7.28 for upstream validation")
	}
	versionOutput, err := exec.Command(xray, "version").CombinedOutput()
	if err != nil || !strings.Contains(string(versionOutput), "Xray 26.7.28") {
		t.Fatalf("XRAY_BIN must point to Xray-core v26.7.28, got: %v\n%s", err, versionOutput)
	}
	node := model.Node{
		Name:        "official-validation",
		Protocol:    "vless",
		Server:      "example.com",
		Port:        443,
		UUID:        "custom-id",
		Encryption:  "none",
		Flow:        "xtls-rprx-vision",
		Network:     "xhttp",
		Security:    "tls",
		SNI:         "example.com",
		Fingerprint: "chrome",
		Host:        "example.com",
		Path:        "/api",
		UDP:         true,
	}
	if err := EnsureXrayOutbound(&node); err != nil {
		t.Fatal(err)
	}
	stream, _ := mapFromMap(node.XrayOutbound, "streamSettings")
	xhttp := ensureChildMap(stream, "xhttpSettings")
	xhttp["mode"] = "stream-up"
	xhttp["xPaddingBytes"] = "100-1000"
	outbound, err := XrayOutboundJSON(node)
	if err != nil {
		t.Fatal(err)
	}
	var rawOutbound any
	if err := json.Unmarshal(outbound, &rawOutbound); err != nil {
		t.Fatal(err)
	}
	encryptionKey := make([]byte, 32)
	for index := range encryptionKey {
		encryptionKey[index] = byte(index + 1)
	}
	encryptedNode := model.Node{
		Name:     "official-vless-encryption",
		Protocol: "vless",
		Server:   "example.com",
		Port:     443,
		UUID:     "11111111-1111-1111-1111-111111111111",
		Encryption: "mlkem768x25519plus.native.0rtt." +
			base64.RawURLEncoding.EncodeToString(encryptionKey),
		Network:  "raw",
		Security: "none",
		UDP:      true,
	}
	encryptedOutboundJSON, err := XrayOutboundJSON(encryptedNode)
	if err != nil {
		t.Fatal(err)
	}
	var encryptedOutbound any
	if err := json.Unmarshal(encryptedOutboundJSON, &encryptedOutbound); err != nil {
		t.Fatal(err)
	}
	config, err := json.Marshal(map[string]any{
		"log":       map[string]any{"loglevel": "none"},
		"outbounds": []any{rawOutbound, encryptedOutbound},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, config, 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(xray, "run", "-test", "-c", path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("official Xray rejected generated config: %v\n%s\n%s", err, output, config)
	}
}
