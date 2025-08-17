package usm

import "runtime"

func newDefaultPreferences() *Preferences {
	// Detect if running on macOS to set toolbar default
	showToolbar := runtime.GOOS != "darwin"

	return &Preferences{
		FaviconDownloader: FaviconDownloaderPreferences{
			Disabled: false,
		},
		Password: PasswordPreferences{
			Passphrase: PassphrasePasswordPreferences{
				DefaultLength: PassphrasePasswordDefaultLength,
				MaxLength:     PassphrasePasswordMaxLength,
				MinLength:     PassphrasePasswordMinLength,
			},
			Pin: PinPasswordPreferences{
				DefaultLength: PinPasswordDefaultLength,
				MaxLength:     PinPasswordMaxLength,
				MinLength:     PinPasswordMinLength,
			},
			Random: RandomPasswordPreferences{
				DefaultLength: RandomPasswordDefaultLength,
				DefaultFormat: RandomPasswordDefaultFormat,
				MaxLength:     RandomPasswordMaxLength,
				MinLength:     RandomPasswordMinLength,
			},
		},
		TOTP: TOTPPreferences{
			Digits:   TOTPDigitsDefault,
			Hash:     TOTPHashDefault,
			Interval: TOTPIntervalDefault,
		},
		Theme: ThemePreferences{
			DefaultColour: "", // Empty string means use system default
		},
		Toolbar: ToolbarPreferences{
			Show: showToolbar,
		},
	}
}

type Preferences struct {
	FaviconDownloader FaviconDownloaderPreferences `json:"favicon_downloader,omitempty"`
	Password          PasswordPreferences          `json:"password,omitempty"`
	TOTP              TOTPPreferences              `json:"totp,omitempty"`
	Theme             ThemePreferences             `json:"theme,omitempty"`
	Toolbar           ToolbarPreferences           `json:"toolbar,omitempty"`
}

// FaviconDownloaderPreferences represents the preferences for the favicon downloader.
// FaviconDownloader tool is opt-out, hence the default value is false.
type FaviconDownloaderPreferences struct {
	Disabled bool `json:"disabled,omitempty"` // Disabled is true if the favicon downloader is disabled.
}

type PasswordPreferences struct {
	Passphrase PassphrasePasswordPreferences `json:"passphrase,omitempty"`
	Pin        PinPasswordPreferences        `json:"pin,omitempty"`
	Random     RandomPasswordPreferences     `json:"random,omitempty"`
}

type PassphrasePasswordPreferences struct {
	DefaultLength int `json:"default_length,omitempty"`
	MaxLength     int `json:"max_length,omitempty"`
	MinLength     int `json:"min_length,omitempty"`
}

type PinPasswordPreferences struct {
	DefaultLength int `json:"default_length,omitempty"`
	MaxLength     int `json:"max_length,omitempty"`
	MinLength     int `json:"min_length,omitempty"`
}
type RandomPasswordPreferences struct {
	DefaultLength int    `json:"default_length,omitempty"`
	DefaultFormat Format `json:"default_format,omitempty"`
	MaxLength     int    `json:"max_length,omitempty"`
	MinLength     int    `json:"min_length,omitempty"`
}

type TOTPPreferences struct {
	Digits   int      `json:"digits,omitempty"`
	Hash     TOTPHash `json:"hash,omitempty"`
	Interval int      `json:"interval,omitempty"`
}

type ThemePreferences struct {
	DefaultColour string `json:"default_colour,omitempty"` // Default application colour (hex value), empty means system default
}

type ToolbarPreferences struct {
	Show bool `json:"show"` // Show toolbar (in-app menu bar) - default is false on macOS, true elsewhere
}

// Predefined colour options for vault themes
var (
	// PredefinedColours represents the 6 predefined colour options plus default
	PredefinedColours = []ColourOption{
		{Name: "Default", Value: ""},       // Empty string means use default theme
		{Name: "Blue", Value: "#2196F3"},   // Material Blue
		{Name: "Green", Value: "#4CAF50"},  // Material Green
		{Name: "Purple", Value: "#9C27B0"}, // Material Purple
		{Name: "Orange", Value: "#FF9800"}, // Material Orange
		{Name: "Red", Value: "#F44336"},    // Material Red
		{Name: "Teal", Value: "#009688"},   // Material Teal
	}
)

// ColourOption represents a colour choice with a display name and hex value
type ColourOption struct {
	Name  string // Display name for the colour
	Value string // Hex colour value (e.g., "#2196F3"), empty for default
}

// IsValidHexColour validates if a string is a valid hex colour
func IsValidHexColour(colour string) bool {
	if colour == "" {
		return true // Empty string is valid (means default)
	}
	if len(colour) != 7 || colour[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := colour[i]
		if (c < '0' || c > '9') && (c < 'A' || c > 'F') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
