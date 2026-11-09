package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"open-tj.local/desktop/internal/client"
	"open-tj.local/desktop/internal/credentials"
	"open-tj.local/desktop/internal/logging"
	"open-tj.local/desktop/internal/settings"
	"open-tj.local/desktop/internal/vpn"
)

// Runner is the supervisor the controller drives. Tests replace it.
type Runner interface {
	Run(context.Context, client.Credentials)
	Sleep()
	Wake()
}

// Config is what the controller needs to run.
type Config struct {
	VPN       vpn.Config
	Log       *logging.Sink
	Store     credentials.Store
	Prefs     settings.Preferences
	SavePrefs func(settings.Preferences) error
	// SetOpenAtLogin registers or removes the login item. Defaults to
	// mygo.App.SetOpenAtLogin; tests replace it.
	SetOpenAtLogin func(bool) error
	// OpenAtLogin reports the login item as the system has it, which is the
	// truth when a user or another tool changed it. Defaults to
	// mygo.App.OpenAtLogin.
	OpenAtLogin func() bool
	// Notify shows a desktop notification. Defaults to a MyGo notification.
	// A test that wants silence sets it to a no-op.
	Notify func(title, body string)
	// Version is shown on the settings page. Empty reads mygo.App.Version,
	// which is empty outside a packaged app.
	Version string
	// Now is the clock the views and the probe age use. Defaults to time.Now.
	Now func() time.Time
	// NewRunner builds the supervisor. Defaults to vpn.New.
	NewRunner func(vpn.Config, vpn.Reporter) Runner
	// Redraw asks for a frame. Defaults to a no-op, as in tests.
	Redraw func()
	// Post runs a state change on the UI thread. Defaults to running it now.
	Post func(func())
	// LayoutChanged is called when ShowLog changed, so the app resizes the
	// window. Optional.
	LayoutChanged func(showLog bool)
	// OpenMain shows the main window from the menu bar panel. Optional.
	OpenMain func()
	// Quit quits the app from the menu bar panel. Optional.
	Quit func()
	// Supported reports whether this build can open a tunnel. Defaults to
	// vpn.Supported; tests of the connection flow set it true.
	Supported func() bool
}

// Account is the form's state.
type Account struct {
	Username string
	Password string
	Method   string
	Remember bool
}

// LogEntry is one line of the in-app log.
type LogEntry struct {
	ID        int
	Time      time.Time
	Level     string
	Component string
	Message   string
}

// Summary is the first line, for a collapsed entry.
func (e LogEntry) Summary() string {
	if strings.HasPrefix(e.Message, "{") {
		return "连接结果与网络恢复详情"
	}
	line, _, _ := strings.Cut(e.Message, "\n")
	return line
}

// HasDetails reports whether the entry has more to show than its summary.
func (e LogEntry) HasDetails() bool {
	return strings.Contains(e.Message, "\n") || len(e.Message) > 180
}

// Controller holds everything the windows show and drives the VPN supervisor.
// Its state is read and changed on the UI thread: the views run there, and
// reporter callbacks enter it through Post.
type Controller struct {
	cfg     Config
	account Account

	// UI state, owned by the main thread.
	// Page is the sidebar selection: overview, account, log or settings.
	Page         string
	ShowLog      bool
	ShowPassword bool
	FollowLogs   bool
	LogLevel     string
	LogSearch    string
	Expanded     map[int]bool
	// SplitWidth is the dashboard pane's width in DIPs when the log shows.
	SplitWidth float32
	LogList    ui.ListState

	active    bool
	stopping  bool
	connected bool
	// tunnelDNS is the resolver the gateway handed the tunnel, so a manual
	// connectivity check can compare it with the system's.
	tunnelDNS []string
	// remembered is the saved "keep the password" choice: the keychain is read
	// on the first connection, not at launch.
	remembered      bool
	credentialError string
	// lastRates is what each traffic card showed in the frame before, for
	// rolling its number the direction it moved.
	lastRates   map[string]float64
	connectedAt time.Time
	status      string
	detail      string
	iface       string

	// probes is the latest connectivity check, written only inside post.
	probes   []client.ProbeResult
	probesAt time.Time
	checking bool

	// loginError is why the last change of the login item failed.
	loginError string

	uploadBPS     float64
	downloadBPS   float64
	uploadBytes   int64
	downloadBytes int64
	uploadHistory []float64
	downHistory   []float64

	entries   []LogEntry
	nextID    int
	logText   strings.Builder
	runner    Runner
	cancel    context.CancelFunc
	onStopped func()

	redraw func()
	post   func(func())
}

