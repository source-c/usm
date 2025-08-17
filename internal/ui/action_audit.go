package ui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync/atomic"

	"apps.z7.ai/usm/internal/haveibeenpwned"
	"apps.z7.ai/usm/internal/icon"
	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// auditPasswordView returns a view to audit passwords
func (a *app) makeAuditPasswordView() fyne.CanvasObject {
	auditBtn := widget.NewButtonWithIcon("Audit", icon.ChecklistOutlinedIconThemed, func() {
		ctx, cancel := context.WithCancel(context.Background())

		itemMetadata := a.vault.FilterItemMetadata(&usm.VaultFilterOptions{ItemType: usm.PasswordItemType | usm.LoginItemType})

		modalTitle := widget.NewLabel("Auditing items...")
		progressBind := binding.NewFloat()
		progressbar := widget.NewProgressBarWithData(progressBind)
		progressbar.TextFormatter = func() string {
			v, _ := progressBind.Get()
			return fmt.Sprintf("%.0f of %d", v, len(itemMetadata))
		}

		var cancelButton *widget.Button
		cancelButton = widget.NewButton("Cancel", func() {
			modalTitle.SetText("Cancelling auditing, please wait...")
			progressbar.Hide()
			cancelButton.Disable()
			cancel()
		})

		modalContent := container.NewBorder(modalTitle, nil, nil, nil, container.NewCenter(container.NewVBox(progressbar, cancelButton)))
		modal := widget.NewModalPopUp(modalContent, a.win.Canvas())

		var counter uint32
		pwendItems := []haveibeenpwned.Pwned{}

		sem := semaphore.NewWeighted(int64(maxWorkers))
		g := &errgroup.Group{}

		go func() {
			for _, meta := range itemMetadata {
				meta := meta

				err := sem.Acquire(ctx, 1)
				if err != nil {
					cancel()
					break
				}

				g.Go(func() error {
					defer sem.Release(1)

					item, err := a.storage.LoadItem(a.vault, meta)
					if err != nil {
						return err
					}

					isPwend, count, err := haveibeenpwned.Search(ctx, item)
					if err != nil {
						return err
					}
					if isPwend {
						pwendItems = append(pwendItems, haveibeenpwned.Pwned{Item: item, Count: count})
					}

					v := atomic.AddUint32(&counter, 1)
					_ = progressBind.Set(float64(v))
					return nil
				})
			}

			defer modal.Hide()
			err := g.Wait()
			if err != nil || errors.Is(ctx.Err(), context.Canceled) {
				dialog.ShowError(err, a.win)
				return
			}

			sort.Slice(pwendItems, func(i, j int) bool { return pwendItems[i].Count > pwendItems[j].Count })

			num := len(pwendItems)
			if num == 0 {
				image := imageFromResource(icon.CircleCheckOutlinedIconThemed)
				text := widget.NewLabel("No password found in data breaches")
				text.Wrapping = fyne.TextWrapWord
				text.Alignment = fyne.TextAlignCenter
				content := container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, container.NewVBox(image, text))
				a.setContentWithToolbar(content)
				return
			}

			image := imageFromResource(theme.WarningIcon())
			text := widget.NewLabel("Passwords of the items below have been found in a data breaches and should not be used")
			text.Wrapping = fyne.TextWrapWord
			text.Alignment = fyne.TextAlignCenter
			list := widget.NewList(
				func() int {
					return len(pwendItems)
				},
				func() fyne.CanvasObject {
					return container.NewBorder(
						nil, nil,
						widget.NewIcon(icon.PasswordOutlinedIconThemed),
						widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil),
						widget.NewLabel("item label"),
					)
				},
				func(lii widget.ListItemID, co fyne.CanvasObject) {
					v := pwendItems[lii]
					item := v.Item
					metadata := item.GetMetadata()
					fyneItemWidget := NewFyneItemWidget(v.Item, a.state.Preferences)
					co.(*fyne.Container).Objects[0].(*widget.Label).SetText(fmt.Sprintf("%s (found %d times)", metadata.Name, v.Count))
					co.(*fyne.Container).Objects[1].(*widget.Icon).SetResource(fyneItemWidget.Icon())
					co.(*fyne.Container).Objects[2].(*widget.Button).OnTapped = func() {
						a.showEditItemView(fyneItemWidget)
					}
				},
			)
			list.OnSelected = func(id widget.ListItemID) {
				fyneItemWidget := NewFyneItemWidget(pwendItems[id].Item, a.state.Preferences)
				a.showItemView(fyneItemWidget)
			}
			c := container.NewBorder(container.NewVBox(image, text), nil, nil, nil, list)
			content := container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, c)
			a.setContentWithToolbar(content)
		}()
		modal.Show()
	})

	image := imageFromResource(icon.ChecklistOutlinedIconThemed)
	text := widget.NewLabel("Check Vault passwords against existing data breaches")
	text.Wrapping = fyne.TextWrapWord
	text.Alignment = fyne.TextAlignCenter
	c := container.NewBorder(container.NewVBox(image, text, auditBtn), nil, nil, nil, nil)
	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, c)
}
