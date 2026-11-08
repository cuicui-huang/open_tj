package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/sys/windows"
)

// RunHelper runs the privileged helper on Windows and returns when it stops.
func RunHelper(ctx context.Context, endpoint, token, receiptPath string) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("需要管理员授权")
	}
	dialer := net.Dialer{Timeout: 15 * time.Second}
	c, e := dialer.DialContext(ctx, "tcp", endpoint)
	if e != nil {
		writeReceipt(receiptPath, Receipt{Error: e.Error(), Clean: true})
		return e
	}
	defer c.Close()
	if _, e = fmt.Fprintln(c, token); e != nil {
		writeReceipt(receiptPath, Receipt{Error: e.Error(), Clean: true})
		return e
	}
	err := runHelper(ctx, c, receiptPath, windowsTunnelName)
	if err == io.EOF {
		return nil
	}
	return err
}
