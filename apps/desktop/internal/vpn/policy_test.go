package vpn

import "testing"

func TestRetryPolicy(t *testing.T) {
	clean := &SiteResult{
		HostNetworkModified: true,
		CleanupConfirmed:    true,
		ParentStateRestored: true,
		OuterRouteRestored:  true,
	}
	result := Result{Site: clean}
	if !retryPolicy(result, false, false) {
		t.Fatal("restored disconnect should retry")
	}
	if retryPolicy(result, false, true) {
		t.Fatal("manual stop must not retry")
	}
	clean.StartupFailed = true
	if retryPolicy(result, false, false) {
		t.Fatal("startup failure must stop")
	}
	if !retryPolicy(result, true, false) {
		t.Fatal("sleep cancellation should resume after cleanup")
	}
	for name, reset := range map[string]func(){
		"cleanup":     func() { clean.CleanupConfirmed = false },
		"parent":      func() { clean.ParentStateRestored = false },
		"outer route": func() { clean.OuterRouteRestored = false },
	} {
		reset()
		if retryPolicy(result, true, false) {
			t.Fatalf("sleep must not bypass failed recovery: %s", name)
		}
		clean.CleanupConfirmed, clean.ParentStateRestored, clean.OuterRouteRestored = true, true, true
	}
	if !retryPolicy(Result{Error: "context canceled"}, true, false) {
		t.Fatal("sleep during authentication must not end supervisor")
	}
	if retryPolicy(Result{Error: "authentication failed"}, false, false) {
		t.Fatal("authentication must not retry")
	}
	if retryPolicy(Result{Site: &SiteResult{}}, false, false) {
		t.Fatal("disconnect without sleep must stop")
	}
}

func TestGatewayTerminationStopsRetry(t *testing.T) {
	site := &SiteResult{
		HostNetworkModified: true,
		CleanupConfirmed:    true,
		ParentStateRestored: true,
		OuterRouteRestored:  true,
		GatewayReason:       8,
	}
	if retryPolicy(Result{Site: site}, false, false) {
		t.Fatal("a gateway termination must not be retried")
	}
	if retryPolicy(Result{Site: site}, true, false) {
		t.Fatal("sleep must not bypass a gateway termination")
	}
}

func TestGatewayMessages(t *testing.T) {
	cases := map[int]string{
		1: "账号已在其他设备登录，本连接被顶下线。请先断开其他设备后重试。",
		8: "网关拒绝了本次会话。请确认没有其他设备或客户端使用同一账号在线，然后重试。",
	}
	for reason, want := range cases {
		if got := GatewayMessage(reason); got != want {
			t.Errorf("GatewayMessage(%d) = %q", reason, got)
		}
	}
	result := Result{Site: &SiteResult{
		HostNetworkModified: true,
		CleanupConfirmed:    true,
		ParentStateRestored: true,
		OuterRouteRestored:  true,
		GatewayReason:       8,
		ConnectionError:     "网关终止会话，原因码 8",
	}}
	status, detail := result.Summary()
	if status != "连接失败" || detail != cases[8] {
		t.Fatalf("summary %q %q", status, detail)
	}
	if result.Level() != "error" {
		t.Fatalf("level %q", result.Level())
	}
}
