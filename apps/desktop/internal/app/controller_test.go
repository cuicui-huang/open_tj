package app

import (
	"context"
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
		Store: store,
		Post:  q.Post,
		NewRunner: func(vpn.Config, vpn.Reporter) Runner {
			if runner == nil {
				return newFakeRunner()
			}
			return runner
		},
		SavePrefs: func(settings.Preferences) error { return nil },
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
	c.Connected("utun7")
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
	c.Account().Username = "student"
	c.Account().Password = "secret"

	tt := ui.NewTester(c.ViewMain, 420, 600)
	if !tt.HasText("未连接") || !tt.HasText("网络流量") {
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

	c.Connected("utun7")
	c.Traffic(client.TrafficSample{DownloadBPS: 4096, UploadBPS: 1280, DownloadBytes: 8192})
	q.Drain()
	tt.Frame()
	if !tt.HasText("已连接") {
		t.Fatalf("connected texts %q", tt.Texts())
	}
	if !tt.HasText("4.00") || !tt.HasText("1.25") {
		t.Fatalf("rate texts %q", tt.Texts())
	}

	c.AddLog("warning", "backend", "retry test")
	c.ShowLog = true
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
	tt.SetSize(940, 650)
	q.Drain()
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
