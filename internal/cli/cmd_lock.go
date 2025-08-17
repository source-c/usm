package cli

import (
	"fmt"
	"os"

	"apps.z7.ai/usm/internal/agent"
	"apps.z7.ai/usm/internal/sshkey"
	"apps.z7.ai/usm/internal/usm"
)

// Lock locks a USM vault removing all the associated sessions from the agent
type LockCmd struct {
	vaultName string
}

// Name returns the one word command name
func (cmd *LockCmd) Name() string {
	return "lock"
}

// Description returns the command description
func (cmd *LockCmd) Description() string {
	return "Lock a vault"
}

// Usage displays the command usage
func (cmd *LockCmd) Usage() {
	template := `Usage: usm cli lock [OPTION] VAULT

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *LockCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(1)
	}

	cmd.vaultName = flagSet.Arg(0)
	return nil
}

// Run runs the command
func (cmd *LockCmd) Run(s usm.Storage) error {
	c, err := agent.NewClient(s.SocketAgentPath())
	if err != nil {
		return fmt.Errorf("agent not available: %w", err)
	}
	err = c.Lock(cmd.vaultName)
	if err != nil {
		return err
	}

	fmt.Println("Removing SSH keys from the agent...")
	err = cmd.removeSSHKeysFromAgent(c, s)
	if err != nil {
		fmt.Println("could not remove SSH keys from the agent:", err)
	}
	fmt.Println("[✓] vault locked")
	return nil
}

func (cmd *LockCmd) removeSSHKeysFromAgent(c agent.USMAgent, s usm.Storage) error {
	os.Setenv(usm.ENV_SESSION, "")
	key, err := loadVaultKey(s, cmd.vaultName)
	if err != nil {
		return err
	}
	vault, err := s.LoadVault(cmd.vaultName, key)
	if err != nil {
		return err
	}
	vault.Range(func(id string, meta *usm.Metadata) bool {
		item, err := s.LoadItem(vault, meta)
		if err != nil {
			return false
		}
		if item.GetMetadata().Type != usm.SSHKeyItemType {
			return true
		}
		v := item.(*usm.SSHKey)
		if !v.AddToAgent {
			return true
		}
		k, err := sshkey.ParseKey([]byte(v.PrivateKey))
		if err != nil {
			return true
		}

		err = c.RemoveSSHKey(k.PublicKey())
		if err != nil {
			fmt.Printf("Could not remove SSH key from the agent. Error: %q - Public key: %s", err, k.MarshalPublicKey())
			return true
		}
		fmt.Printf("Removed key: %s", k.MarshalPublicKey())
		return true
	})
	return nil
}
