//go:build darwin

package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
)

// RunHelper runs the privileged helper on macOS and returns when it stops.
func RunHelper(ctx context.Context, endpoint, token, receiptPath string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("需要管理员授权")
	}
	c, e := net.Dial("unix", endpoint)
	if e != nil {
		writeReceipt(receiptPath, Receipt{Error: e.Error(), Clean: true})
		return e
	}
	defer c.Close()
	if _, e = fmt.Fprintln(c, token); e != nil {
		writeReceipt(receiptPath, Receipt{Error: e.Error(), Clean: true})
		return e
	}
	err := runHelper(ctx, c, receiptPath, "utun")
	if err == io.EOF {
		return nil
	}
	return err
}
