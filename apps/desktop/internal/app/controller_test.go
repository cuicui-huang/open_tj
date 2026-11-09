package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"open-tj.local/desktop/internal/client"
	"open-tj.local/desktop/internal/credentials"
	"open-tj.local/desktop/internal/settings"
	"open-tj.local/desktop/internal/vpn"
)

type memStore struct {
	mu      sync.Mutex
	saved   credentials.Credentials
	found   bool
	deleted bool
}

func (s *memStore) Load() (credentials.Credentials, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saved, s.found, nil
}
func (s *memStore) Save(c credentials.Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved, s.found = c, true
	return nil
}
func (s *memStore) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = true
	return nil
}

type fakeRunner struct {
	started chan client.Credentials
	done    chan struct{}
	once    sync.Once
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{started: make(chan client.Credentials, 1), done: make(chan struct{})}
}

func (f *fakeRunner) Run(ctx context.Context, cred client.Credentials) {
	f.started <- cred
	<-ctx.Done()
	f.once.Do(func() { close(f.done) })
}
func (f *fakeRunner) Sleep() {}
func (f *fakeRunner) Wake()  {}

// postQueue stands in for the window: it runs state changes on the test
// goroutine, as Window.Update runs them on the UI thread.
type postQueue struct {
	mu  sync.Mutex
	fns []func()
}

func (q *postQueue) Post(fn func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.fns = append(q.fns, fn)
}

func (q *postQueue) Drain() {
	for {
		q.mu.Lock()
		fns := q.fns
		q.fns = nil
		q.mu.Unlock()
		if len(fns) == 0 {
			return
		}
		for _, fn := range fns {
			fn()
		}
	}
}

func newTestController(t *testing.T, store credentials.Store, runner Runner) (*Controller, *postQueue) {
	t.Helper()
	q := &postQueue{}
	c := New(Config{
		Store:     store,
		Post:      q.Post,
		Supported: func() bool { return true },
		NewRunner: func(vpn.Config, vpn.Reporter) Runner {
			if runner == nil {
				return newFakeRunner()
			}
			return runner
		},
		SavePrefs: func(settings.Preferences) error { return nil },
		// The real calls need the app's main thread, which a test has not:
		// they would block forever.
		SetOpenAtLogin: func(bool) error { return nil },
		OpenAtLogin:    func() bool { return false },
	})
	t.Cleanup(func() {
		if c.cancel != nil {
			c.cancel()
		}
	})
	return c, q
}

func waitFor(t *testing.T, q *postQueue, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		q.Drain()
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met")
}

func TestConnectRequiresAccount(t *testing.T) {
	c, _ := newTestController(t, &memStore{}, nil)
	if c.Connect() {
		t.Fatal("connected without credentials")
	}
	if c.Active() {
		t.Fatal("active without credentials")
	}
}

func TestConnectDisconnectAndQuit(t *testing.T) {
	store := &memStore{}
	runner := newFakeRunner()
	c, q := newTestController(t, store, runner)
	c.Account().Username = "  student  "
	c.Account().Password = "secret"
	c.Account().Remember = true

	if !c.Connect() {
		t.Fatal("Connect returned false")
	}
	cred := <-runner.started
	if cred.Username != "student" || cred.Password != "secret" || cred.Method != settings.DefaultMethod {
		t.Fatalf("credentials %+v", cred)
	}
	if !store.found || store.saved.Username != "student" {
		t.Fatalf("saved credentials %+v", store.saved)
	}
	if !c.Active() || c.Stopping() || c.Status() != "正在连接" {
		t.Fatalf("state active=%v stopping=%v status=%q", c.Active(), c.Stopping(), c.Status())
	}

	quit := make(chan struct{})
	c.RequestQuit(func() { close(quit) })
	if !c.Stopping() {
		t.Fatal("RequestQuit did not disconnect")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		q.Drain()
		select {
		case <-quit:
		default:
			if time.Now().After(deadline) {
				t.Fatal("quit not called after the session ended")
			}
			time.Sleep(5 * time.Millisecond)
			continue
		}
		break
	}
	if c.Active() {
		t.Fatal("still active after quit")
	}
}

func TestDisconnectWithoutRememberDeletesSecret(t *testing.T) {
	store := &memStore{saved: credentials.Credentials{Username: "u", Password: "p"}, found: true}
	c, _ := newTestController(t, store, newFakeRunner())
	c.Account().Username = "u"
	c.Account().Password = "p"
	c.Account().Remember = false
	c.Persist()
	if !store.deleted {
		t.Fatal("secret not deleted")
	}
}

