package ui

import (
	"fmt"
	"log"
	"strconv"

	"apps.z7.ai/usm/internal/usm"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type pwgenOptions struct {
	DefaultMode usm.PasswordMode
	PassphrasePasswordOptions
	PinPasswordOptions
	RandomPasswordOptions
}

type PassphrasePasswordOptions struct {
	DefaultLength int
	MinLength     int
	MaxLength     int
}

type PinPasswordOptions struct {
	DefaultLength int
	MinLength     int
	MaxLength     int
}

type RandomPasswordOptions struct {
	DefaultFormat usm.Format
	DefaultMode   usm.PasswordMode
	DefaultLength int
	MinLength     int
	MaxLength     int
}

type pwgenDialog struct {
	key     *usm.Key
	options pwgenOptions
}

func NewPasswordGenerator(key *usm.Key, ps usm.PasswordPreferences) *pwgenDialog {
	pd := &pwgenDialog{
		key: key,
		options: pwgenOptions{
			RandomPasswordOptions: RandomPasswordOptions{
				DefaultFormat: ps.Random.DefaultFormat,
				DefaultMode:   usm.CustomPassword,
				DefaultLength: ps.Random.DefaultLength,
				MinLength:     ps.Random.MinLength,
				MaxLength:     ps.Random.MaxLength,
			},
			PinPasswordOptions: PinPasswordOptions{
				DefaultLength: ps.Pin.DefaultLength,
				MinLength:     ps.Pin.MinLength,
				MaxLength:     ps.Pin.MaxLength,
			},
			PassphrasePasswordOptions: PassphrasePasswordOptions{
				DefaultLength: ps.Passphrase.DefaultLength,
				MinLength:     ps.Passphrase.MinLength,
				MaxLength:     ps.Passphrase.MaxLength,
			},
		},
	}

	return pd
}

func (pd *pwgenDialog) ShowPasswordGenerator(bind binding.String, password *usm.Password, w fyne.Window) {
	passwordBind := binding.NewString()
	passwordEntry := widget.NewEntryWithData(passwordBind)
	passwordEntry.Validator = nil
	refreshButton := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		secret, err := pwgen(pd.key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	})

	content := container.NewStack(widget.NewLabel(""))
	typeOptions := []string{
		usm.RandomPassword.String(),
		usm.PassphrasePassword.String(),
		usm.PinPassword.String(),
	}
	typeList := widget.NewSelect(typeOptions, func(s string) {
		switch s {
		case usm.PassphrasePassword.String():
			content.Objects[0] = passphraseOptions(pd.key, passwordBind, password, pd.options.PassphrasePasswordOptions)
		case usm.PinPassword.String():
			content.Objects[0] = pinOptions(pd.key, passwordBind, password, pd.options.PinPasswordOptions)
		default:
			content.Objects[0] = randomPasswordOptions(pd.key, passwordBind, password, pd.options.RandomPasswordOptions)
		}
		content.Refresh()
	})
	switch password.Mode.String() {
	case usm.CustomPassword.String():
		password.Mode = usm.RandomPassword
		typeList.SetSelected(usm.RandomPassword.String())
	default:
		typeList.SetSelected(password.Mode.String())
	}

	form := container.New(layout.NewFormLayout())
	form.Add(labelWithStyle("Password"))
	form.Add(container.NewBorder(nil, nil, nil, refreshButton, passwordEntry))
	form.Add(labelWithStyle("Type"))
	form.Add(typeList)
	c := container.NewBorder(form, nil, nil, nil, content)

	d := dialog.NewCustomConfirm("Generate password", "Use", "Cancel", c, func(b bool) {
		if b {
			value, _ := passwordBind.Get()
			_ = bind.Set(value)
		}
	}, w)
	d.Resize(fyne.NewSize(400, 300))
	d.Show()
}

func passphraseOptions(key *usm.Key, passwordBind binding.String, password *usm.Password, opts PassphrasePasswordOptions) fyne.CanvasObject {
	if password.Length == 0 || password.Length < opts.MinLength || password.Length > opts.MaxLength {
		password.Length = opts.DefaultLength
	}

	if password.Mode != usm.PassphrasePassword {
		password.Mode = usm.PassphrasePassword
	}

	lengthBind := binding.BindInt(&password.Length)
	lengthEntry := widget.NewEntryWithData(binding.IntToString(lengthBind))
	lengthEntry.Disabled()
	lengthEntry.Validator = nil
	lengthEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		l, err := strconv.Atoi(s)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		if l < opts.MinLength || l > opts.MaxLength {
			log.Printf("password lenght must be between %d and %d, got %d", opts.MinLength, opts.MaxLength, l)
			return
		}
		_ = lengthBind.Set(l)
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	}

	lengthSlider := widget.NewSlider(float64(opts.MinLength), float64(opts.MaxLength))
	lengthSlider.OnChanged = func(f float64) {
		_ = lengthBind.Set(int(f))
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	}
	lengthSlider.SetValue(float64(password.Length))

	secret, err := pwgen(key, password)
	if err != nil {
		// TODO show dialog
		log.Println(err)
	}
	_ = passwordBind.Set(secret)

	form := container.New(layout.NewFormLayout())
	form.Add(labelWithStyle("Length"))
	form.Add(container.NewBorder(nil, nil, nil, lengthEntry, lengthSlider))

	return form
}

