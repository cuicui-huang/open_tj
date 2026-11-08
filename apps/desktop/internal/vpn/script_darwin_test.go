//go:build darwin

package vpn

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAuthorizationScriptCompiles(t *testing.T) {
	command := shellQuote("/tmp/中文 path/OpenTJ") + " --helper " + shellQuote("/tmp/sock 'quoted'") + " " + shellQuote("token")
	script := "do shell script " + appleScriptString(command) + " with administrator privileges"
	runOSACompile(t, script)
}

func runOSACompile(t *testing.T, script string) {
	t.Helper()
	cmd := exec.Command("/usr/bin/osacompile", "-o", filepath.Join(t.TempDir(), "helper.scpt"), "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}
