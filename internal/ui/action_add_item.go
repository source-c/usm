package ui

import (
	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (a *app) makeAddItemView() fyne.CanvasObject {
	c := container.NewVBox()
	for _, item := range a.makeEmptyItems() {
		i := item
		metadata := i.GetMetadata()
		fyneItem := NewFyneItemWidget(i, a.state.Preferences)
		o := widget.NewButtonWithIcon(metadata.Type.Label(), fyneItem.Icon(), func() {
			a.showEditItemView(fyneItem)
		})
		o.Alignment = widget.ButtonAlignLeading
		c.Add(o)
	}

	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, container.NewCenter(c))
}

// makeEmptyItems returns a slice of empty usm.Item ready to use as template for
// item's creation
func (a *app) makeEmptyItems() []usm.Item {
	note := usm.NewNote()
	password := usm.NewPassword()
	website := usm.NewLogin()
	website.TOTP = &usm.TOTP{
		Digits:   a.state.Preferences.TOTP.Digits,
		Hash:     a.state.Preferences.TOTP.Hash,
		Interval: a.state.Preferences.TOTP.Interval,
	}
	sshkey := usm.NewSSHKey()

	return []usm.Item{
		note,
		password,
		website,
		sshkey,
	}
}
