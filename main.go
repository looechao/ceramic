package main

import (
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	service := newApp(filepath.Join(dir, "Ceramic", "session.json"))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	service.origin = "http://" + listener.Addr().String()
	server := &http.Server{Handler: service.routes(), ReadHeaderTimeout: requestTimeout}
	go server.Serve(listener)
	defer server.Close()
	reader := newReader(service)
	mygo.App.WhenReady(func() {
		window := mygo.NewWindow(mygo.WindowOptions{Title: "Ceramic", TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 52, TrafficLightPosition: &mygo.Point{X: 12, Y: 18}, Vibrancy: mygo.VibrancySidebar, Width: 1240, Height: 820, MinWidth: 420, MinHeight: 560, Content: ui.View(reader.view)})
		reader.update = window.Update
		reader.closeWindow = window.Close
		if menu := mygo.App.Menu(); menu != nil {
			menu = mygo.NewMenu(columnCloseMenu(menu.Items(), func() { window.Update(reader.closeFocused) }))
			mygo.App.SetMenu(menu)
			items := []*mygo.MenuItem{}
			for i, page := range []string{"profile", "home", "search"} {
				title := []string{"我的", "首页", "Search"}[i]
				items = append(items, &mygo.MenuItem{Label: title, Accelerator: fmt.Sprintf("CmdOrCtrl+%d", i+1), Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
					window.Update(func() {
						if reader.session != nil {
							reader.navigate(page)
						}
					})
				}})
			}
			items = append(items, mygo.Separator(), &mygo.MenuItem{Label: "刷新", Accelerator: "CmdOrCtrl+R", Click: func(_ *mygo.MenuItem, _ *mygo.Window) { window.Update(func() { reader.load(false) }) }})
			menu.Append(&mygo.MenuItem{Label: "Navigate", Submenu: items})
		}
		window.OnClosed(func() { mygo.App.Quit() })
		if reader.session != nil {
			reader.load(false)
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
			result = append(result, &mygo.MenuItem{Label: "关闭当前列或窗口", Accelerator: "CmdOrCtrl+W", Click: func(_ *mygo.MenuItem, _ *mygo.Window) { close() }})
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
