package cli

import (
	"fmt"

	"apps.z7.ai/usm/internal/usm"
)

// PwGenCmd generates a password
type PwGenCmd struct{}

// Name returns the one word command name
func (cmd *PwGenCmd) Name() string {
	return "pwgen"
}

// Description returns the command description
func (cmd *PwGenCmd) Description() string {
	return "Generates a password"
}

// Usage displays the command usage
func (cmd *PwGenCmd) Usage() {
	template := `Usage: usm cli pwgen [OPTION]

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *PwGenCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)

	return nil
}

// Run runs the command
func (cmd *PwGenCmd) Run(s usm.Storage) error {
	modes := []usm.PasswordMode{
		usm.RandomPassword,
		usm.PassphrasePassword,
		usm.PinPassword,
	}
	password, err := cmd.Pwgen(nil, modes, usm.RandomPassword)
	if err != nil {
		return err
	}

	fmt.Println(password.Value)
	return nil
}

func (cmd *PwGenCmd) Pwgen(key *usm.Key, modes []usm.PasswordMode, defaultMode usm.PasswordMode) (*usm.Password, error) {
	var err error

	if key == nil {
		key, err = usm.MakeOneTimeKey()
		if err != nil {
			return nil, err
		}
	}

	choice, err := askPasswordMode("Password type", modes, defaultMode)
	if err != nil {
		return nil, err
	}

	switch choice {
	case usm.CustomPassword:
		return cmd.makeCustomPassword()
	case usm.RandomPassword:
		return cmd.makeRandomPassword(key)
	case usm.PassphrasePassword:
		return cmd.makePassphrasePassword(key)
	case usm.PinPassword:
		return cmd.makePinPassword(key)
	}
	return nil, fmt.Errorf("unsupported password type: %q", choice)
}

// Parse parses the arguments and set the usage for the command
func (cmd *PwGenCmd) makeRandomPassword(key *usm.Key) (*usm.Password, error) {
	p := usm.NewRandomPassword()
	length, err := askIntWithDefaultAndRange("Password length", p.Length, 6, 64)
	if err != nil {
		return nil, err
	}
	p.Length = length
	p.Format = usm.UppercaseFormat | usm.LowercaseFormat | usm.DigitsFormat

	wantSymbols, err := askYesNo("Password should contains symbols?", true)
	if err != nil {
		return nil, err
	}
	if wantSymbols {
		p.Format |= usm.SymbolsFormat
	}
	v, err := key.Secret(p)
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

func (cmd *PwGenCmd) makeCustomPassword() (*usm.Password, error) {
	p := usm.NewCustomPassword()
	v, err := askPasswordWithConfirm()
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

func (cmd *PwGenCmd) makePassphrasePassword(key *usm.Key) (*usm.Password, error) {
	p := usm.NewPassphrasePassword()
	length, err := askIntWithDefaultAndRange("Passphrase words", p.Length, 2, 12)
	if err != nil {
		return nil, err
	}
	v, err := key.Passphrase(length)
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

func (cmd *PwGenCmd) makePinPassword(key *usm.Key) (*usm.Password, error) {
	p := usm.NewPinPassword()
	length, err := askIntWithDefaultAndRange("Pin length", p.Length, 4, 10)
	if err != nil {
		return nil, err
	}
	p.Length = length
	p.Format = usm.DigitsFormat
	v, err := key.Secret(p)
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}
