package usm

import (
	"time"
)

// Declare conformity to Item interface
var _ Item = (*Note)(nil)

type Note struct {
	Value     string `json:"value,omitempty"`
	*Metadata `json:"metadata,omitempty"`
}

func NewNote() *Note {
	now := time.Now().UTC()
	return &Note{
		Metadata: &Metadata{
			Type:     NoteItemType,
			Created:  now,
			Modified: now,
		},
	}
}
