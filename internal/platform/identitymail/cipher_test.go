package identitymail

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestEncryptedEnvelopeBindingTamperingAndRandomness(t *testing.T) {
	c, err := NewEnvelopeCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	m := Message{Purpose: PasswordReset, Recipient: "synthetic@example.test", Token: strings.Repeat("A", 43), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	first, err := c.Seal("challenge-one", m)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Seal("challenge-one", m)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) || bytes.Contains(first, []byte(m.Token)) || bytes.Contains(first, []byte(m.Recipient)) {
		t.Fatal("payload is deterministic or plaintext")
	}
	got, err := c.Open("challenge-one", first)
	if err != nil || got != m {
		t.Fatal("roundtrip", err)
	}
	if _, err = c.Open("challenge-two", first); err != ErrEnvelope {
		t.Fatal("row substitution accepted")
	}
	first[len(first)-1] ^= 1
	if _, err = c.Open("challenge-one", first); err != ErrEnvelope {
		t.Fatal("tampering accepted")
	}
	for _, key := range []string{"", strings.Repeat("A", 32), strings.Repeat("!", 44)} {
		if _, err := NewEnvelopeCipher(key); err != ErrEnvelope {
			t.Fatal("invalid key accepted")
		}
	}
	if _, err = c.Open("challenge-one", second[:5]); err != ErrEnvelope {
		t.Fatal("truncation accepted")
	}
	other, _ := NewEnvelopeCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)))
	if _, err = other.Open("challenge-one", second); err != ErrEnvelope {
		t.Fatal("wrong key accepted")
	}
}
