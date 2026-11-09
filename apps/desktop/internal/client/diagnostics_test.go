package client

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProbeDoesNotFollowRedirectOrStoreBody(t *testing.T) {
	called := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.Header().Set("Location", "https://example.invalid/secret")
		w.WriteHeader(302)
		w.Write([]byte("private page"))
	}))
	defer s.Close()
	r := Probe(context.Background(), s.URL, net.DefaultResolver, net.DefaultResolver)
	if r.Status != 302 || r.Error != "" || called != 1 || r.Location != "https://example.invalid/secret" {
		t.Fatalf("%+v calls=%d", r, called)
	}
}
func TestProbeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	r := Probe(ctx, "https://example.invalid", net.DefaultResolver, net.DefaultResolver)
	if r.Error == "" || time.Since(start) > time.Second {
		t.Fatalf("cancellation failed: %+v", r)
	}
}
func TestProbeRefusesUntrustedTLS(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("untrusted request sent") }))
	defer s.Close()
	r := Probe(context.Background(), s.URL, net.DefaultResolver, net.DefaultResolver)
	if r.Error == "" || r.Status != 0 {
		t.Fatal("untrusted TLS accepted")
	}
}

// Go names the resolver from resolv.conf in its error, not the one Dial used,
// so the message must say which server was really asked.
func TestTunnelDNSErrorNamesTheDialedServer(t *testing.T) {
	r := ProbeResult{VPNDNSError: "lookup a.test on 223.6.6.6:53: dial udp 10.5.0.1:53: i/o timeout"}
	nameDialTarget(&r, "10.5.0.1:53")
	if !strings.HasPrefix(r.VPNDNSError, "查询 10.5.0.1:53") {
		t.Fatalf("dialed server not named: %q", r.VPNDNSError)
	}
	// Idempotent: a second pass must not nest the prefix.
	nameDialTarget(&r, "10.5.0.1:53")
	if strings.Count(r.VPNDNSError, "查询 ") != 1 {
		t.Fatalf("prefix applied twice: %q", r.VPNDNSError)
	}
	// A healthy probe stays quiet.
	ok := ProbeResult{Status: 200}
	nameDialTarget(&ok, "10.5.0.1:53")
	if ok.VPNDNSError != "" {
		t.Fatalf("healthy probe gained an error: %q", ok.VPNDNSError)
	}
}

// The probes must target the university, not a third-party site, and must
// cover three different campus services plus one public reference.
func TestProbeTargetsAreCampus(t *testing.T) {
	if len(targets) < 3 {
		t.Fatalf("only %d probes", len(targets))
	}
	seenCampus, seenPublic := 0, 0
	for _, target := range targets {
		u, err := url.Parse(target.URL)
		if err != nil {
			t.Fatalf("%s: %v", target.URL, err)
		}
		host := u.Hostname()
		switch {
		case strings.HasSuffix(host, "tongji.edu.cn"):
			seenCampus++
			if target.Label == "" || target.Proves == "" {
				t.Errorf("%s: a campus probe needs a label and a reason", host)
			}
		default:
			t.Fatalf("probe target %q is not a tongji site", host)
		}
		if !strings.HasPrefix(target.URL, "https://") {
			t.Errorf("%s: probes must use https", target.URL)
		}
		if u.Hostname() == "www.tongji.edu.cn" {
			seenPublic++
		}
	}
	if seenCampus < 3 {
		t.Fatalf("%d campus probes, want at least 3", seenCampus)
	}
	if seenPublic != 1 {
		t.Fatalf("%d public reference probes, want exactly 1", seenPublic)
	}
}

// A redirect is followed only while it stays on the university's sites.
func TestCampusHostStopsForeignRedirects(t *testing.T) {
	ok := []string{"tongji.edu.cn", "sso.tongji.edu.cn", "www.tongji.edu.cn"}
	for _, h := range ok {
		if !campusHost(h) {
			t.Errorf("campusHost(%q) = false", h)
		}
	}
	for _, h := range []string{"ping0.cc", "evil.tongji.edu.cn.example.com", "tongji.edu.cn.evil.com"} {
		if campusHost(h) {
			t.Errorf("campusHost(%q) = true, want false", h)
		}
	}
}
