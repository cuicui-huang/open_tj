//go:build !windows

package client

// wintunDLL is empty off Windows: there is no tunnel device to load.
var wintunDLL []byte
