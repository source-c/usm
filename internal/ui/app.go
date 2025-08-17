package ui

import (
	"fmt"
	"image/color"
	"log"
	"runtime"

	"apps.z7.ai/usm/internal/agent"
	"apps.z7.ai/usm/internal/icon"
	"apps.z7.ai/usm/internal/sshkey"
	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/godbus/dbus/v5"
)

const (
	// AppID represents the application ID
	AppID = "ai.z7.apps.usm"
	// AppTitle represents the application title
	AppTitle = "USM"
)

// maxWorkers represents the max number of workers to use in parallel processing
var maxWorkers = runtime.NumCPU()

type app struct {
	win     fyne.Window
	main    *container.Scroll
	state   *usm.AppState
	storage usm.Storage

	unlockedVault map[string]*usm.Vault // this act as cache

	vault *usm.Vault

	filter map[string]*usm.VaultFilterOptions

	// USM agent client
	client agent.USMAgent

	// Toolbar instance - create once and reuse
	toolbar fyne.CanvasObject

	// Current theme instance for colour customisation
	currentTheme fyne.Theme
}

func MakeApp(s usm.Storage, w fyne.Window) fyne.CanvasObject {
	appState, err := s.LoadAppState()
	if err != nil {
		dialog.NewError(err, w)
	}

	a := &app{
		state:         appState,
		filter:        make(map[string]*usm.VaultFilterOptions),
		storage:       s,
		unlockedVault: make(map[string]*usm.Vault),
		win:           w,
		currentTheme:  theme.DefaultTheme(),
	}

	// Apply default theme colour if set
	a.applyThemeColour("")

	a.win.SetMainMenu(a.makeMainMenu())

	a.main = a.makeApp()
	a.makeSysTray()

	// Create toolbar once and store it
	a.toolbar = a.makeToolbar()

	// Create main container based on toolbar visibility preference
	if a.state.Preferences.Toolbar.Show {
		return container.NewBorder(a.toolbar, nil, nil, nil, a.main)
	} else {
		return container.NewBorder(nil, nil, nil, nil, a.main)
	}
}

// customTheme wraps the default theme to override the primary colour
type customTheme struct {
	fyne.Theme
	primaryColour color.Color
}

// Color returns custom primary colour or falls back to base theme
func (ct *customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		if ct.primaryColour != nil {
			// log.Printf("Returning custom primary colour for theme")
			return ct.primaryColour
		}
	case theme.ColorNameFocus:
		if ct.primaryColour != nil {
			return ct.primaryColour
		}
	case theme.ColorNameSelection:
		if ct.primaryColour != nil {
			// Make selection colour slightly transparent
			nrgba := convertColorToNRGBA(ct.primaryColour)
			nrgba.A = 100
			return nrgba
		}
	case theme.ColorNameButton:
		if ct.primaryColour != nil {
			return ct.primaryColour
		}
	case theme.ColorNameHover:
		if ct.primaryColour != nil {
			// Slightly darker for hover effect
			nrgba := convertColorToNRGBA(ct.primaryColour)
			if nrgba.R > 30 {
				nrgba.R -= 30
			}
			if nrgba.G > 30 {
				nrgba.G -= 30
			}
			if nrgba.B > 30 {
				nrgba.B -= 30
			}
			return nrgba
		}
	}
	return ct.Theme.Color(name, variant)
}

// applyThemeColour applies a colour to the application theme
func (a *app) applyThemeColour(vaultColour string) {
	var colourToUse string

	// Determine which colour to use
	if vaultColour != "" && usm.IsValidHexColour(vaultColour) {
		colourToUse = vaultColour
		log.Printf("Applying vault colour: %s", vaultColour)
	} else if a.state.Preferences.Theme.DefaultColour != "" && usm.IsValidHexColour(a.state.Preferences.Theme.DefaultColour) {
		colourToUse = a.state.Preferences.Theme.DefaultColour
		log.Printf("Applying default colour: %s", a.state.Preferences.Theme.DefaultColour)
	}

	if colourToUse != "" {
		// Parse and apply custom colour - use the parseHexColour from vault.go
		if col := parseHexColour(colourToUse); col != nil {
			log.Printf("Successfully parsed colour, applying theme...")
			customTheme := &customTheme{
				Theme:         theme.DefaultTheme(),
				primaryColour: col,
			}
			a.currentTheme = customTheme
			fyne.CurrentApp().Settings().SetTheme(customTheme)
		} else {
			log.Printf("Failed to parse colour: %s", colourToUse)
		}
	} else {
		log.Printf("No colour to apply, using default theme")
		// Reset to default theme
		a.currentTheme = theme.DefaultTheme()
		fyne.CurrentApp().Settings().SetTheme(theme.DefaultTheme())
	}

	// Force refresh of the entire application
	a.win.Content().Refresh()
	if a.main != nil {
		a.main.Refresh()
	}
}

