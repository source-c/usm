package ui

import (
	"context"

	"apps.z7.ai/usm/internal/icon"
	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Declare conformity to FyneItem interface
var _ FyneItemWidget = (*noteItemWidget)(nil)

func NewNoteWidget(item *usm.Note) FyneItemWidget {
	return &noteItemWidget{
		item: item,
	}
}

type noteItemWidget struct {
	item *usm.Note

	validator []fyne.Validatable
}

// OnSubmit implements FyneItem.
func (iw *noteItemWidget) OnSubmit() (usm.Item, error) {
	for _, v := range iw.validator {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	}
	return iw.Item(), nil
}

func (iw *noteItemWidget) Item() usm.Item {
	copy := usm.NewNote()
	err := deepCopyItem(iw.item, copy)
	if err != nil {
		panic(err)
	}
	return copy
}

func (iw *noteItemWidget) Icon() fyne.Resource {
	return icon.NoteOutlinedIconThemed
}

func (iw *noteItemWidget) Edit(ctx context.Context, key *usm.Key, w fyne.Window) fyne.CanvasObject {
	titleEntry := widget.NewEntryWithData(binding.BindString(&iw.item.Metadata.Name))
	titleEntry.Validator = requiredValidator("The title cannot be emtpy")
	titleEntry.PlaceHolder = "Untitled note"

	noteEntry := newNoteEntryWithData(binding.BindString(&iw.item.Value))

	_ = titleEntry.Validate()

	iw.validator = append(iw.validator, titleEntry)

	// Header with icon and title, full-width
	header := container.NewBorder(nil, nil, widget.NewIcon(iw.Icon()), nil, titleEntry)
	// Center area: note editor should fill all available vertical space with scrolling
	scroll := container.NewVScroll(noteEntry)

	return container.NewBorder(header, nil, nil, nil, scroll)
}

func (iw *noteItemWidget) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	if iw == nil {
		return container.New(layout.NewFormLayout(), widget.NewLabel(""))
	}
	rt := widget.NewRichTextFromMarkdown(iw.item.Value)
	rt.Wrapping = fyne.TextWrapWord
	return container.NewScroll(rt)
}

// noteEntry is a multiline entry widget that does not accept tab
// This will allow to change the widget focus when tab is pressed
type noteEntry struct {
	widget.Entry
}

func newNoteEntryWithData(bind binding.String) *noteEntry {
	ne := &noteEntry{
		Entry: widget.Entry{
			MultiLine: true,
			Wrapping:  fyne.TextWrap(fyne.TextTruncateEllipsis),
		},
	}
	ne.ExtendBaseWidget(ne)
	ne.Bind(bind)
	ne.Validator = nil
	return ne
}

// AcceptsTab returns if Entry accepts the Tab key or not.
//
// Implements: fyne.Tabbable
func (ne *noteEntry) AcceptsTab() bool {
	return false
}
