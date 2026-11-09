//go:build windows && amd64

package client

import _ "embed"

//go:embed resources/wintun-amd64.dll
var wintunDLL []byte