// New creates a controller. It loads preferences and credentials; a keychain
// error is logged rather than fatal, so the user can still connect.
func New(cfg Config) *Controller {
	if cfg.NewRunner == nil {
		cfg.NewRunner = func(v vpn.Config, r vpn.Reporter) Runner { return vpn.New(v, r) }
	}
	if cfg.Redraw == nil {
		cfg.Redraw = func() {}
	}
	if cfg.Post == nil {
		cfg.Post = func(fn func()) { fn() }
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.SetOpenAtLogin == nil {
		cfg.SetOpenAtLogin = mygo.App.SetOpenAtLogin
	}
	if cfg.OpenAtLogin == nil {
		cfg.OpenAtLogin = mygo.App.OpenAtLogin
	}
	if cfg.Supported == nil {
		cfg.Supported = vpn.Supported
	}
	prefs := cfg.Prefs
	prefs.Normalize()
	page := pageOverview
	if strings.TrimSpace(prefs.Username) == "" {
		page = pageAccount
	}
	c := &Controller{
		cfg:        cfg,
		account:    Account{Username: prefs.Username, Method: prefs.Method, Remember: prefs.Remember},
		FollowLogs: true,
		LogList:    ui.ListState{FollowEnd: true},
		LogLevel:   "all",
		Expanded:   map[int]bool{},
		SplitWidth: prefs.SplitWidth,
		ShowLog:    prefs.ShowLog,
		Page:       page,
		status:     "未连接",
		detail:     "输入同济 VPN 账号后连接",
		redraw:     cfg.Redraw,
		post:       cfg.Post,
	}
	if prefs.ShowLog && page == pageOverview {
		c.Page = pageLog
	}
	// Reading the keychain makes macOS ask for the user's login password the
	// first time, so it waits until a password is actually needed: at launch
	// the window opens with no system prompt in front of it.
	c.remembered = prefs.Remember
	if c.cfg.Notify == nil {
		c.cfg.Notify = func(title, body string) {
			n := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body})
			go func() { _ = n.Show() }()
		}
	}
	return c
}

// Account returns the form's state for binding.
func (c *Controller) Account() *Account { return &c.account }

// Preferences returns the preferences to persist.
func (c *Controller) Preferences() settings.Preferences {
	saved := c.cfg.Prefs
	saved.Username = c.account.Username
	saved.Method = c.account.Method
	saved.Remember = c.account.Remember
	saved.ShowLog = c.ShowLog
	saved.SplitWidth = c.SplitWidth
	return saved
}

// SetOpenAtLogin turns the login item on or off and remembers the choice.
// A failure is kept for the settings page to show, and the choice is not saved.
func (c *Controller) SetOpenAtLogin(on bool) {
	err := c.cfg.SetOpenAtLogin(on)
	if err != nil {
		c.loginError = "无法更改开机启动：" + err.Error()
		c.addLog("error", "app", c.loginError)
		c.redraw()
		return
	}
	c.loginError = ""
	c.cfg.Prefs.OpenAtLogin = on
	c.Persist()
	c.redraw()
}

// SetNotifyOnConnect remembers whether a new connection should notify.
func (c *Controller) SetNotifyOnConnect(on bool) {
	c.cfg.Prefs.DisableNotify = !on
	c.Persist()
}