func TestReporterUpdatesStateAndTraffic(t *testing.T) {
	c, q := newTestController(t, &memStore{}, nil)
	c.Connecting()
	q.Drain()
	if c.Status() != "正在连接" {
		t.Fatalf("status %q", c.Status())
	}
	c.Connected("utun7", []string{"10.5.0.1"})
	q.Drain()
	if !c.IsConnected() || c.ConnectedAt().IsZero() || !strings.Contains(c.Detail(), "utun7") {
		t.Fatalf("connected=%v detail=%q", c.IsConnected(), c.Detail())
	}
	for i := 0; i < 70; i++ {
		c.Traffic(client.TrafficSample{DownloadBPS: float64(i), UploadBPS: 1, DownloadBytes: int64(i)})
	}
	q.Drain()
	snapshot := c.Snapshot()
	if len(snapshot.DownHistory) != historyLength || len(snapshot.UploadHistory) != historyLength {
		t.Fatalf("history %d/%d", len(snapshot.DownHistory), len(snapshot.UploadHistory))
	}
	if snapshot.DownloadBPS != 69 {
		t.Fatalf("rate %v", snapshot.DownloadBPS)
	}
	c.Reconnecting(2)
	q.Drain()
	if c.IsConnected() || c.Status() != "等待重连" {
		t.Fatalf("reconnecting state %q", c.Status())
	}
	c.Sleeping()
	q.Drain()
	if c.Status() != "已暂停" {
		t.Fatalf("sleeping state %q", c.Status())
	}
	c.Result(vpn.Result{Error: "boom"})
	q.Drain()
	if c.Status() != "连接失败" || c.Detail() != "boom" {
		t.Fatalf("result state %q %q", c.Status(), c.Detail())
	}
}

func TestLogBoundsFilterAndExpansion(t *testing.T) {
	c, _ := newTestController(t, &memStore{}, nil)
	c.AddLog("warning", "backend", "retry test")
	c.AddLog("error", "backend", "boom")
	c.AddLog("info", "app", "hello\nmore")

	if got := len(c.FilteredEntries()); got != 3 {
		t.Fatalf("entries %d", got)
	}
	c.LogLevel = "warning"
	if got := c.FilteredEntries(); len(got) != 1 || got[0].Message != "retry test" {
		t.Fatalf("filtered %+v", got)
	}
	c.LogLevel = "all"
	c.LogSearch = "boom"
	if got := c.FilteredEntries(); len(got) != 1 {
		t.Fatalf("search %+v", got)
	}
	c.LogSearch = ""

	for _, entry := range c.Entries() {
		if entry.HasDetails() {
			c.ToggleExpanded(entry.ID)
		}
	}
	old := c.Entries()[0].ID
	for i := 0; i < maxLogEntries+1; i++ {
		c.AddLog("info", "app", "filler")
	}
	if len(c.Entries()) != maxLogEntries {
		t.Fatalf("entries %d, want %d", len(c.Entries()), maxLogEntries)
	}
	if c.Expanded[old] {
		t.Fatal("expanded entry survived eviction")
	}
	c.ClearLog()
	if len(c.Entries()) != 0 || c.LogText() != "" {
		t.Fatalf("log not cleared: %d entries", len(c.Entries()))
	}
}

func TestViewRendersAndConnects(t *testing.T) {
	runner := newFakeRunner()
	c, q := newTestController(t, &memStore{}, runner)
	c.cfg.Prefs.Username = "student"
	c.Account().Username = "student"
	c.Account().Password = "secret"
	c.choosePage(pageOverview)

	tt := ui.NewTester(c.ViewMain, 860, 640)
	if !tt.HasText("未连接") || !tt.HasText("连通性") {
		t.Fatalf("initial texts %q", tt.Texts())
	}
	if err := tt.Click("连接 VPN"); err != nil {
		t.Fatal(err)
	}
	<-runner.started
	tt.Frame()
	if !tt.HasText("正在连接") {
		t.Fatalf("connecting texts %q", tt.Texts())
	}

	c.Connected("utun7", []string{"10.5.0.1"})
	c.Traffic(client.TrafficSample{DownloadBPS: 4096, UploadBPS: 1280, DownloadBytes: 8192})
	q.Drain()
	tt.Frame()
	if !tt.HasText("已连接") {
		t.Fatalf("connected texts %q", tt.Texts())
	}
	if !tt.HasText("4") || !tt.HasText("1.25") {
		t.Fatalf("rate texts %q", tt.Texts())
	}

	c.AddLog("warning", "backend", "retry test")
	c.ShowLogToggle()
	q.Drain()
	tt.Frame()
	if !tt.HasText("日志") || !tt.HasText("retry test") {
		t.Fatalf("log texts %q", tt.Texts())
	}
	c.LogLevel = "error"
	q.Drain()
	tt.Frame()
	if tt.HasText("retry test") {
		t.Fatalf("filtered texts still show entry: %q", tt.Texts())
	}
	c.LogLevel = "all"
	tt.SetSize(860, 640)
	q.Drain()
	tt.Frame()

	// Disconnect lives on the overview, which the log page covers.
	c.ShowLogToggle()
	tt.Frame()
	if err := tt.Click("断开"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, func() bool { return !c.Active() })
	q.Drain()
	tt.Frame()
	if tt.HasText("恢复中…") {
		t.Fatalf("still stopping: %q", tt.Texts())
	}
	if !strings.Contains(c.LogText(), "收到断开请求") {
		t.Fatalf("disconnect not logged: %q", c.LogText())
	}
}

