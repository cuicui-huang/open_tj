// Package vpn runs the VPN session supervisor: it authenticates with the
// campus gateway, asks for administrator rights to configure the tunnel,
// forwards traffic and reconnects when the connection is lost.
//
// It is independent of the user interface. A Reporter receives every change
// the UI shows, so the same supervisor can drive the native window, the
// menu bar panel, tests and the JSONL diagnostic log.
package vpn

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"open-tj.local/desktop/internal/client"
)

// DefaultGateway is the campus VPN gateway.
const DefaultGateway = "vpn.tongji.cn"

// Config is what the supervisor needs from the application.
type Config struct {
	// Gateway is the host to authenticate with. Empty means DefaultGateway.
	Gateway string
	// HelperPath is the executable started again with "with administrator
	// privileges". Empty means the running executable.
	HelperPath string
}

// Reporter receives the supervisor's progress. Every method may be called
// from any goroutine.
type Reporter interface {
	// Connecting starts an attempt.
	Connecting()
	// Progress reports a step, optionally with connectivity probes.
	Progress(message, component string, probes []client.ProbeResult)
	// Connected reports that the tunnel is up.
	Connected(iface string)
	// Reconnecting reports a wait before the next attempt.
	Reconnecting(seconds int)
	// Sleeping reports that the machine is asleep and the session is closed.
	Sleeping()
	// Failed reports an attempt that never started.
	Failed(message string)
	// Result reports the outcome of an attempt.
	Result(result Result)
	// Log is a line for the in-app log and the JSONL log file.
	Log(level, component, message string)
	// Traffic is a traffic sample, at most once a second.
	Traffic(sample client.TrafficSample)
}

// Supervisor owns the reconnect loop and the sleep/wake state.
type Supervisor struct {
	cfg      Config
	reporter Reporter
	wake     chan struct{}

	mu            sync.Mutex
	sleeping      bool
	sessionCancel context.CancelFunc
}

// New creates a supervisor that reports to r.
func New(cfg Config, r Reporter) *Supervisor {
	if cfg.Gateway == "" {
		cfg.Gateway = DefaultGateway
	}
	return &Supervisor{cfg: cfg, reporter: r, wake: make(chan struct{}, 1)}
}

// Sleep pauses the supervisor. The current session is cancelled; a
// connection that never touched the network ends quietly.
func (s *Supervisor) Sleep() {
	s.mu.Lock()
	s.sleeping = true
	cancel := s.sessionCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Wake resumes the supervisor after Sleep.
func (s *Supervisor) Wake() {
	s.mu.Lock()
	s.sleeping = false
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Supervisor) isSleeping() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sleeping
}

func (s *Supervisor) setSessionCancel(cancel context.CancelFunc) {
	s.mu.Lock()
	s.sessionCancel = cancel
	sleeping := s.sleeping
	s.mu.Unlock()
	if cancel != nil && sleeping {
		cancel()
	}
}

// Run connects until ctx is cancelled or a failure makes recovery unsafe.
func (s *Supervisor) Run(ctx context.Context, cred client.Credentials) {
	attempt := 0
	for ctx.Err() == nil {
		if s.isSleeping() {
			s.reporter.Sleeping()
			select {
			case <-s.wake:
			case <-ctx.Done():
				return
			}
			continue
		}
		connectionCtx, stop := context.WithCancel(ctx)
		s.setSessionCancel(stop)
		s.reporter.Connecting()

		result, connected := s.attempt(connectionCtx, cred)
		stop()
		s.setSessionCancel(nil)

		s.reporter.Result(result)
		retry := retryPolicy(result, s.isSleeping(), ctx.Err() != nil)
		if !retry || ctx.Err() != nil {
			return
		}
		if s.isSleeping() {
			continue
		}
		if connected {
			attempt = 0
		}
		attempt++
		seconds := min(30, 1<<min(attempt, 5))
		s.reporter.Reconnecting(seconds)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(seconds) * time.Second):
		}
	}
}

// attempt runs one login/tunnel/cleanup cycle.
func (s *Supervisor) attempt(ctx context.Context, cred client.Credentials) (Result, bool) {
	gateway := client.Gateway{Host: s.cfg.Gateway}
	session, err := gateway.Login(ctx, cred)
	if err != nil {
		return Result{Error: err.Error()}, false
	}
	s.reporter.Log("info", "session", "认证成功，TLS 隧道配置已读取")
	interval := keepaliveInterval(session)
	s.reporter.Log("info", "session", fmt.Sprintf("会话已建立 · 心跳间隔 %d 秒", int(interval.Seconds())))

	site := s.tunnel(ctx, session)
	session.Conn.Close()
	result := Result{Login: "welcome", Site: &site}
	if err := session.Gateway.Logout(session.Cookie); err != nil {
		result.LogoutError = err.Error()
	}
	return result, site.Connected
}

