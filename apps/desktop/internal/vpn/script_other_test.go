//go:build !darwin

package vpn

import "testing"

func runOSACompile(t *testing.T, script string) {
	t.Skip("osascript is only available on macOS")
}
