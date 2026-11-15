package identitymail

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
)

// Fixed-size encrypted request intents keep admission independent of account
// state. Verification values are session digests, never raw bearer tokens.
func validRequest(purpose, value string) bool {
	if len(value) > 254 {
		return false
	}
	if purpose == PasswordReset {
		return true
	}
	decoded, err := hex.DecodeString(value)
	return purpose == EmailVerification && len(value) == 64 && err == nil && len(decoded) == 32
}
func (c *EnvelopeCipher) SealRequest(binding, purpose, value string) ([]byte, error) {
	if binding == "" || !validRequest(purpose, value) {
		return nil, ErrEnvelope
	}
	data := make([]byte, 512)
	if _, err := rand.Read(data); err != nil {
		return nil, ErrEnvelope
	}
	data[0] = 1
	if purpose == EmailVerification {
		data[0] = 2
	}
	binary.BigEndian.PutUint16(data[1:3], uint16(len(value)))
	copy(data[3:], value)
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, ErrEnvelope
	}
	return c.aead.Seal(nonce, nonce, data, []byte("anime-identity-request:v1:"+binding)), nil
}
func (c *EnvelopeCipher) OpenRequest(binding string, data []byte) (string, string, error) {
	if binding == "" || len(data) != c.aead.NonceSize()+512+c.aead.Overhead() {
		return "", "", ErrEnvelope
	}
	plaintext, err := c.aead.Open(nil, data[:c.aead.NonceSize()], data[c.aead.NonceSize():], []byte("anime-identity-request:v1:"+binding))
	if err != nil {
		return "", "", ErrEnvelope
	}
	size := int(binary.BigEndian.Uint16(plaintext[1:3]))
	if size > 254 {
		return "", "", ErrEnvelope
	}
	purpose := ""
	switch plaintext[0] {
	case 1:
		purpose = PasswordReset
	case 2:
		purpose = EmailVerification
	default:
		return "", "", ErrEnvelope
	}
	value := string(plaintext[3 : 3+size])
	if !validRequest(purpose, value) {
		return "", "", ErrEnvelope
	}
	return purpose, value, nil
}
