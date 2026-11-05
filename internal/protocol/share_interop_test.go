package protocol

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

const shareTestURI = "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls"

func TestShareJSONEncoding(t *testing.T) {
	for _, field := range []string{"extra", "fm"} {
		object := `{"tcp":[]}`
		if field == "extra" {
			object = `{"noGRPCHeader":true,"headers":{"X-Test":"a+b & c"}}`
		}
		node, err := ParseURI(shareTestURI + "&type=xhttp&" + field + "=" + url.QueryEscape(object))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := URI(node)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(raw)
		if !json.Valid([]byte(u.Query().Get(field))) {
			t.Fatalf("not plain JSON: %s", raw)
		}
		if _, err := ParseURI(raw); err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{`null`, `[]`, `false`, `{"broken"`, `e30`} {
			if _, err := ParseURI(shareTestURI + "&type=xhttp&" + field + "=" + url.QueryEscape(bad)); err == nil {
				t.Fatalf("accepted %s=%s", field, bad)
			}
		}
	}
}

func TestShareProjectionBoundary(t *testing.T) {
	base := `{"protocol":"vless","settings":{"address":"example.com","port":443,"id":"audit-id","encryption":"none"},"streamSettings":{"method":"xhttp","security":"tls","xhttpSettings":{"path":"/","xmux":{"maxConnections":"2-4"},"headers":{"X-Test":"audit"}}}}`
	nodes, err := ParseText(base)
	if err != nil {
		t.Fatal(err)
	}
	node := nodes[0]
	raw, err := URI(node)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	if !strings.Contains(u.Query().Get("extra"), `"xmux"`) || !strings.Contains(u.Query().Get("extra"), `"headers"`) {
		t.Fatal(raw)
	}
	// Xray ignores all outer advanced options when extra is supplied.
	replaced := node
	replaced.XrayOutbound = cloneMap(node.XrayOutbound)
	x := ensureChildMap(ensureChildMap(replaced.XrayOutbound, "streamSettings"), "xhttpSettings")
	x["extra"] = map[string]any{"noGRPCHeader": true}
	if err := ApplyXrayOutbound(&replaced); err != nil {
		t.Fatal(err)
	}
	replacementURI, err := URI(replaced)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := url.Parse(replacementURI)
	if strings.Contains(r.Query().Get("extra"), "X-Test") || !strings.Contains(r.Query().Get("extra"), "noGRPCHeader") {
		t.Fatal(replacementURI)
	}
	for _, field := range []string{"mux", "sockopt", "tls", "hysteria"} {
		t.Run(field, func(t *testing.T) {
			n := node
			n.XrayOutbound = cloneMap(node.XrayOutbound)
			stream := ensureChildMap(n.XrayOutbound, "streamSettings")
			switch field {
			case "mux":
				n.XrayOutbound["mux"] = map[string]any{"enabled": true, "concurrency": 8}
			case "sockopt":
				stream["sockopt"] = map[string]any{"tcpFastOpen": true}
			case "tls":
				ensureChildMap(stream, "tlsSettings")["minVersion"] = "1.3"
			case "hysteria":
				n.Network = "hysteria"
				stream["method"] = "hysteria"
				stream["hysteriaSettings"] = map[string]any{"version": 2, "auth": "test"}
			}
			if _, err := XrayOutboundJSON(n); err != nil {
				t.Fatal(err)
			}
			if _, err := URI(n); err == nil || !strings.Contains(err.Error(), "JSON") {
				t.Fatalf("projection should fail: %v", err)
			}
		})
	}
}

func TestUDPPolicySurvivesApplyAndURI(t *testing.T) {
	node, err := ParseURI(shareTestURI + "&udp=false")
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyXrayOutbound(&node); err != nil {
		t.Fatal(err)
	}
	if node.UDP {
		t.Fatal("Apply enabled UDP")
	}
	if _, err := URI(node); err == nil || !strings.Contains(err.Error(), "udp=false") {
		t.Fatalf("UDP policy must not silently change in consumers: %v", err)
	}
}

func TestSOCKSCredentialsAndShareBoundary(t *testing.T) {
	for _, scheme := range []string{"socks", "socks5", "socks5h"} {
		node, err := ParseURI(scheme + "://dXNlcjpwYXNz@example.com:1080")
		if err != nil || node.Username != "user" || node.Password != "pass" {
			t.Fatalf("%+v %v", node, err)
		}
		if _, err := URI(node); err != nil {
			t.Fatal(err)
		}
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		userinfo := url.User(encoding.EncodeToString([]byte("user:p:a@ss"))).String()
		node, err := ParseURI("socks5h://" + userinfo + "@[::1]:1080")
		if err != nil || node.Username != "user" || node.Password != "p:a@ss" || node.Server != "::1" {
			t.Fatalf("credentials: %+v %v", node, err)
		}
	}
	plain, err := ParseURI("socks5://dXNlcjpwYXNz:p%40ss@example.com:1080")
	if err != nil || plain.Username != "dXNlcjpwYXNz" || plain.Password != "p@ss" {
		t.Fatalf("explicit plaintext was decoded: %+v %v", plain, err)
	}
	for _, query := range []string{"tls=true", "sni=example.com", "allowInsecure=true", "udp=true", "unknown=value"} {
		n, err := ParseURI("socks5://user:pass@example.com:1080?" + query)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := URI(n); err == nil {
			t.Fatalf("silently shared %s", query)
		}
	}
}

func TestMixedImportFailsAtomically(t *testing.T) {
	for _, content := range []string{
		shareTestURI + "\ntrojan://test@example.com:443",
		`["` + shareTestURI + `", "trojan://test@example.com:443"]`,
		`{"proxies":[{"type":"socks5","name":"test","server":"example.com","port":1080},{"type":"trojan","name":"unsupported"}]}`,
		`{"proxies":[{"type":"socks5","name":"test","server":"example.com","port":1080},42]}`,
		`["` + shareTestURI + `", {"type":"trojan"}]`,
		`{"outbounds":[{"protocol":"vless","settings":{"address":"example.com","port":443,"id":"test","encryption":"none"}},{"protocol":"freedom"}]}`,
	} {
		if nodes, err := ParseText(content); err == nil || len(nodes) > 0 {
			t.Fatalf("silently accepted subset: %+v %v", nodes, err)
		}
	}
}

func TestInvalidShareQueries(t *testing.T) {
	for _, suffix := range []string{"&udp=maybe", "&udp=false&udp=true", "&extra=%7B%7D", "&mode=multi", "&packetEncoding=packet", "&bad=%zz"} {
		if _, err := ParseURI(shareTestURI + suffix); err == nil {
			t.Fatalf("accepted invalid/unsupported query %s", suffix)
		}
	}
	for _, suffix := range []string{"?udp=maybe", "?tls=false&tls=true", "?tls=%zz"} {
		if _, err := ParseURI("socks5://example.com:1080" + suffix); err == nil {
			t.Fatalf("accepted invalid query %s", suffix)
		}
	}
}
