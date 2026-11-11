// Package app is the window and everything drawn in it.
package app

import (
	"ceramic/locales"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

// Run opens the window and blocks until the app quits.
func Run() {
	mygo.Use(native.Plugin)
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	service := mastodon.New(filepath.Join(dir, "Ceramic", "session.json"), serviceMessage)
	stop, err := service.StartCallbackServer()
	if err != nil {
		log.Fatal(err)
	}
	defer stop()
	reader := newReader(service)
	reader.applyAppearance = mygo.Theme.SetSource
	reader.loadPreferences(filepath.Join(dir, "Ceramic", "preferences.json"))
	mygo.App.WhenReady(func() {
		mygo.Theme.SetSource(appearanceSources[reader.appearance])
		options := mygo.WindowOptions{Title: "Ceramic", Width: 1240, Height: 820, MinWidth: 420, MinHeight: 560, Content: ui.View(reader.view)}
		// Elsewhere the system's title bar stays, because its buttons would cover
		// the toolbar of whichever column reaches the right edge.
		if runtime.GOOS == "darwin" {
			options.TitleBarStyle = mygo.TitleBarHiddenInset
			options.TitleBarHeight = 52
			options.TrafficLightPosition = &mygo.Point{X: 12, Y: 18}
			options.Vibrancy = mygo.VibrancySidebar
		}
		window := mygo.NewWindow(options)
		reader.parentWindow = window
		var closed atomic.Bool
		reader.update = func(apply func()) {
			if !closed.Load() {
				window.Update(func() {
					if !closed.Load() {
						apply()
					}
				})
			}
		}
		window.OnClosed(func() {
			closed.Store(true)
			reader.appViewModel.close()
		})
		reader.closeWindow = window.Close
		reader.openWriting = func() { reader.showComposer() }
		reader.checkUpdates = updater.CheckForUpdates
		reader.setAutomaticUpdates = updater.SetAutomaticChecks
		reader.automaticUpdates = updater.AutomaticChecks()
		offUpdates := updater.OnChange(func() {
			window.Update(func() { reader.automaticUpdates = updater.AutomaticChecks() })
		})
		window.OnClosed(offUpdates)
		baseMenu := mygo.App.Menu()
		rebuildMenu := func() {
			if menu := baseMenu; menu != nil {
				menuItems := columnCloseMenu(menu.Items(), func() {
					window.Update(func() {
						reader.closeFocused()
					})
				})
				if runtime.GOOS == "darwin" {
					for _, item := range menuItems {
						if item.Role == mygo.RoleAppMenu {
							settings := &mygo.MenuItem{Label: locales.Settings() + "…", Accelerator: "CmdOrCtrl+,", Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
								window.Show()
								window.Focus()
								window.Update(func() { reader.settingsOpen = true })
							}}
							// Place Settings after About and its separator.
							at := min(2, len(item.Submenu))
							item.Submenu = append(item.Submenu[:at], append([]*mygo.MenuItem{updater.MenuItem(), settings, mygo.Separator()}, item.Submenu[at:]...)...)
							break
						}
					}
				}
				menu = mygo.NewMenu(menuItems)
				mygo.App.SetMenu(menu)
				items := []*mygo.MenuItem{}
				for i, page := range navigationPages {
					title := navigationLabels()[i]
					items = append(items, &mygo.MenuItem{Label: title, Accelerator: fmt.Sprintf("CmdOrCtrl+%d", i+1), Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						window.Update(func() {
							if reader.session != nil {
								reader.navigate(page)
							}
						})
					}})
				}
				items = append(items, &mygo.MenuItem{Label: locales.WriteAPost(), Accelerator: "CmdOrCtrl+N", Click: func(_ *mygo.MenuItem, _ *mygo.Window) { window.Update(reader.openWriting) }}, mygo.Separator(), &mygo.MenuItem{Label: locales.Refresh(), Accelerator: "CmdOrCtrl+R", Click: func(_ *mygo.MenuItem, _ *mygo.Window) { window.Update(func() { reader.load(false) }) }})
				menu.Append(&mygo.MenuItem{Label: locales.Navigate(), Submenu: items})
			}
		}
		rebuildMenu()
		mygo.App.OnLocaleChanged(func(string) { rebuildMenu() })
		window.OnClose(func(e *mygo.CloseEvent) {
			if !reader.prepareClose() {
				e.PreventDefault()
			}
		})
		window.OnClosed(func() { mygo.App.Quit() })
		if reader.session != nil {
			reader.load(false)
			reader.startStream()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// Replace the native Close command so the menu and Cmd-W share column semantics.
func columnCloseMenu(items []*mygo.MenuItem, close func()) []*mygo.MenuItem {
	result := make([]*mygo.MenuItem, 0, len(items))
	for _, item := range items {
		if item.Role == mygo.RoleClose {
			result = append(result, &mygo.MenuItem{Label: locales.CloseCurrentColumnOrWindow(), Accelerator: "CmdOrCtrl+W", Click: func(_ *mygo.MenuItem, _ *mygo.Window) { close() }})
		} else {
			copy := &mygo.MenuItem{ID: item.ID, Label: item.Label, Type: item.Type, Role: item.Role, Accelerator: item.Accelerator, Disabled: item.Disabled, Hidden: item.Hidden, Checked: item.Checked, Click: item.Click}
			if item.Submenu != nil {
				copy.Submenu = columnCloseMenu(item.Submenu, close)
			}
			result = append(result, copy)
		}
	}
	return result
}
