//go:build windows

package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWintunDLLIsBundled(t *testing.T) {
	if len(wintunDLL) < 100_000 {
		t.Fatalf("bundled DLL is %d bytes: the embed did not take", len(wintunDLL))
	}
	// A PE file starts with "MZ".
	if wintunDLL[0] != 'M' || wintunDLL[1] != 'Z' {
		t.Fatalf("bundled DLL is not a PE file: %x %x", wintunDLL[0], wintunDLL[1])
	}
}

func TestSameBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.dll")
	if sameBytes(path, []byte("abc")) {
		t.Fatal("missing file reported as matching")
	}
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !sameBytes(path, []byte("abc")) {
		t.Fatal("identical file reported as different")
	}
	if sameBytes(path, []byte("abd")) {
		t.Fatal("different file reported as matching")
	}
	if sameBytes(path, []byte("abcd")) {
		t.Fatal("longer content reported as matching")
	}
}
