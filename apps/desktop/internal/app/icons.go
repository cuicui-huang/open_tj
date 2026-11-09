package app

import "github.com/egoist/mygo/ui"

// Icons drawn where the system has no SF Symbol of the same name, which is
// everywhere but macOS. Each is a 24×24 outline that Icon paints in the
// current text color.

var (
	iconShield  = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" d="M12 3 5 6v5.2c0 4.4 2.9 7.6 7 8.8 4.1-1.2 7-4.4 7-8.8V6z"/></svg>`))
	iconGauge   = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" d="M5 16a8 8 0 1 1 14 0M12 13l3.2-3.2"/><circle cx="12" cy="13" r="1.3" fill="currentColor" stroke="none"/></svg>`))
	iconPerson  = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><circle cx="12" cy="8.2" r="3.1" fill="none" stroke="currentColor" stroke-width="1.7"/><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" d="M5.5 19.2c1.2-2.6 3.5-3.9 6.5-3.9s5.3 1.3 6.5 3.9"/></svg>`))
	iconDoc     = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" d="M7 3.8h7l4.2 4.2V20a1.2 1.2 0 0 1-1.2 1.2H7A1.2 1.2 0 0 1 5.8 20V5A1.2 1.2 0 0 1 7 3.8z"/><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" d="M14 3.8V8h4.2M8.6 12.4h6.8M8.6 16h4.6"/></svg>`))
	iconDown    = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" d="M12 5v10M7.5 11.5 12 16l4.5-4.5"/></svg>`))
	iconUp      = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" d="M12 19V9M7.5 12.5 12 8l4.5 4.5"/></svg>`))
	iconCopy    = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><rect x="8.2" y="8.2" width="10" height="10" rx="1.6" fill="none" stroke="currentColor" stroke-width="1.7"/><path fill="none" stroke="currentColor" stroke-width="1.7" d="M15.8 8.2V6.4A1.6 1.6 0 0 0 14.2 4.8H6.4A1.6 1.6 0 0 0 4.8 6.4v7.8a1.6 1.6 0 0 0 1.6 1.6h1.8"/></svg>`))
	iconTrash   = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" d="M5 7.5h14M9.2 7.4V5.6A1.2 1.2 0 0 1 10.4 4.4h3.2a1.2 1.2 0 0 1 1.2 1.2v1.8M8 7.5l.7 11.2a1.2 1.2 0 0 0 1.2 1.1h4.2a1.2 1.2 0 0 0 1.2-1.1L16 7.5"/></svg>`))
	iconEye     = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" d="M2.8 12S6.2 6.6 12 6.6 21.2 12 21.2 12 17.8 17.4 12 17.4 2.8 12 2.8 12z"/><circle cx="12" cy="12" r="2.4" fill="none" stroke="currentColor" stroke-width="1.7"/></svg>`))
	iconGear    = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="1.7"/><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" d="m12 3.2 1.4 1.8 2.2-.4 1 1.9 2 .9-.2 2.2 1.6 1.4-1.6 1.4.2 2.2-2 .9-1 1.9-2.2-.4L12 20.8l-1.4-1.8-2.2.4-1-1.9-2-.9.2-2.2L4 12.9l1.6-1.4-.2-2.2 2-.9 1-1.9 2.2.4z"/></svg>`))
	iconWaves   = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" d="M8.2 9.2a5.4 5.4 0 0 0 0 5.6M5.4 6.6a9 9 0 0 0 0 10.8M15.8 9.2a5.4 5.4 0 0 1 0 5.6M18.6 6.6a9 9 0 0 1 0 10.8"/></svg>`))
	iconWarn    = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8" fill="none" stroke="currentColor" stroke-width="1.7"/><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" d="M12 8v4.6M12 16.2v.2"/></svg>`))
	iconBolt    = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="currentColor" d="M13.2 3.2 5.4 13.4h5.2L10.2 20.8l7.8-10.4h-5.2z"/></svg>`))
	iconX       = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" d="M7 7l10 10M17 7 7 17"/></svg>`))
	iconSwap    = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" d="M7.4 7.6h9.2M13.4 4.4l3.2 3.2-3.2 3.2M16.6 16.4H7.4M10.6 19.6 7.4 16.4l3.2-3.2"/></svg>`))
	iconRefresh = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" d="M19.2 12a7.2 7.2 0 1 1-2.1-5.1M19.2 4.6V9h-4.4"/></svg>`))
	iconWindow  = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><rect x="4" y="5" width="16" height="14" rx="2" fill="none" stroke="currentColor" stroke-width="1.7"/><path fill="none" stroke="currentColor" stroke-width="1.7" d="M4 9h16"/></svg>`))
	iconPower   = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" d="M12 4.2v7.2M7.4 7.6a6.6 6.6 0 1 0 9.2 0"/></svg>`))
	iconChevron = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" d="M8 10l4-3 4 3M8 14l4 3 4-3"/></svg>`))
)
