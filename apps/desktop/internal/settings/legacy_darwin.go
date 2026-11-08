//go:build darwin

package settings

import (
	"os/exec"
	"strings"
)

// LegacyDefaults reads the preferences the SwiftUI client kept in the app's
// user defaults, so an upgrade keeps the account name, method and "remember
// password" choice. A missing key is ignored.
func LegacyDefaults(domain string) Preferences {
	read := func(key string) string {
		out, err := exec.Command("/usr/bin/defaults", "read", domain, key).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	var p Preferences
	p.Username = read("username")
	p.Method = read("authMethod")
	p.Remember = read("remember") == "1"
	p.Normalize()
	return p
}
