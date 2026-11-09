package client

import (
	"fmt"
	"net/netip"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

const (
	// windowsTunnelName is the Wintun adapter the helper creates.
	windowsTunnelName = "OpenTJ"
	// windowsProxyKey is the per-user system proxy setting.
	windowsProxyKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
)

// Snapshot is the host network state restored after the tunnel closes.
type Snapshot struct {
	Service       string          `json:"service"`
	DNS           []string        `json:"dns,omitempty"`
	IPv6          map[string]bool `json:"ipv6,omitempty"`
	ProxyEnable   bool            `json:"proxy_enable,omitempty"`
	ProxyServer   string          `json:"proxy_server,omitempty"`
	ProxyOverride string          `json:"proxy_override,omitempty"`
	AutoConfigURL string          `json:"auto_config_url,omitempty"`
}

// powershell runs a script and folds PowerShell's own error stream into the
// text it returns. Without that, a failing cmdlet leaves only "exit status 1",
// which says nothing about which command failed or why — and the user, who
// cannot open the tunnel to see anything else, has nothing to report.
func powershell(run Runner, script string) (string, error) {
	wrapped := "$ErrorActionPreference = 'Stop'; try { " + script +
		" } catch { Write-Output ('__PSError__ ' + $_.Exception.Message); exit 1 }"
	out, err := run("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", wrapped)
	const marker = "__PSError__ "
	if i := strings.Index(out, marker); i >= 0 {
		message := strings.TrimSpace(out[i+len(marker):])
		if message == "" {
			message = "PowerShell 命令失败，但没有给出原因"
		}
		return out, &PowerShellError{Message: message}
	}
	return out, err
}

// PowerShellError is a cmdlet that failed, with the message it gave. The
// message is what the user sees, so it is kept verbatim.
type PowerShellError struct{ Message string }

func (e *PowerShellError) Error() string { return e.Message }

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// findRoute returns the physical interface and next hop for address.
func findRoute(run Runner, address string) (alias, router string, err error) {
	script := fmt.Sprintf(`$r = Find-NetRoute -RemoteIPAddress %s -ErrorAction Stop | Where-Object { $_.NextHop -and $_.InterfaceAlias } | Select-Object -First 1
if (-not $r) { throw 'no route to %s' }
'{0}|{1}' -f $r.InterfaceAlias, $r.NextHop`, psQuote(address), address)
	out, err := powershell(run, script)
	if err != nil {
		return "", "", err
	}
	alias, router, ok := parseRouteOutput(out)
	if !ok {
		return "", "", fmt.Errorf("无法识别网关路由：%s", strings.TrimSpace(out))
	}
	return alias, router, nil
}

// Inspect finds the physical path to the gateway and the state to restore.
func Inspect(run Runner, gateway string) (Info, error) {
	alias, router, err := findRoute(run, gateway)
	if err != nil {
		return Info{}, err
	}
	if strings.EqualFold(alias, windowsTunnelName) || strings.HasPrefix(strings.ToLower(alias), "loopback") {
		return Info{}, fmt.Errorf("网关出口不是物理网络，请先停用其他全局 VPN")
	}
	before, err := TakeSnapshot(run, alias)
	if err != nil {
		return Info{}, err
	}
	return Info{Router: router, Interface: alias, Service: alias, Before: before}, nil
}

// OuterRoute returns the physical next hop and interface for gateway.
func OuterRoute(run Runner, gateway string) (router, iface string, err error) {
	iface, router, err = findRoute(run, gateway)
	return router, iface, err
}

func TakeSnapshot(run Runner, service string) (Snapshot, error) {
	s := Snapshot{Service: service}
	out, err := powershell(run, fmt.Sprintf(`(Get-DnsClientServerAddress -InterfaceAlias %s -AddressFamily IPv4 -ErrorAction Stop).ServerAddresses`, psQuote(service)))
	if err != nil {
		return s, err
	}
	if s.DNS, err = parseDNSServers(out); err != nil {
		return s, err
	}

	script := fmt.Sprintf(`Get-NetAdapter -ErrorAction Stop | Where-Object { $_.Name -ne %s } | ForEach-Object {
  $b = Get-NetAdapterBinding -Name $_.Name -ComponentID ms_tcpip6 -ErrorAction SilentlyContinue
  if ($b) { '{0}`+"\t"+`{1}' -f $_.Name, [int][bool]$b.Enabled }
}`, psQuote(windowsTunnelName))
	out, err = powershell(run, script)
	if err != nil {
		return s, err
	}
	s.IPv6 = parseIPv6Bindings(out)

	s.ProxyEnable, s.ProxyServer, s.ProxyOverride, s.AutoConfigURL = readProxy()
	return s, nil
}

func readProxy() (enabled bool, server, override, autoconfig string) {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsProxyKey, registry.QUERY_VALUE)
	if err != nil {
		return false, "", "", ""
	}
	defer key.Close()
	if value, _, err := key.GetIntegerValue("ProxyEnable"); err == nil {
		enabled = value != 0
	}
	server, _, _ = key.GetStringValue("ProxyServer")
	override, _, _ = key.GetStringValue("ProxyOverride")
	autoconfig, _, _ = key.GetStringValue("AutoConfigURL")
	return enabled, server, override, autoconfig
}

