package app

import (
	"math"
	"testing"
	"time"
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
