package app

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"open-tj.local/desktop/internal/client"
)

// Bytes formats a byte count as ByteCountFormatter does with the binary
// style: "512 B", "1.5 KB", "4 KB", "1.2 MB".
func Bytes(value float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := math.Max(0, value)
	index := 0
	for v >= 1024 && index < len(units)-1 {
		v /= 1024
		index++
	}
	if index == 0 {
		return fmt.Sprintf("%.0f B", v)
	}
	text := fmt.Sprintf("%.1f", v)
	text = strings.TrimSuffix(text, ".0")
	return text + " " + units[index]
}

// RateParts splits a bytes-per-second rate into the value and its unit, as
// the speed columns show them.
func RateParts(value float64) (string, string) {
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	v := math.Max(0, value)
	index := 0
	for v >= 1024 && index < len(units)-1 {
		v /= 1024
		index++
	}
	if index == 0 {
		return fmt.Sprintf("%.0f", v), units[index]
	}
	// Two decimals keep 1.25 KB/s exact; trailing zeros are trimmed so the
	// big numbers of a fast link read as "4.4" and not "4.40".
	text := strings.TrimRight(fmt.Sprintf("%.2f", v), "0")
	text = strings.TrimSuffix(text, ".")
	return text, units[index]
}

// GraphCeiling is the power of two the traffic graph is scaled to, at least
// 16 KiB, as the SwiftUI client chose it.
func GraphCeiling(values []float64) float64 {
	peak := 16 * 1024.0
	for _, value := range values {
		if value > peak {
			peak = value
		}
	}
	return math.Pow(2, math.Ceil(math.Log2(peak)))
}

// Duration formats the time since start as mm:ss, or h:mm:ss after an hour.
func Duration(start, now time.Time) string {
	if start.IsZero() {
		return ""
	}
	seconds := int(now.Sub(start).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

// LevelLabel abbreviates a log level for the log rows.
func LevelLabel(level string) string {
	if level == "warning" {
		return "WARN"
	}
	return strings.ToUpper(level)
}

// LogLine formats one entry for the copied log.
func LogLine(entry LogEntry) string {
	return fmt.Sprintf("[%s] [%s] %s", entry.Time.Format("15:04:05"), LevelLabel(entry.Level), entry.Message)
}

// ExplainProbe turns one connectivity probe into a sentence a person can act
// on. The raw error is only shown when none of the known cases match, and
// then only its first line, shortened.
func ExplainProbe(r client.ProbeResult) string {
	switch {
	case r.SystemDNSError != "" && r.VPNDNSError == "":
		return "系统 DNS 解析失败，隧道内正常；可能是本机 DNS 配置问题"
	case r.SystemDNSError != "" && r.VPNDNSError != "":
		return "隧道内也无法解析，请尝试断开重连"
	case r.VPNDNSError != "":
		return "隧道 DNS 不可用，正在使用系统 DNS"
	case r.SystemDNSError == "" && r.VPNDNSError == "" && r.TLSMS == 0 && r.TCPMS > 0:
		return "能连通但证书握手未完成，可能被中间设备拦截"
	case r.Status >= 500:
		return fmt.Sprintf("校园网服务端异常（HTTP %d），稍后重试", r.Status)
	case r.Error != "":
		return "检测失败：" + shorten(firstLine(r.Error), 80)
	default:
		return "正常"
	}
}

// ProbeHost is the host a probe row is labeled with.
func ProbeHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return raw
	}
	return u.Hostname()
}

// ProbeLevel ranks a probe: 0 fine, 1 a warning, 2 an error.
func ProbeLevel(r client.ProbeResult) int {
	switch {
	case r.Error != "" || r.Status >= 500 || (r.TLSMS == 0 && r.TCPMS > 0 && r.SystemDNSError == "" && r.VPNDNSError == ""):
		return 2
	case r.SystemDNSError != "" || r.VPNDNSError != "":
		return 1
	default:
		return 0
	}
}

// Millis formats a probe timing. Zero, which the probe leaves unset, is a dash.
func Millis(v int64) string {
	if v <= 0 {
		return "—"
	}
	if v < 1000 {
		return fmt.Sprintf("%dms", v)
	}
	return fmt.Sprintf("%d.%ds", v/1000, (v%1000)/100)
}

// RelativeTime says how long ago at was, for a line that updates itself.
func RelativeTime(at, now time.Time) string {
	if at.IsZero() {
		return ""
	}
	seconds := int(now.Sub(at).Seconds())
	if seconds < 5 {
		return "刚刚"
	}
	if seconds < 60 {
		return fmt.Sprintf("%d 秒前", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%d 分钟前", seconds/60)
	}
	return fmt.Sprintf("%d 小时前", seconds/3600)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(line)
}

func shorten(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