func TestProgressShowsProbeCard(t *testing.T) {
	c, q := newTestController(t, &memStore{}, nil)
	c.cfg.Prefs.Username = "student"
	c.Account().Username = "student"
	c.Account().Password = "secret"
	c.choosePage(pageOverview)

	view := ui.NewTester(c.ViewMain, 940, 700)
	if !view.HasText("连接后自动检测校园网连通性") {
		t.Fatalf("idle overview %q", view.Texts())
	}
	if view.HasText("IPv4 · 同济大学") {
		t.Fatal("sidebar still shows the hardcoded footer")
	}

	c.Progress("连接诊断：…", "diagnostics", []client.ProbeResult{{
		URL:         "https://xlcp.tongji.edu.cn/user/web-v2",
		SystemDNSMS: 11, TCPMS: 24, TLSMS: 58, Status: 200,
		Redirects: []client.ProbeResult{{
			URL: "https://ids.tongji.edu.cn/login", TotalMS: 40, Status: 200,
		}},
	}})
	q.Drain()
	view.Frame()
	if !view.HasText("连通性") || !view.HasText("xlcp.tongji.edu.cn") {
		t.Fatalf("probe card %q", view.Texts())
	}
	if !view.HasText("ids.tongji.edu.cn") {
		t.Fatalf("redirect not shown: %q", view.Texts())
	}
	if view.HasText("连接后自动检测校园网连通性") {
		t.Fatal("empty state survived a real probe")
	}
}

func TestSettingsTogglesPersist(t *testing.T) {
	var saved settings.Preferences
	c, _ := newTestController(t, &memStore{}, nil)
	c.cfg.SavePrefs = func(p settings.Preferences) error { saved = p; return nil }
	c.cfg.SetOpenAtLogin = func(on bool) error {
		if on {
			return nil
		}
		return nil
	}

	c.SetOpenAtLogin(true)
	if !saved.OpenAtLogin || c.LoginError() != "" {
		t.Fatalf("login item %+v err %q", saved, c.LoginError())
	}
	c.cfg.SetOpenAtLogin = func(bool) error { return errLogin }
	c.SetOpenAtLogin(false)
	if !saved.OpenAtLogin || c.LoginError() == "" {
		t.Fatalf("a failed change was saved: %+v %q", saved, c.LoginError())
	}

	c.SetNotifyOnConnect(false)
	if saved.NotifyOnConnect() {
		t.Fatal("notify toggle did not persist")
	}
	c.SetAutoConnect(true)
	if !saved.AutoConnect {
		t.Fatal("auto connect toggle did not persist")
	}

	tt := ui.NewTester(c.ViewMain, 940, 700)
	c.choosePage(pageSettings)
	tt.Frame()
	if !tt.HasText("开机时启动") || !tt.HasText("连接成功后发送通知") || !tt.HasText("启动时自动连接") {
		t.Fatalf("settings page %q", tt.Texts())
	}
	if !tt.HasText(c.LoginError()) {
		t.Fatalf("login error not shown: %q", tt.Texts())
	}
}

var errLogin = loginError("系统拒绝了这个登录项")

type loginError string

func (e loginError) Error() string { return string(e) }

func TestFirstLaunchOpensAccount(t *testing.T) {
	blank, _ := newTestController(t, &memStore{}, nil)
	if blank.Page != pageAccount {
		t.Fatalf("page %q, want the account page", blank.Page)
	}
	tt := ui.NewTester(blank.ViewMain, 860, 640)
	if !tt.HasText("首次使用？用你的学号和校园网密码登录") {
		t.Fatalf("first run %q", tt.Texts())
	}

	known := New(Config{
		Prefs: settings.Preferences{Username: "student"},
		Post:  func(fn func()) { fn() },
	})
	if known.Page != pageOverview {
		t.Fatalf("page %q, want the overview", known.Page)
	}
}

