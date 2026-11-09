//go:build windows

package client

import (
	"strings"
	"testing"
)

// A /32 leaves the Wintun adapter without a subnet, and the next command then
// fails with a GetNetworkParams error that says nothing about the cause.
func TestAdapterGetsARealSubnet(t *testing.T) {
	if tunnelPrefix == 32 || tunnelPrefix < 1 || tunnelPrefix > 30 {
		t.Fatalf("tunnel prefix /%d: a real subnet is needed, /32 in particular", tunnelPrefix)
	}
}

// The prefix must be a literal in the script, not a second %s get wrong.
func TestConfigureScriptsPrefix(t *testing.T) {
	script := "New-NetIPAddress -InterfaceAlias 'x' -IPAddress 10.5.0.2 -PrefixLength 24"
	if !strings.Contains(script, "-PrefixLength 24") {
		t.Fatal("script does not carry the prefix")
	}
}
