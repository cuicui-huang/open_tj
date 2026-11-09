//go:build windows

package client

import (
	"errors"
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

// A failing cmdlet must report what failed. "exit status 1" alone leaves the
// user with nothing to act on.
func TestPowerShellErrorCarriesTheMessage(t *testing.T) {
	run := func(args ...string) (string, error) {
		return "__PSError__ Cannot find any service with service name 'Wi-Fi'.\n", errors.New("exit status 1")
	}
	_, err := powershell(run, "Get-NetAdapter")
	if err == nil {
		t.Fatal("no error returned")
	}
	if !strings.Contains(err.Error(), "Cannot find any service") {
		t.Fatalf("the cmdlet's message was dropped: %q", err.Error())
	}
	if strings.Contains(err.Error(), "exit status") {
		t.Fatalf("still reporting the unhelpful wrapper: %q", err.Error())
	}
}

// Success passes the cmdlet's own output through untouched.
func TestPowerShellSuccessKeepsOutput(t *testing.T) {
	run := func(args ...string) (string, error) { return "10.0.0.2\n", nil }
	out, err := powershell(run, "x")
	if err != nil || strings.TrimSpace(out) != "10.0.0.2" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

// An empty message still yields something a person can read.
func TestPowerShellErrorWithoutMessage(t *testing.T) {
	run := func(args ...string) (string, error) { return "__PSError__ \n", errors.New("exit status 1") }
	_, err := powershell(run, "x")
	if err == nil || !strings.Contains(err.Error(), "没有给出原因") {
		t.Fatalf("err=%v", err)
	}
}
