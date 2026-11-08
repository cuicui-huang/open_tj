package vpn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// launchHelper starts the helper with the system's administrator prompt and
// returns the accepted UNIX socket connection.
func launchHelper(ctx context.Context, reporter Reporter, helperPath string) (conn net.Conn, token, receiptPath string, wait func() error, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "opentj-")
	if err != nil {
		return nil, "", "", nil, nil, err
	}
	socketPath := filepath.Join(dir, "control.sock")
	receiptPath = filepath.Join(dir, "receipt.json")
	listener, err := net.Listen("unix", socketPath)
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
		executable, _ = os.Executable()
	}
	command := shellQuote(executable) + " --helper " + shellQuote(socketPath) + " " +
		shellQuote(token) + " " + shellQuote(receiptPath)
	script := "do shell script " + appleScriptString(command) + " with administrator privileges"

	cmd := exec.Command("/usr/bin/osascript", "-e", script)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	reporter.Progress("请在系统弹窗中授权创建 VPN 隧道", "helper", nil)
	if err := cmd.Start(); err != nil {
		listener.Close()
		os.RemoveAll(dir)
		return nil, "", "", nil, nil, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var closeOnce sync.Once
	cleanup = func() {
		closeOnce.Do(func() {
			listener.Close()
			os.RemoveAll(dir)
		})
	}
	stopProcess := func() {
		cmd.Process.Kill()
		<-done
	}
	wait = func() error {
		select {
		case err := <-done:
			return err
		case <-time.After(45 * time.Second):
			cmd.Process.Kill()
			return <-done
		}
	}

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			select {
			case accepted <- conn:
			case <-ctx.Done():
				conn.Close()
			}
		}
	}()

	select {
	case conn = <-accepted:
	case <-done:
		cleanup()
		return nil, "", "", nil, nil, fmt.Errorf("授权辅助程序未启动：%s", stderr.String())
	case <-ctx.Done():
		stopProcess()
		cleanup()
		return nil, "", "", nil, nil, ctx.Err()
	case <-time.After(90 * time.Second):
		stopProcess()
		cleanup()
		return nil, "", "", nil, nil, fmt.Errorf("管理员授权超时")
	}
	return conn, token, receiptPath, wait, cleanup, nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'" }

func appleScriptString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
