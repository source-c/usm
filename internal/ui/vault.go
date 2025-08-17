package ui

import (
	"fmt"
	"image/color"
	"log"
	"strconv"
	"strings"
	"time"

	"apps.z7.ai/usm/internal/icon"
	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *app) makeCreateVaultView() fyne.CanvasObject {
	logo := usmLogo()

	heading := headingText("Create a new vault")

	name := widget.NewEntry()
	name.SetPlaceHolder("Name")

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	// Colour selection widget - start with default application colour
	defaultColour := a.state.Preferences.Theme.DefaultColour
	colourWidget := a.makeColourSelectionWidget(defaultColour)

	btn := widget.NewButton("Create", func() {
		key, err := a.storage.CreateVaultKey(name.Text, password.Text)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		vault, err := a.storage.CreateVault(name.Text, key)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}

		// Set the selected colour
		selectedColour := a.getSelectedColourFromWidget(colourWidget)
		vault.Colour = selectedColour
		vault.Modified = time.Now().UTC()
		log.Printf("Creating vault with colour: %s", selectedColour)

		// Store the updated vault
		err = a.storage.StoreVault(vault)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}

		a.setVaultView(vault)
		a.showCurrentVaultView()

		// Apply the vault's colour theme
		log.Printf("Applying theme for new vault: %s", vault.Colour)
		a.applyThemeColour(vault.Colour)

		// Defer menu updates to avoid menu corruption during vault creation
		go func() {
			time.Sleep(100 * time.Millisecond) // Brief delay to let vault creation complete
			a.refreshMenusAfterVaultChange()
		}()
	})

	return container.NewCenter(container.NewVBox(logo, heading, name, password, colourWidget, btn))
}

func (a *app) makeSelectVaultView(vaults []string) fyne.CanvasObject {
	heading := headingText("Select a Vault")
	heading.Alignment = fyne.TextAlignCenter

	c := container.NewVBox(usmLogo(), heading)

	for _, v := range vaults {
		name := v
		resource := icon.LockOpenOutlinedIconThemed
		if _, ok := a.unlockedVault[name]; !ok {
			resource = icon.LockOutlinedIconThemed
		}
		btn := widget.NewButtonWithIcon(name, resource, func() {
			a.setVaultViewByName(name)
		})
		btn.Alignment = widget.ButtonAlignLeading
		c.Add(btn)
	}
	return container.NewCenter(c)
}

func (a *app) makeUnlockVaultView(vaultName string) fyne.CanvasObject {
	return NewUnlockerVaultWidget(vaultName, a)
}