// SetRemember keeps or forgets the password, and makes the rest of this run
// follow the new choice without waiting for a relaunch.
func (c *Controller) SetRemember(on bool) {
	c.account.Remember = on
	c.remembered = on
	c.Persist()
	c.redraw()
}

// SetAutoConnect remembers whether launch should connect on its own.
func (c *Controller) SetAutoConnect(on bool) {
	c.cfg.Prefs.AutoConnect = on
	c.Persist()
}

// LoginError is the last failure to change the login item, or empty.
func (c *Controller) LoginError() string { return c.loginError }

// OpenAtLogin reports the login item as the system has it. The saved
// preference is only a cache: the user may remove the login item in System
// Settings, and the switch must then show the truth.
func (c *Controller) OpenAtLogin() bool {
	return c.cfg.OpenAtLogin()
}

// NotifyOnConnect reports the saved choice. An old file, which never stored
// one, reports true.
func (c *Controller) NotifyOnConnect() bool { return c.cfg.Prefs.NotifyOnConnect() }

// AutoConnect reports the saved choice.
func (c *Controller) AutoConnect() bool { return c.cfg.Prefs.AutoConnect }

// Version is the version the settings page shows.
func (c *Controller) Version() string {
	if c.cfg.Version != "" {
		return c.cfg.Version
	}
	return mygo.App.Version()
}

// Gateway is the host the session authenticates with.
func (c *Controller) Gateway() string {
	if c.cfg.VPN.Gateway != "" {
		return c.cfg.VPN.Gateway
	}
	return vpn.DefaultGateway
}

// Interface is the tunnel interface, once one is up.
func (c *Controller) Interface() string { return c.iface }

// Probes is the latest connectivity check and when it finished.
func (c *Controller) Probes() ([]client.ProbeResult, time.Time) {
	return c.probes, c.probesAt
}

// Checking reports a connectivity check in progress.
func (c *Controller) Checking() bool { return c.checking }

// RefreshProbes asks for a new connectivity check. It does nothing unless a
// tunnel is up, and it never blocks the UI thread.
func (c *Controller) RefreshProbes() {
	if !c.connected || c.checking {
		return
	}
	c.checking = true
	c.redraw()
	dns := c.tunnelDNS
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		results := client.ProbeSitesWith(ctx, net.DefaultResolver, tunnelResolver(dns))
		c.post(func() {
			c.checking = false
			if ctx.Err() == nil {
				c.probes = results
				c.probesAt = c.cfg.Now()
			}
			c.redraw()
		})
	}()
}

// tunnelResolver dials the first resolver the gateway supplied. Without one
// it falls back to the system's, so a refresh still measures the HTTPS path.
func tunnelResolver(dns []string) *net.Resolver {
	if len(dns) == 0 {
		return net.DefaultResolver
	}
	addr := dns[0]
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "53")
	}
	return &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}}
}

// MaybeAutoConnect connects at launch when the user asked for it and the
// saved account is complete. A missing password is logged, not an error.
func (c *Controller) MaybeAutoConnect() {
	if !c.cfg.Prefs.AutoConnect {
		return
	}
	if !c.account.Remember || strings.TrimSpace(c.account.Username) == "" || c.account.Password == "" {
		c.addLog("info", "app", "已开启启动时自动连接，但没有保存完整的账号，已跳过")
		return
	}
	c.Connect()
}

// Persist saves preferences and, when remembering, the password.
func (c *Controller) Persist() {
	prefs := c.Preferences()
	if c.cfg.SavePrefs != nil {
		_ = c.cfg.SavePrefs(prefs)
	}
	if !c.account.Remember {
		if c.cfg.Store != nil {
			_ = c.cfg.Store.Delete()
		}
		return
	}
	if c.cfg.Store != nil && c.account.Password != "" {
		if err := c.cfg.Store.Save(credentials.Credentials{
			Username: c.account.Username,
			Password: c.account.Password,
		}); err != nil {
			c.addLog("error", "app", "无法保存钥匙串凭据："+err.Error())
		}
	}
}

