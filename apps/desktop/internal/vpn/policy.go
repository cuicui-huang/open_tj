package vpn

// retryPolicy separates session cancellation from recovery safety: sleeping
// may interrupt authentication before any network change and must not end the
// supervisor, while an uncertain network restoration always stops it.
func retryPolicy(result Result, sleeping, stopped bool) bool {
	if stopped {
		return false
	}
	if result.Site == nil {
		return sleeping
	}
	// A session the gateway itself ended is not a transient network drop:
	// retrying would fight whatever ended it, such as another login.
	if result.Site.GatewayReason != 0 {
		return false
	}
	restored := result.Site.NetworkRestored()
	if result.Site.HostNetworkModified {
		return restored && (sleeping || !result.Site.StartupFailed)
	}
	return sleeping
}