func (n *Network) Configure(name string) error {
	n.Routes.Run = n.Run
	n.Settings.Run = n.Run
	n.Modes = map[string]string{}
	c := n.Config
	if !regexp.MustCompile(`^[A-Za-z0-9 _-]{1,64}$`).MatchString(name) ||
		c.Interface == "" || strings.ContainsAny(c.Interface, "\x00\r\n|") ||
		strings.EqualFold(c.Interface, name) ||
		c.Before.Service != c.Interface {
		return fmt.Errorf("invalid network interface configuration")
	}
	for _, ip := range append([]string{c.IP, c.Gateway, c.Router}, c.DNS...) {
		a, e := netip.ParseAddr(ip)
		if e != nil || !a.Is4() {
			return fmt.Errorf("invalid IPv4 configuration")
		}
	}
	if len(c.DNS) == 0 || len(c.DNS) > 4 {
		return fmt.Errorf("invalid DNS list")
	}

	now, err := TakeSnapshot(n.Run, c.Before.Service)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(now, c.Before) {
		return fmt.Errorf("网络设置已变化，请重新连接")
	}
	router, alias, err := OuterRoute(n.Run, c.Gateway)
	if err != nil {
		return err
	}
	if alias != c.Interface || router != c.Router {
		return fmt.Errorf("网关出口已变化")
	}

	index, err := tunnelIndex(n.Run, name)
	if err != nil {
		return err
	}
	if err := n.configureAdapter(name, c); err != nil {
		return err
	}

	// Point the physical interface's resolver at the VPN DNS, as the macOS
	// backend does, and restore it on cleanup.
	dnsList := make([]string, len(c.DNS))
	for i, address := range c.DNS {
		dnsList[i] = psQuote(address)
	}
	setDNS := powerShellArgs(fmt.Sprintf(`Set-DnsClientServerAddress -InterfaceAlias %s -ServerAddresses (%s) -ErrorAction Stop`,
		psQuote(c.Interface), strings.Join(dnsList, ",")))
	var undoDNS []string
	if len(c.Before.DNS) == 0 {
		undoDNS = powerShellArgs(fmt.Sprintf(`Set-DnsClientServerAddress -InterfaceAlias %s -ResetServerAddresses -ErrorAction Stop`, psQuote(c.Interface)))
	} else {
		old := make([]string, len(c.Before.DNS))
		for i, address := range c.Before.DNS {
			old[i] = psQuote(address)
		}
		undoDNS = powerShellArgs(fmt.Sprintf(`Set-DnsClientServerAddress -InterfaceAlias %s -ServerAddresses (%s) -ErrorAction Stop`,
			psQuote(c.Interface), strings.Join(old, ",")))
	}
	if err := n.Settings.Apply(setDNS, undoDNS); err != nil {
		return err
	}

	// Disable the system proxy and PAC, as the tunnel carries all traffic.
	if c.Before.ProxyEnable {
		if err := n.Settings.Apply(
			[]string{"reg.exe", "ADD", `HKCU\` + windowsProxyKey, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f"},
			[]string{"reg.exe", "ADD", `HKCU\` + windowsProxyKey, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f"},
		); err != nil {
			return err
		}
	}
	if c.Before.AutoConfigURL != "" {
		if err := n.Settings.Apply(
			[]string{"reg.exe", "ADD", `HKCU\` + windowsProxyKey, "/v", "AutoConfigURL", "/t", "REG_SZ", "/d", "", "/f"},
			[]string{"reg.exe", "ADD", `HKCU\` + windowsProxyKey, "/v", "AutoConfigURL", "/t", "REG_SZ", "/d", c.Before.AutoConfigURL, "/f"},
		); err != nil {
			return err
		}
	}

	// Disable IPv6 on the adapters that had it, so no traffic leaks around
	// the IPv4-only tunnel. The states are restored on cleanup.
	for adapter, enabled := range c.Before.IPv6 {
		if !enabled {
			continue
		}
		add := powerShellArgs(fmt.Sprintf(`Disable-NetAdapterBinding -Name %s -ComponentID ms_tcpip6 -ErrorAction Stop`, psQuote(adapter)))
		undo := powerShellArgs(fmt.Sprintf(`Enable-NetAdapterBinding -Name %s -ComponentID ms_tcpip6 -ErrorAction Stop`, psQuote(adapter)))
		if err := n.Settings.Apply(add, undo); err != nil {
			return err
		}
	}

	n.bestEffort("route.exe", "DELETE", c.Gateway, "MASK", "255.255.255.255")
	for _, prefix := range [][2]string{{"0.0.0.0", "128.0.0.0"}, {"128.0.0.0", "128.0.0.0"}} {
		n.bestEffort("route.exe", "DELETE", prefix[0], "MASK", prefix[1], "0.0.0.0", "IF", index)
	}
	if err := n.Routes.Apply(
		[]string{"route.exe", "ADD", c.Gateway, "MASK", "255.255.255.255", c.Router, "METRIC", "1"},
		[]string{"route.exe", "DELETE", c.Gateway, "MASK", "255.255.255.255", c.Router},
	); err != nil {
		return err
	}
	for _, prefix := range [][2]string{{"0.0.0.0", "128.0.0.0"}, {"128.0.0.0", "128.0.0.0"}} {
		if err := n.Routes.Apply(
			[]string{"route.exe", "ADD", prefix[0], "MASK", prefix[1], "0.0.0.0", "METRIC", "1", "IF", index},
			[]string{"route.exe", "DELETE", prefix[0], "MASK", prefix[1], "0.0.0.0", "IF", index},
		); err != nil {
			return err
		}
	}
	return n.Verify()
}

func (n *Network) configureAdapter(name string, c NetworkConfig) error {
	if _, err := powershell(n.Run, fmt.Sprintf(
		`Get-NetIPAddress -InterfaceAlias %s -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue`,
		psQuote(name))); err != nil {
		return err
	}
	// Wintun's adapter is a normal Ethernet-style interface, not a
	// point-to-point one: a /32 leaves it without a subnet, and the next
	// command fails with GetNetworkParams telling you nothing. Give it the
	// /24 its address sits in. The routes added later are what send traffic
	// through the tunnel, so the prefix here only defines the on-link subnet.
	script := fmt.Sprintf(`New-NetIPAddress -InterfaceAlias %s -IPAddress %s -PrefixLength %d -ErrorAction Stop | Out-Null
Set-NetIPInterface -InterfaceAlias %s -AddressFamily IPv4 -NlMtuBytes 1280 -ErrorAction Stop`,
		psQuote(name), c.IP, tunnelPrefix, psQuote(name))
	_, err := powershell(n.Run, script)
	return err
}

// tunnelPrefix is the prefix the tunnel adapter is configured with. The
// gateway hands out a single address, so the subnet is chosen to contain it:
// a /24 keeps the adapter valid without claiming a range the gateway might
// route elsewhere.
const tunnelPrefix = 24

func tunnelIndex(run Runner, name string) (string, error) {
	out, err := powershell(run, fmt.Sprintf(`(Get-NetAdapter -Name %s -ErrorAction Stop).ifIndex`, psQuote(name)))
	if err != nil {
		return "", err
	}
	index := strings.TrimSpace(out)
	if _, err := strconv.Atoi(index); err != nil {
		return "", fmt.Errorf("无法识别隧道接口索引：%s", index)
	}
	return index, nil
}

func powerShellArgs(script string) []string {
	return []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}
}

func (n *Network) bestEffort(args ...string) {
	_, _ = n.Run(args...)
}

func (n *Network) Verify() error {
	alias, _, err := findRoute(n.Run, "1.1.1.1")
	if err != nil {
		return err
	}
	if !strings.EqualFold(alias, windowsTunnelName) {
		return fmt.Errorf("IPv4 默认路由未指向隧道")
	}
	out, err := powershell(n.Run, fmt.Sprintf(`(Get-DnsClientServerAddress -InterfaceAlias %s -AddressFamily IPv4 -ErrorAction Stop).ServerAddresses`, psQuote(n.Config.Interface)))
	if err != nil {
		return err
	}
	dns := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			dns[line] = true
		}
	}
	for _, want := range n.Config.DNS {
		if !dns[want] {
			return fmt.Errorf("隧道 DNS 未生效")
		}
	}
	out, err = powershell(n.Run, fmt.Sprintf(`Get-NetAdapter -ErrorAction Stop | Where-Object { $_.Name -ne %s } | ForEach-Object {
  $b = Get-NetAdapterBinding -Name $_.Name -ComponentID ms_tcpip6 -ErrorAction SilentlyContinue
  if ($b -and $b.Enabled) { $_.Name }
}`, psQuote(windowsTunnelName)))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name := strings.TrimSpace(line)
		if name != "" && n.Config.Before.IPv6[name] {
			return fmt.Errorf("IPv6 仍在 %s 上启用", name)
		}
	}
	return nil
}

func (n *Network) Cleanup() []string {
	var failures []string
	failures = append(failures, n.Routes.Rollback()...)
	failures = append(failures, n.Settings.Rollback()...)

	var (
		after Snapshot
		err   error
	)
	for attempt := 0; attempt < 10; attempt++ {
		after, err = TakeSnapshot(n.Run, n.Config.Before.Service)
		if err == nil && reflect.DeepEqual(after, n.Config.Before) {
			return failures
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err != nil {
		failures = append(failures, "网络设置恢复核对失败: "+err.Error())
	} else if !reflect.DeepEqual(after, n.Config.Before) {
		failures = append(failures, "网络设置恢复核对失败")
	}
	return failures
}
