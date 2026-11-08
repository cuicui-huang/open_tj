package vpn

import "runtime"

// Supported reports whether this build can configure a VPN tunnel. Linux
// builds are provided so the interface can be developed and reviewed there.
func Supported() bool { return runtime.GOOS == "darwin" || runtime.GOOS == "windows" }
