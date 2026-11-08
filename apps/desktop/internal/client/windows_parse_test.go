package client

import "testing"

func TestParseRouteOutput(t *testing.T) {
	alias, router, ok := parseRouteOutput("Wi-Fi|192.168.1.1\r\n")
	if !ok || alias != "Wi-Fi" || router != "192.168.1.1" {
		t.Fatalf("%q %q %v", alias, router, ok)
	}
	if _, _, ok := parseRouteOutput("garbage"); ok {
		t.Fatal("accepted garbage")
	}
}

func TestParseIPv6Bindings(t *testing.T) {
	got := parseIPv6Bindings("Ethernet\t1\nWi-Fi\t0\nbroken\n")
	if !got["Ethernet"] || got["Wi-Fi"] || len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

func TestParseDNSServers(t *testing.T) {
	got, err := parseDNSServers("10.0.0.1\r\n10.0.0.2\n")
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := parseDNSServers("not-an-ip"); err == nil {
		t.Fatal("accepted invalid DNS")
	}
	if got, err := parseDNSServers(""); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
}
