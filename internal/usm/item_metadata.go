package usm

import (
	"encoding/hex"
	"time"

	"golang.org/x/crypto/blake2b"
)

// MetadataSubtitler is the interface to implement to provide a subtitle to an item
type MetadataSubtitler interface {
	Subtitle() string
}

// Item represents the basic usm identity
type Metadata struct {
	// Name reprents the item name
	Name string `json:"name,omitempty"`
	// Subtitle represents the item subtitle
	Subtitle string `json:"subtitle,omitempty"`
	// Type represents the item type
	Type ItemType `json:"type,omitempty"`
	// Modified holds the modification date
	Modified time.Time `json:"modified,omitempty"`
	// Created holds the creation date
	Created time.Time `json:"created,omitempty"`
	// Icon
	Favicon *Favicon `json:"favicon,omitempty"`
	// Autofill
	Autofill *Autofill `json:"autofill,omitempty"`
}

func (m *Metadata) ID() string {
	key := append([]byte(m.Type.String()), []byte(m.Name)...)
	// ATTN: blake2b keyed mode requires key <= 64 bytes;
	// pre-hash to 32 bytes for long item names to avoid failure
	if len(key) > 64 {
		sum := blake2b.Sum256(key)
		key = sum[:]
	}
	hash, _ := blake2b.New256(key)
	return hex.EncodeToString(hash.Sum(nil))
}

func (m *Metadata) GetMetadata() *Metadata {
	return m
}

func (m *Metadata) IsEmpty() bool {
	return m.Name == ""
}

func (m *Metadata) String() string {
	return m.Name
}

// ByID implements sort.Interface Metadata on the ID value.
type ByString []*Metadata

func (s ByString) Len() int { return len(s) }
func (s ByString) Less(i, j int) bool {
	return s[i].String() < s[j].String()
}
func (s ByString) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
