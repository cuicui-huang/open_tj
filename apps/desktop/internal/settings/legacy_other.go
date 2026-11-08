//go:build !darwin

package settings

// LegacyDefaults is empty where the SwiftUI client never ran.
func LegacyDefaults(string) Preferences {
	var p Preferences
	p.Normalize()
	return p
}
