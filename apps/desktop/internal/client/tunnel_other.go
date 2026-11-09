//go:build !windows

package client

// PrepareTunnelDevice does nothing off Windows: there is no device to load.
func PrepareTunnelDevice() error { return nil }

// prepareTunnelDevice is the platform hook, empty here.
func prepareTunnelDevice() error { return nil }
