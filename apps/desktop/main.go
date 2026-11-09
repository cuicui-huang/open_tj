// Command OpenTJ is the macOS client of the Tongji campus VPN, written in Go
// with MyGo's native UI.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"open-tj.local/desktop/internal/app"
	"open-tj.local/desktop/internal/client"
	"open-tj.local/desktop/internal/credentials"
	"open-tj.local/desktop/internal/logging"
	"open-tj.local/desktop/internal/settings"
	"open-tj.local/desktop/internal/vpn"
)

const (
	// The app has four short pages, so the window is sized for them rather
	// than for a dashboard: small enough to sit beside an editor, large enough
	// that the overview needs no scrolling at the overview's densest.
	windowWidth     = 660
	windowHeight    = 470
	minimumWidth    = 580
	minimumHeight   = 400
	backgroundClose = "Open TJ 会继续在菜单栏运行"
)

func main() {
	// The tunnel device loads wintun.dll from beside the executable, so the
	// file has to exist before either process needs it: the helper is the one
	// that creates the device, and it starts before the UI does anything.
	if err := client.PrepareTunnelDevice(); err != nil {
		log.Println("wintun:", err)
	}

	// The privileged helper and the diagnostic probe must run before MyGo
	// starts: the helper is root, and neither needs a user interface.
	if headless(os.Args) {
		return
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	logsDir, err := mygo.App.Path(mygo.PathLogs)
	if err != nil {
		log.Fatal(err)
	}
	userData, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	prefsPath := filepath.Join(userData, "preferences.json")
	prefs := settings.Load(prefsPath)
	if _, err := os.Stat(prefsPath); os.IsNotExist(err) {
		// First launch after the SwiftUI client: keep its account and choice.
		legacy := settings.LegacyDefaults("local.motionvpn.prototype")
		if legacy.Username != "" {
			prefs.Username = legacy.Username
		}
		prefs.Method = legacy.Method
		prefs.Remember = legacy.Remember
	}

	var (
		window   *mygo.Window
		tray     *app.Tray
		quitting bool
	)

	ctrl := app.New(app.Config{
		VPN:   vpn.Config{Gateway: vpn.DefaultGateway},
		Log:   logging.NewSink(filepath.Join(logsDir, "connections-go.jsonl")),
		Store: credentials.NewKeychain(credentials.DefaultService),
		Prefs: prefs,
		SavePrefs: func(p settings.Preferences) error {
			return settings.Save(prefsPath, p)
		},
		Post: func(fn func()) {
			if window != nil {
				window.Update(fn)
				return
			}
			fn()
		},
		Redraw: func() {
			if window != nil {
				window.Invalidate()
			}
			if tray != nil {
				tray.Refresh()
			}
		},
		OpenMain: func() {
			if window != nil {
				window.Show()
				window.Focus()
			}
		},
		Quit: func() { mygo.App.Quit() },
	})
	mygo.App.OnWindowAllClosed(func() {
		// Keep running in the menu bar; quitting is explicit.
	})
	mygo.App.OnActivate(func(visible bool) {
		if !visible && window != nil {
			window.Show()
			window.Focus()
		}
	})
	mygo.App.OnSecondInstance(func([]string, string) {
		if window != nil {
			window.Show()
			window.Focus()
		}
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if tray != nil {
			tray.Destroy()
		}
	})
	mygo.App.OnBeforeQuit(func(event *mygo.QuitEvent) {
		if quitting {
			return
		}
		quitting = true
		if ctrl.Active() {
			event.PreventDefault()
			ctrl.RequestQuit(func() { mygo.App.Quit() })
		}
	})

	mygo.App.WhenReady(func() {
		window = mygo.NewWindow(mygo.WindowOptions{
			Title:           "Open TJ",
			Width:           windowWidth,
			Height:          windowHeight,
			MinWidth:        minimumWidth,
			MinHeight:       minimumHeight,
			StateKey:        "main",
			TitleBarStyle:   mygo.TitleBarHidden,
			Vibrancy:        mygo.VibrancySidebar,
			BackgroundColor: "light-dark(#ffffff, #141416)",
			Content:         ui.View(ctrl.ViewMain),
		})
		window.OnClose(func(event *mygo.CloseEvent) {
			if quitting {
				return
			}
			event.PreventDefault()
			window.Hide()
			if !ctrl.BackgroundCloseExplained() {
				// Explain once, after the window is hidden.
				ctrl.MarkBackgroundCloseExplained()
				go alreadyInMenuBar()
			}
		})

		t, err := app.NewTray(ctrl)
		if err != nil {
			log.Println("tray:", err)
		} else {
			tray = t
		}

		mygo.Power.OnSuspend(ctrl.Sleep)
		mygo.Power.OnResume(ctrl.Wake)

		// Connect on launch when the user asked for it and saved an account.
		// Runs on the window's thread, as Connect changes UI state.
		ctrl.MaybeAutoConnect()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// alreadyInMenuBar explains that closing the window keeps the VPN running.
func alreadyInMenuBar() {
	_, _ = mygo.Dialog.Message(mygo.MessageOptions{
		Type:          mygo.MessageInfo,
		Title:         backgroundClose,
		Message:       backgroundClose,
		Detail:        "关闭窗口不会断开 VPN。你可以从菜单栏查看状态、断开连接或重新打开窗口。选择“退出 Open TJ”会先断开并恢复网络。",
		Buttons:       []string{"继续在后台运行"},
		DefaultButton: 0,
	})
}

// headless runs the modes that never show a window and reports whether it
// handled the command line.
func headless(args []string) bool {
	if len(args) == 5 && args[1] == "--helper" {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := client.RunHelper(ctx, args[2], args[3], args[4]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return true
	}
	if len(args) == 3 && args[1] == "--diagnose" {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		_ = json.NewEncoder(os.Stdout).Encode(client.ProbeSites(ctx, args[2]))
		return true
	}
	return false
}
