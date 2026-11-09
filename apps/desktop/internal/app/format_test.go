package app

import (
	"math"
	"strings"
	"testing"
	"time"

	"open-tj.local/desktop/internal/client"
)

func TestBytes(t *testing.T) {
	cases := map[float64]string{
		0: "0 B", 512: "512 B", 1536: "1.5 KB", 4096: "4 KB",
		1024 * 1024 * 1.2: "1.2 MB", 1024 * 1024 * 1024: "1 GB",
	}
	for value, want := range cases {
		if got := Bytes(value); got != want {
			t.Errorf("Bytes(%v) = %q, want %q", value, got, want)
		}
	}
}

func TestRateParts(t *testing.T) {
	if value, unit := RateParts(80); value != "80" || unit != "B/s" {
		t.Errorf("RateParts(80) = %q %q", value, unit)
	}
	if value, unit := RateParts(1280); value != "1.25" || unit != "KB/s" {
		t.Errorf("RateParts(1280) = %q %q", value, unit)
	}
	if value, _ := RateParts(-1); value != "0" {
		t.Errorf("RateParts(-1) = %q", value)
	}
	if value, unit := RateParts(4.6 * 1024 * 1024); value != "4.6" || unit != "MB/s" {
		t.Errorf("RateParts(4.6 MiB) = %q %q, want no trailing zero", value, unit)
	}
	if value, _ := RateParts(612 * 1024); value != "612" {
		t.Errorf("RateParts(612 KiB) = %q, want a whole number", value)
	}
}

func TestGraphCeiling(t *testing.T) {
	if got := GraphCeiling([]float64{80, 348}); got != 16384 {
		t.Errorf("GraphCeiling(small) = %v, want 16384", got)
	}
	if got := GraphCeiling([]float64{20000}); got != 32768 {
		t.Errorf("GraphCeiling(20000) = %v, want 32768", got)
	}
	if got := GraphCeiling(nil); got != math.Pow(2, 14) {
		t.Errorf("GraphCeiling(nil) = %v", got)
	}
}

func TestDuration(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := Duration(start, start.Add(65*time.Second)); got != "01:05" {
		t.Errorf("Duration(65s) = %q", got)
	}
	if got := Duration(start, start.Add(3665*time.Second)); got != "1:01:05" {
		t.Errorf("Duration(3665s) = %q", got)
	}
	if got := Duration(time.Time{}, start); got != "" {
		t.Errorf("Duration(zero) = %q", got)
	}
}

func TestExplainProbe(t *testing.T) {
	cases := []struct {
		name  string
		probe client.ProbeResult
		want  string
	}{
		{"system dns only", client.ProbeResult{SystemDNSError: "no such host"}, "系统 DNS 解析失败，隧道内正常；可能是本机 DNS 配置问题"},
		{"both resolvers", client.ProbeResult{SystemDNSError: "timeout", VPNDNSError: "timeout"}, "隧道内也无法解析，请尝试断开重连"},
		{"tunnel dns only", client.ProbeResult{VPNDNSError: "refused"}, "隧道 DNS 不可用，正在使用系统 DNS"},
		{"tls unfinished", client.ProbeResult{TCPMS: 40, TLSMS: 0}, "能连通但证书握手未完成，可能被中间设备拦截"},
		{"server error", client.ProbeResult{Status: 503, TCPMS: 20, TLSMS: 30}, "校园网服务端异常（HTTP 503），稍后重试"},
		{"raw error", client.ProbeResult{Error: "dial tcp: connection refused"}, "检测失败：dial tcp: connection refused"},
		{"healthy", client.ProbeResult{Status: 200, TCPMS: 12, TLSMS: 30}, "正常"},
	}
	for _, tc := range cases {
		if got := ExplainProbe(tc.probe); got != tc.want {
			t.Errorf("%s: ExplainProbe = %q, want %q", tc.name, got, tc.want)
		}
	}
	long := ExplainProbe(client.ProbeResult{Error: strings.Repeat("错", 90)})
	if !strings.HasPrefix(long, "检测失败：") || !strings.HasSuffix(long, "…") {
		t.Errorf("long error not shortened: %q", long)
	}
}

func TestProbeHostAndTiming(t *testing.T) {
	if got := ProbeHost("https://xlcp.tongji.edu.cn/user/web-v2"); got != "xlcp.tongji.edu.cn" {
		t.Errorf("ProbeHost = %q", got)
	}
	if got := Millis(0); got != "—" {
		t.Errorf("Millis(0) = %q", got)
	}
	if got := Millis(58); got != "58ms" {
		t.Errorf("Millis(58) = %q", got)
	}
	if got := ProbeLevel(client.ProbeResult{Status: 200, TCPMS: 10, TLSMS: 20}); got != 0 {
		t.Errorf("healthy level %d", got)
	}
	if got := ProbeLevel(client.ProbeResult{SystemDNSError: "x"}); got != 1 {
		t.Errorf("warning level %d", got)
	}
	if got := ProbeLevel(client.ProbeResult{Status: 500}); got != 2 {
		t.Errorf("error level %d", got)
	}
}
