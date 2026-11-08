// Package credentials stores the VPN password in the system keychain.
package credentials

import (
	"encoding/json"
	"errors"
	"os/exec"
	"runtime"

	"github.com/egoist/mygo"
)

// Credentials is what the keychain keeps: the account and its password.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Store is the interface the app uses, so tests can replace the keychain.
type Store interface {
	Load() (Credentials, bool, error)
	Save(Credentials) error
	Delete() error
}

// LegacyService is the service the SwiftUI client used. It is still read once
// so that upgrading users keep their saved password.
const LegacyService = "local.motionvpn.prototype.credentials"

// DefaultService is the keychain service for new credentials. It matches the
// app identifier, so the keychain groups them with the app.
const DefaultService = LegacyService

// key is the account name of the single item the app stores.
const key = "credentials"

// Keychain stores credentials with mygo.Secrets, the system keychain.
type Keychain struct {
	Service string
}

// NewKeychain returns a keychain store. An empty service uses DefaultService.
func NewKeychain(service string) Keychain {
	if service == "" {
		service = DefaultService
	}
	return Keychain{Service: service}
}

func (k Keychain) store() mygo.SecretsModule {
	return mygo.Secrets.Service(k.Service)
}

// Load reads the stored credentials, migrating the item the SwiftUI client
// wrote the first time. found is false when nothing is stored.
func (k Keychain) Load() (Credentials, bool, error) {
	data, err := k.store().Get(key)
	if err == nil {
		var c Credentials
		if err := json.Unmarshal(data, &c); err != nil {
			return Credentials{}, false, err
		}
		return c, c.Username != "" && c.Password != "", nil
	}
	if !errors.Is(err, mygo.ErrSecretNotFound) {
		return Credentials{}, false, err
	}
	legacy, err := loadLegacy()
	if err != nil || legacy.Username == "" {
		return Credentials{}, false, nil
	}
	// Keep the migrated item for later launches; a failure to write is not
	// fatal, the user can simply save again.
	_ = k.Save(legacy)
	return legacy, true, nil
}

// Save stores the credentials.
func (k Keychain) Save(c Credentials) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return k.store().Set(key, data)
}

// Delete removes the stored credentials. Deleting a missing item is fine.
func (k Keychain) Delete() error {
	err := k.store().Delete(key)
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return nil
	}
	return err
}

// loadLegacy reads the generic password the Swift client stored, through the
// security tool: the item is addressed only by its service, which mygo's
// account-based store cannot read.
func loadLegacy() (Credentials, error) {
	if runtime.GOOS != "darwin" {
		return Credentials{}, nil
	}
	out, err := exec.Command("/usr/bin/security", "find-generic-password",
		"-s", LegacyService, "-w").Output()
	if err != nil {
		return Credentials{}, err
	}
	var c Credentials
	if err := json.Unmarshal(trimNewline(out), &c); err != nil {
		return Credentials{}, err
	}
	return c, nil
}

func trimNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
