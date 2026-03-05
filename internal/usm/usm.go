package usm

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"time"

	"apps.z7.ai/usm/internal/age/bech32"
	"filippo.io/age"
	"filippo.io/age/armor"
	"golang.org/x/crypto/hkdf"

	ageusm "apps.z7.ai/usm/internal/age"
)

const (
	ID            = "ai.z7.apps.usm"
	ServicePrefix = "usm/"
	// UserAgentFaviconDownloader is the UserAgent user by the favicon downloader
	UserAgentFaviconDownloader = "usm/1.0 (+https://apps.z7.ai/usm-go)"
)

// Build metadata — set at link time via -ldflags -X
var (
	BuildVersion string
	BuildID      string
	BuildTime    string
)

type Ruler interface {
	Template() (string, error)
	Len() int
}

type Seeder interface {
	Ruler
	// Salt returns the salt used to generate the secret
	Salt() []byte
	// Info holds the info used to generate the secret
	Info() []byte
}

type SecretMaker interface {
	Secret(seeder Seeder) (string, error)
}

type Key struct {
	ageIdentity *age.X25519Identity
}

// MakeOneTimeKey generates a one time age secret key.
// The key can be used to generate random passwords
func MakeOneTimeKey() (key *Key, err error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("usm: makekey error: %w", err)
	}

	// Generate the age X25519 Identity
	ageIdentity, ierr := age.GenerateX25519Identity()
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}
	key = &Key{
		ageIdentity: ageIdentity,
	}
	return
}

// MakeKey generates an age secret key. The key is encrypted to w and protect using the provided password
func MakeKey(password string, w io.Writer) (key *Key, err error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("usm: makekey error: %w", err)
	}

	// Generate the age X25519 Identity
	ageIdentity, ierr := age.GenerateX25519Identity()
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	ageScryptRecipient, ierr := age.NewScryptRecipient(password)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	a := armor.NewWriter(w)
	defer func() {
		// make sure to handle the error, if any
		if ierr := a.Close(); ierr != nil {
			err = wrapErr(ierr)
			return
		}
	}()
	e, err := age.Encrypt(a, ageScryptRecipient)
	if err != nil {
		err = wrapErr(err)
		return
	}

	data := &bytes.Buffer{}
	fmt.Fprintf(data, "# created: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(data, "# public key: %s\n", ageIdentity.Recipient())
	fmt.Fprintf(data, "%s\n", ageIdentity)

	_, err = e.Write(data.Bytes())
	if err != nil {
		err = wrapErr(err)
		return
	}
	err = e.Close()
	if err != nil {
		err = wrapErr(err)
		return
	}

	key = &Key{
		ageIdentity: ageIdentity,
	}
	return
}

// LoadKey decrypts an age secret key from the reader r using the provided password
func LoadKey(password string, r io.Reader) (key *Key, err error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("usm: loadkey error: %w", err)
	}

	ageScryptIdentity, ierr := age.NewScryptIdentity(password)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	a := armor.NewReader(r)
	d, ierr := age.Decrypt(a, ageScryptIdentity)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	// Generate the age X25519 Identity
	ageIdentities, ierr := age.ParseIdentities(d)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	if len(ageIdentities) > 1 {
		err = wrapErr(fmt.Errorf("only one identity per file is supported, found %d", len(ageIdentities)))
		return
	}

	ageIdentity, ok := ageIdentities[0].(*age.X25519Identity)
	if !ok {
		err = wrapErr(fmt.Errorf("only *age.X25519Identity are supported, got %T", ageIdentities[0]))
		return
	}

	key = &Key{
		ageIdentity: ageIdentity,
	}
	return
}

func (k *Key) Passphrase(numWords int) (string, error) {
	var words []string
	for i := 0; i < numWords; i++ {
		words = append(words, ageusm.RandomWord())
	}
	return strings.Join(words, "-"), nil
}

// Secret derives a secret from the seeder
func (k *Key) Secret(seeder Seeder) (string, error) {
	// Underlying hash function for HMAC.
	hash := sha256.New
	salt := seeder.Salt()
	if salt == nil {
		salt = make([]byte, hash().Size())
		// ATTN: panic is intentional — crypto/rand.Read failing means the OS CSPRNG is broken.
		// Continuing with a predictable salt would silently compromise all generated secrets.
		if _, err := rand.Read(salt); err != nil {
			panic(err)
		}
	}

	// decode the age identity to be used as secret for HKDF function
	_, data, err := bech32.Decode(k.ageIdentity.String())
	if err != nil {
		return "", fmt.Errorf("could not decode the age identity: %w", err)
	}

	// reader to derive a key
	reader := hkdf.New(sha256.New, data, salt, seeder.Info())
	template, err := seeder.Template()
	if err != nil {
		return "", err
	}

	chars := []byte(template)
	if len(chars) == 0 {
		return "", fmt.Errorf("empty character template for secret generation")
	}

	// ATTN: limit iterations to prevent infinite loop when HKDF output rarely matches the character template.
	// With a reasonable template, the probability of needing more than seeder.Len()*256 reads is negligible.
	maxIterations := seeder.Len() * 256
	var secret bytes.Buffer
	for i := 0; i < maxIterations; i++ {
		buf := make([]byte, 1)
		_, err := io.ReadFull(reader, buf)
		if err != nil {
			return "", err
		}

		if !bytes.Contains(chars, buf) {
			continue
		}

		secret.WriteByte(buf[0])
		if secret.Len() == seeder.Len() {
			return secret.String(), nil
		}
	}

	return "", fmt.Errorf("secret generation exceeded maximum iterations (%d)", maxIterations)
}

// Decrypt decrypts the message
func (k *Key) Decrypt(src io.Reader) (io.Reader, error) {
	return age.Decrypt(src, k.ageIdentity)
}

// Encrypt a message
func (k *Key) Encrypt(dst io.Writer) (io.WriteCloser, error) {
	return age.Encrypt(dst, k.ageIdentity.Recipient())
}

// Fingerprint returns a SHA256 hash for the public recipient used as key identifier
func (k *Key) Fingerprint() string {
	recipient := k.ageIdentity.Recipient().String()
	hash := sha256.Sum256([]byte(recipient))
	return fmt.Sprintf("%x", hash)
}

func (k *Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.ageIdentity.String())
}

func (k *Key) UnmarshalJSON(data []byte) error {
	var v string
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	ageIdentity, err := age.ParseX25519Identity(v)
	if err != nil {
		return err
	}
	k.ageIdentity = ageIdentity
	return nil
}

func (k *Key) String() string {
	return k.ageIdentity.String()
}

// Version returns the USM's version
func Version() string {
	if BuildVersion != "" {
		return BuildVersion
	}

	info, ok := debug.ReadBuildInfo()
	if ok {
		return info.Main.Version
	}
	return "(unknown)"
}

// BuildInfo returns a human-readable build identification string
func BuildInfo() string {
	v := Version()
	if BuildID != "" || BuildTime != "" {
		parts := []string{}
		if BuildID != "" {
			parts = append(parts, BuildID)
		}
		if BuildTime != "" {
			parts = append(parts, BuildTime)
		}
		return v + " (" + strings.Join(parts, " ") + ")"
	}
	return v
}

// ServiceVersion returns the USM's service version
func ServiceVersion() string {
	return ServicePrefix + Version()
}
