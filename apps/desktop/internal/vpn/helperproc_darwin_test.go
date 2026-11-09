package vpn

import (
	"strings"
	"testing"
)

// Declining the administrator prompt is the common case, and the user must
// read a sentence, not an AppleScript location.
func TestHelperStartErrorExplainsCancellation(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   string
	}{
		{"canceled", "0:55: execution error: User canceled. (-128)\n", "已取消管理员授权"},
		{"refused", "0:55: execution error: Open TJ is not allowed to send keystrokes. (1002)", "隐私与安全性"},
		{"silent", "", "没有启动"},
		{"other", "0:55: execution error: something odd happened (42)\nmore", "something odd happened"},
	}
	for _, tc := range cases {
		if got := helperStartError(tc.stderr).Error(); !strings.Contains(got, tc.want) {
			t.Errorf("%s: helperStartError = %q, want it to mention %q", tc.name, got, tc.want)
		}
	}
	if got := helperStartError("0:1: execution error: User canceled. (-128)").Error(); strings.Contains(got, "execution error") ||
		strings.Contains(got, "-128") {
		t.Errorf("machine text leaked to the user: %q", got)
	}
}

func TestFirstSentenceDropsLocation(t *testing.T) {
	if got := firstSentence("0:55: execution error: real reason (1)"); got != "real reason (1)" {
		t.Errorf("firstSentence = %q", got)
	}
}
