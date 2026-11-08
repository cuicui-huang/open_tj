package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "preferences.json")
	p := Load(path)
	if p.Method != DefaultMethod {
		t.Fatalf("method %q, want %q", p.Method, DefaultMethod)
	}
	if p.SplitWidth < 320 {
		t.Fatalf("split width %v not normalized", p.SplitWidth)
	}
	p.Username = "student"
	p.Remember = true
	p.SplitWidth = 500
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	loaded := Load(path)
	if loaded.Username != "student" || !loaded.Remember || loaded.SplitWidth != 500 {
		t.Fatalf("loaded %+v", loaded)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
}

func TestLoadIgnoresBrokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if p := Load(path); p.Method != DefaultMethod {
		t.Fatalf("method %q", p.Method)
	}
}
