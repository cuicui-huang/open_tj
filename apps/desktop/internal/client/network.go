package client

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner runs a system command and returns its combined output.
type Runner func(...string) (string, error)

// SystemRun runs a system command with a timeout, without showing a console
// window on Windows.
func SystemRun(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	hideWindow(cmd)
	p, e := cmd.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("%s: %w: %s", strings.Join(args[:min(3, len(args))], " "), e, strings.TrimSpace(string(p)))
	}
	return string(p), nil
}

// Journal applies commands and remembers how to undo them, in order.
type Journal struct {
	Run  Runner
	Undo [][]string
}

func (j *Journal) Apply(add, undo []string) error {
	if _, e := j.Run(add...); e != nil {
		return e
	}
	j.Undo = append(j.Undo, undo)
	return nil
}

func (j *Journal) Rollback() []string {
	var failed [][]string
	var errors []string
	for i := len(j.Undo) - 1; i >= 0; i-- {
		if _, e := j.Run(j.Undo[i]...); e != nil {
			failed = append([][]string{j.Undo[i]}, failed...)
			errors = append(errors, e.Error())
		}
	}
	j.Undo = failed
	return errors
}

// NetworkConfig is what the supervisor sends to the privileged helper: the
// tunnel address and DNS, the physical route to the gateway, and the host
// network state to restore.
type NetworkConfig struct {
	IP        string   `json:"ip"`
	DNS       []string `json:"dns"`
	Gateway   string   `json:"gateway"`
	Router    string   `json:"router"`
	Interface string   `json:"interface"`
	Before    Snapshot `json:"before"`
}

// Network applies the tunnel's host network configuration and restores it.
type Network struct {
	Run              Runner
	Config           NetworkConfig
	Routes, Settings Journal
	Modes            map[string]string
}

// Info describes the physical route to the gateway and the state to restore.
type Info struct {
	Router    string
	Interface string
	Service   string
	Before    Snapshot
}
