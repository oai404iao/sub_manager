package protocol

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func mihomoTestProxy() map[string]any {
	return map[string]any{
		"name": "example", "type": "vless", "server": "example.com", "port": 443,
		"uuid": "11111111-1111-1111-1111-111111111111", "tls": true,
		"alpn": []string{"h2", "http/1.1"},
	}
}

func TestMihomoPreservesEncryptionAndALPN(t *testing.T) {
	p := mihomoTestProxy()
	encryption := "mlkem768x25519plus.native.0rtt." + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	p["encryption"] = encryption
	p["alpn"] = []string{"h2", "http/1.1"}
	node, err := parseMihomoProxy(p)
	if err != nil {
		t.Fatal(err)
	}
	settings := node.XrayOutbound["settings"].(map[string]any)
	tls := node.XrayOutbound["streamSettings"].(map[string]any)["tlsSettings"].(map[string]any)
	if node.Encryption != encryption || settings["encryption"] != encryption || !reflect.DeepEqual(tls["alpn"], []any{"h2", "http/1.1"}) {
		t.Fatalf("lost encryption/ALPN: %#v", node)
	}
	p["tls"] = false
	delete(p, "alpn")
	if _, err := parseMihomoProxy(p); err != nil {
		t.Fatalf("encrypted plaintext transport: %v", err)
	}
}

func TestMihomoEffectiveALPN(t *testing.T) {
	for network, want := range map[string]string{"ws": "http/1.1", "grpc": "h2"} {
		p := mihomoTestProxy()
		p["network"], p["alpn"] = network, []string{"ignored-by-mihomo"}
		node, err := parseMihomoProxy(p)
		if err != nil || !reflect.DeepEqual(node.ALPN, []string{want}) {
			t.Fatalf("%s: %v %v", network, node.ALPN, err)
		}
	}
	p := mihomoTestProxy()
	delete(p, "alpn")
	if _, err := parseMihomoProxy(p); err == nil || !strings.Contains(err.Error(), "ALPN") {
		t.Fatalf("raw absent ALPN: %v", err)
	}
}

func TestMihomoPreservesXHTTPOptions(t *testing.T) {
	p := mihomoTestProxy()
	p["network"] = "xhttp"
	p["xhttp-opts"] = map[string]any{
		"host": "cdn.example.com", "path": "/split", "mode": "packet-up",
		"headers": map[string]any{"X-Test": "value"}, "no-grpc-header": true,
		"x-padding-bytes": "100-200", "sc-max-each-post-bytes": "1000000",
		"sc-min-posts-interval-ms": "30",
	}
	node, err := parseMihomoProxy(p)
	if err != nil {
		t.Fatal(err)
	}
	if node.Host != "cdn.example.com" || node.Path != "/split" || node.XHTTPMode != "packet-up" {
		t.Fatalf("lost transport: %#v", node)
	}
	want := map[string]any{"headers": map[string]any{"X-Test": "value"}, "noGRPCHeader": true, "xPaddingBytes": "100-200", "scMaxEachPostBytes": "1000000", "scMinPostsIntervalMs": "30"}
	if !reflect.DeepEqual(node.XHTTPExtra, want) {
		t.Fatalf("extra=%#v", node.XHTTPExtra)
	}
	if node.SNI != "example.com" || node.Fingerprint != "unsafe" {
		t.Fatalf("changed TLS defaults: %#v", node)
	}
	data, err := XrayOutboundJSON(node)
	if err != nil {
		t.Fatal(err)
	}
	var outbound map[string]any
	if err := json.Unmarshal(data, &outbound); err != nil {
		t.Fatal(err)
	}
	if err := ApplyXrayOutbound(&node); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(node.XHTTPExtra, want) {
		t.Fatalf("round-trip lost extra: %#v", outbound)
	}
}

