package vpn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// launchHelper starts the helper through UAC and returns the accepted
// loopback TCP connection. A random token authenticates the helper.
func launchHelper(ctx context.Context, reporter Reporter, helperPath string) (conn net.Conn, token, receiptPath string, wait func() error, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "opentj-")
	if err != nil {
		return nil, "", "", nil, nil, err
	}
	receiptPath = filepath.Join(dir, "receipt.json")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.RemoveAll(dir)
		return nil, "", "", nil, nil, err
	}

	var tokenBytes [32]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		listener.Close()
		os.RemoveAll(dir)
		return nil, "", "", nil, nil, err
	}
	token = hex.EncodeToString(tokenBytes[:])

	executable := helperPath
	if executable == "" {
		executable, err = os.Executable()
		if err != nil {
			listener.Close()
			os.RemoveAll(dir)
			return nil, "", "", nil, nil, err
		}
	}
	endpoint := listener.Addr().String()
	reporter.Progress("请在 UAC 弹窗中允许创建 VPN 隧道", "helper", nil)
	if err := shellExecuteRunas(executable, []string{"--helper", endpoint, token, receiptPath}); err != nil {
		listener.Close()
		os.RemoveAll(dir)
		return nil, "", "", nil, nil, err
	}

	var closeOnce sync.Once
	cleanup = func() {
		closeOnce.Do(func() {
			listener.Close()
			os.RemoveAll(dir)
		})
	}
	wait = func() error {
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(receiptPath); err == nil {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return fmt.Errorf("等待管理员 helper 退出超时")
	}

	type result struct {
		conn net.Conn
		err  error
	}
	accepted := make(chan result, 1)
	go func() {
		conn, err := listener.Accept()
		accepted <- result{conn, err}
	}()

	select {
	case got := <-accepted:
		if got.err != nil {
			cleanup()
			return nil, "", "", nil, nil, got.err
		}
		return got.conn, token, receiptPath, wait, cleanup, nil
	case <-ctx.Done():
		cleanup()
		return nil, "", "", nil, nil, ctx.Err()
	case <-time.After(90 * time.Second):
		cleanup()
		return nil, "", "", nil, nil, fmt.Errorf("管理员授权超时")
	}
}

// shellExecuteRunas starts path elevated, showing the UAC prompt.
func shellExecuteRunas(path string, args []string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	escaped := make([]string, len(args))
	for i, arg := range args {
		escaped[i] = windows.EscapeArg(arg)
	}
	params, err := windows.UTF16PtrFromString(strings.Join(escaped, " "))
	if err != nil {
		return err
	}
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		0,
		uintptr(windows.SW_HIDE),
	)
	if ret <= 32 {
		switch ret {
		case 5:
			return fmt.Errorf("管理员授权被拒绝")
		case 1223:
			return fmt.Errorf("用户取消了管理员授权")
		default:
			return fmt.Errorf("无法启动管理员 helper（ShellExecute 错误码 %d）", ret)
		}
	}
	return nil
}
