package ui

import (
	"context"
	"log"
	"strconv"

	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	usmsync "apps.z7.ai/usm/internal/sync"
)

func (a *app) makePreferencesView() fyne.CanvasObject {
	content := container.NewVScroll(
		container.NewVBox(
			a.makeFaviconDownloaderPreferencesCard(),
			a.makePasswordPreferencesCard(),
			a.makeTOTPPreferencesCard(),
			a.makeThemePreferencesCard(),
			a.makeToolbarPreferencesCard(),
			a.makeSyncPreferencesCard(),
		),
	)

	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, content)
}

func (a *app) storePreferences() {
	err := a.storage.StoreAppState(a.state)
	if err != nil {
		dialog.ShowError(err, a.win)
	}
}

func (a *app) makeFaviconDownloaderPreferencesCard() fyne.CanvasObject {
	checkbox := widget.NewCheck("Disabled", func(disabled bool) {
		a.state.Preferences.FaviconDownloader.Disabled = disabled
		a.storePreferences()
	})
	checkbox.Checked = a.state.Preferences.FaviconDownloader.Disabled

	return widget.NewCard(
		"Favicon Downloader",
		"",
		checkbox,
	)
}

func (a *app) makePasswordPreferencesCard() fyne.CanvasObject {
	passphraseCard := widget.NewCard(
		"Passphrase",
		"",
		a.makePreferenceLenghtWidget(
			&a.state.Preferences.Password.Passphrase.DefaultLength,
			a.state.Preferences.Password.Passphrase.MinLength,
			a.state.Preferences.Password.Passphrase.MaxLength,
		),
	)
	pinCard := widget.NewCard(
		"Pin",
		"",
		a.makePreferenceLenghtWidget(
			&a.state.Preferences.Password.Pin.DefaultLength,
			a.state.Preferences.Password.Pin.MinLength,
			a.state.Preferences.Password.Pin.MaxLength,
		),
	)
	randomCard := widget.NewCard(
		"Random Password",
		"",
		a.makePreferenceLenghtWidget(
			&a.state.Preferences.Password.Random.DefaultLength,
			a.state.Preferences.Password.Random.MinLength,
			a.state.Preferences.Password.Random.MaxLength,
		),
	)
	return container.NewVBox(passphraseCard, pinCard, randomCard)
}

func (a *app) makeTOTPPreferencesCard() fyne.CanvasObject {
	form := container.New(layout.NewFormLayout())

	hashOptions := []string{string(usm.SHA1), string(usm.SHA256), string(usm.SHA512)}
	hashSelect := widget.NewSelect(hashOptions, func(selected string) {
		a.state.Preferences.TOTP.Hash = usm.TOTPHash(selected)
		a.storePreferences()
	})
	hashSelect.Selected = string(a.state.Preferences.TOTP.Hash)
	form.Add(labelWithStyle("Hash Algorithm"))
	form.Add(hashSelect)

	digitsOptions := []string{"5", "6", "7", "8", "9", "10"}
	digitsSelect := widget.NewSelect(digitsOptions, func(selected string) {
		a.state.Preferences.TOTP.Digits, _ = strconv.Atoi(selected)
		a.storePreferences()
	})
	digitsSelect.Selected = strconv.Itoa(a.state.Preferences.TOTP.Digits)
	form.Add(labelWithStyle("Digits"))
	form.Add(digitsSelect)

	intervalBind := binding.BindInt(&a.state.Preferences.TOTP.Interval)
	intervalSlider := widget.NewSlider(5, 60)
	intervalSlider.Step = 5
	intervalSlider.OnChanged = func(f float64) {
		_ = intervalBind.Set(int(f))
		a.storePreferences()
	}
	intervalSlider.Value = float64(a.state.Preferences.TOTP.Interval)
	intervalEntry := widget.NewLabelWithData(binding.IntToString(intervalBind))
	form.Add(labelWithStyle("Interval"))
	form.Add(container.NewBorder(nil, nil, nil, intervalEntry, intervalSlider))

	return widget.NewCard(
		"2FA & TOTP",
		"",
		form,
	)
}

func (a *app) makePreferenceLenghtWidget(lenght *int, min, max int) fyne.CanvasObject {
	lengthBind := binding.BindInt(lenght)
	lengthEntry := widget.NewEntryWithData(binding.IntToString(lengthBind))
	lengthEntry.Disabled()
	lengthEntry.Validator = nil
	lengthEntry.OnChanged = func(value string) {
		if value == "" {
			return
		}
		l, err := strconv.Atoi(value)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		if l < min || l > max {
			log.Printf("lenght must be between %d and %d, got %d", min, max, l)
			return
		}
		_ = lengthBind.Set(l)
		a.storePreferences()
	}

	lengthSlider := widget.NewSlider(float64(min), float64(max))
	lengthSlider.OnChanged = func(f float64) {
		_ = lengthBind.Set(int(f))
		a.storePreferences()
	}
	lengthSlider.SetValue(float64(*lenght))
	return container.NewBorder(nil, nil, widget.NewLabel("Default lenght"), lengthEntry, lengthSlider)
}