func pinOptions(key *usm.Key, passwordBind binding.String, password *usm.Password, opts PinPasswordOptions) fyne.CanvasObject {
	if password.Length == 0 || password.Length < opts.MinLength || password.Length > opts.MaxLength {
		password.Length = opts.DefaultLength
	}

	// with PIN we want only digits
	password.Format = usm.DigitsFormat
	if password.Mode != usm.PinPassword {
		password.Mode = usm.PinPassword
	}

	lengthBind := binding.BindInt(&password.Length)
	if password.Length == 0 || password.Mode != usm.PinPassword {
		_ = lengthBind.Set(opts.DefaultLength)
	}

	lengthEntry := widget.NewEntryWithData(binding.IntToString(lengthBind))
	lengthEntry.Disabled()
	lengthEntry.Validator = nil
	lengthEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		l, err := strconv.Atoi(s)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		if l < opts.MinLength || l > opts.MaxLength {
			log.Printf("password lenght must be between %d and %d, got %d", opts.MinLength, opts.MaxLength, l)
			return
		}
		_ = lengthBind.Set(l)
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	}

	lengthSlider := widget.NewSlider(float64(opts.MinLength), float64(opts.MaxLength))
	lengthSlider.OnChanged = func(f float64) {
		_ = lengthBind.Set(int(f))
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	}
	lengthSlider.SetValue(float64(password.Length))

	secret, err := pwgen(key, password)
	if err != nil {
		// TODO show dialog
		log.Println(err)
	}
	_ = passwordBind.Set(secret)

	form := container.New(layout.NewFormLayout())
	form.Add(labelWithStyle("Length"))
	form.Add(container.NewBorder(nil, nil, nil, lengthEntry, lengthSlider))

	return form
}

func randomPasswordOptions(key *usm.Key, passwordBind binding.String, password *usm.Password, opts RandomPasswordOptions) fyne.CanvasObject {
	if password.Length == 0 || password.Length < opts.MinLength || password.Length > opts.MaxLength {
		password.Length = opts.DefaultLength
	}

	if password.Format == 0 {
		password.Format = opts.DefaultFormat
	}

	if password.Mode != usm.RandomPassword {
		password.Mode = usm.RandomPassword
		password.Format = opts.DefaultFormat
	}

	lengthBind := binding.BindInt(&password.Length)
	lengthEntry := widget.NewEntryWithData(binding.IntToString(lengthBind))
	lengthEntry.Disabled()
	lengthEntry.Validator = nil
	lengthEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		l, err := strconv.Atoi(s)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		if l < opts.MinLength || l > opts.MaxLength {
			log.Printf("password lenght must be between %d and %d, got %d", opts.MinLength, opts.MaxLength, l)
			return
		}
		_ = lengthBind.Set(l)
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	}

	lengthSlider := widget.NewSlider(float64(opts.MinLength), float64(opts.MaxLength))
	lengthSlider.OnChanged = func(f float64) {
		_ = lengthBind.Set(int(f))
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	}
	lengthSlider.SetValue(float64(password.Length))

	lowercaseButton := widget.NewCheck("a-z", func(isChecked bool) {
		if isChecked {
			password.Format |= usm.LowercaseFormat
		} else {
			password.Format &^= usm.LowercaseFormat
		}
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	})
	if (password.Format & usm.LowercaseFormat) != 0 {
		lowercaseButton.SetChecked(true)
	} else {
		lowercaseButton.SetChecked(false)
	}

	uppercaseButton := widget.NewCheck("A-Z", func(isChecked bool) {
		if isChecked {
			password.Format |= usm.UppercaseFormat
		} else {
			password.Format &^= usm.UppercaseFormat
		}
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	})
	if (password.Format & usm.UppercaseFormat) != 0 {
		uppercaseButton.SetChecked(true)
	} else {
		uppercaseButton.SetChecked(false)
	}

	digitsButton := widget.NewCheck("0-9", func(isChecked bool) {
		if isChecked {
			password.Format |= usm.DigitsFormat
		} else {
			password.Format &^= usm.DigitsFormat
		}
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	})
	if (password.Format & usm.DigitsFormat) != 0 {
		digitsButton.SetChecked(true)
	} else {
		digitsButton.SetChecked(false)
	}

	symbolsButton := widget.NewCheck("!%$", func(isChecked bool) {
		if isChecked {
			password.Format |= usm.SymbolsFormat
		} else {
			password.Format &^= usm.SymbolsFormat
		}
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		_ = passwordBind.Set(secret)
	})
	if (password.Format & usm.SymbolsFormat) != 0 {
		symbolsButton.SetChecked(true)
	} else {
		symbolsButton.SetChecked(false)
	}

	secret, err := pwgen(key, password)
	if err != nil {
		// TODO show dialog
		log.Println(err)
	}
	_ = passwordBind.Set(secret)

	form := container.New(layout.NewFormLayout())
	form.Add(labelWithStyle("Length"))
	form.Add(container.NewBorder(nil, nil, nil, lengthEntry, lengthSlider))
	form.Add(widget.NewLabel(""))
	form.Add(container.NewGridWithColumns(4, lowercaseButton, uppercaseButton, digitsButton, symbolsButton))

	return form
}

func pwgen(key *usm.Key, password *usm.Password) (string, error) {
	if password.Mode == usm.PassphrasePassword {
		return key.Passphrase(password.Length)
	}
	secret, err := key.Secret(password)
	if err != nil {
		return "", fmt.Errorf("could not generate password: %w", err)
	}
	return secret, nil
}
