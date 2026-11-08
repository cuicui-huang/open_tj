package vpn

import "runtime"

// Supported reports whether this build can configure a VPN tunnel. Only the
// macOS backend is implemented; Windows and Linux builds are provided so the
// interface can be developed and reviewed there.
func Supported() bool { return runtime.GOOS == "darwin" }