// tunnel configures the host network through the privileged helper, forwards
// traffic until the session ends and always restores the network afterwards.
func (s *Supervisor) tunnel(ctx context.Context, session *client.Session) SiteResult {
	site := SiteResult{}

	fail := func(err error) SiteResult {
		site.ConnectionError = err.Error()
		site.StartupFailed = true
		return site
	}

	ip, err := client.IPv4(session.Network["client_ipv4"])
	if err != nil {
		return fail(err)
	}
	dns, err := tunnelDNS(session.Network["dns_servers"])
	if err != nil {
		return fail(err)
	}
	gatewayAddress, _, _ := net.SplitHostPort(session.Conn.RemoteAddr().String())
	info, err := client.Inspect(client.SystemRun, gatewayAddress)
	if err != nil {
		return fail(err)
	}
	s.reporter.Log("info", "network",
		fmt.Sprintf("网络预检完成 · 服务 %s · 出口 %s · VPN DNS %d 个", info.Service, info.Interface, len(dns)))

	cfg := client.NetworkConfig{
		IP:        ip,
		DNS:       dns,
		Gateway:   gatewayAddress,
		Router:    info.Router,
		Interface: info.Interface,
		Before:    info.Before,
	}

	// Keep the gateway session alive while the administrator authorizes the
	// helper and the tunnel is configured; that can take tens of seconds.
	stopKeepalive := startSetupKeepalive(ctx, session.Conn, keepaliveInterval(session))
	defer stopKeepalive()

	setupStarted := time.Now()
	helper, finish, err := s.startHelper(ctx, cfg,
		func() { site.HostNetworkModified = true },
		func(receipt client.Receipt) {
			s.checkRestored(&site, receipt, info, gatewayAddress)
		})
	if err != nil {
		return fail(err)
	}
	defer func() {
		receipt := finish()
		s.checkRestored(&site, receipt, info, gatewayAddress)
	}()
	s.reporter.Log("info", "helper", fmt.Sprintf("隧道配置完成，用时 %.1f 秒", time.Since(setupStarted).Seconds()))

	keepalive := 0
	json.Unmarshal(session.Common["keepalive_interval"], &keepalive)

	stats := &client.BridgeStats{}
	telemetryCtx, stopTelemetry := context.WithCancel(ctx)
	telemetryDone := make(chan struct{})
	go func() {
		defer close(telemetryDone)
		meter := client.NewTrafficMeter(time.Now())
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				s.reporter.Traffic(meter.Sample(now, stats))
			case <-telemetryCtx.Done():
				return
			}
		}
	}()

	probeCtx, stopProbes := context.WithTimeout(ctx, 25*time.Second)
	probeDone := make(chan []client.ProbeResult, 1)
	go func() {
		var results []client.ProbeResult
		if len(dns) > 0 {
			results = client.ProbeSites(probeCtx, dns[0])
		}
		if probeCtx.Err() == nil {
			s.reporter.Progress(client.ProbeSummary(results), "diagnostics", results)
		}
		probeDone <- results
	}()

	stopKeepalive()
	started := time.Now()
	bridgeErr := client.Bridge(ctx, session.Conn, session.Reader,
		&client.BufferedConn{Conn: helper, Reader: helper.Reader},
		time.Duration(keepalive)*time.Second, stats)
	stopTelemetry()
	<-telemetryDone
	s.reporter.Traffic(client.TrafficSample{
		UploadBytes:   stats.UploadBytes.Load(),
		DownloadBytes: stats.DownloadBytes.Load(),
	})
	s.reporter.Log("info", "network", "数据转发已停止，正在等待管理员 helper 恢复网络")
	stopProbes()
	site.ConnectivityProbes = <-probeDone
	site.Diagnostics = Diagnostics{
		DownloadBytes:      stats.DownloadBytes.Load(),
		UploadBytes:        stats.UploadBytes.Load(),
		ConnectedSeconds:   time.Since(started).Seconds(),
		PacketsSent:        stats.Sent.Load(),
		PacketsReceived:    stats.Received.Load(),
		HeartbeatsReceived: stats.Heartbeats.Load(),
	}
	site.Connected = true
	if bridgeErr != nil && !errors.Is(bridgeErr, context.Canceled) {
		site.ConnectionError = bridgeErr.Error()
		var closed *client.GatewayClosedError
		if errors.As(bridgeErr, &closed) {
			site.GatewayReason = int(closed.Reason)
		}
	}
	return site
}

