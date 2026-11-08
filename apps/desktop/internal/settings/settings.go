// Package settings keeps the user's preferences: the account name, the
// authentication method and the window layout. Passwords never go here; they
// live in the system keychain (see package credentials).
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// DefaultMethod is the authentication method used for new installs.
const DefaultMethod = "LDAP3"

// Methods are the authentication methods the gateway accepts.
var Methods = []string{"LDAP3", "LDAP2", "LDAP", "default_method_localdb"}

// Preferences is the JSON stored in the user data directory.
type Preferences struct {
	Username                 string  `json:"username,omitempty"`
	Method                   string  `json:"auth_method,omitempty"`
	Remember                 bool    `json:"remember,omitempty"`
	BackgroundCloseExplained bool    `json:"background_close_explained,omitempty"`
	ShowLog                  bool    `json:"show_log,omitempty"`
	SplitWidth               float32 `json:"split_width,omitempty"`
}

// Normalize fills the defaults a missing or old file leaves empty.
func (p *Preferences) Normalize() {
	if p.Method == "" {
		p.Method = DefaultMethod
	}
	if p.SplitWidth < 320 {
		p.SplitWidth = 460
	}
}

// Load reads the preferences at path. A missing or unreadable file gives the
// defaults, without an error: preferences are never worth failing a launch.
func Load(path string) Preferences {
	var p Preferences
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &p)
	}
	p.Normalize()
	return p
}

// Save writes the preferences atomically, creating the directory as needed.
func Save(path string, p Preferences) error {
	if path == "" {
		return errors.New("settings: empty preferences path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
