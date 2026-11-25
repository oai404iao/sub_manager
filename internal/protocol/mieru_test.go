package protocol

import (
	"encoding/base64"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/oai404iao/sub_manager/internal/model"
)

const mieruTestPrefix = "mierus://test%3Auser:p%40ss%2Bword@[2001:db8::1]"

func TestMieruSimpleShareRoundTrip(t *testing.T) {
	raw := mieruTestPrefix + "?profile=test+profile&port=443&protocol=TCP&port=5000-5010&protocol=UDP&port=8443&protocol=TCP" +
		"&mtu=1500&multiplexing=MULTIPLEXING_MIDDLE&handshake-mode=HANDSHAKE_STANDARD&traffic-pattern=CBA%3D#Display%20Name"
	node, err := ParseURI(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := model.MieruOptions{
		Profile: "test profile", MTU: 1500, Multiplexing: "MULTIPLEXING_MIDDLE", HandshakeMode: "HANDSHAKE_STANDARD", TrafficPattern: "CBA=",
		Bindings: []model.MieruBinding{{Port: "443", Transport: "TCP"}, {Port: "5000-5010", Transport: "UDP"}, {Port: "8443", Transport: "TCP"}},
	}
	if node.Protocol != "mieru" || node.Name != "Display Name" || node.Server != "2001:db8::1" || node.Port != 443 || node.Username != "test:user" || node.Password != "p@ss+word" || !node.UDP || !reflect.DeepEqual(node.Mieru, &want) {
		t.Fatalf("unexpected Mieru node: %+v options=%+v", node, node.Mieru)
	}
	uri, err := URI(node)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "mierus" || u.Port() != "" || !reflect.DeepEqual(u.Query()["port"], []string{"443", "5000-5010", "8443"}) || !reflect.DeepEqual(u.Query()["protocol"], []string{"TCP", "UDP", "TCP"}) {
		t.Fatalf("link changed binding order: %s (%v)", uri, err)
	}
	nodes, err := ParseText(base64.StdEncoding.EncodeToString([]byte(uri)))
	if err != nil || len(nodes) != 1 || !reflect.DeepEqual(nodes[0], node) {
		t.Fatalf("subscription round-trip changed node: %+v (%v)", nodes, err)
	}
}

func TestMieruRejectsInvalidLinks(t *testing.T) {
	const base = "mierus://demo:not-logged@example.com"
	const basic = "?profile=test&port=443&protocol=TCP"
	for _, suffix := range []string{
		"?port=443&protocol=TCP", "?profile=test", "?profile=test&port=443&port=9443&protocol=TCP",
		"?profile=test&port=0&protocol=TCP", "?profile=test&port=5000-4999&protocol=TCP", "?profile=test&port=443&protocol=QUIC",
		":443" + basic, "/unexpected" + basic, basic + "&profile=second", basic + "&traffic-pattern=invalid!",
		basic + "&mtu=", basic + "&mtu=1279", basic + "&mtu=1501", basic + "&handshake-mode=UNKNOWN", basic + "&udp=false", basic + "&unexpected=%zz",
	} {
		if _, err := ParseText(base + suffix); err == nil || strings.Contains(err.Error(), "not-logged") {
			t.Errorf("invalid link %q was accepted or disclosed credentials: %v", suffix, err)
		}
	}
	if _, err := ParseURI("mieru://opaque-client-config"); err == nil || !strings.Contains(err.Error(), "mieru export config simple") {
		t.Fatalf("missing simple-link guidance: %v", err)
	}
}

func TestMieruCredentialsValidateUTF8AndByteLength(t *testing.T) {
	const suffix = "@example.com?profile=test&port=443&protocol=TCP"
	valid := strings.Repeat("界", 21) + "a"
	uri := "mierus://" + url.UserPassword(valid, valid).String() + suffix
	if node, err := ParseURI(uri); err != nil || node.Username != valid || node.Password != valid {
		t.Fatalf("valid 64-byte credentials were not preserved: %v", err)
	}
	for _, userinfo := range []string{
		url.UserPassword(valid+"a", valid).String(), url.UserPassword(valid, valid+"a").String(), "%FF:test", "test:%FF",
	} {
		if _, err := ParseURI("mierus://" + userinfo + suffix); err == nil {
			t.Fatal("invalid credential bytes were accepted")
		}
	}
}

func TestMihomoMieruImport(t *testing.T) {
	const content = `proxies:
  - name: Mieru proxy
    type: mieru
    server: example.com
    port-range: 5000-5010
    transport: TCP
    username: test-user
    password: test-password
    udp: true
    multiplexing: MULTIPLEXING_LOW
    handshake-mode: HANDSHAKE_NO_WAIT
`
	nodes, err := ParseText(content)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("import: %+v (%v)", nodes, err)
	}
	uri, err := URI(nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ParseURI(uri)
	if err != nil || !reflect.DeepEqual(decoded, nodes[0]) || decoded.Port != 5000 {
		t.Fatalf("Mihomo → URI lost configuration: %+v (%v)", decoded, err)
	}
	if _, err := ParseText(content + "    port: 443\n"); err == nil {
		t.Fatal("accepted port with port-range")
	}
	nodes, err = ParseText(strings.ReplaceAll(content, "    udp: true\n", ""))
	if err != nil || len(nodes) != 1 || nodes[0].UDP {
		t.Fatalf("changed native Mihomo UDP default: %+v (%v)", nodes, err)
	}
	if _, err := URI(nodes[0]); err == nil || !strings.Contains(err.Error(), "UDP") {
		t.Fatalf("export didn't guard native UDP policy: %v", err)
	}
}
