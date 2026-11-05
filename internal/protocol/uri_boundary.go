package protocol

import (
	"fmt"
	"sort"
	"strings"
)

// URI is a projection, not a container for arbitrary Xray configuration.
// Check the projection before returning a share link; JSON storage is unaffected.
func validateURIOutbound(outbound map[string]any) error {
	check := func(value map[string]any, path, fields string) error {
		var unsupported []string
		for key := range value {
			if !strings.Contains(" "+fields+" ", " "+key+" ") {
				unsupported = append(unsupported, path+"."+key)
			}
		}
		sort.Strings(unsupported)
		if len(unsupported) > 0 {
			return fmt.Errorf("URI 无法无损表达 %s；请使用完整 Xray JSON", strings.Join(unsupported, ", "))
		}
		return nil
	}
	if err := check(outbound, "outbound", "protocol tag settings streamSettings"); err != nil {
		return err
	}
	settings, _ := mapFromMap(outbound, "settings")
	if err := check(settings, "settings", "address port id encryption flow"); err != nil {
		return err
	}
	stream, _ := mapFromMap(outbound, "streamSettings")
	if err := check(stream, "streamSettings", "method security rawSettings tcpSettings xhttpSettings splithttpSettings kcpSettings grpcSettings wsSettings httpupgradeSettings tlsSettings realitySettings finalmask"); err != nil {
		return err
	}
	for key, fields := range map[string]string{
		"rawSettings": "header", "tcpSettings": "header",
		"kcpSettings": "mtu tti", "grpcSettings": "serviceName authority multiMode",
		"wsSettings": "host path headers", "httpupgradeSettings": "host path",
		"tlsSettings":     "serverName fingerprint alpn echConfigList pinnedPeerCertSha256 verifyPeerCertByName",
		"realitySettings": "serverName fingerprint password publicKey shortId spiderX mldsa65Verify",
	} {
		value, _ := mapFromMap(stream, key)
		if err := check(value, "streamSettings."+key, fields); err != nil {
			return err
		}
		if key == "wsSettings" {
			headers, _ := mapFromMap(value, "headers")
			for name := range headers {
				if !strings.EqualFold(name, "Host") {
					return fmt.Errorf("URI 无法无损表达 wsSettings.headers.%s；请使用完整 Xray JSON", name)
				}
			}
		}
		if key == "rawSettings" || key == "tcpSettings" {
			header, _ := mapFromMap(value, "header")
			if err := check(header, "streamSettings."+key+".header", "type"); err != nil {
				return err
			}
		}
	}
	return nil
}
