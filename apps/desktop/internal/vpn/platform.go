package vpn

import "runtime"

// Supported reports whether this build can configure a VPN tunnel.
// macOS and Windows both can; there is no Linux build.
func Supported() bool { return runtime.GOOS == "darwin" || runtime.GOOS == "windows" }
