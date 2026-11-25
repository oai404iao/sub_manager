package protocol

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"
)

func TestHysteria2URI(t *testing.T) {
	const raw = "hy2://demo:p%40ss%2Bword@[2001:db8::1]:443,5000-5002/?sni=hy.example.com&insecure=1&obfs=gecko&obfs-password=test%20key&pinSHA256=" +
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&ech=%2BAAA#Hy%202"
	node, err := ParseURI(raw)
	if err != nil {
		t.Fatal(err)
	}
	if node.Protocol != "hysteria2" || node.Name != "Hy 2" || node.Server != "2001:db8::1" || node.Port != 443 ||
		node.Password != "demo:p@ss+word" || !node.UDP || !node.TLS || !node.AllowInsecure || node.SNI != "hy.example.com" || node.ECHConfigList != "+AAA" {
		t.Fatalf("incorrect URI fields: %+v", node)
	}
	if o := node.Hysteria2; o == nil || o.Ports != "443,5000-5002" || o.Obfs != "gecko" || o.ObfsPassword != "test key" {
		t.Fatalf("incorrect Hy2 options: %+v", o)
	}
	uri, err := URI(node)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uri, "hysteria2://demo:p%40ss+word@[2001:db8::1]:443,5000-5002/?") || !strings.Contains(uri, "obfs-password=test%20key") {
		t.Fatalf("unexpected Hy2 URI: %s", uri)
	}
	nodes, err := ParseText(base64.StdEncoding.EncodeToString([]byte(uri)))
	if err != nil || len(nodes) != 1 || !reflect.DeepEqual(nodes[0], node) {
		t.Fatalf("subscription round-trip changed node: %+v (%v)", nodes, err)
	}
	basic, err := ParseURI("hysteria2://example.com")
	if err != nil || basic.Name != "example.com" || basic.Port != 443 || basic.Password != "" || basic.AllowInsecure || basic.Hysteria2 != nil {
		t.Fatalf("unexpected defaults: %+v (%v)", basic, err)
	}
}

func TestHysteria2RejectsUnsupportedURI(t *testing.T) {
	for _, tail := range []string{
		":0", ":65536", ":5000-4999", ":443,", ":443,65536", ":", "?insecure=true", "?insecure=1&insecure=0",
		"?pinSHA256=deadbeef", "?ech=%zz", "?obfs=salamander", "?obfs-password=example", "?obfs=other&obfs-password=example", "?up=100", "/extra",
	} {
		if _, err := ParseURI("hy2://not-logged@example.com" + tail); err == nil || strings.Contains(err.Error(), "not-logged") {
			t.Errorf("invalid URI %q was accepted or disclosed auth: %v", tail, err)
		}
	}
	if _, err := ParseURI("hy2://not-logged%zz@example.com"); err == nil || strings.Contains(err.Error(), "not-logged") {
		t.Errorf("invalid userinfo was accepted or disclosed: %v", err)
	}
	if _, err := ParseText("hy2:/not-logged%zz@example.com"); err == nil || strings.Contains(err.Error(), "not-logged") {
		t.Errorf("malformed URL was accepted or disclosed: %v", err)
	}
}

func TestMihomoHysteria2Import(t *testing.T) {
	const content = `proxies:
  - type: hysteria2
    name: hy2 test
    server: hy.example.com
    ports: 5000-5002,8443
    password: test:auth
    obfs: salamander
    obfs-password: test-obfs
    sni: another.example.com
    skip-cert-verify: false
    ech-opts:
      enable: true
      config: "+AAA"
`
	nodes, err := ParseText(content)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("%+v %v", nodes, err)
	}
	uri, err := URI(nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ParseURI(uri)
	if err != nil || !reflect.DeepEqual(decoded, nodes[0]) {
		t.Fatalf("Mihomo → URI lost fields: %+v (%v)", decoded, err)
	}
	for _, unsupported := range []string{"hop-interval: 10", "up: 50", "fingerprint: deadbeef", "obfs-min-packet-size: 540"} {
		if _, err := ParseText(content + "    " + unsupported + "\n"); err == nil {
			t.Errorf("ignored unsupported option %q", unsupported)
		}
	}
}
