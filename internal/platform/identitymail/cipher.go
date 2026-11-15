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
type EnvelopeCipher struct{ aead cipher.AEAD }

func NewEnvelopeCipher(encoded string) (*EnvelopeCipher, error) {
	key, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, ErrEnvelope
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrEnvelope
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrEnvelope
	}
	return &EnvelopeCipher{aead}, nil
}
func (*EnvelopeCipher) String() string { return "identity envelope cipher (redacted)" }

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
	plaintext, err := c.aead.Open(nil, data[:c.aead.NonceSize()], data[c.aead.NonceSize():], []byte("anime-identity-outbox:v1:"+binding))
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