func (a *app) makeCurrentVaultView() fyne.CanvasObject {
	vault := a.vault
	filter, ok := a.filter[vault.Name]
	if !ok {
		filter = &usm.VaultFilterOptions{}
		a.filter[vault.Name] = filter
	}

	itemsWidget := newItemsWidget(vault, filter)
	itemsWidget.OnSelected = func(meta *usm.Metadata) {
		item, err := a.storage.LoadItem(vault, meta)
		if err != nil {
			msg := fmt.Sprintf("error loading %q.\nDo you want delete from the vault?", meta.Name)
			fyne.LogError("error loading item from vault", err)
			dialog.NewConfirm(
				"Error",
				msg,
				func(delete bool) {
					if delete {
						item, err = usm.NewItem(meta.Name, meta.Type)
						vault.DeleteItem(item)                // remove item from vault
						_ = a.removeSSHKeyFromAgent(item)     // remove item from ssh agent
						_ = a.storage.DeleteItem(vault, item) // remove item from storage
						now := time.Now().UTC()
						vault.Modified = now
						_ = a.storage.StoreVault(vault) // ensure vault is up-to-date
						a.state.Modified = now
						_ = a.storage.StoreAppState(a.state)
						itemsWidget.Reload(nil, filter)
					}
				},
				a.win,
			).Show()
			return
		}

		fyneItemWidget := NewFyneItemWidget(item, a.state.Preferences)
		a.showItemView(fyneItemWidget)
		itemsWidget.listEntry.UnselectAll()
	}

	// search entries
	search := widget.NewEntry()
	search.SetPlaceHolder("Search")
	search.SetText(filter.Name)
	search.OnChanged = func(s string) {
		filter.Name = s
		itemsWidget.Reload(nil, filter)
	}

	// filter entries
	itemTypeMap := map[string]usm.ItemType{}
	options := []string{fmt.Sprintf("All items (%d)", vault.Size())}
	for _, item := range a.makeEmptyItems() {
		i := item
		t := i.GetMetadata().Type
		name := fmt.Sprintf("%s (%d)", t.Label(), vault.SizeByType(t))
		options = append(options, name)
		itemTypeMap[name] = t
	}

	list := widget.NewSelect(options, func(s string) {
		var v usm.ItemType
		if s == options[0] {
			v = usm.ItemType(0) // No item type will be selected
		} else {
			v = itemTypeMap[s]
		}

		filter.ItemType = v
		itemsWidget.Reload(nil, filter)
	})

	list.SetSelectedIndex(0)

	header := container.NewBorder(nil, nil, nil, a.makeVaultMenu(), list)

	button := widget.NewButtonWithIcon("Add item", theme.ContentAddIcon(), func() {
		a.showAddItemView()
	})

	// layout so we can focus the search box using shift+tab
	return container.NewBorder(search, nil, nil, nil, container.NewBorder(header, button, nil, nil, itemsWidget))
}

// colourSelectionWidget stores the state for colour selection
type colourSelectionWidget struct {
	widget.BaseWidget
	selectedColour       string
	predefinedSelect     *widget.Select
	colourRect           *canvas.Rectangle
	container            *fyne.Container
	app                  *app
	customDialogCallback func() // Optional callback for custom dialog behaviour
}

