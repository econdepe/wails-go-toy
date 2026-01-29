package app

import (
	"context"
	"fmt"
	"os"

	"go-toy/internal/service"
	"go-toy/internal/shared"
)

// App struct
type App struct {
	ctx context.Context
	svc service.Service
}

// New creates a new App application struct
func New() *App {
	return &App{}
}

// Startup is called when the app starts
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.svc = service.NewService()
}

// GetServiceStatus returns the current status of the service
func (a *App) GetServiceStatus() string {
	status, err := a.svc.Status()
	if err != nil {
		return "Error: " + err.Error()
	}
	return status
}

// InstallService installs the service
func (a *App) InstallService() string {
	err := a.svc.Install()
	if err != nil {
		return "Failed to install: " + err.Error()
	}
	return "Service installed successfully"
}

// UninstallService uninstalls the service
func (a *App) UninstallService() string {
	err := a.svc.Uninstall()
	if err != nil {
		return "Failed to uninstall: " + err.Error()
	}
	return "Service uninstalled successfully"
}

// StartService starts the service
func (a *App) StartService() string {
	err := a.svc.Start()
	if err != nil {
		return "Failed to start: " + err.Error()
	}
	return "Service started successfully"
}

// StopService stops the service
func (a *App) StopService() string {
	err := a.svc.Stop()
	if err != nil {
		return "Failed to stop: " + err.Error()
	}
	return "Service stopped successfully"
}

// GetLogPath returns the path to the service log file
func (a *App) GetLogPath() string {
	return shared.GetLogPath()
}

// ReadLog reads the last few lines of the log file
func (a *App) ReadLog() string {
	logPath := shared.GetLogPath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Sprintf("Could not read log: %v", err)
	}

	// Return last 2000 characters
	if len(data) > 2000 {
		return string(data[len(data)-2000:])
	}
	return string(data)
}
