package app

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"open-tj.local/desktop/internal/client"
)

type idleRunner struct{}

func (idleRunner) Run(ctx context.Context, _ client.Credentials) { <-ctx.Done() }
func (idleRunner) Sleep()                                        {}
func (idleRunner) Wake()                                         {}

// A window at rest must stay at rest. If a frame asks for another while
// nothing moves, the app would burn a core drawing the same pixels.
func TestMotionIsIdleAtRest(t *testing.T) {
	c, q := newTestController(t, &memStore{}, idleRunner{})
	c.account.Username = "2610114"
	q.Drain()
	tt := ui.NewTester(c.ViewMain, 860, 640)
	tt.Frame()
	if tt.FramePending() {
		t.Fatal("the window asks for frames while nothing is happening")
	}
}

// A navigation must animate, and must settle: a transition that never ends is
// a stuck animation, indistinguishable from a frozen app.
func TestPageChangeAnimatesThenSettles(t *testing.T) {
	c, q := newTestController(t, &memStore{}, idleRunner{})
	c.account.Username = "2610114"
	q.Drain()
	tt := ui.NewTester(c.ViewMain, 860, 640)
	tt.Frame()

	c.choosePage(pageSettings)
	q.Drain()
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("a page change animated nothing")
	}
	tt.Advance(time.Second)
	tt.Frame()
	if tt.FramePending() {
		t.Fatal("the page transition never settled")
	}
}

// The only looping motion is the live dot, and only while a tunnel is up:
// any other loop would be decoration costing battery.
func TestOnlyLiveDotLoops(t *testing.T) {
	c, q := newTestController(t, &memStore{}, idleRunner{})
	c.account.Username = "2610114"
	c.account.Password = "pw"
	c.Connect()
	q.Drain()
	c.Connected("utun7", []string{"10.5.0.1"})
	q.Drain()

	tt := ui.NewTester(c.ViewMain, 860, 640)
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("the live dot does not pulse while connected")
	}

	c.Disconnect()
	q.Drain()
	c.finishSession()
	tt.Frame()
	tt.Advance(3 * time.Second)
	tt.Frame()
	if tt.FramePending() {
		t.Fatal("the dot keeps looping with no tunnel")
	}
}

// The page transition rises: an element entering from a negative offset goes
// up into place. Getting the sign wrong makes every navigation fall in.
func TestPageEnterRises(t *testing.T) {
	if pageEnter.Y >= 0 {
		t.Fatalf("pageEnter.Y = %v: a page would fall in instead of rising", pageEnter.Y)
	}
	if pageEnter.Opacity != 1 {
		t.Fatalf("pageEnter.Opacity = %v: the page must fade in", pageEnter.Opacity)
	}
	for i, d := range []time.Duration{0, rowDelayTime(0), rowDelayTime(3), rowDelayTime(20)} {
		if d < 0 {
			t.Fatalf("row delay %d is negative", i)
		}
	}
	// The stagger caps out, so a long list does not crawl.
	if rowDelayTime(20) != rowDelayTime(8) {
		t.Fatalf("row delay does not cap: %v vs %v", rowDelayTime(20), rowDelayTime(8))
	}
}
