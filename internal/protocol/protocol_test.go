package protocol

import (
	"strings"
	"testing"
)

func TestVLESSRoundTrip(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&sni=cdn.example.com&fp=chrome&pbk=key&sid=01&type=tcp#demo"
	node, err := ParseURI(raw)
	if err != nil {
		t.Fatal(err)
	}
	if node.Protocol != "vless" || node.PublicKey != "key" || node.Name != "demo" {
		t.Fatalf("unexpected node: %#v", node)
	}
	output, err := URI(node)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output, "vless://") {
		t.Fatalf("unexpected URI: %s", output)
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
	nodes, err := ParseText("dmxlc3M6Ly8xMTExMTExMS0xMTExLTExMTEtMTExMS0xMTExMTExMTExMTFAZXhhbXBsZS5jb206NDQzP2VuY3J5cHRpb249bm9uZSNkZW1v")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes", len(nodes))
	}
}

func TestMihomoYAML(t *testing.T) {
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
      public-key: public-key
      short-id: ab
`
	nodes, err := ParseText(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Security != "reality" || nodes[0].PublicKey != "public-key" {
		t.Fatalf("unexpected nodes: %#v", nodes)
	}
}
