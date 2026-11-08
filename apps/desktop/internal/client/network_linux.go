package client

import (
	"errors"
	"fmt"
)

// Snapshot is unused where the tunnel backend is not implemented.
type Snapshot struct {
	Service string
}

// TakeSnapshot is unsupported on Linux.
func TakeSnapshot(Runner, string) (Snapshot, error) {
	return Snapshot{}, errors.New("Linux 暂不支持 VPN 连接")
}

// Inspect is unsupported on Linux.
func Inspect(Runner, string) (Info, error) {
	return Info{}, errors.New("Linux 暂不支持 VPN 连接")
}

// OuterRoute is unsupported on Linux.
func OuterRoute(Runner, string) (string, string, error) {
	return "", "", errors.New("Linux 暂不支持 VPN 连接")
}

func (n *Network) Configure(string) error {
	return fmt.Errorf("Linux 暂不支持 VPN 连接")
}

func (n *Network) Verify() error { return nil }

func (n *Network) Cleanup() []string { return nil }
