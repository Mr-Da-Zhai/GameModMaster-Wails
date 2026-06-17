package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed data/name_mapping.json
var nameMappingData []byte

func main() {
	appService := NewAppService(nameMappingData)

	app := application.New(application.Options{
		Name:        "GameModMaster",
		Description: "游戏修改器大师",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "GameModMaster - 游戏修改器大师",
		Width:     1200,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})

	// Give the service a window reference so it can emit events to the frontend.
	appService.SetWindow(mainWindow)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
