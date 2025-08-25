package config

import (
	"runtime"

	"apps.z7.ai/usm/internal/usm"
)

// newDefaultPreferences creates default preferences
// ATTN: This duplicates logic from usm package to maintain separation
func (m *Manager) newDefaultPreferences() *usm.Preferences {
	// Detect if running on macOS to set toolbar default
	showToolbar := runtime.GOOS == "darwin"

	return &usm.Preferences{
		FaviconDownloader: usm.FaviconDownloaderPreferences{
			Disabled: false,
		},
		Password: usm.PasswordPreferences{
			Passphrase: usm.PassphrasePasswordPreferences{
				DefaultLength: usm.PassphrasePasswordDefaultLength,
				MaxLength:     usm.PassphrasePasswordMaxLength,
				MinLength:     usm.PassphrasePasswordMinLength,
			},
			Pin: usm.PinPasswordPreferences{
				DefaultLength: usm.PinPasswordDefaultLength,
				MaxLength:     usm.PinPasswordMaxLength,
				MinLength:     usm.PinPasswordMinLength,
			},
			Random: usm.RandomPasswordPreferences{
				DefaultLength: usm.RandomPasswordDefaultLength,
				DefaultFormat: usm.RandomPasswordDefaultFormat,
				MaxLength:     usm.RandomPasswordMaxLength,
				MinLength:     usm.RandomPasswordMinLength,
			},
		},
		TOTP: usm.TOTPPreferences{
			Digits:   usm.TOTPDigitsDefault,
			Hash:     usm.TOTPHashDefault,
			Interval: usm.TOTPIntervalDefault,
		},
		Theme: usm.ThemePreferences{
			DefaultColour: "", // Empty string means use system default
		},
		Toolbar: usm.ToolbarPreferences{
			Show: showToolbar,
		},
	}
}

// extractPreferences converts map to Preferences struct
func (m *Manager) extractPreferences(data map[string]interface{}) *usm.Preferences {
	prefs := m.newDefaultPreferences()

	// Extract FaviconDownloader preferences
	if fd, ok := data["favicon_downloader"].(map[string]interface{}); ok {
		if disabled, ok := fd["disabled"].(bool); ok {
			prefs.FaviconDownloader.Disabled = disabled
		}
	}

	// Extract Password preferences
	if pw, ok := data["password"].(map[string]interface{}); ok {
		// Passphrase
		if pp, ok := pw["passphrase"].(map[string]interface{}); ok {
			if v, ok := pp["default_length"].(float64); ok {
				prefs.Password.Passphrase.DefaultLength = int(v)
			}
			if v, ok := pp["max_length"].(float64); ok {
				prefs.Password.Passphrase.MaxLength = int(v)
			}
			if v, ok := pp["min_length"].(float64); ok {
				prefs.Password.Passphrase.MinLength = int(v)
			}
		}

		// Pin
		if pin, ok := pw["pin"].(map[string]interface{}); ok {
			if v, ok := pin["default_length"].(float64); ok {
				prefs.Password.Pin.DefaultLength = int(v)
			}
			if v, ok := pin["max_length"].(float64); ok {
				prefs.Password.Pin.MaxLength = int(v)
			}
			if v, ok := pin["min_length"].(float64); ok {
				prefs.Password.Pin.MinLength = int(v)
			}
		}

		// Random
		if rnd, ok := pw["random"].(map[string]interface{}); ok {
			if v, ok := rnd["default_length"].(float64); ok {
				prefs.Password.Random.DefaultLength = int(v)
			}
			if v, ok := rnd["default_format"].(float64); ok {
				prefs.Password.Random.DefaultFormat = usm.Format(int(v))
			}
			if v, ok := rnd["max_length"].(float64); ok {
				prefs.Password.Random.MaxLength = int(v)
			}
			if v, ok := rnd["min_length"].(float64); ok {
				prefs.Password.Random.MinLength = int(v)
			}
		}
	}

	// Extract TOTP preferences
	if totp, ok := data["totp"].(map[string]interface{}); ok {
		if v, ok := totp["digits"].(float64); ok {
			prefs.TOTP.Digits = int(v)
		}
		if v, ok := totp["hash"].(string); ok {
			prefs.TOTP.Hash = usm.TOTPHash(v)
		}
		if v, ok := totp["interval"].(float64); ok {
			prefs.TOTP.Interval = int(v)
		}
	}

	// Extract Theme preferences
	if theme, ok := data["theme"].(map[string]interface{}); ok {
		if v, ok := theme["default_colour"].(string); ok {
			prefs.Theme.DefaultColour = v
		}
	}

	// Extract Toolbar preferences
	if toolbar, ok := data["toolbar"].(map[string]interface{}); ok {
		if v, ok := toolbar["show"].(bool); ok {
			prefs.Toolbar.Show = v
		}
	}

	return prefs
}

// preferencesToMap converts Preferences struct to map
func (m *Manager) preferencesToMap(prefs *usm.Preferences) map[string]interface{} {
	return map[string]interface{}{
		"favicon_downloader": map[string]interface{}{
			"disabled": prefs.FaviconDownloader.Disabled,
		},
		"password": map[string]interface{}{
			"passphrase": map[string]interface{}{
				"default_length": prefs.Password.Passphrase.DefaultLength,
				"max_length":     prefs.Password.Passphrase.MaxLength,
				"min_length":     prefs.Password.Passphrase.MinLength,
			},
			"pin": map[string]interface{}{
				"default_length": prefs.Password.Pin.DefaultLength,
				"max_length":     prefs.Password.Pin.MaxLength,
				"min_length":     prefs.Password.Pin.MinLength,
			},
			"random": map[string]interface{}{
				"default_length": prefs.Password.Random.DefaultLength,
				"default_format": int(prefs.Password.Random.DefaultFormat),
				"max_length":     prefs.Password.Random.MaxLength,
				"min_length":     prefs.Password.Random.MinLength,
			},
		},
		"totp": map[string]interface{}{
			"digits":   prefs.TOTP.Digits,
			"hash":     string(prefs.TOTP.Hash),
			"interval": prefs.TOTP.Interval,
		},
		"theme": map[string]interface{}{
			"default_colour": prefs.Theme.DefaultColour,
		},
		"toolbar": map[string]interface{}{
			"show": prefs.Toolbar.Show,
		},
	}
}
