package app

import (
	"fmt"
	"math"
	"strings"
	"time"
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
	format := "%.2f"
	if index == 0 {
		format = "%.0f"
	}
	return fmt.Sprintf(format, v), units[index]
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
