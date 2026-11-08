package client

import (
	"net/netip"
	"strings"
)

// parseRouteOutput reads the "alias|router" line the Windows route probe
// prints.
func parseRouteOutput(out string) (alias, router string, ok bool) {
	line := strings.TrimSpace(strings.Split(out, "\n")[0])
	alias, router, ok = strings.Cut(line, "|")
	alias, router = strings.TrimSpace(alias), strings.TrimSpace(router)
	return alias, router, ok && alias != "" && router != ""
}

// parseIPv6Bindings reads "name<TAB>0|1" lines into adapter states.
func parseIPv6Bindings(out string) map[string]bool {
	bindings := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		bindings[strings.TrimSpace(name)] = strings.TrimSpace(value) == "1"
	}
	return bindings
}

// parseDNSServers reads one address per line, rejecting anything but IPv4.
func parseDNSServers(out string) ([]string, error) {
	var servers []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		address, err := netip.ParseAddr(line)
		if err != nil || !address.Is4() {
			return nil, errInvalidDNS(line)
		}
		servers = append(servers, line)
	}
	return servers, nil
}

type invalidDNS string

func (e invalidDNS) Error() string { return "无法识别 DNS 地址：" + string(e) }

func errInvalidDNS(line string) error { return invalidDNS(line) }
