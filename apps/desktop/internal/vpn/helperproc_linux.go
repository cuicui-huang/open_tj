package vpn

import (
	"context"
	"errors"
	"net"
)

func launchHelper(context.Context, Reporter, string) (net.Conn, string, string, func() error, func(), error) {
	return nil, "", "", nil, nil, errors.New("Linux 暂不支持 VPN 连接")
}
