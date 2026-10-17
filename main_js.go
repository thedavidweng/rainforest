package main

import (
	_ "embed"
	"log"
	"syscall/js"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/preview"
	"github.com/thedavidweng/rainforest/internal/view"
)

//go:embed resources/icon.png
var appIcon []byte

func main() {
	mygo.App.WhenReady(func() {
		locale := js.Global().Get("URLSearchParams").New(js.Global().Get("location").Get("search")).Call("get", "lang")
		if !locale.IsNull() {
			mygo.App.SetLocale(locale.String())
		}
		var win *mygo.Window
		app := view.NewWithServices(preview.Services(), view.Options{
			Preview: true, Icon: appIcon, Version: "preview",
			Update: func(fn func()) { win.Update(fn) }, SetLocale: mygo.App.SetLocale,
		})
		win = mygo.NewWindow(mygo.WindowOptions{Title: "Rainforest", Content: ui.View(app.View)})
		app.Start()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
