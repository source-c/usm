package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"apps.z7.ai/usm/internal/usm"
)

const (
	clipboardWriteTimeout      = 1 * time.Second
	passwordCopiedClipboardMsg = "[✓] password copied to clipboard"
)

// Show shows an item details
type ShowCmd struct {
	itemPath
	clipboard bool
}

// Name returns the one word command name
func (cmd *ShowCmd) Name() string {
	return "show"
}

// Description returns the command description
func (cmd *ShowCmd) Description() string {
	return "Shows an item details"
}

// Usage displays the command usage
func (cmd *ShowCmd) Usage() {
	template := `Usage: usm cli show [OPTION] VAULT_NAME/ITEM_TYPE/ITEM_NAME

{{ . }}

Options:
  -c, --clip                  Do not print the password but instead copy to the clipboard
  -h, --help                  Displays this help and exit
      --session=SESSION_ID    Sets a session ID to use instead of the env var
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *ShowCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{Session: true})
	if err != nil {
		return err
	}

	flagSet.BoolVar(&cmd.clipboard, "c", false, "")
	flagSet.BoolVar(&cmd.clipboard, "clip", false, "")

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(0)
	}
	flags.SetEnv()

	if cmd.clipboard {
		err := initClipboard()
		if err != nil {
			return err
		}
	}

	itemPath, err := parseItemPath(flagSet.Arg(0), itemPathOptions{fullPath: true})
	if err != nil {
		return err
	}
	cmd.itemPath = itemPath
	return nil
}

// Run runs the command
func (cmd *ShowCmd) Run(s usm.Storage) error {
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

	var pclip []byte
	var pclipMsg string
	switch cmd.itemType {
	case usm.LoginItemType:
		v := item.(*usm.Login)
		fmt.Printf("URL: %s\n", v.URL)
		fmt.Printf("Username: %s\n", v.Username)
		if !cmd.clipboard {
			fmt.Printf("Password: %s\n", v.Password.Value)
		} else {
			pclip = []byte(v.Password.Value)
			pclipMsg = passwordCopiedClipboardMsg
		}
		if v.Note != nil {
			fmt.Printf("Note: %s\n", v.Note.Value)
		}
	case usm.PasswordItemType:
		v := item.(*usm.Password)
		if !cmd.clipboard {
			fmt.Printf("Password: %s\n", v.Value)
		} else {
			pclip = []byte(v.Value)
			pclipMsg = passwordCopiedClipboardMsg
		}
		if v.Note != nil {
			fmt.Printf("Note: %s\n", v.Note.Value)
		}
	case usm.SSHKeyItemType:
		v := item.(*usm.SSHKey)
		if !cmd.clipboard {
			fmt.Printf("Private key: %s\n", v.PrivateKey)
		} else {
			pclip = []byte(v.PrivateKey)
			pclipMsg = passwordCopiedClipboardMsg
		}
		if v.Passphrase != nil {
			fmt.Printf("Passphrase: %s\n", v.Passphrase.Value)
		}
		fmt.Printf("Public key: %s\n", v.PublicKey)
		fmt.Printf("Fingerprint: %s\n", v.Fingerprint)
		addToAgent := "No"
		if v.AddToAgent {
			addToAgent = "Yes"
		}
		fmt.Printf("Add to agent: %s\n", addToAgent)
		if v.Note != nil {
			fmt.Printf("Note: %s\n", v.Note.Value)
		}
	case usm.NoteItemType:
		v := item.(*usm.Note)
		fmt.Printf("Note: %s\n", v.Value)
	}

	fmt.Printf("Modified: %s\n", item.GetMetadata().Modified.Local().Format(time.RFC1123))
	fmt.Printf("Created: %s\n", item.GetMetadata().Created.Local().Format(time.RFC1123))

	if pclip != nil {
		ctx, cancel := context.WithTimeout(context.Background(), clipboardWriteTimeout)
		defer cancel()
		err := writeToClipboard(ctx, pclip)
		if err != nil {
			return nil
		}
		fmt.Println(pclipMsg)
	}
	return nil
}