// makeColourSelectionWidget creates a widget for selecting vault colours
func (a *app) makeColourSelectionWidget(currentColour string) *colourSelectionWidget {
	csw := &colourSelectionWidget{
		selectedColour: currentColour,
		app:            a,
	}
	csw.ExtendBaseWidget(csw)

	// Create predefined colour options (without "Custom...")
	colourOptions := make([]string, len(usm.PredefinedColours))
	selectedIndex := 0
	for i, option := range usm.PredefinedColours {
		colourOptions[i] = option.Name
		if option.Value == currentColour {
			selectedIndex = i
		}
	}

	// Check if current colour is custom (not in predefined list)
	isCustom := true
	for _, option := range usm.PredefinedColours {
		if option.Value == currentColour {
			isCustom = false
			break
		}
	}

	// If custom colour, add it as an option and select it
	if isCustom && currentColour != "" {
		customLabel := fmt.Sprintf("Custom (%s)", currentColour)
		colourOptions = append(colourOptions, customLabel)
		selectedIndex = len(colourOptions) - 1
	}

	// Create colour rectangle that shows the actual colour
	csw.colourRect = canvas.NewRectangle(color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	csw.colourRect.StrokeColor = color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	csw.colourRect.StrokeWidth = 1

	// Create invisible button for clicks
	clickButton := &transparentButton{}
	clickButton.Button = *widget.NewButton("", func() {
		if csw.customDialogCallback != nil {
			csw.customDialogCallback()
		} else {
			csw.app.showCustomColourDialog("Custom Colour", csw.selectedColour, func(newColour string) {
				csw.selectedColour = newColour
				csw.updateColourRect()
				csw.updateDropdownForCustomColour(newColour)
			})
		}
	})
	clickButton.ExtendBaseWidget(clickButton)

	// Create container with absolute positioning to force square shape
	colourContainer := container.NewWithoutLayout(csw.colourRect, clickButton)

	// Force exact 24x24 square positioning
	csw.colourRect.Resize(fyne.NewSize(24, 24))
	csw.colourRect.Move(fyne.NewPos(0, 0))
	clickButton.Resize(fyne.NewSize(24, 24))
	clickButton.Move(fyne.NewPos(0, 0))
	colourContainer.Resize(fyne.NewSize(24, 24))

	// Create predefined colour select
	csw.predefinedSelect = widget.NewSelect(colourOptions, func(selected string) {
		// Handle selection change
		if strings.HasPrefix(selected, "Custom (") {
			// Keep the existing custom colour
			return
		}

		// Find the colour value for the selected predefined option
		for _, option := range usm.PredefinedColours {
			if option.Name == selected {
				csw.selectedColour = option.Value
				log.Printf("Selected predefined colour: %s = %s", selected, option.Value)
				break
			}
		}
		csw.updateColourRect()
	})

	// Create a container that matches the dropdown height for alignment
	alignedSquare := container.NewWithoutLayout(colourContainer)
	alignedSquare.Resize(fyne.NewSize(24, 32)) // Match typical dropdown height
	colourContainer.Move(fyne.NewPos(0, 4))    // Move square down 4px to center in 32px height

	// Layout - horizontal layout with height-matched square
	content := container.NewHBox(
		csw.predefinedSelect,
		layout.NewSpacer(), // Add flexible spacer
		alignedSquare,
		layout.NewSpacer(), // Add spacer for balance
	)

	csw.container = content

	// Set initial state
	csw.updateColourRect()
	if selectedIndex < len(colourOptions) {
		csw.predefinedSelect.SetSelectedIndex(selectedIndex)
	}

	return csw
}

// transparentButton is a button that is completely invisible but still clickable
type transparentButton struct {
	widget.Button
}

// CreateRenderer creates a completely transparent renderer for the button
func (tb *transparentButton) CreateRenderer() fyne.WidgetRenderer {
	return &transparentButtonRenderer{button: tb}
}

// transparentButtonRenderer renders nothing (completely invisible)
type transparentButtonRenderer struct {
	button *transparentButton
}

func (r *transparentButtonRenderer) Layout(size fyne.Size) {
	// Nothing to layout - invisible
}

func (r *transparentButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *transparentButtonRenderer) Refresh() {
	// Nothing to refresh - invisible
}

func (r *transparentButtonRenderer) BackgroundColor() color.Color {
	return color.Transparent
}

func (r *transparentButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{} // No visual objects
}

func (r *transparentButtonRenderer) Destroy() {
	// Nothing to destroy
}

// updateColourRect updates the colour rectangle to show the current selection
func (csw *colourSelectionWidget) updateColourRect() {
	switch {
	case csw.selectedColour == "":
		csw.colourRect.FillColor = color.NRGBA{R: 0, G: 0, B: 0, A: 0}

		// Use default application colour if set, otherwise fall back to theme primary
		var primaryColour color.Color
		if csw.app.state.Preferences.Theme.DefaultColour != "" && usm.IsValidHexColour(csw.app.state.Preferences.Theme.DefaultColour) {
			primaryColour = parseHexColour(csw.app.state.Preferences.Theme.DefaultColour)
		} else {
			primaryColour = fyne.CurrentApp().Settings().Theme().Color(theme.ColorNamePrimary, theme.VariantDark)
		}

		semiTransparentPrimary := convertColorToNRGBA(primaryColour)
		semiTransparentPrimary.A = 100 // Make it semi-transparent
		csw.colourRect.StrokeColor = semiTransparentPrimary
	case usm.IsValidHexColour(csw.selectedColour):
		// Parse hex colour
		if col := parseHexColour(csw.selectedColour); col != nil {
			// Show the actual colour
			nrgba := convertColorToNRGBA(col)
			csw.colourRect.FillColor = nrgba
			csw.colourRect.StrokeColor = color.NRGBA{R: 0, G: 0, B: 0, A: 150}
		} else {
			// Invalid colour - show as red
			csw.colourRect.FillColor = color.NRGBA{R: 255, G: 0, B: 0, A: 255}
			csw.colourRect.StrokeColor = color.NRGBA{R: 200, G: 0, B: 0, A: 255}
		}
	default:
		// Invalid colour - show as red
		csw.colourRect.FillColor = color.NRGBA{R: 255, G: 0, B: 0, A: 255}
		csw.colourRect.StrokeColor = color.NRGBA{R: 200, G: 0, B: 0, A: 255}
	}

	// Force refresh
	csw.colourRect.Refresh()
}

// showCustomColourDialog shows a reusable popup dialog for entering a custom hex colour
func (a *app) showCustomColourDialog(title string, currentColour string, onApply func(string)) {
	// Create entry for hex colour input
	hexEntry := widget.NewEntry()
	hexEntry.SetPlaceHolder("#FF0000")

	// No resize - let the container handle it properly
	if currentColour != "" && usm.IsValidHexColour(currentColour) {
		hexEntry.SetText(currentColour)
	}

	// Create colour preview square with default semi-transparent primary styling
	previewSquare := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0})

	// Use default application colour if set, otherwise fall back to theme primary
	var primaryColour color.Color
	if a.state.Preferences.Theme.DefaultColour != "" && usm.IsValidHexColour(a.state.Preferences.Theme.DefaultColour) {
		primaryColour = parseHexColour(a.state.Preferences.Theme.DefaultColour)
	} else {
		primaryColour = fyne.CurrentApp().Settings().Theme().Color(theme.ColorNamePrimary, theme.VariantDark)
	}

	semiTransparentPrimary := convertColorToNRGBA(primaryColour)
	semiTransparentPrimary.A = 100 // Make it semi-transparent
	previewSquare.StrokeColor = semiTransparentPrimary
	previewSquare.StrokeWidth = 1

	// Simple validation feedback
	validationLabel := widget.NewLabel("")

	// Create container with BOTH square and label for perfect alignment
	previewContainer := container.NewWithoutLayout(previewSquare, validationLabel)
	previewContainer.Resize(fyne.NewSize(300, 24)) // Wide enough for both
	previewSquare.Resize(fyne.NewSize(16, 16))
	previewSquare.Move(fyne.NewPos(0, 10))   // Position square lower to match text center, why 10?
	validationLabel.Move(fyne.NewPos(22, 0)) // Position label at same level

	updatePreview := func() {
		if usm.IsValidHexColour(hexEntry.Text) {
			validationLabel.SetText("✓ Valid colour: " + hexEntry.Text)
			validationLabel.Importance = widget.SuccessImportance

			// Update preview square with actual colour
			if col := parseHexColour(hexEntry.Text); col != nil {
				previewSquare.FillColor = convertColorToNRGBA(col)
				previewSquare.StrokeColor = color.NRGBA{R: 0, G: 0, B: 0, A: 150}
			}
		} else {
			validationLabel.SetText("✗ Invalid hex colour")
			validationLabel.Importance = widget.DangerImportance

			// Reset preview square to semi-transparent primary for invalid input
			previewSquare.FillColor = color.NRGBA{R: 0, G: 0, B: 0, A: 0}

			// Use default application colour if set, otherwise fall back to theme primary
			var primaryColour color.Color
			if a.state.Preferences.Theme.DefaultColour != "" && usm.IsValidHexColour(a.state.Preferences.Theme.DefaultColour) {
				primaryColour = parseHexColour(a.state.Preferences.Theme.DefaultColour)
			} else {
				primaryColour = fyne.CurrentApp().Settings().Theme().Color(theme.ColorNamePrimary, theme.VariantDark)
			}

			semiTransparentPrimary := convertColorToNRGBA(primaryColour)
			semiTransparentPrimary.A = 100 // Make it semi-transparent
			previewSquare.StrokeColor = semiTransparentPrimary
		}
		validationLabel.Refresh()
		previewSquare.Refresh()
	}

	hexEntry.OnChanged = func(string) { updatePreview() }
	updatePreview() // Initial update

	// Use the combined container directly
	validationRow := previewContainer

	content := container.NewVBox(
		widget.NewLabel("Enter a hex colour code:"),
		hexEntry,
		validationRow,
		widget.NewLabel("Example: #FF5722, #2196F3"),
	)

	// Use standard dialog - no custom layout mess
	d := dialog.NewCustomConfirm(
		title,
		"Apply",
		"Cancel",
		content,
		func(apply bool) {
			if apply && usm.IsValidHexColour(hexEntry.Text) {
				log.Printf("Applying custom colour: %s", hexEntry.Text)
				onApply(hexEntry.Text)
			}
		},
		a.win,
	)
	d.Show()
}