func TestAutoConnectNeedsSavedAccount(t *testing.T) {
	runner := newFakeRunner()
	c, _ := newTestController(t, &memStore{}, runner)
	c.cfg.Prefs.AutoConnect = true
	c.MaybeAutoConnect()
	if c.Active() {
		t.Fatal("auto connected without a saved account")
	}
	if !strings.Contains(c.LogText(), "已跳过") {
		t.Fatalf("skip not logged: %q", c.LogText())
	}

	c.account.Remember = true
	c.account.Username = "student"
	c.account.Password = "secret"
	c.MaybeAutoConnect()
	if !c.Active() {
		t.Fatalf("refused detail=%q log=%q nil=%v", c.Detail(), c.LogText(), c.cfg.Supported == nil)
	}
	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("auto connect did not start: detail=%q log=%q", c.Detail(), c.LogText())
	}
}

// A stopped session must not leave its probe results on screen: they measured
// the tunnel that just went down.
func TestProbesClearedWhenSessionEnds(t *testing.T) {
	c, q := newTestController(t, &memStore{}, nil)
	c.account.Username = "u"
	c.account.Password = "p"
	c.Connect()
	q.Drain()
	c.Connected("utun7", []string{"10.5.0.1"})
	c.Progress("连接诊断：", "diagnostics", []client.ProbeResult{
		{URL: "https://xlcp.tongji.edu.cn/x", Status: 200, SystemDNSMS: 11, TCPMS: 24, TLSMS: 58},
	})
	q.Drain()
	if probes, _ := c.Probes(); len(probes) != 1 {
		t.Fatalf("connected probes %d, want 1", len(probes))
	}
	c.Disconnect()
	q.Drain()
	waitFor(t, q, func() bool { return !c.Active() })

	if probes, _ := c.Probes(); len(probes) != 0 {
		t.Fatalf("probes survived the session: %d", len(probes))
	}
	c.Page = pageOverview
	tt := ui.NewTester(c.ViewMain, 860, 640)
	tt.Frame()
	if tt.HasText("xlcp.tongji.edu.cn") {
		t.Fatalf("stale probe still shown: %q", tt.Texts())
	}
}

// The log search finds an entry by its component and level too, as the copied
// text shows them.
func TestLogSearchMatchesComponentAndLevel(t *testing.T) {
	c, q := newTestController(t, &memStore{}, nil)
	c.AddLog("error", "network", "helper 恢复网络超时")
	c.AddLog("info", "app", "开始连接")
	q.Drain()

	c.LogSearch = "network"
	if got := c.FilteredEntries(); len(got) != 1 || got[0].Component != "network" {
		t.Fatalf("search by component: %d entries", len(got))
	}
	c.LogSearch = "error"
	if got := c.FilteredEntries(); len(got) != 1 || got[0].Level != "error" {
		t.Fatalf("search by level: %d entries", len(got))
	}
	c.LogSearch = "  开始  "
	if got := c.FilteredEntries(); len(got) != 1 {
		t.Fatalf("search with spaces: %d entries", len(got))
	}
	c.LogSearch = ""
	if got := c.FilteredEntries(); len(got) != 2 {
		t.Fatalf("empty search: %d entries, want 2", len(got))
	}
}

// Launching must not read the keychain: that makes macOS ask for the login
// password before the user did anything.
func TestLaunchDoesNotReadKeychain(t *testing.T) {
	store := &countingStore{}
	c := New(Config{
		Store:          store,
		Prefs:          settings.Preferences{Username: "student", Remember: true},
		Post:           func(fn func()) { fn() },
		SavePrefs:      func(settings.Preferences) error { return nil },
		SetOpenAtLogin: func(bool) error { return nil },
		OpenAtLogin:    func() bool { return false },
		Supported:      func() bool { return true },
	})
	if store.loads != 0 {
		t.Fatalf("keychain read %d times at launch", store.loads)
	}
	if !c.NeedsCredential() {
		t.Fatal("the page cannot tell that a password is saved")
	}
	// Connecting asks for it, since the password is what it needs.
	c.Connect()
	if store.loads != 1 {
		t.Fatalf("keychain read %d times on connect, want 1", store.loads)
	}
	if c.account.Password != "from-keychain" {
		t.Fatalf("password %q, want the stored one", c.account.Password)
	}
}

