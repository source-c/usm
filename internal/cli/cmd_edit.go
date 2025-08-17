package cli

import (
	"fmt"
	"os"
	"time"

	"apps.z7.ai/usm/internal/usm"
)

// Edit edits an item into the vault
type EditCmd struct {
	itemPath
}

// Name returns the one word command name
func (cmd *EditCmd) Name() string {
	return "edit"
}

// Description returns the command description
func (cmd *EditCmd) Description() string {
	return "Edits an item into the vault"
}

// Usage displays the command usage
func (cmd *EditCmd) Usage() {
	template := `Usage: usm cli [OPTION] edit VAULT_NAME/ITEM_TYPE/ITEM_NAME

{{ . }}

Options:
  -h, --help                  Displays this help and exit
      --session=SESSION_ID    Sets a session ID to use instead of the env var
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *EditCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{Session: true})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(1)
	}
	flags.SetEnv()

	itemPath, err := parseItemPath(flagSet.Arg(0), itemPathOptions{fullPath: true})
	if err != nil {
		return err
	}
	cmd.itemPath = itemPath
	return nil
}

// Run runs the command
func (cmd *EditCmd) Run(s usm.Storage) error {
	appState, err := s.LoadAppState()
	if err != nil {
		return err
	}

	key, err := loadVaultKey(s, cmd.vaultName)
	if err != nil {
		return err
	}

	vault, err := s.LoadVault(cmd.vaultName, key)
	if err != nil {
		return err
	}

	item, err := usm.NewItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}

	item, err = s.LoadItem(vault, item.GetMetadata())
	if err != nil {
		return err
	}

	switch cmd.itemType {
	case usm.LoginItemType:
		_ = cmd.editLoginItem(vault.Key(), item)
	case usm.NoteItemType:
		_ = cmd.editNoteItem(item)
	case usm.PasswordItemType:
		_ = cmd.editPasswordItem(vault.Key(), item)
	case usm.SSHKeyItemType:
		_ = cmd.editSSHKeyItem(item)
	default:
		return fmt.Errorf("unsupported item type: %q", cmd.itemType)
	}

	now := time.Now().UTC()

	item.GetMetadata().Modified = now

	err = s.StoreItem(vault, item)
	if err != nil {
		return err
	}

	err = vault.AddItem(item)
	if err != nil {
		return err
	}

	vault.Modified = now
	err = s.StoreVault(vault)
	if err != nil {
		return err
	}

	appState.Modified = now
	err = s.StoreAppState(appState)
	if err != nil {
		return err
	}

	fmt.Printf("[✓] item %q modified\n", cmd.itemName)
	return nil
}

func (cmd *EditCmd) editLoginItem(key *usm.Key, item usm.Item) error {
	v := item.(*usm.Login)

	url, err := askWithDefault("URL", v.URL.String())
	if err != nil {
		return err
	}
	_ = v.URL.Set(url)

	username, err := askWithDefault("Username", v.Username)
	if err != nil {
		return err
	}
	v.Username = username

	pwgenCmd := &PwGenCmd{}
	modes := []usm.PasswordMode{
		usm.CustomPassword,
		usm.RandomPassword,
		usm.PassphrasePassword,
		usm.PinPassword,
	}
	password, err := pwgenCmd.Pwgen(key, modes, v.Mode)
	if err != nil {
		return err
	}
	v.Password.Value = password.Value
	v.Password.Mode = password.Mode
	v.Password.Format = password.Format
	v.Password.Length = password.Length

	note, err := askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}
	v.Note.Value = note

	return nil
}

func (cmd *EditCmd) editNoteItem(item usm.Item) error {
	v := item.(*usm.Note)

	note, err := askWithDefault("Note", v.Value)
	if err != nil {
		return err
	}
	v.Value = note

	return nil
}

func (cmd *EditCmd) editPasswordItem(key *usm.Key, item usm.Item) error {
	v := item.(*usm.Password)

	pwgenCmd := &PwGenCmd{}
	modes := []usm.PasswordMode{
		usm.CustomPassword,
		usm.RandomPassword,
		usm.PassphrasePassword,
		usm.PinPassword,
	}
	password, err := pwgenCmd.Pwgen(key, modes, v.Mode)
	if err != nil {
		return err
	}
	v.Value = password.Value
	v.Mode = password.Mode
	v.Format = password.Format
	v.Length = password.Length

	note, err := askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}

	v.Note.Value = note
	return nil
}

func (cmd *EditCmd) editSSHKeyItem(item usm.Item) error {
	v := item.(*usm.SSHKey)

	addToAgent, err := askYesNo("Add to agent", v.AddToAgent)
	if err != nil {
		return err
	}
	v.AddToAgent = addToAgent

	note, err := askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}

	v.Note.Value = note
	return nil
}