// BackgroundCloseExplained reports whether the panel was explained already.
func (c *Controller) BackgroundCloseExplained() bool {
	return c.cfg.Prefs.BackgroundCloseExplained
}

// MarkBackgroundCloseExplained remembers the explanation was shown.
func (c *Controller) MarkBackgroundCloseExplained() {
	c.cfg.Prefs.BackgroundCloseExplained = true
	if c.cfg.SavePrefs != nil {
		_ = c.cfg.SavePrefs(c.Preferences())
	}
}

// LoadCredentials retries reading the keychain after a launch-time failure.
func (c *Controller) LoadCredentials() { c.loadCredentials() }

// NeedsCredential reports whether a saved password would spare the user typing
// one, so the account page can offer to load it.
func (c *Controller) NeedsCredential() bool {
	return c.remembered && c.account.Password == ""
}

// CredentialError is the reason the last keychain read failed, for the account
// page to show beside the button that retries it.
func (c *Controller) CredentialError() string { return c.credentialError }

func (c *Controller) loadCredentials() {
	if c.cfg.Store == nil {
		return
	}
	stored, found, err := c.cfg.Store.Load()
	if err != nil {
		c.credentialError = "无法读取钥匙串：" + err.Error()
		c.addLog("warning", "app", c.credentialError)
		return
	}
	c.credentialError = ""
	if found {
		c.account.Username = stored.Username
		c.account.Password = stored.Password
	}
}

// LogText is the plain-text log for copying.
func (c *Controller) LogText() string { return c.logText.String() }

// Status is the headline state.
func (c *Controller) Status() string { return c.status }

// Detail explains the state under the headline.
func (c *Controller) Detail() string { return c.detail }

// MissingCredential says which of the username and password is still empty,
// or nothing when both are filled in.
func (c *Controller) MissingCredential() string {
	switch {
	case strings.TrimSpace(c.account.Username) == "" && c.account.Password == "":
		return "还缺用户名和密码"
	case strings.TrimSpace(c.account.Username) == "":
		return "还缺用户名"
	default:
		if c.account.Password == "" {
			return "还缺密码"
		}
		return ""
	}
}

// Active reports a running session, including while disconnecting.
func (c *Controller) Active() bool { return c.active }

// Stopping reports that the current session is being shut down.
func (c *Controller) Stopping() bool { return c.stopping }

// IsConnected reports an established tunnel.
func (c *Controller) IsConnected() bool { return c.connected }

// ConnectedAt is when the tunnel came up, or the zero time.
func (c *Controller) ConnectedAt() time.Time { return c.connectedAt }

// TrafficSnapshot is the traffic the views and the tray show.
type TrafficSnapshot struct {
	UploadBPS     float64
	DownloadBPS   float64
	UploadBytes   int64
	DownloadBytes int64
	UploadHistory []float64
	DownHistory   []float64
}

// displayedRate remembers what each traffic card last showed, keyed by its
// title, so the number can roll in the direction it actually moved. Views run
// on the UI thread, which is the only place this map is written.
func (c *Controller) displayedRate(title string) float64 {
	if c.lastRates == nil {
		c.lastRates = map[string]float64{}
	}
	return c.lastRates[title]
}

// noteDisplayedRate records the rate a card showed this frame.
func (c *Controller) noteDisplayedRate(title string, rate float64) {
	if c.lastRates == nil {
		c.lastRates = map[string]float64{}
	}
	c.lastRates[title] = rate
}

// Snapshot returns the current rates, totals and 60-sample histories.
func (c *Controller) Snapshot() TrafficSnapshot {
	return TrafficSnapshot{
		UploadBPS:     c.uploadBPS,
		DownloadBPS:   c.downloadBPS,
		UploadBytes:   c.uploadBytes,
		DownloadBytes: c.downloadBytes,
		UploadHistory: c.uploadHistory,
		DownHistory:   c.downHistory,
	}
}

// Entries returns the in-app log.
func (c *Controller) Entries() []LogEntry { return c.entries }