type countingStore struct {
	mu    sync.Mutex
	loads int
}

func (s *countingStore) Load() (credentials.Credentials, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loads++
	return credentials.Credentials{Username: "student", Password: "from-keychain"}, true, nil
}
func (s *countingStore) Save(credentials.Credentials) error { return nil }
func (s *countingStore) Delete() error                      { return nil }

// The login item shows what the system has, not what the app last saved.
func TestOpenAtLoginFollowsSystem(t *testing.T) {
	system := false
	c := New(Config{
		Prefs:          settings.Preferences{OpenAtLogin: true},
		Post:           func(fn func()) { fn() },
		SavePrefs:      func(settings.Preferences) error { return nil },
		SetOpenAtLogin: func(on bool) error { system = on; return nil },
		OpenAtLogin:    func() bool { return system },
		Supported:      func() bool { return true },
	})
	if c.OpenAtLogin() {
		t.Fatal("preference leaked: the system has no login item")
	}
	c.SetOpenAtLogin(true)
	if !c.OpenAtLogin() {
		t.Fatal("switch does not follow the system after enabling")
	}
	c.SetOpenAtLogin(false)
	if c.OpenAtLogin() {
		t.Fatal("switch does not follow the system after disabling")
	}
}

// A reconnect drops the tunnel, so the probes measured through it are stale
// there too.
func TestProbesClearedWhenReconnecting(t *testing.T) {
	c, q := newTestController(t, &memStore{}, nil)
	c.account.Username = "u"
	c.account.Password = "p"
	c.Connect()
	q.Drain()
	c.Connected("utun7", []string{"10.5.0.1"})
	c.Progress("连接诊断：", "diagnostics", []client.ProbeResult{
		{URL: "https://ping0.cc", Status: 200, TCPMS: 30, TLSMS: 60},
	})
	q.Drain()
	c.Reconnecting(5)
	q.Drain()
	if probes, _ := c.Probes(); len(probes) != 0 {
		t.Fatalf("probes survived the drop: %d", len(probes))
	}
	if c.Checking() {
		t.Fatal("still reports a check in progress")
	}
}

// A failed keychain read must be recoverable from the page, and the reason
// must be visible: otherwise the user can only retype the password.
func TestKeychainFailureIsVisibleAndRetryable(t *testing.T) {
	store := &failingStore{}
	c := New(Config{
		Store:          store,
		Prefs:          settings.Preferences{Username: "student", Remember: true},
		Post:           func(fn func()) { fn() },
		SavePrefs:      func(settings.Preferences) error { return nil },
		SetOpenAtLogin: func(bool) error { return nil },
		OpenAtLogin:    func() bool { return false },
		Supported:      func() bool { return true },
	})

	c.LoadCredentials()
	if c.CredentialError() == "" {
		t.Fatal("failure not reported to the page")
	}
	if !c.NeedsCredential() {
		t.Fatal("no retry offered after a failed read")
	}
	c.Page = pageAccount
	tt := ui.NewTester(c.ViewMain, 860, 640)
	tt.Frame()
	if !tt.HasText("从钥匙串载入密码") {
		t.Fatalf("no way to retry: %q", tt.Texts())
	}

	store.ok = true
	c.LoadCredentials()
	if c.CredentialError() != "" {
		t.Fatalf("error kept after a good read: %q", c.CredentialError())
	}
	if c.account.Password != "from-keychain" {
		t.Fatalf("password %q", c.account.Password)
	}
}

type failingStore struct{ ok bool }

func (s *failingStore) Load() (credentials.Credentials, bool, error) {
	if !s.ok {
		return credentials.Credentials{}, false, errors.New("用户取消了钥匙串访问")
	}
	return credentials.Credentials{Username: "student", Password: "from-keychain"}, true, nil
}
func (s *failingStore) Save(credentials.Credentials) error { return nil }
func (s *failingStore) Delete() error                      { return nil }

func TestMissingCredentialNamesTheGap(t *testing.T) {
	c, _ := newTestController(t, &memStore{}, nil)
	c.account.Password = "secret"
	if got := c.MissingCredential(); got != "还缺用户名" {
		t.Fatalf("missing %q", got)
	}
	c.account.Username = "student"
	c.account.Password = ""
	if got := c.MissingCredential(); got != "还缺密码" {
		t.Fatalf("missing %q", got)
	}
	c.account.Password = "secret"
	if got := c.MissingCredential(); got != "" {
		t.Fatalf("missing %q", got)
	}
}