// startHelper starts the privileged helper, authenticates it and configures
// the network. finish waits for the helper, reads its restore receipt and
// releases the endpoint; it is safe to call once, on every path.
func (s *Supervisor) startHelper(ctx context.Context, cfg client.NetworkConfig, onAuthorized func(), onCleanup func(client.Receipt)) (helper *client.BufferedConn, finish func() client.Receipt, err error) {
	conn, token, receiptPath, wait, cleanup, err := launchHelper(ctx, s.reporter, s.cfg.HelperPath)
	if err != nil {
		return nil, nil, err
	}

	var (
		once      sync.Once
		receipt   client.Receipt
		receiptMu sync.Mutex
	)
	finish = func() client.Receipt {
		once.Do(func() {
			conn.Close()
			if wait != nil {
				_ = wait()
			}
			receiptMu.Lock()
			receipt = readReceipt(receiptPath)
			receiptMu.Unlock()
			cleanup()
		})
		receiptMu.Lock()
		defer receiptMu.Unlock()
		return receipt
	}

	authorized := false
	failClean := func(err error) (*client.BufferedConn, func() client.Receipt, error) {
		r := finish()
		if authorized && onCleanup != nil {
			onCleanup(r)
		}
		return nil, nil, err
	}

	disconnect := context.AfterFunc(ctx, func() { conn.Close() })
	defer func() {
		if err != nil {
			disconnect()
		}
	}()

	conn.SetDeadline(time.Now().Add(90 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := client.ReadLine(reader, 256)
	if err != nil || !client.VerifyToken(strings.TrimSpace(string(line)), token) {
		return failClean(fmt.Errorf("helper authentication failed"))
	}
	s.reporter.Log("info", "helper", "管理员 helper 已通过握手，开始配置 utun、DNS 和路由")
	authorized = true
	if onAuthorized != nil {
		onAuthorized()
	}

	if err := json.NewEncoder(conn).Encode(cfg); err != nil {
		return failClean(err)
	}
	line, err = client.ReadLine(reader, 65536)
	if err != nil {
		return failClean(err)
	}
	var ready client.Receipt
	if err := json.Unmarshal(line, &ready); err != nil || !ready.Ready {
		return failClean(fmt.Errorf("网络配置失败：%s", ready.Error))
	}
	conn.SetDeadline(time.Time{})
	s.reporter.Connected(ready.Interface)

	// The helper keeps the endpoint open while it forwards traffic.
	release := finish
	finish = func() client.Receipt {
		disconnect()
		return release()
	}
	return &client.BufferedConn{Conn: conn, Reader: reader}, finish, nil
}

// readReceipt reads the helper's restore receipt, if it wrote one.
func readReceipt(path string) client.Receipt {
	var receipt client.Receipt
	if path == "" {
		return receipt
	}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &receipt)
	}
	return receipt
}

// checkRestored compares the host network with the snapshot taken before the
// tunnel and reports what could not be confirmed.
func (s *Supervisor) checkRestored(site *SiteResult, receipt client.Receipt, info client.Info, gateway string) {
	site.CleanupConfirmed = receipt.Clean
	site.NetworkRestoreFailures = receipt.Failures
	site.HelperError = receipt.Error

	after, err := client.TakeSnapshot(client.SystemRun, info.Service)
	site.ParentStateRestored = err == nil && reflect.DeepEqual(after, info.Before)

	router, iface, err := client.OuterRoute(client.SystemRun, gateway)
	site.OuterRouteRestored = err == nil && router == info.Router && iface == info.Interface

	level, message := "info", "网络恢复核对完成"
	if !site.CleanupConfirmed || !site.ParentStateRestored || !site.OuterRouteRestored {
		level, message = "warning", "网络恢复尚未全部确认，请检查连接结果"
	}
	s.reporter.Log(level, "cleanup", message)
}

// keepaliveInterval is the gateway's heartbeat period, 30 seconds when it
// does not say.
func keepaliveInterval(session *client.Session) time.Duration {
	var seconds int
	json.Unmarshal(session.Common["keepalive_interval"], &seconds)
	if seconds <= 0 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

// startSetupKeepalive keeps the tunnel session alive while the helper is
// authorized and the utun device is configured. It returns a function that
// stops the heartbeat and waits for it, and is safe to call more than once.
func startSetupKeepalive(ctx context.Context, conn net.Conn, interval time.Duration) func() {
	period := interval / 2
	if period < 2*time.Second {
		period = 2 * time.Second
	}
	if period > 15*time.Second {
		period = 15 * time.Second
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	send := func() bool {
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, err := conn.Write(client.ATP(1, 0))
		_ = conn.SetWriteDeadline(time.Time{})
		return err == nil
	}
	go func() {
		defer close(done)
		timer := time.NewTimer(min(period, time.Second))
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				if !send() {
					return
				}
				timer.Reset(period)
			case <-stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(stop)
			<-done
		})
	}
}

func tunnelDNS(raw json.RawMessage) ([]string, error) {
	var items []struct {
		IPv4 []json.RawMessage `json:"ipv4"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	var dns []string
	for _, item := range items {
		for _, value := range item.IPv4 {
			address, err := client.IPv4(value)
			if err != nil {
				return nil, err
			}
			dns = append(dns, address)
		}
	}
	if len(dns) > 4 {
		dns = dns[:4]
	}
	return dns, nil
}