func (a *app) makeThemePreferencesCard() fyne.CanvasObject {
	// Create a colour selection widget for default application colour
	colourWidget := a.makeColourSelectionWidget(a.state.Preferences.Theme.DefaultColour)

	// Create a wrapper to handle preference saving and global theme update
	savePrefs := func() {
		a.state.Preferences.Theme.DefaultColour = colourWidget.selectedColour
		a.storePreferences()
		// Apply the new default colour to the entire application theme
		a.applyThemeColour("")
	}

	// Add change handler to save preferences when colour changes
	originalOnChange := colourWidget.predefinedSelect.OnChanged
	colourWidget.predefinedSelect.OnChanged = func(selected string) {
		// Call original handler first
		if originalOnChange != nil {
			originalOnChange(selected)
		}
		// Save the new default colour
		savePrefs()
	}

	// Override the custom dialog callback to save preferences
	colourWidget.customDialogCallback = func() {
		a.showCustomColourDialogForPreferences(colourWidget, savePrefs)
	}

	return widget.NewCard(
		"Theme & Appearance",
		"Set the default application colour theme",
		container.NewVBox(
			widget.NewLabel("Default Application Colour:"),
			colourWidget,
		),
	)
}

// showCustomColourDialogForPreferences shows the custom colour dialog for preferences with save callback
func (a *app) showCustomColourDialogForPreferences(csw *colourSelectionWidget, saveCallback func()) {
	// Use the reusable custom colour dialog
	a.showCustomColourDialog("Default Application Colour", csw.selectedColour, func(newColour string) {
		csw.selectedColour = newColour
		csw.updateColourRect()
		csw.updateDropdownForCustomColour(newColour)
		saveCallback()
		// Refresh the entire window to apply theme changes
		a.win.Content().Refresh()
	})
}

func (a *app) makeToolbarPreferencesCard() fyne.CanvasObject {
	checkbox := widget.NewCheck("Show toolbar", func(show bool) {
		a.state.Preferences.Toolbar.Show = show
		a.storePreferences()
		// Refresh the current window content with new toolbar state
	})
	checkbox.Checked = a.state.Preferences.Toolbar.Show

	return widget.NewCard(
		"User Interface",
		"Toggle visibility of the in-app menu bar",
		checkbox,
	)
}

func (a *app) makeSyncPreferencesCard() fyne.CanvasObject {
	modeOptions := []string{"Disabled", "Relaxed", "Strict"}
	modeMap := map[string]string{
		"Disabled": usm.SyncModeDisabled,
		"Relaxed":  usm.SyncModeRelaxed,
		"Strict":   usm.SyncModeStrict,
	}
	reverseMap := map[string]string{
		usm.SyncModeDisabled: "Disabled",
		usm.SyncModeRelaxed:  "Relaxed",
		usm.SyncModeStrict:   "Strict",
	}

	discoveryBtn := widget.NewButton("Open Discovery", func() {
		a.showDiscoveryView()
	})
	discoveryBtn.Importance = widget.HighImportance
	discoveryBtn.Hide()
	if a.syncService != nil && a.syncService.IsRunning() {
		discoveryBtn.Show()
	}

	modeSelect := widget.NewSelect(modeOptions, func(selected string) {
		val, ok := modeMap[selected]
		if !ok {
			return
		}
		a.state.Preferences.Sync.Mode = val
		a.storePreferences()
		a.applySyncMode(val, discoveryBtn)
	})
	modeSelect.Selected = reverseMap[a.state.Preferences.Sync.Mode]

	form := container.New(layout.NewFormLayout())
	form.Add(labelWithStyle("Sync Mode"))
	form.Add(modeSelect)

	return widget.NewCard(
		"LAN Sync",
		"Peer discovery over local network",
		container.NewVBox(form, discoveryBtn),
	)
}

// applySyncMode starts or stops the sync service when the user changes the preference
func (a *app) applySyncMode(mode string, discoveryBtn *widget.Button) {
	enabled := mode == usm.SyncModeRelaxed || mode == usm.SyncModeStrict

	if enabled && (a.syncService == nil || !a.syncService.IsRunning()) {
		svc, err := usmsync.NewService(usmsync.ServiceConfig{
			PeerKeyPath:      a.storage.PeerKeyPath(),
			TrustedPeersPath: a.storage.TrustedPeersPath(),
			StorageRoot:      a.storage.Root(),
			SyncMode:         mode,
			Storage:          a.storage,
		})
		if err != nil {
			log.Println("Could not create sync service:", err)
			dialog.ShowError(err, a.win)
			return
		}
		if err := svc.Start(context.Background()); err != nil {
			log.Println("Could not start sync service:", err)
			dialog.ShowError(err, a.win)
			return
		}
		a.syncService = svc
		discoveryBtn.Show()
		// Rebuild toolbar and menu to include discovery button
		a.toolbar = a.makeToolbar()
		a.win.SetMainMenu(a.makeMainMenu())
	}

	if !enabled && a.syncService != nil && a.syncService.IsRunning() {
		_ = a.syncService.Stop()
		a.syncService = nil
		discoveryBtn.Hide()
		a.toolbar = a.makeToolbar()
		a.win.SetMainMenu(a.makeMainMenu())
	}
}
