package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
)

const nativeMessagingManifestFileName = "usm.json"

const (
	osDarwin  = "darwin"
	osWindows = "windows"

	firefoxExtensionIDs = `{"usm@apps.z7.ai"}`
	firefoxManifestTpl  = `{
		"name": "USM",
		"description": "Native manifest for the USM browser extension",
		"path": "{{ .Path }}",
		"type": "stdio",
		"allowed_extensions": {{ .ExtensionIDs }}
	}
	`
)

const (
	chromeExtensionIDs = `{"chrome-extension://lkncfaojhcgoefgkjpfoniakecdiclof/"}`
	chromeManifestTpl  = `{
		"name": "USM",
		"description": "Native manifest for the USM browser extension",
		"path": "{{ .Path }}",
		"type": "stdio",
		"allowed_origins": {{ .ExtensionIDs }}
	}
	`
)

type manifestTplData struct {
	Path         string
	ExtensionIDs string
}

func getUSMExecutablePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exePath)
}

// WriteNativeManifests writes native manifests
// Currently only chrome and firefox on linux are supported.
// TODO add support for macOS, windows and mobile.
func WriteNativeManifests() error {
	usmPath, err := getUSMExecutablePath()
	if err != nil {
		return err
	}

	firefoxData := manifestTplData{Path: usmPath, ExtensionIDs: firefoxExtensionIDs}
	firefoxNativeManifestLocations, err := firefoxNativeManifestLocations()
	if err != nil {
		return err
	}
	_ = writeNativeManifest(firefoxManifestTpl, firefoxData, firefoxNativeManifestLocations)

	chromeData := manifestTplData{Path: usmPath, ExtensionIDs: chromeExtensionIDs}
	chromeNativeManifestLocations, err := chromeNativeManifestLocations()
	if err != nil {
		return err
	}
	_ = writeNativeManifest(chromeManifestTpl, chromeData, chromeNativeManifestLocations)
	return nil
}

func writeNativeManifest(tpl string, data manifestTplData, locations []string) error {
	tmpl, err := template.New("manifest").Parse(tpl)
	if err != nil {
		return err
	}

	for _, location := range locations {
		_ = os.MkdirAll(filepath.Dir(location), 0o700)

		file, err := os.Create(location) //nolint:gosec // location is application-controlled path
		if err != nil {
			return err
		}
		defer file.Close()

		err = tmpl.Execute(file, data)
		if err != nil {
			return err
		}
	}

	return nil
}

// chromeNativeManifestLocations defines the native manifest locations for chrome/chromium
// see: https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging
// TODO: handle darwin and windows
func chromeNativeManifestLocations() ([]string, error) {
	if runtime.GOOS == osDarwin {
		return []string{}, nil
	}

	if runtime.GOOS == osWindows {
		return []string{}, nil
	}

	// fallback to linux and *nix OSes
	uhd, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get the user home directory: %w", err)
	}

	return []string{
		filepath.Join(uhd, ".config/google-chrome/NativeMessagingHosts", nativeMessagingManifestFileName),
		filepath.Join(uhd, ".config/chromium/NativeMessagingHosts", nativeMessagingManifestFileName),
	}, nil
}

// firefoxNativeManifestLocations defines the native manifest locations for firefox
// See: https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_manifests
// TODO: handle darwin and windows
func firefoxNativeManifestLocations() ([]string, error) {
	if runtime.GOOS == osDarwin {
		return []string{}, nil
	}

	if runtime.GOOS == osWindows {
		return []string{}, nil
	}

	// fallback to linux and *nix OSes
	uhd, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get the user home directory: %w", err)
	}

	return []string{
		filepath.Join(uhd, ".mozilla/native-messaging-hosts", nativeMessagingManifestFileName),
	}, nil
}
