package main

import (
	"embed"
	"log"
	"os"

	"go-toy/internal/app"
	"go-toy/internal/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// If invoked with service commands, run as the background task runner.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "run", "install", "uninstall", "start", "stop", "status":
			service.Run()
			return
		}
	}

	// Create an instance of the app structure
	application := app.New()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "go-toy",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        application.Startup,
		Bind: []interface{}{
			application,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
