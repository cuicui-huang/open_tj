//go:build windows

package client

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// The tunnel device loads wintun.dll from the application directory or
// System32 and nowhere else, so the DLL has to sit beside the executable. It
// also has to be there before the process that creates the device starts,
// which is the elevated helper, not the window.
//
// Carrying it inside the binary keeps Open TJ a single file: the first launch
// writes it out, and every launch after that finds it already in place.
//
// Only one payload is compiled in; the build tag picks the one that matches
// the target, so an amd64 build does not carry the arm64 DLL.

// PrepareTunnelDevice unpacks the bundled DLL beside the executable when it is
// missing or out of date.
func PrepareTunnelDevice() error {
	if len(wintunDLL) == 0 {
		return fmt.Errorf("wintun: no DLL bundled for this architecture")
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("wintun: locate the executable: %w", err)
	}
	target := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if sameBytes(target, wintunDLL) {
		return nil
	}
	// Write to a temporary name first: a half-written DLL left by a crash
	// would fail to load, and the next launch would trust it.
	tmp := target + ".new"
	if err := os.WriteFile(tmp, wintunDLL, 0o644); err != nil {
		return fmt.Errorf("wintun: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("wintun: replace %s: %w", target, err)
	}
	return nil
}

// sameBytes reports whether the file at path already holds exactly data.
func sameBytes(path string, data []byte) bool {
	info, err := os.Stat(path)
	if err != nil || info.Size() != int64(len(data)) {
		return false
	}
	existing, err := os.ReadFile(path)
	if err != nil || len(existing) != len(data) {
		return false
	}
	for i := range existing {
		if existing[i] != data[i] {
			return false
		}
	}
	return true
}
