package client

import (
	"context"
	"errors"
)

// RunHelper is unsupported on Linux.
func RunHelper(context.Context, string, string, string) error {
	return errors.New("Linux 暂不支持 VPN 连接")
}
