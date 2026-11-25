package protocol

import (
	"strings"
	"testing"

	"github.com/oai404iao/sub_manager/internal/model"
)

func TestMihomoSOCKSExportUsesServerForSNI(t *testing.T) {
	node := model.Node{Server: "socks.example.com", SNI: "socks.example.com", TLS: true, AllowInsecure: true}
	proxy := map[string]any{"server": node.Server}
	if err := mihomoSOCKSProxy(node, proxy); err != nil {
		t.Fatal(err)
	}
	if proxy["tls"] != true || proxy["skip-cert-verify"] != true || proxy["udp"] != false {
		t.Fatalf("unexpected SOCKS5 settings: %#v", proxy)
	}
	for _, key := range []string{"sni", "servername", "name-cert-verify"} {
		if _, exists := proxy[key]; exists {
			t.Fatalf("unexpected SOCKS5 SNI option %q", key)
		}
	}
	node.SNI = "tls.example.com"
	if err := mihomoSOCKSProxy(node, proxy); err == nil || !strings.Contains(err.Error(), "SNI") {
		t.Fatalf("SOCKS5 SNI override must fail, got %v", err)
	}
}

func TestMihomoHysteria2ExportPortListLimit(t *testing.T) {
	node := model.Node{Hysteria2: &model.Hysteria2Options{Ports: strings.Repeat("443,", 27) + "444-445"}}
	proxy := map[string]any{"port": 444}
	if err := mihomoHysteria2Proxy(node, proxy); err != nil {
		t.Fatal(err)
	}
	if proxy["port"] != 443 || proxy["ports"] != node.Hysteria2.Ports {
		t.Fatalf("unexpected Hysteria 2 endpoint: %#v", proxy)
	}
	node.Hysteria2.Ports += ",446"
	if err := mihomoHysteria2Proxy(node, proxy); err == nil || !strings.Contains(err.Error(), "28") {
		t.Fatalf("too many Mihomo Hysteria 2 ports must fail, got %v", err)
	}
}

func TestMihomoMieruExportSingleBindingKeepsName(t *testing.T) {
	node := model.Node{
		Name: "Node", Server: "mieru.example.com", Username: "example-user", Password: "example-password",
		Mieru: &model.MieruOptions{
			Profile: "client-config", MTU: 1400,
			Bindings: []model.MieruBinding{{Port: "05000-05010", Transport: "UDP"}},
		},
	}
	proxies, err := mihomoMieruProxies(node, map[string]any{"name": node.Name, "port": 5000})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("got %d proxies for one binding", len(proxies))
	}
	proxy := proxies[0]
	if proxy["name"] != "Node" || proxy["port-range"] != "5000-5010" || proxy["transport"] != "UDP" || proxy["udp"] != false {
		t.Fatal("Mieru name, port range, transport or UDP policy changed")
	}
	for _, key := range []string{"port", "profile", "mtu"} {
		if _, exists := proxy[key]; exists {
			t.Fatalf("unexpected Mieru option %q", key)
		}
	}
}
