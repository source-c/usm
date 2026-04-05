package ui

import (
	"fmt"
	"time"

	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// showChangePasswordDialog shows dialog for changing current vault password
func (a *app) showChangePasswordDialog() {
	if a.vault == nil {
		return
	}

	oldPasswordEntry := widget.NewPasswordEntry()
	oldPasswordEntry.SetPlaceHolder("Current Password")

	newPasswordEntry := widget.NewPasswordEntry()
	newPasswordEntry.SetPlaceHolder("New Password")

	confirmPasswordEntry := widget.NewPasswordEntry()
	confirmPasswordEntry.SetPlaceHolder("Confirm New Password")

	form := container.NewVBox(
		widget.NewLabel("Update the vault master password"),
		oldPasswordEntry,
		newPasswordEntry,
		confirmPasswordEntry,
	)

	dialog.NewCustomConfirm(
		"Change Password",
		"Change",
		"Cancel",
		form,
		func(confirmed bool) {
			if !confirmed {
				return
			}

			if oldPasswordEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("current password is required"), a.win)
				return
			}
			if newPasswordEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("new password is required"), a.win)
				return
			}
			if newPasswordEntry.Text != confirmPasswordEntry.Text {
				dialog.ShowError(fmt.Errorf("new passwords do not match"), a.win)
				return
			}

			progressDialog := dialog.NewCustomWithoutButtons(
				"Changing Password",
				widget.NewProgressBarInfinite(),
				a.win,
			)
			progressDialog.Show()

			go func() {
				newVault, err := a.storage.ChangeVaultPassword(
					a.vault,
					oldPasswordEntry.Text,
					newPasswordEntry.Text,
				)

				fyne.Do(func() {
					progressDialog.Hide()
					if err != nil {
						dialog.ShowError(err, a.win)
						return
					}

					a.vault = newVault
					a.unlockedVault[newVault.Name] = newVault

					if a.state.VaultCatalogue == nil {
						a.state.VaultCatalogue = make(map[string]*usm.VaultEntry)
					}
					usm.UpdateVaultCatalogue(a.state.VaultCatalogue, newVault, a.storage)
					a.state.Modified = time.Now().UTC()
					if err := a.storage.StoreAppState(a.state); err != nil {
						dialog.ShowError(err, a.win)
						return
					}

					dialog.ShowInformation(
						"Password Updated",
						"Vault password changed successfully. Previous version stored as backup.",
						a.win,
					)
				})
			}()
		},
		a.win,
	).Show()
}
