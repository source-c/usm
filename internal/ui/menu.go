package ui

import (
	"log"
	"net/url"
	"time"

	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *app) makeMainMenu() *fyne.MainMenu {
	// a Quit item will is appended automatically by Fyne to the first menu item
	vaultItem := fyne.NewMenuItem("Vault", nil)
	vaultItem.ChildMenu = fyne.NewMenu("", a.makeMainMenuVaultItems()...)

	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Vault", func() {
			a.showCreateVaultView()
		}),
		vaultItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Preferences", func() {
			a.showPreferencesView()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Close Window", func() {
			a.win.Hide()
		}),
		fyne.NewMenuItem("Quit", func() {
			a.win.SetCloseIntercept(nil)
			a.win.Close()
		}),
	)

	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", a.about),
	)

	return fyne.NewMainMenu(
		fileMenu,

		helpMenu,
	)
}

func (a *app) about() {
	u, _ := url.Parse("https://apps.z7.ai/usm-go")
	l := widget.NewLabel("USM - " + usm.Version())
	l.Alignment = fyne.TextAlignCenter
	link := widget.NewHyperlink("https://apps.z7.ai/usm-go", u)
	link.Alignment = fyne.TextAlignCenter
	co := container.NewCenter(
		container.NewVBox(
			usmLogo(),
			l,
			link,
		),
	)
	d := dialog.NewCustom("About USM", "Ok", co, a.win)
	d.Show()
}

func (a *app) makeVaultMenu() fyne.CanvasObject {
	d := fyne.CurrentApp().Driver()

	menuItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Password Audit", a.showAuditPasswordView),
		fyne.NewMenuItem("Import From File", a.importFromFile),
		fyne.NewMenuItem("Export To File", a.exportToFile),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Vault Settings", a.showVaultSettingsDialog),
		fyne.NewMenuItem("Destroy Vault", func() {
			if a.vault == nil {
				return
			}
			name := a.vault.Name
			dialog.NewConfirm(
				"Destroy vault",
				"You are going to destroy the vault.",
				func(first bool) {
					if !first {
						return
					}
					dialog.NewConfirm(
						"Please confirm",
						"This action can not be undone.",
						func(second bool) {
							if !second {
								return
							}
							if err := a.storage.DeleteVault(name); err != nil {
								dialog.ShowError(err, a.win)
								return
							}
							delete(a.unlockedVault, name)
							a.vault = nil
							a.applyThemeColour("")
							a.state.Modified = time.Now().UTC()
							_ = a.storage.StoreAppState(a.state)

							a.refreshMenusAfterVaultChange()
							a.main = a.makeApp()
							a.showCurrentVaultView()
							a.refreshSysTray()
						},
						a.win,
					).Show()
				},
				a.win,
			).Show()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Lock Vault", func() {
			a.main.Content = a.makeUnlockVaultView(a.vault.Name)
			a.lockVault()
			a.main.Refresh()
		}),
	}

	popUpMenu := widget.NewPopUpMenu(fyne.NewMenu("", menuItems...), a.win.Canvas())

	var button *widget.Button
	button = widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), func() {
		buttonPos := d.AbsolutePositionForObject(button)
		buttonSize := button.Size()
		popUpMin := popUpMenu.MinSize()

		var popUpPos fyne.Position
		popUpPos.X = buttonPos.X + buttonSize.Width - popUpMin.Width
		popUpPos.Y = buttonPos.Y + buttonSize.Height
		popUpMenu.ShowAtPosition(popUpPos)
	})

	return button
}

// makeMainMenuVaultItems creates vault menu items for main menu (no shortcuts to avoid macOS conflicts)
func (a *app) makeMainMenuVaultItems() []*fyne.MenuItem {
	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Printf("Failed to get vaults for main menu: %v", err)
		// Return a placeholder item instead of empty slice to prevent menu corruption
		return []*fyne.MenuItem{
			fyne.NewMenuItem("No vaults available", func() {}),
		}
	}

	// Handle empty vault list gracefully
	if len(vaults) == 0 {
		return []*fyne.MenuItem{
			fyne.NewMenuItem("No vaults created", func() {}),
		}
	}

	mi := make([]*fyne.MenuItem, len(vaults))
	for i, vaultName := range vaults {
		vaultName := vaultName // capture loop variable
		// Validate vault name before creating menu item
		if vaultName == "" {
			continue
		}
		mi[i] = fyne.NewMenuItem(vaultName, func() {
			a.setVaultViewByName(vaultName)
		})
	}

	// Filter out any nil items
	filteredItems := make([]*fyne.MenuItem, 0, len(mi))
	for _, item := range mi {
		if item != nil {
			filteredItems = append(filteredItems, item)
		}
	}

	// Ensure we always return at least one item
	if len(filteredItems) == 0 {
		return []*fyne.MenuItem{
			fyne.NewMenuItem("No valid vaults", func() {}),
		}
	}

	return filteredItems
}

// makeSimpleVaultMenuItems creates simplified vault menu items for system tray (no shortcuts)
func (a *app) makeSimpleVaultMenuItems() []*fyne.MenuItem {
	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Printf("Failed to get vaults for system tray: %v", err)
		return []*fyne.MenuItem{}
	}

	mi := make([]*fyne.MenuItem, len(vaults))
	for i, vaultName := range vaults {
		vaultName := vaultName // capture loop variable
		mi[i] = fyne.NewMenuItem(vaultName, func() {
			defer a.win.Show()
			a.setVaultViewByName(vaultName)
		})
	}
	return mi
}