// FilteredEntries returns the entries matching the level filter and search.
func (c *Controller) FilteredEntries() []LogEntry {
	out := make([]LogEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		if c.LogLevel != "all" && entry.Level != c.LogLevel {
			continue
		}
		if c.LogSearch != "" && !matchesSearch(entry, c.LogSearch) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// matchesSearch reports whether an entry mentions the query in any of the
// fields the log shows or copies: the message, the component and the level.
func matchesSearch(entry LogEntry, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	for _, field := range []string{entry.Message, entry.Component, entry.Level} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

// Counts returns the number of warnings and errors.
func (c *Controller) Counts() (warnings, errors int) {
	for _, entry := range c.entries {
		switch entry.Level {
		case "warning":
			warnings++
		case "error":
			errors++
		}
	}
	return warnings, errors
}

// supported reports whether this build can open a tunnel.
func (c *Controller) supported() bool {
	if c.cfg.Supported != nil {
		return c.cfg.Supported()
	}
	return vpn.Supported()
}

// Connect starts a session. It reports false when the form is incomplete.
func (c *Controller) Connect() bool {
	if c.active {
		return false
	}
	if !c.supported() {
		c.status = "暂不可用"
		c.detail = "这个版本不能建立 VPN 连接。"
		c.addLog("warning", "app", c.detail)
		c.redraw()
		return false
	}
	c.account.Username = strings.TrimSpace(c.account.Username)
	if c.account.Password == "" && c.remembered {
		c.loadCredentials()
	}
	if missing := c.MissingCredential(); missing != "" {
		c.detail = missing
		c.redraw()
		return false
	}
	c.Persist()

	c.active = true
	c.stopping = false
	c.connected = false
	c.connectedAt = time.Time{}
	c.iface = ""
	c.probes = nil
	c.probesAt = time.Time{}
	c.checking = false
	c.status = "正在连接"
	c.detail = "正在验证 VPN 账号…"
	c.resetTotals()
	c.addLog("info", "app", "开始连接 vpn.tongji.cn · 认证方式 "+c.account.Method)

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.runner = c.cfg.NewRunner(c.cfg.VPN, c)
	runner := c.runner
	cred := client.Credentials{
		Username: c.account.Username,
		Password: c.account.Password,
		Method:   c.account.Method,
	}
	c.redraw()
	go func() {
		runner.Run(ctx, cred)
		c.post(c.finishSession)
	}()
	return true
}

// Disconnect asks the current session to stop and restore the network.
func (c *Controller) Disconnect() {
	if !c.active || c.stopping {
		return
	}
	c.stopping = true
	c.status = "正在断开"
	c.detail = "正在关闭隧道并恢复网络设置…"
	c.addLog("info", "app", "收到断开请求，等待网络恢复")
	if c.cancel != nil {
		c.cancel()
	}
	c.redraw()
}

// RequestQuit disconnects first, then calls quit. Without a session it calls
// quit at once.
func (c *Controller) RequestQuit(quit func()) {
	if !c.active {
		quit()
		return
	}
	c.onStopped = quit
	if !c.stopping {
		c.Disconnect()
	}
}

// Sleep closes the session and pauses reconnects.
func (c *Controller) Sleep() {
	if c.runner != nil {
		c.runner.Sleep()
	}
}

// Wake resumes reconnects after Sleep.
func (c *Controller) Wake() {
	if c.runner != nil {
		c.runner.Wake()
	}
}

// ShowLogToggle opens the log page, or the overview when it is already
// open. ⌘L does this.
func (c *Controller) ShowLogToggle() {
	if c.Page == pageLog {
		c.choosePage(pageOverview)
	} else {
		c.choosePage(pageLog)
	}
	c.redraw()
}

// SetLogLevel filters the log by level ("all", "info", "warning", "error").
func (c *Controller) SetLogLevel(level string) { c.LogLevel = level }

// ToggleExpanded expands or collapses a log entry.
func (c *Controller) ToggleExpanded(id int) {
	if !c.Expanded[id] {
		c.Expanded[id] = true
		return
	}
	delete(c.Expanded, id)
}

// ClearLog empties the in-app log.
// OpenMain shows the main window from the menu bar.
func (c *Controller) OpenMain() {
	if c.cfg.OpenMain != nil {
		c.cfg.OpenMain()
	}
}

// Quit quits the app, disconnecting first.
func (c *Controller) Quit() {
	if c.cfg.Quit != nil {
		c.cfg.Quit()
	}
}

// ClearLog empties the in-app log.
func (c *Controller) ClearLog() {
	c.entries = nil
	c.logText.Reset()
	c.Expanded = map[int]bool{}
}

func (c *Controller) finishSession() {
	c.active = false
	c.stopping = false
	c.connected = false
	c.connectedAt = time.Time{}
	c.iface = ""
	c.checking = false
	c.tunnelDNS = nil
	// The probes measured the tunnel that just went down, so they say nothing
	// about the next one: showing them next to "未连接" would be a lie.
	c.probes = nil
	c.probesAt = time.Time{}
	c.cancel = nil
	c.runner = nil
	if cb := c.onStopped; cb != nil {
		c.onStopped = nil
		cb()
	}
}

// AddLog appends a line to the in-app log and asks for a frame.
func (c *Controller) AddLog(level, component, message string) {
	c.addLog(level, component, message)
	c.redraw()
}

func (c *Controller) addLog(level, component, message string) {
	if c.nextID == 0 {
		c.nextID = 1
	}
	entry := LogEntry{ID: c.nextID, Time: time.Now(), Level: level, Component: component, Message: message}
	c.nextID++
	c.entries = append(c.entries, entry)
	if len(c.entries) > maxLogEntries {
		drop := len(c.entries) - maxLogEntries
		for _, old := range c.entries[:drop] {
			delete(c.Expanded, old.ID)
		}
		c.entries = append([]LogEntry(nil), c.entries[drop:]...)
	}
	c.logText.WriteString("[")
	c.logText.WriteString(entry.Time.Format("15:04:05"))
	c.logText.WriteString("] [")
	c.logText.WriteString(strings.ToUpper(level))
	c.logText.WriteString("] ")
	c.logText.WriteString(message)
	c.logText.WriteByte('\n')
	if c.logText.Len() > 64_000 {
		text := c.logText.String()
		c.logText.Reset()
		c.logText.WriteString(text[len(text)-48_000:])
	}
}

const maxLogEntries = 2000

func (c *Controller) resetRates() {
	c.uploadBPS, c.downloadBPS = 0, 0
}

func (c *Controller) resetTotals() {
	c.resetRates()
	c.uploadBytes, c.downloadBytes = 0, 0
	c.uploadHistory, c.downHistory = nil, nil
}

// ---- vpn.Reporter ----

// Connecting implements vpn.Reporter.
func (c *Controller) Connecting() {
	c.post(func() {
		if !c.stopping {
			c.status = "正在连接"
			c.detail = "正在认证并读取隧道配置…"
		}
		c.redraw()
	})
	c.cfg.Log.Write("connecting", nil)
}

// Progress implements vpn.Reporter.
func (c *Controller) Progress(message, component string, probes []client.ProbeResult) {
	c.post(func() {
		if component == "diagnostics" && probes != nil {
			c.probes = probes
			c.probesAt = c.cfg.Now()
			c.checking = false
		}
		if !c.stopping && !c.connected {
			c.detail = message
		}
		c.addLog("info", component, message)
		c.redraw()
	})
	fields := map[string]any{"message": message, "component": component}
	if component == "diagnostics" && probes != nil {
		fields["probes"] = probes
	}
	c.cfg.Log.Write("progress", fields)
}

// Connected implements vpn.Reporter.
func (c *Controller) Connected(iface string, dns []string) {
	c.post(func() {
		c.addLog("info", "app", "隧道已连接 · "+iface+" · IPv4")
		c.tunnelDNS = append([]string(nil), dns...)
		became := !c.connected && !c.stopping
		if !c.stopping {
			c.status = "已连接"
			c.connected = true
			c.connectedAt = c.cfg.Now()
			c.iface = iface
			c.detail = "IPv4 流量通过 VPN · " + iface
		}
		if became && c.cfg.Prefs.NotifyOnConnect() && c.cfg.Notify != nil {
			c.cfg.Notify("Open TJ", "已连接同济校园网 · "+Duration(c.connectedAt, c.cfg.Now()))
		}
		c.redraw()
	})
	c.cfg.Log.Write("connected", map[string]any{"interface": iface, "dns": dns})
}

// Reconnecting implements vpn.Reporter.
func (c *Controller) Reconnecting(seconds int) {
	c.post(func() {
		c.resetRates()
		c.connected = false
		c.connectedAt = time.Time{}
		c.iface = ""
		// The tunnel is gone, so are the measurements through it.
		c.probes = nil
		c.probesAt = time.Time{}
		c.checking = false
		c.tunnelDNS = nil
		c.addLog("warning", "app", "连接中断，稍后重试")
		if !c.stopping {
			c.status = "等待重连"
			c.detail = fmt.Sprintf("将在 %d 秒后重新连接", seconds)
		}
		c.redraw()
	})
	c.cfg.Log.Write("reconnecting", map[string]any{"level": "warning", "seconds": seconds})
}

// Sleeping implements vpn.Reporter.
func (c *Controller) Sleeping() {
	c.post(func() {
		c.resetRates()
		c.connected = false
		c.connectedAt = time.Time{}
		c.iface = ""
		c.addLog("info", "app", "系统睡眠，连接已暂停")
		if !c.stopping {
			c.status = "已暂停"
			c.detail = "唤醒电脑后自动连接"
		}
		c.redraw()
	})
	c.cfg.Log.Write("sleeping", nil)
}

// Failed implements vpn.Reporter.
func (c *Controller) Failed(message string) {
	c.post(func() {
		c.status = "连接失败"
		c.detail = message
		c.addLog("error", "app", message)
		c.redraw()
	})
	c.cfg.Log.Write("failed", map[string]any{"level": "error", "message": message})
}

// Result implements vpn.Reporter.
func (c *Controller) Result(result vpn.Result) {
	c.post(func() {
		c.resetRates()
		c.connected = false
		c.connectedAt = time.Time{}
		c.iface = ""
		c.status, c.detail = result.Summary()
		if result.Site != nil && result.Site.GatewayReason != 0 {
			c.addLog("warning", "gateway", vpn.GatewayMessage(result.Site.GatewayReason))
		}
		if data, err := json.MarshalIndent(result, "", "  "); err == nil {
			c.addLog(result.Level(), "result", string(data))
		}
		c.redraw()
	})
	c.cfg.Log.Write("result", map[string]any{"level": result.Level(), "result": result})
}

// Traffic implements vpn.Reporter.
func (c *Controller) Traffic(sample client.TrafficSample) {
	c.post(func() {
		c.uploadBPS, c.downloadBPS = sample.UploadBPS, sample.DownloadBPS
		c.uploadBytes, c.downloadBytes = sample.UploadBytes, sample.DownloadBytes
		c.uploadHistory = appendHistory(c.uploadHistory, sample.UploadBPS)
		c.downHistory = appendHistory(c.downHistory, sample.DownloadBPS)
		c.redraw()
	})
}

// Log implements vpn.Reporter.
func (c *Controller) Log(level, component, message string) {
	c.post(func() {
		c.addLog(level, component, message)
		c.redraw()
	})
	c.cfg.Log.Write("log", map[string]any{"level": level, "component": component, "message": message})
}

func appendHistory(history []float64, value float64) []float64 {
	history = append(history, value)
	if len(history) > historyLength {
		history = append([]float64(nil), history[len(history)-historyLength:]...)
	}
	return history
}

const historyLength = 60
