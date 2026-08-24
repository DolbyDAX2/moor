// Moor GUI — Wails v2 masaüstü uygulaması (tarayıcı yok, gömülü webview).
package gui

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// Run GUI'yi başlatır (moor gui alt komutu).
func Run() error {
	app := NewApp()

	return wails.Run(&options.App{
		Title:     "Moor — Sunucu Kontrol",
		Width:     1100,
		Height:    720,
		MinWidth:  900,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})
}
