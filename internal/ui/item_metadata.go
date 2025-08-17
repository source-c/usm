package ui

import (
	"time"

	"apps.z7.ai/usm/internal/icon"
	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Declare conformity to Item interface
var _ usm.Item = (*Metadata)(nil)

// Item represents the basic usm identity
type Metadata struct {
	*usm.Metadata
}

func (m *Metadata) Item() usm.Item {
	return m.Metadata
}

func (m *Metadata) Icon() fyne.Resource {
	if m.Favicon != nil {
		return m.Favicon
	}
	switch m.Type {
	case usm.NoteItemType:
		return icon.NoteOutlinedIconThemed
	case usm.PasswordItemType:
		return icon.PasswordOutlinedIconThemed
	case usm.LoginItemType:
		return icon.WorldWWWOutlinedIconThemed
	case usm.SSHKeyItemType:
		return icon.KeyOutlinedIconThemed
	}
	return icon.USMIcon
}

func ShowMetadata(m *usm.Metadata) fyne.CanvasObject {
	return container.New(
		layout.NewFormLayout(),
		widget.NewLabel("Modified"),
		widget.NewLabel(m.Modified.Local().Format(time.RFC1123)),
		widget.NewLabel("Created"),
		widget.NewLabel(m.Created.Local().Format(time.RFC1123)),
	)
}
