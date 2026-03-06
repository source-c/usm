package usm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDefaultPreferences_SyncDisabledByDefault(t *testing.T) {
	prefs := newDefaultPreferences()
	assert.Equal(t, SyncModeDisabled, prefs.Sync.Mode)
}

func TestSyncMode_IsEnabled(t *testing.T) {
	tests := []struct {
		mode    string
		enabled bool
	}{
		{SyncModeDisabled, false},
		{SyncModeRelaxed, true},
		{SyncModeStrict, true},
		{"", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			p := SyncPreferences{Mode: tt.mode}
			assert.Equal(t, tt.enabled, p.IsEnabled())
		})
	}
}
