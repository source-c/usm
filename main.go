package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"apps.z7.ai/usm/internal/agent"
	"apps.z7.ai/usm/internal/browser"
	"apps.z7.ai/usm/internal/cli"
	"apps.z7.ai/usm/internal/icon"
	"apps.z7.ai/usm/internal/ui"
	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

// appType detects the application type from the command line arguments and the runtime
type appType struct {
	args []string
}

// IsCLI returns true if the application is a CLI app
func (a *appType) IsCLI() bool {
	return len(a.args) > 1 && a.args[1] == "cli"
}

// IsGUI returns true if the application is a GUI app
func (a *appType) IsGUI() bool {
	return !a.IsCLI()
}

// IsMessageFromBrowserExtension returns true if the application is a message from the browser extension
func (a *appType) IsMessageFromBrowserExtension() bool {
	return len(a.args) > 1 && browser.MessageFromExtension(a.args[1:])
}

// IsMobile returns true if the application is running on a mobile device
func (a *appType) IsMobile() bool {
	return runtime.GOOS == "android" || runtime.GOOS == "ios"
}

// IsWindowsOS returns true if the application is running on Windows
func (a *appType) IsWindowsOS() bool {
	return runtime.GOOS == "windows"
}

func main() {
	at := &appType{args: os.Args}

	// handle application start: CLI, GUI
	if at.IsCLI() && at.IsMobile() {
		fmt.Fprintln(os.Stderr, "CLI app is unsupported on this OS")
		os.Exit(1)
	}

	if !at.IsCLI() && at.IsWindowsOS() {
		// On Windows, to ship a single binary for GUI and CLI we need to build as
		// "console binary" and detach the console when running as GUI
		ui.DetachConsole()
	}

	fyneApp := app.NewWithID(ui.AppID)
	fyneApp.SetIcon(icon.USMIcon)
	s, err := makeStorage(at, fyneApp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Write the native manifests to support browser native messaging for the current OS
	// TODO: this should be once at installation time
	_ = browser.WriteNativeManifests()

	// Handle message from browser extension
	if at.IsMessageFromBrowserExtension() {
		browser.HandleNativeMessage(s)
		return
	}

	if at.IsCLI() {
		// Run the CLI app
		cli.Run(os.Args, s)
		return
	}

	// check for running instance looking at the health service
	if ui.HealthServiceCheck(s.LockFilePath()) {
		fmt.Fprintln(os.Stderr, "USM GUI is already running")
		os.Exit(1)
	}
	// start the health service
	go func() { _, _ = ui.HealthService(s.LockFilePath()) }()

	// agent could be already running (e.g. from CLI)
	// if not, start it
	var agentType agent.Type
	c, err := agent.NewClient(s.SocketAgentPath())
	if err == nil {
		agentType, _ = c.Type()
	}

	// start the GUI agent if not already running
	if agentType.IsZero() {
		go agent.Run(agent.NewGUI(), s.SocketAgentPath())
	}

	// create window and run the app
	w := fyneApp.NewWindow(ui.AppTitle)
	w.SetMaster()
	w.Resize(fyne.NewSize(400, 600))
	w.SetContent(ui.MakeApp(s, w))

	// Set up graceful shutdown handler
	w.SetOnClosed(func() {
		// Clean up resources before exit
		fyne.Do(func() {
			fyneApp.Quit()
		})
	})

	// Handle OS interrupt signals (including Cmd+Q on macOS)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		// Gracefully quit the application
		fyne.Do(func() {
			fyneApp.Quit()
		})
	}()

	w.ShowAndRun()
}

// makeStorage create the storage
func makeStorage(at *appType, fyneApp fyne.App) (usm.Storage, error) {
	if at.IsMobile() {
		// Mobile app returns the Fyne storage
		return usm.NewFyneStorage(fyneApp.Storage())
	}
	// Otherwise returns the OS storage
	return usm.NewOSStorage()
}
