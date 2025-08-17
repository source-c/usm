package agent

import (
	"bytes"
	"crypto"
	"encoding/json"
	"errors"
	"time"

	"apps.z7.ai/usm/internal/usm"
	"golang.org/x/crypto/ssh"

	sshagent "golang.org/x/crypto/ssh/agent"
)

const (
	dialTimeout = 100 * time.Millisecond
)

type USMAgent interface {
	SSHAgent
	USMSessionExtendedAgent
	USMTypeExtendedAgent
}

// SSHAgent wraps the method for the agent client to handle SSH keys
type SSHAgent interface {
	AddSSHKey(key crypto.PrivateKey, comment string) error
	RemoveSSHKey(key ssh.PublicKey) error
}

// USMSessionExtendedAgent wraps the method for the agent client to handle sessions
type USMSessionExtendedAgent interface {
	Key(vaultName string, sessionID string) (*usm.Key, error)
	Lock(vaultName string) error
	Sessions() ([]Session, error)
	Unlock(vaultName string, key *usm.Key, lifetime time.Duration) (string, error)
}

// USMTypeExtendedAgent wraps the method for the agent client to handle sessions
type USMTypeExtendedAgent interface {
	Type() (Type, error)
}

var _ USMAgent = &client{}

type client struct {
	sshclient sshagent.ExtendedAgent
}

// NewClient returns an agent client to manage sessions and SSH keys
// The communication with agent is done using the SSH agent protocol.
func NewClient(socketPath string) (USMAgent, error) {
	a, err := dialWithTimeout(socketPath, dialTimeout)
	if err != nil {
		return nil, err
	}

	c := &client{
		sshclient: sshagent.NewClient(a),
	}

	return c, nil
}

// AddSSHKey adds an SSH key to agent along with a comment
func (c *client) AddSSHKey(key crypto.PrivateKey, comment string) error {
	return c.sshclient.Add(sshagent.AddedKey{
		PrivateKey: key,
		Comment:    comment,
	})
}

// RemoveSSHKey removes an SSH key from the agent
func (c *client) RemoveSSHKey(key ssh.PublicKey) error {
	return c.sshclient.Remove(key)
}

// Sessions returns the list of active sessions
func (c *client) Sessions() ([]Session, error) {
	request := bytes.Buffer{}
	request.WriteByte(SessionActionList)

	response, err := c.sshclient.Extension(SessionExtension, request.Bytes())
	if err != nil {
		return nil, err
	}
	sessions := []Session{}
	err = json.Unmarshal(response, &sessions)
	return sessions, err
}

// Lock locks vaultName removing from the agent all the active sessions from the agent
func (c *client) Lock(vaultName string) error {
	request := bytes.Buffer{}
	request.WriteByte(SessionActionLock)
	request.WriteString(vaultName)
	response, err := c.sshclient.Extension(SessionExtension, request.Bytes())
	if err != nil {
		return errors.New(string(response))
	}
	return nil
}

// Key returns a USM key associated to the vaultName's session from the agent
func (c *client) Key(vaultName string, sessionID string) (*usm.Key, error) {
	session := &Session{
		ID:    sessionID,
		Vault: vaultName,
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	request := bytes.Buffer{}
	request.WriteByte(SessionActionKey)
	request.Write(payload)

	response, err := c.sshclient.Extension(SessionExtension, request.Bytes())
	if err != nil {
		return nil, err
	}

	key := &usm.Key{}
	err = json.Unmarshal(response, key)
	return key, err
}

// Unlock unlocks the vault vaultName and adds a new session to the agent. Lifetime defines the session life, default to forever.
func (c *client) Unlock(vaultName string, key *usm.Key, lifetime time.Duration) (string, error) {
	session := &Session{
		Lifetime: lifetime,
		Key:      key,
		Vault:    vaultName,
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	request := bytes.Buffer{}
	request.WriteByte(SessionActionUnlock)
	request.Write(payload)

	response, err := c.sshclient.Extension(SessionExtension, request.Bytes())
	if err != nil {
		return "", err
	}
	return string(response), err
}

// Type implements USMAgent
func (c *client) Type() (Type, error) {
	response, err := c.sshclient.Extension(TypeExtension, nil)
	if err != nil {
		return "", err
	}
	return Type(response), err
}