func (a *app) agentClient() agent.USMAgent {
	if a.client != nil {
		return a.client
	}
	c, err := agent.NewClient(a.storage.SocketAgentPath())
	if err != nil {
		log.Println("agent not available: %w", err)
		return nil
	}
	return c
}

func (a *app) makeSysTray() {
	if desk, ok := fyne.CurrentApp().(desktop.App); ok {
		if err := checkStatusNotifierWatcher(); err != nil {
			log.Println("systray not available: %w", err)
			return
		}
		a.win.SetCloseIntercept(func() {
			// Hide window on close button, but allow Cmd+Q to quit
			// Store current content, then safely hide window
			a.win.Hide()
		})
		a.refreshSysTray()
		desk.SetSystemTrayIcon(icon.USMSystray)
	}
}

// refreshSysTray updates the system tray menu with current vaults
func (a *app) refreshSysTray() {
	if desk, ok := fyne.CurrentApp().(desktop.App); ok {
		// Create simplified menu items for system tray (no shortcuts to avoid macOS issues)
		menuItems := a.makeSimpleVaultMenuItems()
		menu := fyne.NewMenu("Vaults", menuItems...)
		desk.SetSystemTrayMenu(menu)
	}
}

// refreshMenusAfterVaultChange safely updates all menus after vault changes
func (a *app) refreshMenusAfterVaultChange() {
	// Validate that vaults can be retrieved before updating menus
	_, err := a.storage.Vaults()
	if err != nil {
		log.Printf("Failed to refresh menus: %v", err)
		return
	}

	// Ensure UI operations run on the main Fyne thread
	fyne.Do(func() {
		// Update main menu safely
		a.win.SetMainMenu(a.makeMainMenu())

		// Update system tray
		a.refreshSysTray()
	})
}

func (a *app) makeApp() *container.Scroll {
	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}

	var o fyne.CanvasObject

	switch len(vaults) {
	case 0:
		o = a.makeCreateVaultView()
	case 1:
		o = a.makeUnlockVaultView(vaults[0])
	default:
		o = a.makeSelectVaultView(vaults)
	}
	return container.NewVScroll(o)
}

func (a *app) setVaultViewByName(name string) {
	vault, ok := a.unlockedVault[name]
	if !ok {
		a.vault = nil
		a.main.Content = a.makeUnlockVaultView(name)
		a.main.Refresh()
		a.setWindowTitle()
		a.showCurrentVaultView()
		return
	}
	a.setVaultView(vault)
	// Apply the vault's colour theme
	a.applyThemeColour(vault.Colour)
	a.showCurrentVaultView()
}

func (a *app) addSSHKeyToAgent(item usm.Item) error {
	if item.GetMetadata().Type != usm.SSHKeyItemType {
		return nil
	}
	v := item.(*usm.SSHKey)
	if !v.AddToAgent {
		return nil
	}
	k, err := sshkey.ParseKey([]byte(v.PrivateKey))
	if err != nil {
		return fmt.Errorf("unable to parse SSH raw key: %w", err)
	}
	if c := a.agentClient(); c != nil {
		return c.AddSSHKey(k.PrivateKey(), v.Comment)
	}
	return nil
}

func (a *app) removeSSHKeyFromAgent(item usm.Item) error {
	if item.GetMetadata().Type != usm.SSHKeyItemType {
		return nil
	}
	v := item.(*usm.SSHKey)
	k, err := sshkey.ParseKey([]byte(v.PrivateKey))
	if err != nil {
		return fmt.Errorf("unable to parse SSH raw key: %w", err)
	}
	if c := a.agentClient(); c != nil {
		return c.RemoveSSHKey(k.PublicKey())
	}
	return nil
}

func (a *app) addSSHKeysToAgent() {
	a.vault.Range(func(id string, meta *usm.Metadata) bool {
		item, err := a.storage.LoadItem(a.vault, meta)
		if err != nil {
			return false
		}
		err = a.addSSHKeyToAgent(item)
		if err != nil {
			log.Println("unable to add SSH Key to agent:", err)
		}
		return true
	})
}

func (a *app) setVaultView(vault *usm.Vault) {
	a.vault = vault
	a.unlockedVault[vault.Name] = vault
	a.main.Content = a.makeCurrentVaultView()
	a.main.Refresh()
	a.setWindowTitle()
}

// makeToolbar creates a toolbar with common actions including preferences and vault list
func (a *app) makeToolbar() fyne.CanvasObject {
	preferencesBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		a.showPreferencesView()
	})
	preferencesBtn.Importance = widget.LowImportance

	// Create vault list button with dropdown
	vaultListBtn := a.makeVaultListButton()

	// Create a toolbar container with vault list on the left and preferences button on the right
	toolbar := container.NewBorder(nil, nil, vaultListBtn, preferencesBtn, widget.NewLabel(""))
	return toolbar
}

