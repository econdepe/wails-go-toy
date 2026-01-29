package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-toy/internal/shared"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	serviceRunnerDirName = ".toy-servicerunner"
	serviceLogFileName   = "toy-service.log"
	heartbeatInterval    = 10 * time.Second
	logMaxSizeMB         = 5  // Max size of log file in megabytes
	logMaxBackups        = 3  // Max number of old log files to retain
	logMaxAgeDays        = 28 // Max age of log file in days
)

func Run() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	service := NewService()

	switch os.Args[1] {
	case "run":
		runService()
	case "install":
		if err := service.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service installed successfully")
	case "uninstall":
		if err := service.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to uninstall service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service uninstalled successfully")
	case "start":
		if err := service.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service started successfully")
	case "stop":
		if err := service.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop service: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service stopped successfully")
	case "status":
		status, err := service.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get service status: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Service status: %s\n", status)
	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go-service run        Run as a service")
	fmt.Println("  go-service install    Install the service")
	fmt.Println("  go-service uninstall  Uninstall the service")
	fmt.Println("  go-service start      Start the service")
	fmt.Println("  go-service stop       Stop the service")
	fmt.Println("  go-service status     Check service status")
}

func runService() {
	// Setup log directory
	if err := shared.EnsureLogDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating log dir: %v\n", err)
		os.Exit(1)
	}

	logPath := shared.GetLogPath()
	if logPath == "" {
		fmt.Fprintf(os.Stderr, "Error getting log path\n")
		os.Exit(1)
	}

	// Configure rolling logger
	logWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    logMaxSizeMB,
		MaxBackups: logMaxBackups,
		MaxAge:     logMaxAgeDays,
		Compress:   true,
	}

	defer logWriter.Close()

	// Log startup
	shared.LogMessage(logWriter, "Service started")

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create ticker for periodic task
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// Main service loop
	firstTick := true
	for {
		select {
		case <-ticker.C:
			if firstTick {
				shared.LogMessage(logWriter, "I'm alive")
				firstTick = false
			} else {
				shared.LogMessage(logWriter, "Staying alive")
			}
		case sig := <-sigChan:
			shared.LogMessage(logWriter, fmt.Sprintf("Received signal: %v, shutting down. Bye!", sig))
			return
		}
	}
}
