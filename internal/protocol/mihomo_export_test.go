package protocol

import (
	"net/url"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/oai404iao/sub_manager/internal/model"
)

func TestMihomoYAMLExportsOnlyProxies(t *testing.T) {
	xhttp := shareTestURI + "&type=xhttp&host=cdn.example.com&path=%2Fx&extra=" + url.QueryEscape(`{"xmux":{"maxConnections":"2-4","hKeepAlivePeriod":-1},"headers":{"X-Test":"x"},"noGRPCHeader":false}`) + "#mixed"
	nodes, err := ParseText(strings.Join([]string{
		xhttp,
		"hysteria2://test@example.com:443,5000-5004/?ech=%2BAAA&sni=cdn.example.com#mixed",
		"mierus://demo:test@example.com?profile=test&port=443&protocol=TCP&port=5010-5011&protocol=UDP#mixed",
		"socks5://demo:test@example.com:1080?udp=true#mixed",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	nodes[0].UDP = false
	nodes[2].UDP = false
	data, err := MihomoYAML(nodes)
	if err != nil {
		t.Fatal(err)
	}
	var yamlDocument map[string][]map[string]any
	if err := yaml.Unmarshal([]byte(data), &yamlDocument); err != nil || len(yamlDocument) != 1 || len(yamlDocument["proxies"]) != 5 {
		t.Fatalf("wrong proxy provider content: %s (%v)", data, err)
	}
	proxies := yamlDocument["proxies"]
	var names []any
	for _, proxy := range proxies {
		names = append(names, proxy["name"])
	}
	if !reflect.DeepEqual(names, []any{"mixed", "mixed (2)", "mixed:443/TCP", "mixed:5010-5011/UDP", "mixed (3)"}) {
		t.Fatalf("proxy names not unique or order changed: %v", names)
	}
	x := proxies[0]["xhttp-opts"].(map[string]any)
	reuse := x["reuse-settings"].(map[string]any)
	if reuse["max-connections"] != "2-4" || reuse["h-keep-alive-period"] != -1 || x["host"] != "cdn.example.com" || x["no-grpc-header"] != false || proxies[0]["udp"] != false {
		t.Fatalf("XHTTP settings lost: %+v", proxies[0])
	}
	if proxies[1]["ports"] != "443,5000-5004" || proxies[1]["ech-opts"].(map[string]any)["config"] != "+AAA" {
		t.Fatalf("Hy2 settings lost: %+v", proxies[1])
	}
	if proxies[2]["port"] != 443 || proxies[2]["transport"] != "TCP" || proxies[3]["port-range"] != "5010-5011" || proxies[3]["transport"] != "UDP" || proxies[3]["udp"] != false || proxies[4]["udp"] != true {
		t.Fatalf("Mieru or SOCKS settings lost: %+v", proxies[2:])
	}
	if n, err := ParseText(data); err != nil || len(n) != 5 {
		t.Fatalf("generated YAML no longer imports: %d (%v)", len(n), err)
	}
}

func TestMihomoVLESSExportWebSocket(t *testing.T) {
	nodes, err := ParseText(`{"protocol":"vless","settings":{"address":"example.com","port":443,"id":"test-id","encryption":"none"},"streamSettings":{"method":"websocket","security":"tls","wsSettings":{"host":"cdn.example.com","path":"/socket?ed=2048&token=demo","headers":{"X-Test":"value"}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	data, err := MihomoYAML(nodes)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string][]map[string]any
	if err := yaml.Unmarshal([]byte(data), &root); err != nil {
		t.Fatal(err)
	}
	proxy := root["proxies"][0]
	ws := proxy["ws-opts"].(map[string]any)
	headers := ws["headers"].(map[string]any)
	if ws["path"] != "/socket?token=demo" || ws["max-early-data"] != 2048 || ws["early-data-header-name"] != "Sec-WebSocket-Protocol" || headers["X-Test"] != "value" || headers["Host"] != "cdn.example.com" || proxy["servername"] != "example.com" {
		t.Fatalf("WebSocket settings lost or TLS SNI changed: %s", data)
	}
}

func TestMihomoYAMLRejectsLossyMappings(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{shareTestURI + "&type=xhttp&extra=" + url.QueryEscape(`{"downloadSettings":{"address":"test.invalid"}}`), "downloadSettings"},
		{shareTestURI + "&type=ws&alpn=h2", "ALPN"},
		{shareTestURI + "&type=ws&path=%2Fedge%252Fvless", "WebSocket 路径"},
		{shareTestURI + "&type=tcp&alpn=http%2F1.1&fp=chrome", "ALPN"},
		{shareTestURI + "&sni=frommitm", "frommitm"},
		{shareTestURI + "&type=grpc", "serviceName"},
		{shareTestURI + "&type=grpc&serviceName=alpha%2Fbeta", "serviceName"},
		{shareTestURI + "&type=grpc&serviceName=my%20service", "serviceName"},
		{strings.ReplaceAll(shareTestURI, "example.com", "203.0.113.1") + "&type=grpc&serviceName=GunService", "authority"},
		{`{"protocol":"vless","settings":{"address":"example.com","port":443,"id":"test","encryption":"none"},"streamSettings":{"security":"tls"},"mux":{"enabled":true,"concurrency":8}}`, "outbound.mux"},
		{"hysteria2://test@example.com/?pinSHA256=" + strings.Repeat("a", 64), "pinSHA256"},
		{"mierus://test:test@example.com?profile=test&port=443&protocol=TCP&mtu=1450", "MTU"},
	} {
		nodes, err := ParseText(tc.input)
		if err != nil {
			t.Fatal(err)
		}
		if data, err := MihomoYAML(append([]model.Node{{Name: "valid", Protocol: "socks5", Server: "example.com", Port: 1080}}, nodes...)); data != "" || err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("did not reject entire provider with unsupported %s: %v (%s)", tc.want, err, data)
		}
	}
}

func TestMihomoPlainXHTTPRoundTrip(t *testing.T) {
	node, err := ParseURI("vless://demo@localhost:8080?security=none&type=xhttp&path=%2Fx")
	if err != nil {
		t.Fatal(err)
	}
	data, err := MihomoYAML([]model.Node{node})
	if err != nil || !strings.Contains(data, "http/1.1") {
		t.Fatalf("did not configure Mihomo cleartext XHTTP: %s (%v)", data, err)
	}
	nodes, err := ParseText(data)
	if err != nil || len(nodes) != 1 || nodes[0].Security != "none" || nodes[0].Network != "xhttp" || len(nodes[0].ALPN) != 0 {
		t.Fatalf("cleartext XHTTP round-trip changed semantics: %+v (%v)", nodes, err)
	}
}