func TestMihomoXHTTPXMUXRoundTrip(t *testing.T) {
	const yamlInput = `proxies:
  - name: xmux
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    tls: true
    network: xhttp
    xhttp-opts:
      path: /split
      reuse-settings:
        max-concurrency: "16-32"
        max-connections: 0
        c-max-reuse-times: "0"
        h-max-request-times: "600-900"
        h-max-reusable-secs: 1800
        h-keep-alive-period: -1
`
	const jsonInput = `{"proxies":[{"name":"xmux","type":"vless","server":"example.com","port":443,
"uuid":"11111111-1111-1111-1111-111111111111","tls":true,"network":"xhttp",
"xhttp-opts":{"path":"/split","reuse-settings":{"max-concurrency":"0-0","max-connections":"2-4","h-keep-alive-period":0}}}]}`
	for _, tc := range []struct {
		name, input, want string
	}{
		{"YAML", yamlInput, `{"xmux":{"maxConcurrency":"16-32","maxConnections":0,"cMaxReuseTimes":"0","hMaxRequestTimes":"600-900","hMaxReusableSecs":1800,"hKeepAlivePeriod":-1}}`},
		{"JSON", jsonInput, `{"xmux":{"maxConcurrency":"0-0","maxConnections":"2-4","hKeepAlivePeriod":0}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := ParseText(tc.input)
			if err != nil || len(nodes) != 1 {
				t.Fatalf("import = %#v, %v", nodes, err)
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatal(err)
			}
			data, err := XrayOutboundJSON(nodes[0])
			if err != nil {
				t.Fatal(err)
			}
			var outbound map[string]any
			if err := json.Unmarshal(data, &outbound); err != nil {
				t.Fatal(err)
			}
			stream, _ := mapFromMap(outbound, "streamSettings")
			xhttp, _ := mapFromMap(stream, "xhttpSettings")
			if _, hasMux := outbound["mux"]; hasMux || xhttp["xmux"] != nil || !reflect.DeepEqual(xhttp["extra"], want) {
				t.Fatalf("incorrect Xray XMUX settings: %s", data)
			}
			uri, err := URI(nodes[0])
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(uri)
			if err != nil {
				t.Fatal(err)
			}
			q := parsed.Query()
			var extra map[string]any
			if err := json.Unmarshal([]byte(q.Get("extra")), &extra); err != nil || q.Has("xmux") || !reflect.DeepEqual(extra, want) {
				t.Fatalf("incorrect URI XMUX settings: %s (%v)", uri, err)
			}
			node, err := ParseURI(uri)
			if err != nil || !reflect.DeepEqual(node.XHTTPExtra, want) {
				t.Fatalf("URI round-trip extra = %#v, %v", node.XHTTPExtra, err)
			}
		})
	}
}

func TestMihomoXHTTPXMUXRejectsInvalidSettings(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		settings   any
	}{
		{"null", "必须为对象", nil},
		{"scalar", "必须为对象", "disabled"},
		{"empty", "XMUX 默认值", map[string]any{}},
		{"zero", "XMUX 默认值", map[string]any{"max-concurrency": "0-0", "max-connections": 0, "h-keep-alive-period": 0}},
		{"conflict", "上限不能同时大于 0", map[string]any{"max-connections": 2, "max-concurrency": "0-4"}},
		{"reversed", "max-concurrency", map[string]any{"max-concurrency": "4-2"}},
		{"negative", "max-concurrency", map[string]any{"max-concurrency": -1}},
		{"fraction", "max-concurrency", map[string]any{"max-concurrency": 0.5}},
		{"overflow", "max-concurrency", map[string]any{"max-concurrency": "1-2147483648"}},
		{"null range", "max-concurrency", map[string]any{"max-concurrency": nil}},
		{"unknown", "future-option", map[string]any{"max-concurrency": 2, "future-option": true}},
		{"keepalive range", "整数秒", map[string]any{"h-keep-alive-period": "1-2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := mihomoTestProxy()
			bad["network"] = "xhttp"
			bad["xhttp-opts"] = map[string]any{"reuse-settings": tc.settings}
			nodes, recognized, err := parseMihomoProxies([]map[string]any{mihomoTestProxy(), bad})
			if err == nil || !recognized || nodes != nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unexpected import result: %#v, %v, %v", nodes, recognized, err)
			}
		})
	}
}

func TestMihomoPreservesWebSocketHeadersAndEarlyData(t *testing.T) {
	input := `proxies:
  - name: ws
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    tls: true
    network: ws
    ws-opts:
      path: /socket?token=test
      headers:
        Host: cdn.example.com
        X-Test: value
      max-early-data: 2048
      early-data-header-name: Sec-WebSocket-Protocol
`
	nodes, err := ParseText(input)
	if err != nil {
		t.Fatal(err)
	}
	node := nodes[0]
	if node.Host != "cdn.example.com" || node.SNI != "cdn.example.com" || !strings.Contains(node.Path, "ed=2048") || !strings.Contains(node.Path, "token=test") {
		t.Fatalf("lost WS settings: %#v", node)
	}
	data, err := XrayOutboundJSON(node)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"X-Test": "value"`) {
		t.Fatalf("lost header: %s", data)
	}
	if err := ApplyXrayOutbound(&node); err != nil {
		t.Fatal(err)
	}
	data, err = XrayOutboundJSON(node)
	if err != nil || !strings.Contains(string(data), `"X-Test": "value"`) {
		t.Fatalf("lost header on round-trip: %s, %v", data, err)
	}
}

func TestMihomoRejectsLossyAndInvalidOptionsAtomically(t *testing.T) {
	cases := map[string]map[string]any{
		"unknown":                  {"future-option": true},
		"smux":                     {"smux": map[string]any{"enabled": true}},
		"packet encoding":          {"packet-encoding": "xudp"},
		"packet addr":              {"packet-addr": true},
		"xudp":                     {"xudp": true},
		"unsupported protocol":     {"type": "ss"},
		"unsupported network":      {"network": "hysteria"},
		"bad encryption":           {"encryption": "unsupported"},
		"insecure TLS":             {"skip-cert-verify": true},
		"ALPN type":                {"alpn": "h2"},
		"port type":                {"port": "443"},
		"TLS type":                 {"tls": "true"},
		"no TLS ALPN":              {"tls": false, "alpn": []string{"h2"}},
		"reality no TLS":           {"tls": false, "reality-opts": map[string]any{"public-key": "test"}},
		"unknown reality":          {"reality-opts": map[string]any{"future": true}},
		"mismatched options":       {"ws-opts": map[string]any{"path": "/ws"}},
		"unknown ws":               {"network": "ws", "ws-opts": map[string]any{"future": true}},
		"path early data":          {"network": "ws", "ws-opts": map[string]any{"max-early-data": 2048}},
		"custom early data header": {"network": "ws", "ws-opts": map[string]any{"max-early-data": 2048, "early-data-header-name": "X-Early"}},
		"too much early data":      {"network": "ws", "ws-opts": map[string]any{"max-early-data": 8193}},
		"negative early data":      {"network": "ws", "ws-opts": map[string]any{"max-early-data": -1}},
		"unknown xhttp":            {"network": "xhttp", "xhttp-opts": map[string]any{"future": true}},
		"xhttp download":           {"network": "xhttp", "xhttp-opts": map[string]any{"download-settings": map[string]any{"server": "other.example.com"}}},
		"xhttp Host header":        {"network": "xhttp", "xhttp-opts": map[string]any{"headers": map[string]any{"Host": "cdn.example.com"}}},
		"grpc tuning":              {"network": "grpc", "grpc-opts": map[string]any{"max-connections": 2}},
		"socks unknown":            {"type": "socks5", "uuid": nil, "network": "ws"},
	}
	for name, changes := range cases {
		t.Run(name, func(t *testing.T) {
			bad := mihomoTestProxy()
			for key, value := range changes {
				bad[key] = value
			}
			nodes, recognized, err := parseMihomoProxies([]map[string]any{mihomoTestProxy(), bad})
			if err == nil || !recognized || nodes != nil {
				t.Fatalf("partial or silent import: %#v, %v, %v", nodes, recognized, err)
			}
		})
	}
}

func TestMihomoSOCKSAndTLSDefaults(t *testing.T) {
	p := map[string]any{"name": "socks", "type": "socks5", "server": "example.com", "port": 1080, "username": "example-user", "password": "example-password", "udp": true, "tls": true, "skip-cert-verify": true}
	node, err := parseMihomoProxy(p)
	if err != nil {
		t.Fatal(err)
	}
	if node.Protocol != "socks5" || !node.TLS || !node.UDP || !node.AllowInsecure || node.Username != "example-user" || node.Password != "example-password" {
		t.Fatalf("lost SOCKS options: %#v", node)
	}
	for _, fingerprint := range []string{"", "none", "chrome"} {
		p := mihomoTestProxy()
		p["client-fingerprint"] = fingerprint
		node, err := parseMihomoProxy(p)
		if err != nil {
			t.Fatal(err)
		}
		want := fingerprint
		if want == "" || want == "none" {
			want = "unsafe"
		}
		if node.Fingerprint != want {
			t.Fatalf("fingerprint %q mapped to %q", fingerprint, node.Fingerprint)
		}
	}
	p = mihomoTestProxy()
	p["network"] = "ws"
	p["ws-opts"] = map[string]any{"headers": map[string]any{"Host": "cdn.example.com"}}
	node, err = parseMihomoProxy(p)
	if err != nil {
		t.Fatal(err)
	}
	ws := node.XrayOutbound["streamSettings"].(map[string]any)["wsSettings"].(map[string]any)
	if _, exists := ws["headers"]; exists {
		t.Fatalf("Host-only header was not normalized: %#v", ws)
	}
}