// updateDropdownForCustomColour updates the dropdown to show the custom colour
func (csw *colourSelectionWidget) updateDropdownForCustomColour(customColour string) {
	// Rebuild options with the new custom colour
	colourOptions := make([]string, len(usm.PredefinedColours))
	for i, option := range usm.PredefinedColours {
		colourOptions[i] = option.Name
	}

	customLabel := fmt.Sprintf("Custom (%s)", customColour)
	colourOptions = append(colourOptions, customLabel)

	// Update the select widget
	csw.predefinedSelect.Options = colourOptions
	csw.predefinedSelect.SetSelectedIndex(len(colourOptions) - 1)
	csw.predefinedSelect.Refresh()
}

// CreateRenderer creates the renderer for the colour selection widget
func (csw *colourSelectionWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(csw.container)
}

// getSelectedColourFromWidget extracts the selected colour from the widget
func (a *app) getSelectedColourFromWidget(widget *colourSelectionWidget) string {
	return widget.selectedColour
}

// parseHexColour converts a hex colour string to a color.Color
// This function is used across the UI package for colour parsing
func parseHexColour(hex string) color.Color {
	if hex == "" || len(hex) != 7 || hex[0] != '#' {
		return nil
	}

	r, err := strconv.ParseUint(hex[1:3], 16, 8)
	if err != nil {
		return nil
	}
	g, err := strconv.ParseUint(hex[3:5], 16, 8)
	if err != nil {
		return nil
	}
	b, err := strconv.ParseUint(hex[5:7], 16, 8)
	if err != nil {
		return nil
	}

	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

// convertColorToNRGBA safely converts any color.Color to color.NRGBA
// Since parseHexColour already returns color.NRGBA, this is a simple cast
func convertColorToNRGBA(col color.Color) color.NRGBA {
	// parseHexColour always returns color.NRGBA, so this cast is safe
	return col.(color.NRGBA)
}

// showVaultSettingsDialog shows the vault settings dialog
func (a *app) showVaultSettingsDialog() {
	if a.vault == nil {
		return
	}

	// Create colour selection widget with current vault colour
	colourWidget := a.makeColourSelectionWidget(a.vault.Colour)

	// Create form
	form := container.NewVBox(
		widget.NewCard("Vault Colour", "Choose a colour theme for this vault", colourWidget),
	)

	// Create dialog
	d := dialog.NewCustomConfirm(
		"Vault Settings",
		"Save",
		"Cancel",
		form,
		func(save bool) {
			if save {
				// Get selected colour
				newColour := a.getSelectedColourFromWidget(colourWidget)
				log.Printf("Vault settings: attempting to save colour: %s", newColour)

				// Validate custom colour if entered
				if newColour != "" && !usm.IsValidHexColour(newColour) {
					dialog.ShowError(fmt.Errorf("invalid hex colour: %s", newColour), a.win)
					return
				}

				// Update vault colour
				a.vault.Colour = newColour
				a.vault.Modified = time.Now().UTC()
				log.Printf("Vault colour updated to: %s", a.vault.Colour)

				// Store the updated vault
				err := a.storage.StoreVault(a.vault)
				if err != nil {
					dialog.ShowError(err, a.win)
					return
				}

				// Update app state and refresh UI
				a.state.Modified = time.Now().UTC()
				_ = a.storage.StoreAppState(a.state)

				// Apply the new colour theme to the UI
				a.applyThemeColour(newColour)
			}
		},
		a.win,
	)

	d.Resize(fyne.NewSize(400, 300))
	d.Show()
}
