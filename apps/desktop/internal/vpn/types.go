package vpn

import (
	"fmt"

	"open-tj.local/desktop/internal/client"
)

// Result is the outcome of one connection attempt. Its JSON form is what the
// diagnostic log keeps, matching the fields earlier versions wrote.
type Result struct {
	Login       string      `json:"login,omitempty"`
	Error       string      `json:"error,omitempty"`
	LogoutError string      `json:"logout_error,omitempty"`
	Site        *SiteResult `json:"site_test,omitempty"`
}

// SiteResult describes the tunnel and how the host network was restored.
type SiteResult struct {
	HostNetworkModified    bool                 `json:"host_network_modified"`
	ConnectionError        string               `json:"connection_error,omitempty"`
	StartupFailed          bool                 `json:"startup_failed,omitempty"`
	CleanupConfirmed       bool                 `json:"cleanup_confirmed,omitempty"`
	NetworkRestoreFailures []string             `json:"network_restore_failures,omitempty"`
	HelperError            string               `json:"helper_error,omitempty"`
	ParentStateRestored    bool                 `json:"parent_state_restored,omitempty"`
	OuterRouteRestored     bool                 `json:"outer_route_restored,omitempty"`
	ConnectivityProbes     []client.ProbeResult `json:"connectivity_probes,omitempty"`
	Diagnostics            Diagnostics          `json:"diagnostics,omitempty"`
	Connected              bool                 `json:"connected,omitempty"`
	// GatewayReason is set when the gateway ended the session itself.
	GatewayReason int `json:"gateway_reason,omitempty"`
}

// Diagnostics are the tunnel's counters at teardown.
type Diagnostics struct {
	DownloadBytes      int64   `json:"download_bytes"`
	UploadBytes        int64   `json:"upload_bytes"`
	ConnectedSeconds   float64 `json:"connected_seconds"`
	PacketsSent        int64   `json:"packets_sent"`
	PacketsReceived    int64   `json:"packets_received"`
	HeartbeatsReceived int64   `json:"heartbeats_received"`
}

// NetworkRestored reports whether every host network change was confirmed
// undone. A session that never changed the network counts as restored.
func (s *SiteResult) NetworkRestored() bool {
	if s == nil || !s.HostNetworkModified {
		return true
	}
	return s.CleanupConfirmed && s.ParentStateRestored && s.OuterRouteRestored
}

// GatewayMessage explains a gateway termination in user terms.
func GatewayMessage(reason int) string {
	switch reason {
	case 1:
		return "账号已在其他设备登录，本连接被顶下线。请先断开其他设备后重试。"
	case 8:
		return "网关拒绝了本次会话。请确认没有其他设备或客户端使用同一账号在线，然后重试。"
	default:
		return fmt.Sprintf("网关终止了会话（原因码 %d）。请稍后重试；若持续出现，请确认账号未在其他设备登录。", reason)
	}
}

// Level is the log level of the result: "error" for a failed attempt or an
// uncertain network restore, "info" otherwise.
func (r Result) Level() string {
	if r.Error != "" {
		return "error"
	}
	if r.Site != nil && (r.Site.ConnectionError != "" || !r.Site.NetworkRestored()) {
		return "error"
	}
	return "info"
}

// Summary is a short, user-facing description of the result.
func (r Result) Summary() (status, detail string) {
	if r.Site != nil && r.Site.HostNetworkModified && !r.Site.NetworkRestored() {
		return "网络恢复需要检查", "部分网络设置未能确认恢复，请查看连接详情"
	}
	if r.Site != nil && r.Site.GatewayReason != 0 {
		return "连接失败", GatewayMessage(r.Site.GatewayReason)
	}
	err := r.Error
	if err == "" && r.Site != nil {
		err = r.Site.ConnectionError
		if err == "" {
			err = r.Site.HelperError
		}
	}
	if err != "" {
		return "连接失败", err
	}
	return "已断开", "网络设置已恢复"
}
