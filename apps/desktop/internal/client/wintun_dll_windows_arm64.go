//go:build windows && arm64

package client

import _ "embed"

//go:embed resources/wintun-arm64.dll
var wintunDLL []byte
