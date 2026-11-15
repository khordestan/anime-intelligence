package identitymail

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

var ErrEnvelope = errors.New("invalid identity delivery envelope")

// EnvelopeCipher encrypts delivery credentials with a key held outside the DB.
// Associated data binds ciphertext to the immutable challenge ID and version.
type EnvelopeCipher struct {
	aead     cipher.AEAD
	previous []cipher.AEAD
}

func NewEnvelopeCipher(encoded string) (*EnvelopeCipher, error) {
	return NewEnvelopeKeyring(encoded, nil)
}

// NewEnvelopeKeyring always encrypts with primary and accepts up to three
// previous decryption keys. Existing ciphertext formats remain compatible.
func NewEnvelopeKeyring(primary string, previous []string) (*EnvelopeCipher, error) {
	if len(previous) > 3 {
		return nil, ErrEnvelope
	}
	c := &EnvelopeCipher{}
	seen := make(map[string]bool)
	for i, encoded := range append([]string{primary}, previous...) {
		key, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(key) != 32 || base64.StdEncoding.EncodeToString(key) != encoded || seen[encoded] {
			return nil, ErrEnvelope
		}
		seen[encoded] = true
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, ErrEnvelope
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, ErrEnvelope
		}
		if i == 0 {
			c.aead = aead
		} else {
			c.previous = append(c.previous, aead)
		}
	}
	return c, nil
}
func (c *EnvelopeCipher) open(data []byte, aad string) ([]byte, error) {
	// Callers validate length before this trusted helper. All keys use AES-GCM's
	// standard nonce/tag sizes; never guess formats or accept unauthenticated data.
	for _, key := range append([]cipher.AEAD{c.aead}, c.previous...) {
		plaintext, err := key.Open(nil, data[:key.NonceSize()], data[key.NonceSize():], []byte(aad))
		if err == nil {
			return plaintext, nil
		}
	}
	return nil, ErrEnvelope
}
func (*EnvelopeCipher) String() string     { return "identity envelope cipher (redacted)" }
func (c *EnvelopeCipher) GoString() string { return c.String() }

type envelope struct {
	Purpose, Recipient, Token string
	ExpiresAt                 time.Time
}

func validEnvelope(m Message) bool {
	token, err := base64.RawURLEncoding.Strict().DecodeString(m.Token)
	return (m.Purpose == PasswordReset || m.Purpose == EmailVerification) && len(m.Recipient) > 0 && len(m.Recipient) <= 254 && len(m.Token) == 43 && err == nil && len(token) == 32 && !m.ExpiresAt.IsZero()
}
func (c *EnvelopeCipher) Seal(binding string, m Message) ([]byte, error) {
	if binding == "" || !validEnvelope(m) {
		return nil, ErrEnvelope
	}
	data, err := json.Marshal(envelope{m.Purpose, m.Recipient, m.Token, m.ExpiresAt})
	if err != nil {
		return nil, ErrEnvelope
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, ErrEnvelope
	}
	return c.aead.Seal(nonce, nonce, data, []byte("anime-identity-outbox:v1:"+binding)), nil
}
func (c *EnvelopeCipher) Open(binding string, data []byte) (Message, error) {
	if binding == "" || len(data) < c.aead.NonceSize()+c.aead.Overhead() || len(data) > 4096 {
		return Message{}, ErrEnvelope
	}
	plaintext, err := c.open(data, "anime-identity-outbox:v1:"+binding)
	if err != nil {
		return Message{}, ErrEnvelope
	}
	var e envelope
	if json.Unmarshal(plaintext, &e) != nil {
		return Message{}, ErrEnvelope
	}
	m := Message{Purpose: e.Purpose, Recipient: e.Recipient, Token: e.Token, ExpiresAt: e.ExpiresAt}
	if !validEnvelope(m) {
		return Message{}, ErrEnvelope
	}
	return m, nil
}
