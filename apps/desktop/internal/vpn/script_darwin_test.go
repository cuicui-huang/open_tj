//go:build darwin

package vpn

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func runOSACompile(t *testing.T, script string) {
	t.Helper()
	cmd := exec.Command("/usr/bin/osacompile", "-o", filepath.Join(t.TempDir(), "helper.scpt"), "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}
