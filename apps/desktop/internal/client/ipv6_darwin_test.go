//go:build darwin

package client

import "testing"

func TestIPv6LocalAndExternalPaths(t *testing.T) {
	if len(IPv6Conflicts("fd07::/64 link#25 UC bridge100\n240a::1 aa UHL lo0\nfe80::%en0/64 link#11 UC en0")) != 0 {
		t.Fatal("local routes blocked")
	}
	for _, row := range []string{"2000::/3 fe80::1 UG utun8", "fd07::/64 link#25 UC utun8", "fd07::/64 fe80::1 UG bridge100"} {
		if len(IPv6Conflicts(row)) != 1 {
			t.Fatal(row)
		}
	}
}