// makeVaultListButton creates a button with a dropdown menu showing available vaults
func (a *app) makeVaultListButton() fyne.CanvasObject {
	button := widget.NewButtonWithIcon("", icon.ChecklistOutlinedIconThemed, func() {
		// Use simplified menu items without shortcuts to avoid memory corruption
		baseItems := a.makeSimpleVaultMenuItems()
		var menuItems []*fyne.MenuItem
		if len(baseItems) > 0 {
			menuItems = append(menuItems, baseItems...)
			menuItems = append(menuItems, fyne.NewMenuItemSeparator())
		}
		menuItems = append(menuItems, fyne.NewMenuItem("New Vault", func() {
			a.showCreateVaultView()
		}))

		// Create and show popup menu at a safe position
		popUpMenu := widget.NewPopUpMenu(fyne.NewMenu("", menuItems...), a.win.Canvas())
		popUpMenu.ShowAtPosition(fyne.NewPos(10, 40)) // Simple fixed position below toolbar
	})
	button.Importance = widget.LowImportance

	return button
}

// setContentWithToolbar wraps content with a toolbar and sets it as window content
func (a *app) setContentWithToolbar(content fyne.CanvasObject) {
	// Use the single toolbar instance - avoid creating multiple instances
	var mainContainer fyne.CanvasObject
	if a.state.Preferences.Toolbar.Show {
		mainContainer = container.NewBorder(a.toolbar, nil, nil, nil, content)
	} else {
		mainContainer = container.NewBorder(nil, nil, nil, nil, content)
	}
	a.win.SetContent(mainContainer)
}

func (a *app) showAuditPasswordView() {
	a.setContentWithToolbar(a.makeAuditPasswordView())
}

func (a *app) showCreateVaultView() {
	a.setContentWithToolbar(a.makeCreateVaultView())
}

func (a *app) showCurrentVaultView() {
	// Use the single toolbar instance when showing current vault view
	var mainContainer fyne.CanvasObject
	if a.state.Preferences.Toolbar.Show {
		mainContainer = container.NewBorder(a.toolbar, nil, nil, nil, a.main)
	} else {
		mainContainer = container.NewBorder(nil, nil, nil, nil, a.main)
	}
	a.win.SetContent(mainContainer)
}

func (a *app) setWindowTitle() {
	title := "USM"
	if a.vault != nil {
		title = a.vault.Name + " - " + title
	}
	a.win.SetTitle(title)
}

func (a *app) showAddItemView() {
	a.setContentWithToolbar(a.makeAddItemView())
}

func (a *app) showItemView(fyneItemWidget FyneItemWidget) {
	a.setContentWithToolbar(a.makeShowItemView(fyneItemWidget))
}

func (a *app) showEditItemView(fyneItemWidget FyneItemWidget) {
	a.setContentWithToolbar(a.makeEditItemView(fyneItemWidget))
}

func (a *app) showPreferencesView() {
	a.setContentWithToolbar(a.makePreferencesView())
}

func (a *app) lockVault() {
	delete(a.unlockedVault, a.vault.Name)
	a.vault = nil
}

func (a *app) refreshCurrentView() {
	a.main.Content = a.makeCurrentVaultView()
	a.main.Refresh()
}

func (a *app) makeCancelHeaderButton() fyne.CanvasObject {
	var left, right fyne.CanvasObject
	if fyne.CurrentDevice().IsMobile() {
		right = widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
			a.showCurrentVaultView()
		})
	} else {
		left = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
			a.showCurrentVaultView()
		})
	}
	return container.NewBorder(nil, nil, left, right, widget.NewLabel(""))
}

// headingText returns a text formatted as heading
func headingText(text string) *canvas.Text {
	t := canvas.NewText(text, theme.Color(theme.ColorNameForeground))
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = theme.TextSubHeadingSize()
	return t
}

// logo returns the USM logo as a canvas image with the specified dimensions
func usmLogo() *canvas.Image {
	return imageFromResource(icon.USMIcon)
}

func imageFromResource(resource fyne.Resource) *canvas.Image {
	img := canvas.NewImageFromResource(resource)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(64, 64))
	return img
}

// checkStatusNotifierWatcher checks if the StatusNotifierWatcher is available on the supported unix system
func checkStatusNotifierWatcher() error {
	// systrayUnixSupportedOSes list the supported unix system by https://github.com/fyne-io/systray
	// see: https://github.com/fyne-io/systray/blob/master/systray_unix.go#L1
	systrayUnixSupportedOSes := map[string]bool{
		"linux":   true,
		"freebsd": true,
		"openbsd": true,
		"netbsd":  true,
	}
	if !systrayUnixSupportedOSes[runtime.GOOS] {
		return nil
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}
	defer conn.Close()

	obj := conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher")
	call := obj.Call("org.kde.StatusNotifierWatcher.RegisterStatusNotifierItem", 0, "/StatusNotifierItem")
	return call.Err
}
